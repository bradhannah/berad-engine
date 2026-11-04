package mapview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// gameWait is a Wait of game minutes, in a container.
func gameWait(t *testing.T, seed byte, minutes int, parent byte) flow.Node {
	t.Helper()
	made, err := flow.NewNode(flow.KindWait)
	if err == nil {
		made, err = made.WithParam("duration", flow.Duration{Amount: minutes, Unit: flow.UnitMinutes, Clock: flow.ClockGame})
	}
	if err != nil {
		t.Fatal(err)
	}
	made.ID, made.Parent = frametest.ID(seed), frametest.ID(parent)

	return made
}

// TestLanesInThePreview starts the clerk's day with a Together of two
// lanes: the preview's state lists each lane and what it is doing, and a
// step stops at the next change in a lane, naming it.
func TestLanesInThePreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withWorkingClerk(t, f)
	together, err := flow.NewNode(flow.KindTogether)
	if err == nil {
		together, err = together.WithParam(flow.ParamLanes, []flow.Lane{{Port: "l1", Label: "Twice"}, {Port: "l2", Label: "Long"}})
	}
	if err != nil {
		t.Fatal(err)
	}
	together.ID = frametest.ID(90)
	f.docs.View(routineID, func(document client.Document) {
		routine := document.Model.(*workflowplugin.Document).Workflow
		if err := routine.Insert([]flow.Node{
			together, gameWait(t, 91, 1, 90), gameWait(t, 93, 1, 90), gameWait(t, 92, 5, 90),
		}, nil, []flow.Link{
			{From: frametest.ID(80), Output: flow.OutputNext, To: together.ID},
			{From: together.ID, Output: "l1", To: frametest.ID(91)}, {From: frametest.ID(91), Output: flow.OutputNext, To: frametest.ID(93)},
			{From: together.ID, Output: "l2", To: frametest.ID(92)},
			{From: together.ID, Output: flow.OutputDone, To: frametest.ID(81)},
		}); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.Touch(routineID)
	f.startPreview("cell:3,3")
	f.idle(3)
	clerk := f.lastState().Entities[1]
	if clerk.Node != together.ID || len(clerk.Lanes) != 2 || clerk.Lanes[0].Label != "Twice" ||
		clerk.Lanes[0].Node != frametest.ID(91) || !strings.HasSuffix(clerk.Lanes[1].Doing, "of 5 min (game time)") {
		t.Fatalf("each lane, and what it is doing: %+v", clerk)
	}
	f.control(protocol.ControlWatch, clerkPlacement)
	f.control(protocol.ControlPause, clerkPlacement)
	f.idle(2)
	f.control(protocol.ControlStep, clerkPlacement)
	f.idle(2)
	state := f.lastState()
	if len(state.Step) != 1 || state.Step[0].From != frametest.ID(91) || state.Step[0].Lane != "l1" ||
		state.Entities[1].Lanes[0].Node != frametest.ID(93) {
		t.Fatalf("the step stops at the first lane's next step, naming the lane: %+v", state.Step)
	}
}
