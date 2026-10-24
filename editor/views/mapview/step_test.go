package mapview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
)

// control sends the preview a control, as a Watch window does.
func (f *fixture) control(action string, placement ids.ULID) {
	f.state.Changed(client.Update{PreviewControl: &protocol.PreviewControl{
		Run: runID, Action: action, Placement: placement,
	}}, f.ctx)
}

// TestStepGoesStraightToTheNextNode steps the clerk past its hour-long Wait
// in one frame, keeping the way it went until the NPCs resume; at the real
// rate a step runs over several frames, and comes back round to the Wait
// when the wall blocks the Move; with no NPC watched, Step does nothing.
func TestStepGoesStraightToTheNextNode(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withWorkingClerk(t, f)
	f.startPreview("cell:3,3")
	f.idle(play.StepTicks + 2)
	f.control(protocol.ControlWatch, clerkPlacement)
	f.control(protocol.ControlPause, clerkPlacement)
	f.idle(2)
	before := f.lastState()
	f.control(protocol.ControlStep, clerkPlacement)
	f.idle(2)
	state := f.lastState()
	wait, move := frametest.ID(82), frametest.ID(81)
	if !state.NPCsPaused || state.Stepping || state.Entities[1].Node != move || len(state.Step) != 2 ||
		state.Step[0].From != wait || state.Step[1].To != move {
		t.Fatalf("one step past the Wait, back round to the Move: %+v", state)
	}
	if passed := state.Clock - before.Clock; passed < 3500 || passed > 3700 {
		t.Fatalf("the clock moved on by the rest of the hour: %v", passed)
	}
	f.control(protocol.ControlStep, clerkPlacement)
	f.idle(play.StepTicks + 2)
	if state := f.lastState(); state.Entities[1].Node != wait || len(state.Step) != 1 || state.Step[0].From != move {
		t.Fatalf("the next step walks the tile to the Wait: %+v", state)
	}
	f.control(protocol.ControlResume, clerkPlacement)
	f.idle(2)
	if state := f.lastState(); state.NPCsPaused || len(state.Step) != 0 {
		t.Fatalf("resuming forgets the step: %+v", state)
	}
	f.click("ui:rate-1")
	f.control(protocol.ControlPause, clerkPlacement)
	f.control(protocol.ControlStep, clerkPlacement)
	f.idle(2)
	if state := f.lastState(); !state.Stepping || state.NPCsPaused {
		t.Fatalf("an hour at the real rate takes more than a frame: %+v", state)
	}
	f.idle(70)
	state = f.lastState()
	if state.Stepping || !state.NPCsPaused || state.Entities[1].Node != wait || len(state.Step) != 3 ||
		state.Step[2].From != move || state.Step[2].Status != npcrun.StatusBlocked {
		t.Fatalf("the step ends back at the Wait, the wall blocking the Move: %+v", state)
	}
	f.control(protocol.ControlWatch, ids.ULID{})
	f.control(protocol.ControlStep, ids.ULID{})
	f.idle(2)
	if state := f.lastState(); state.Stepping || !state.NPCsPaused {
		t.Fatalf("with nothing watched, Step does nothing: %+v", state)
	}
}
