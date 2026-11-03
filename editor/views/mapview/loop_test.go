package mapview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var loopID = frametest.ID(90)

// withLoop gives Town a loop of a kind that rings a bell and waits an
// hour, running in Town or in any zone.
func withLoop(t *testing.T, f *fixture, purpose flow.Purpose, inTown bool) {
	t.Helper()
	loop, err := flow.NewFor(frametest.ID(91), purpose)
	if err != nil {
		t.Fatal(err)
	}
	if inTown {
		loop.SetRunsIn(zoneID)
	}
	ring, err := flow.NewNode(flow.KindPlaySound)
	if err == nil {
		ring, err = ring.WithParam("cue", "market/bell")
	}
	ring.ID = frametest.ID(92)
	rest, restErr := flow.NewNode(flow.KindWait)
	if restErr == nil {
		rest, restErr = rest.WithParam("duration", flow.Duration{Amount: 1, Unit: flow.UnitHours, Clock: flow.ClockGame})
	}
	rest.ID = frametest.ID(93)
	if err != nil || restErr != nil || loop.Insert([]flow.Node{ring, rest}, nil, []flow.Link{
		{From: frametest.ID(91), Output: flow.OutputNext, To: ring.ID}, {From: ring.ID, Output: flow.OutputNext, To: rest.ID},
	}) != nil {
		t.Fatalf("the loop: %v %v", err, restErr)
	}
	f.docs.Put(loopID, "Market day", &workflowplugin.Document{Workflow: loop})
	f.docs.View(zoneID, func(document client.Document) { document.Model.(*zoneplugin.Document).Zone.Loop = loopID })
	f.docs.Touch(zoneID)
	var entries []project.Entry
	f.docs.View(project.IndexID(), func(document client.Document) {
		entries = document.Model.(*project.Index).Entries()
	})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(append(entries, project.Entry{
		ID: loopID, Name: "Market day", Format: flow.FormatName, Folder: "", Tags: nil, Kind: string(purpose),
	})))
}

// TestTheZoneLoopRunsInThePreview rings the zone loop's bell once as the
// preview starts; a workflow that is no zone loop rings nothing.
func TestTheZoneLoopRunsInThePreview(t *testing.T) {
	t.Parallel()
	for _, each := range []struct {
		purpose flow.Purpose
		rings   int
	}{{flow.PurposeZoneLoop, 1}, {flow.PurposeBehaviour, 0}} {
		f := newFixture(t)
		withLoop(t, f, each.purpose, true)
		f.startPreview("cell:3,3")
		f.idle(3)
		sounds, ok := f.ctx.Sounds.(*frametest.Sounds)
		if !ok {
			t.Fatal("the test window plays through the fake")
		}
		if len(sounds.Played) != each.rings || (each.rings > 0 && !f.has("heard:market/bell")) {
			t.Fatalf("a %s rings %d times: %v", each.purpose, each.rings, sounds.Played)
		}
	}
}

// TestWatchingTheZoneLoop carries Town's loop in the preview's state, apart
// from the NPCs the map draws; watched, a step takes it past its hour-long
// Wait to its bell.
func TestWatchingTheZoneLoop(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withLoop(t, f, flow.PurposeZoneLoop, true)
	f.startPreview("cell:3,3")
	f.idle(2)
	entities := f.lastState().Entities
	loop := entities[len(entities)-1]
	if loop.Loop != zoneID || loop.ID != zoneID.String() || !loop.NPC.IsZero() || loop.Node != frametest.ID(93) {
		t.Fatalf("the state carries the zone loop, on its Wait: %+v", loop)
	}
	f.state.Changed(client.Update{PreviewControl: &protocol.PreviewControl{
		Run: runID, Action: protocol.ControlWatch, Placement: zoneID, Rate: 0, Seconds: 0,
	}}, f.ctx)
	f.state.Changed(client.Update{PreviewControl: &protocol.PreviewControl{
		Run: runID, Action: protocol.ControlStep, Placement: zoneID, Rate: 0, Seconds: 0,
	}}, f.ctx)
	f.idle(2)
	state := f.lastState()
	if state.Watched != zoneID || !state.NPCsPaused || len(state.Step) == 0 || state.Step[0].From != frametest.ID(93) {
		t.Fatalf("a step takes the watched loop past its Wait: %+v", state)
	}
}
