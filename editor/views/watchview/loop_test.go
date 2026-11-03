package watchview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// sendLoop gives the window a preview state with Town's loop on its Wait,
// watched or not, and its values.
func (f *fixture) sendLoop(tick uint64, watched ids.ULID) {
	loop := protocol.PreviewEntity{
		ID: zoneID.String(), Floor: ids.ULID{}, X: 0, Y: 0, Face: [2]int{0, 1}, NPC: ids.ULID{}, Workflow: routineID,
		Node: waitID, Activity: "Wait 40 min of 1 hr (game time)", Status: "",
		Last: protocol.PreviewChange{From: startID, Output: flow.OutputNext, To: waitID, Tick: tick, Status: ""},
		Changes: 1, Calls: nil, Way: nil, Loop: zoneID,
	}
	state := protocol.PreviewState{
		Run: runID, Tick: tick, Entities: []protocol.PreviewEntity{loop}, Clock: 9 * 3600, Rate: 60,
		ClockStopped: false, NPCsPaused: false, Watched: watched, Stepping: false, Step: nil,
		Values: []protocol.PreviewValue{{Path: "zone.open", Value: "true"}},
	}
	f.state.Changed(client.Update{PreviewState: &state}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestWatchingAZoneLoop offers Watch zone loop while the preview runs one,
// asks the preview to watch it, and then shows it under its zone's name,
// on its Wait, with its values.
func TestWatchingAZoneLoop(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone", Folder: "", Tags: nil, Kind: ""},
		{ID: floorID, Name: "Ground", Format: floor.FormatName, Folder: "", Tags: nil, Kind: ""},
		{ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName, Folder: "", Tags: nil, Kind: ""},
		{ID: routineID, Name: "Market day", Format: flow.FormatName, Folder: "", Tags: nil,
			Kind: string(flow.PurposeZoneLoop)},
	}))
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	f.sendLoop(5, ids.ULID{})
	f.click("ui:watch-loop")
	controls := f.requests.PreviewControls
	if len(controls) != 1 || controls[0].Action != protocol.ControlWatch || controls[0].Placement != zoneID {
		t.Fatalf("Watch zone loop asks the preview to watch Town's loop: %+v", controls)
	}
	f.sendLoop(6, zoneID)
	if !f.has("watching:Town") || !f.has("live:"+flow.KindWait) || !f.has("value:zone.open=true") {
		t.Fatal("the zone loop is watched under its zone's name, on its Wait, with its values")
	}
}
