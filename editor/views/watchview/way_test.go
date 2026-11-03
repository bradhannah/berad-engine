package watchview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestWatchSaysTheFloorsOnTheWay reads the floors a watched NPC's Go to
// crosses, up the stairs and down again.
func TestWatchSaysTheFloorsOnTheWay(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	upper := frametest.ID(40)
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	clerk := protocol.PreviewEntity{
		ID: placementID.String(), Floor: floorID, X: 4, Y: 2, Face: [2]int{0, 1}, NPC: clerkID, Workflow: routineID,
		Node: waitID, Activity: "Walking to Desk, 9 tiles to go", Status: "",
		Last: protocol.PreviewChange{From: startID, Output: flow.OutputNext, To: waitID, Tick: 3}, Changes: 1,
		Way: []protocol.PreviewClimb{
			{From: floorID, Col: 1, Row: 3, Up: true, To: upper}, {From: upper, Col: 6, Row: 6, Up: false, To: floorID},
		},
	}
	state := protocol.PreviewState{
		Run: runID, Tick: 3, Entities: []protocol.PreviewEntity{clerk}, Clock: 9 * 3600, Rate: 60,
		ClockStopped: false, NPCsPaused: false, Watched: placementID, Stepping: false, Step: nil,
	}
	f.state.Changed(client.Update{PreviewState: &state}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("way=Ground → stairs up at 1, 3 → a floor → stairs down at 6, 6 → Ground") {
		t.Fatal("Watch names the floors on the way and the stairs between them")
	}
}
