package watchview_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/watchview"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var (
	zoneID      = frametest.ID(1)
	floorID     = frametest.ID(2)
	clerkID     = frametest.ID(3)
	routineID   = frametest.ID(4)
	placementID = frametest.ID(5)
	startID     = frametest.ID(6)
	waitID      = frametest.ID(7)
	runID       = frametest.ID(8)
)

type fixture struct {
	t        *testing.T
	state    *watchview.State
	docs     *frametest.Docs
	requests *frametest.Requests
	ctx      *frame.Context
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	routine, err := flow.New(startID)
	if err != nil {
		t.Fatal(err)
	}
	wait, err := flow.NewNode(flow.KindWait)
	if err != nil {
		t.Fatal(err)
	}
	wait.ID = waitID
	if err := routine.Insert([]flow.Node{wait}, nil, []flow.Link{{From: startID, Output: flow.OutputNext, To: waitID}}); err != nil {
		t.Fatal(err)
	}
	docs := frametest.NewDocs()
	docs.Put(routineID, "Clerk day", &workflowplugin.Document{Workflow: routine})
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: floorID, Name: "Ground", Format: floor.FormatName}, {ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName},
		{ID: routineID, Name: "Clerk day", Format: flow.FormatName},
	}))
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(1000, 700))
	state := watchview.New(frametest.ID(99), zoneID, theme)
	state.Start(ctx)
	f := &fixture{t: t, state: state, docs: docs, requests: requests, ctx: ctx}
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

func (f *fixture) frame(input ui.Input) {
	f.state.Update(&input, f.ctx)
}

func (f *fixture) has(state string) bool {
	_, ok := f.state.Locate("state:"+state, f.docs)

	return ok
}

func (f *fixture) click(name string) {
	f.t.Helper()
	at, ok := f.state.Locate(name, f.docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}
	press := ui.NewInput(at)
	press.Pressed, press.Down = true, true
	f.frame(press)
	release := ui.NewInput(at)
	release.Released = true
	f.frame(release)
}

// send gives the window a preview state watching the clerk on its Wait.
func (f *fixture) send(tick uint64, paused bool) {
	f.sendStep(tick, paused, false, nil)
}

// sendStep gives the window a preview state watching the clerk on its Wait,
// stepping or after a step.
func (f *fixture) sendStep(tick uint64, paused, stepping bool, step []protocol.PreviewChange) {
	clerk := protocol.PreviewEntity{
		ID: placementID.String(), Floor: floorID, X: 4, Y: 2, Face: [2]int{0, 1}, NPC: clerkID, Workflow: routineID,
		Node: waitID, Activity: "Wait 12 min of 30 min (game time)", Status: "",
		Last: protocol.PreviewChange{From: startID, Output: flow.OutputNext, To: waitID, Tick: tick}, Changes: 2,
	}
	state := protocol.PreviewState{
		Run: runID, Tick: tick, Entities: []protocol.PreviewEntity{clerk}, Clock: 9 * 3600, Rate: 60,
		ClockStopped: false, NPCsPaused: paused, Watched: placementID, Stepping: stepping, Step: step,
	}
	f.state.Changed(client.Update{PreviewState: &state}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestWatchFollowsTheWatchedNPC shows nothing until a preview runs, then the
// watched clerk's workflow with its Wait live, and sends pause, resume and
// step.
func TestWatchFollowsTheWatchedNPC(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	if f.has("previewing") {
		t.Fatal("no preview yet")
	}
	if len(f.requests.Subscribed) == 0 || f.requests.Subscribed[0] != project.IndexID() {
		t.Fatalf("it loads the project index, for names: %v", f.requests.Subscribed)
	}
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	f.send(5, false)
	if !f.has("previewing") || !f.has("watching:Ada Clerk") || !f.has("live:"+flow.KindWait) {
		t.Fatal("it watches the clerk on its Wait")
	}
	if len(f.requests.Subscribed) == 0 || f.requests.Subscribed[len(f.requests.Subscribed)-1] != routineID {
		t.Fatalf("it loads the clerk's workflow: %v", f.requests.Subscribed)
	}
	if f.state.Title(f.docs) == "" {
		t.Fatal("title")
	}
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	f.click("ui:pause")
	f.send(6, true)
	if !f.has("paused") {
		t.Fatal("the state says the NPCs are paused")
	}
	f.click("ui:pause")
	f.click("ui:step")
	controls := f.requests.PreviewControls
	if len(controls) != 3 || controls[0].Action != protocol.ControlPause || controls[1].Action != protocol.ControlResume ||
		controls[2].Action != protocol.ControlStep || controls[2].Run != runID {
		t.Fatalf("controls %+v", controls)
	}
	f.state.Changed(client.Update{PreviewStop: &protocol.PreviewStop{Run: runID, Reason: "stopped"}}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if f.has("previewing") || f.has("watching:Ada Clerk") {
		t.Fatal("the stop clears the window")
	}
}

// TestStepShowsTheWayItWent shows a step running, with Step disabled, then
// the way the step went under STEP, with why a node carried on, until the
// NPCs run again and LAST shows the latest change.
func TestStepShowsTheWayItWent(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	f.sendStep(5, false, true, nil)
	f.click("ui:step")
	if len(f.requests.PreviewControls) != 0 || f.has("stepped") {
		t.Fatalf("Step is disabled while a step runs: %+v", f.requests.PreviewControls)
	}
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	step := []protocol.PreviewChange{
		{From: waitID, Output: flow.OutputNext, To: startID, Tick: 9, Status: npcrun.StatusBlocked},
		{From: startID, Output: flow.OutputNext, To: waitID, Tick: 9, Status: ""},
	}
	f.sendStep(9, true, false, step)
	if !f.has("stepped") {
		t.Fatal("paused after a step")
	}
	if label, way := f.state.Way(f.docs, 1000); label != "STEP" || way != "Wait · 30 min: blocked → Start → Wait · 30 min" {
		t.Fatalf("the way the step went: %s %q", label, way)
	}
	if _, way := f.state.Way(f.docs, 100); way != "… → Wait · 30 min" {
		t.Fatalf("a narrow window leaves out the earliest nodes: %q", way)
	}
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	f.send(12, false)
	if label, way := f.state.Way(f.docs, 1000); f.has("stepped") || label != "LAST" || way != "Start → Wait · 30 min" {
		t.Fatalf("running, the latest change: %s %q", label, way)
	}
}
