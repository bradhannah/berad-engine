package mapview_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/play"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var clerkPlacement = frametest.ID(71)

// withWorkingClerk places Ada Clerk at (9, 8), following "move right one
// tile, then wait an hour".
func withWorkingClerk(t *testing.T, f *fixture) {
	t.Helper()
	withClerk(f)
	routine, err := flow.New(frametest.ID(80))
	if err != nil {
		t.Fatal(err)
	}
	move, err := flow.NewNode(flow.KindMove)
	if err != nil {
		t.Fatal(err)
	}
	move.ID = frametest.ID(81)
	if move, err = move.WithParam("direction", flow.DirectionRight); err != nil {
		t.Fatal(err)
	}
	wait, err := flow.NewNode(flow.KindWait)
	if err != nil {
		t.Fatal(err)
	}
	wait.ID = frametest.ID(82)
	if wait, err = wait.WithParam("duration", flow.Duration{Amount: 1, Unit: flow.UnitHours, Clock: flow.ClockGame}); err != nil {
		t.Fatal(err)
	}
	if err := routine.Insert([]flow.Node{move, wait}, nil, []flow.Link{
		{From: frametest.ID(80), Output: flow.OutputNext, To: move.ID},
	}); err != nil {
		t.Fatal(err)
	}
	moving, _ := routine.Node(move.ID)
	if _, err := routine.Replace(moving.WithLink(flow.OutputNext, wait.ID)); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(routineID, "Clerk day", &workflowplugin.Document{Workflow: routine})
	ground := groundFloor(t)
	if err := ground.Floor.InsertPlacement(0, floor.Placement{ID: clerkPlacement, NPC: clerkID, Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(groundID, "Ground", ground)
}

func (f *fixture) lastState() protocol.PreviewState {
	f.t.Helper()
	states := f.requests.PreviewStates
	if len(states) == 0 {
		f.t.Fatal("no preview state sent")
	}

	return states[len(states)-1]
}

func (f *fixture) typeInto(field, text string) {
	f.click("ui:" + field)
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
	f.keys("Enter")
}

// TestNPCsFollowTheirWorkflowsInThePreview starts a preview at 08:00: the
// clerk moves one tile right and waits, its state goes to the other views,
// the clock's rate changes, a jump restarts it, a click watches it, and a
// pause stops it.
func TestNPCsFollowTheirWorkflowsInThePreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withWorkingClerk(t, f)
	f.startPreview("cell:3,3")
	f.idle(play.StepTicks + 2)
	if !f.has("npc:Ada Clerk=10,8") || !f.has("npcs-moved") {
		t.Fatal("the clerk moved one tile right")
	}
	state := f.lastState()
	if len(state.Entities) != 2 || state.Entities[1].NPC != clerkID || state.Entities[1].Node != frametest.ID(82) ||
		state.Rate != 60 || state.Clock < 8*3600 || state.Clock > 8*3600+60 {
		t.Fatalf("the state carries the clerk and the clock: %+v", state)
	}
	f.click("ui:rate-600")
	f.idle(2)
	if f.lastState().Rate != 600 {
		t.Fatal("the rate buttons change the clock's rate")
	}
	f.typeInto("jump-time", "12:00")
	f.idle(play.StepTicks + 2)
	if clock := f.lastState().Clock; clock < 12*3600 || clock > 12*3600+600 {
		t.Fatalf("the jump moved the clock: %v", clock)
	}
	if !f.has("npc:Ada Clerk=11,8") {
		t.Fatal("after the jump the clerk starts again and moves another tile")
	}
	f.click("live-npc:Ada Clerk")
	f.idle(2)
	if !f.has("watching") || f.lastState().Watched != clerkPlacement {
		t.Fatal("clicking the clerk watches it")
	}
	f.state.Changed(client.Update{PreviewControl: &protocol.PreviewControl{
		Run: runID, Action: protocol.ControlPause, Placement: clerkPlacement,
	}}, f.ctx)
	clock := f.lastState().Clock
	f.idle(10)
	if !f.lastState().NPCsPaused || f.lastState().Clock != clock {
		t.Fatal("paused NPCs hold the clock")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
}
