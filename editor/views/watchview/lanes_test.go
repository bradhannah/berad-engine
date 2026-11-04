package watchview_test

import (
	"image"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Nodes of the clerk's Together: the Together, and a Wait in each lane.
var (
	togetherID = frametest.ID(20)
	shakeID    = frametest.ID(21)
	chimeID    = frametest.ID(22)
)

// withTogether gives the clerk's day a Together of two lanes, Shake and
// Chime, each a Wait, after its Wait.
func (f *fixture) withTogether() {
	f.t.Helper()
	together, err := flow.NewNode(flow.KindTogether)
	if err == nil {
		together, err = together.WithParam(flow.ParamLanes, []flow.Lane{{Port: "l1", Label: "Shake"}, {Port: "l2", Label: "Chime"}})
	}
	if err != nil {
		f.t.Fatal(err)
	}
	together.ID = togetherID
	lane := func(id ids.ULID) flow.Node {
		made, err := flow.NewNode(flow.KindWait)
		if err != nil {
			f.t.Fatal(err)
		}
		made.ID, made.Parent = id, togetherID

		return made
	}
	f.docs.View(routineID, func(document client.Document) {
		routine := document.Model.(*workflowplugin.Document).Workflow
		if err := routine.Insert([]flow.Node{together, lane(shakeID), lane(chimeID)}, nil, []flow.Link{
			{From: waitID, Output: flow.OutputNext, To: togetherID}, {From: togetherID, Output: "l1", To: shakeID},
			{From: togetherID, Output: "l2", To: chimeID},
		}); err != nil {
			f.t.Fatal(err)
		}
	})
	f.docs.Touch(routineID)
}

// sendLanes gives the window a preview state watching the clerk on its
// Together, its first lane waiting and its second done, after a change in
// the first lane.
func (f *fixture) sendLanes(tick uint64) {
	clerk := protocol.PreviewEntity{
		ID: placementID.String(), Floor: floorID, X: 4, Y: 2, Face: [2]int{0, 1}, NPC: clerkID, Workflow: routineID,
		Node: togetherID, Activity: "Together, 1 of 2 lanes running",
		Last:    protocol.PreviewChange{From: togetherID, Output: "l1", To: shakeID, Tick: tick, Lane: ""},
		Changes: 4,
		Lanes: []protocol.PreviewLane{
			{Port: "l1", Label: "Shake", Node: shakeID, Doing: "Wait 10 min of 30 min (game time)"},
			{Port: "l2", Label: "Chime", Node: chimeID, Doing: "Done", Done: true},
		},
	}
	state := protocol.PreviewState{
		Run: runID, Tick: tick, Entities: []protocol.PreviewEntity{clerk}, Clock: 9 * 3600, Rate: 60,
		ClockStopped: false, NPCsPaused: true, Watched: placementID, Stepping: false,
		Step: []protocol.PreviewChange{{From: chimeID, Output: flow.OutputNext, To: shakeID, Tick: tick, Lane: "l2"}},
	}
	f.state.Changed(client.Update{PreviewState: &state}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestWatchListsLanes watches the clerk on a Together: each lane says what
// it is doing or that it is done, the running lane's node is lit, and a
// step names the lane its change was in.
func TestWatchListsLanes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withTogether()
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	f.sendLanes(9)
	if !f.has("live:"+flow.KindTogether) || !f.has("lane:1=Wait 10 min of 30 min (game time)") ||
		!f.has("lane-done:2") || f.has("lane-done:1") {
		t.Fatal("the first lane waits and the second is done")
	}
	if lit := f.state.LiveLanes(); !slices.Equal(lit, []ids.ULID{shakeID, {}}) {
		t.Fatalf("the running lane's node is lit: %v", lit)
	}
	if label, way := f.state.Way(f.docs, 1000); label != "STEP" || way != "Chime: Wait · 30 min → Wait · 30 min" {
		t.Fatalf("the step names its lane: %s %q", label, way)
	}
}
