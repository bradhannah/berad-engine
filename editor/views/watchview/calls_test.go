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
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var (
	moduleID = frametest.ID(20)
	goToID   = frametest.ID(21)
	callID   = frametest.ID(22)
)

// withModule adds Go to work, a workflow the clerk's day calls, on its Go
// to.
func (f *fixture) withModule() {
	f.t.Helper()
	module, err := flow.New(frametest.ID(23))
	if err != nil {
		f.t.Fatal(err)
	}
	goTo, err := flow.NewNode(flow.KindGoTo)
	if err != nil {
		f.t.Fatal(err)
	}
	goTo.ID = goToID
	if err := module.Insert([]flow.Node{goTo}, nil, nil); err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(moduleID, "Go to work", &workflowplugin.Document{Workflow: module})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: floorID, Name: "Ground", Format: floor.FormatName}, {ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName},
		{ID: routineID, Name: "Clerk day", Format: flow.FormatName},
		{ID: moduleID, Name: "Go to work", Format: flow.FormatName},
	}))
}

// sendCall gives the window the clerk inside a call from its day to Go to
// work, or back in its day.
func (f *fixture) sendCall(tick uint64, inside bool) {
	clerk := protocol.PreviewEntity{
		ID: placementID.String(), Floor: floorID, X: 4, Y: 2, Face: [2]int{0, 1}, NPC: clerkID, Workflow: routineID,
		Node: waitID, Activity: "Wait", Status: "", Last: protocol.PreviewChange{From: ids.ULID{}, Output: "",
			To: ids.ULID{}, Tick: 0, Status: ""}, Changes: tick, Calls: nil,
	}
	if inside {
		clerk.Workflow, clerk.Node = moduleID, goToID
		clerk.Calls = []protocol.PreviewCall{{Workflow: routineID, Call: callID}}
	}
	state := protocol.PreviewState{
		Run: runID, Tick: tick, Entities: []protocol.PreviewEntity{clerk}, Clock: 9 * 3600, Rate: 60,
		ClockStopped: false, NPCsPaused: false, Watched: placementID, Stepping: false, Step: nil,
	}
	f.state.Changed(client.Update{PreviewState: &state}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
}

// TestWatchFollowsCalls names the path down through the call the clerk is
// inside and shows the workflow it is in, shows its day with the Call lit
// when the day is clicked, follows it again from the last part, and shows
// its day once the call returns.
func TestWatchFollowsCalls(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withModule()
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	f.sendCall(5, true)
	if !f.has("path=Clerk day › Go to work › Go to") || !f.has("showing:Go to work") || !f.has("live:go-to") {
		t.Fatal("the path names the call, and the workflow it is in shows")
	}
	f.click("ui:path-1")
	f.sendCall(6, true)
	if !f.has("showing:Clerk day") {
		t.Fatal("clicking the day shows it while the clerk is inside its call")
	}
	f.click("ui:path-2")
	f.sendCall(7, true)
	if !f.has("showing:Go to work") {
		t.Fatal("clicking the last part follows the clerk again")
	}
	f.click("ui:path-1")
	f.sendCall(8, false)
	if !f.has("path=") || !f.has("showing:Clerk day") {
		t.Fatal("back from the call, its day shows, with no path")
	}
}
