package browser_test

import (
	"testing"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var (
	harbourID  = frametest.ID(6)
	quayID     = frametest.ID(7)
	deskID     = frametest.ID(8)
	customerID = frametest.ID(9)
)

// withMoves gives the fixture a second zone, Harbour, with its floor Quay;
// Ada Clerk lives in Town, placed on Ground, and follows Clerk day, whose
// Go to goes to Ground's Desk; Customer lives in Town too and follows Clerk
// day as well.
func withMoves(t *testing.T, f *fixture, placed bool) {
	t.Helper()
	harbour := zoneplugin.NewDocument()
	if err := harbour.Zone.Insert(0, quayID); err != nil {
		t.Fatal(err)
	}
	quay, err := tilefloor.NewDocument(10, 8, "grass")
	if err != nil {
		t.Fatal(err)
	}
	ground, err := tilefloor.NewDocument(10, 8, "grass")
	if err != nil {
		t.Fatal(err)
	}
	if err := ground.Floor.InsertAnchor(0, floor.Anchor{ID: deskID, Name: "Desk", Col: 2, Row: 2}); err != nil {
		t.Fatal(err)
	}
	if placed {
		if err := ground.Floor.InsertPlacement(0, floor.Placement{ID: frametest.ID(30), NPC: clerkID, Col: 3, Row: 4}); err != nil {
			t.Fatal(err)
		}
	}
	day := newWorkflow(t)
	goTo, err := flow.NewNode(flow.KindGoTo)
	if err == nil {
		goTo, err = goTo.WithParam("destination", floor.AnchorRef{Floor: floorID, Anchor: deskID})
	}
	goTo.ID = frametest.ID(31)
	if err != nil || day.Insert([]flow.Node{goTo}, nil, nil) != nil {
		t.Fatalf("Clerk day: %v", err)
	}
	for id, behaviour := range map[ids.ULID]ids.ULID{clerkID: dayID, customerID: dayID} {
		person := npcplugin.NewDocument()
		person.NPC.Zone, person.NPC.Behaviour = zoneID, behaviour
		f.docs().Put(id, "", person)
	}
	docs := f.docs()
	docs.Put(harbourID, "Harbour", harbour)
	docs.Put(quayID, "Quay", quay)
	docs.Put(floorID, "Ground", ground)
	docs.Put(dayID, "Clerk day", &workflowplugin.Document{Workflow: day})
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"}, {ID: floorID, Name: "Ground", Format: floor.FormatName},
		{ID: harbourID, Name: "Harbour", Format: "zone"}, {ID: quayID, Name: "Quay", Format: floor.FormatName},
		{ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName}, {ID: customerID, Name: "Customer", Format: npc.FormatName},
		{ID: dayID, Name: "Clerk day", Format: flow.FormatName},
	}))
}

// newWorkflow makes a workflow with only its start, as the orchestrator
// would.
func newWorkflow(t *testing.T) *flow.Workflow {
	t.Helper()
	data, err := workflowplugin.Prepare(workflowplugin.NewData(), func() (ids.ULID, error) { return frametest.ID(50), nil })
	if err != nil {
		t.Fatal(err)
	}
	made, err := flow.Decode(data)
	if err != nil {
		t.Fatal(err)
	}

	return made
}

// chooseHarbour opens the Zone box's picker for Ada Clerk and chooses
// Harbour.
func (f *fixture) chooseHarbour() {
	f.t.Helper()
	f.click("row:Ada Clerk", false)
	f.click("ui:npc-zone", false)
	f.click("zone:Harbour", false)
	f.press("Enter")
	f.press("Shift")
}

// TestMovingAnNPCAsksFirst moves an NPC that is placed and whose behaviour
// goes to a place in its old zone: the dialog says it comes off its map,
// that its behaviour also runs for Customer, and lists the place it could
// no longer go to; Move takes it off its map and into the new zone in one
// undo step.
func TestMovingAnNPCAsksFirst(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withMoves(t, f, true)
	f.chooseHarbour()
	if len(f.requests.RunnerAsks) != 1 || f.requests.RunnersOf[0] != dayID || f.has("dialog") {
		t.Fatalf("the move asks who else runs Clerk day before asking: %v", f.requests.RunnersOf)
	}
	f.requests.RunnerAsks[0]([]protocol.Use{
		{Document: clerkID, Name: "Ada Clerk", Format: npc.FormatName, Label: protocol.RunsAsBehaviour, Item: ""},
		{Document: customerID, Name: "Customer", Format: npc.FormatName, Label: protocol.RunsAsBehaviour, Item: ""},
	}, nil)
	f.press("Shift")
	for _, line := range []string{
		"Town → Harbour", "Ada Clerk comes off Ground in Town.", "Clerk day also runs for Customer",
		"Places Ada Clerk could no longer go:",
	} {
		if !f.has("dialog-line=" + line) {
			t.Fatalf("the dialog says %q", line)
		}
	}
	if !f.has("dialog-row=Desk") || len(f.requests.Of(npcplugin.SetZoneEdit)) != 0 {
		t.Fatal("the dialog lists Desk and nothing moves before Move")
	}
	f.click("ui:zone-move", false)
	removed, homed := f.requests.Of(tilefloor.RemovePlacementEdit), f.requests.Of(npcplugin.SetZoneEdit)
	var home npcplugin.SetZoneArgs
	if len(removed) != 1 || len(homed) != 1 || homed[0].Decode(&home) != nil || home.Zone != harbourID ||
		removed[0].Command.Gesture == "" || removed[0].Command.Gesture != homed[0].Command.Gesture {
		t.Fatalf("Move takes it off its map and into Harbour in one step: %+v %+v", removed, homed)
	}
}

// TestMovingAnNPCThatBreaksNothing moves an NPC placed nowhere whose
// behaviour names no place outside the new zone at once, without asking.
func TestMovingAnNPCThatBreaksNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withMoves(t, f, false)
	f.docs().Put(dayID, "Clerk day", &workflowplugin.Document{Workflow: newWorkflow(t)})
	f.chooseHarbour()
	f.requests.RunnerAsks[0](nil, nil)
	f.press("Shift")
	homed := f.requests.Of(npcplugin.SetZoneEdit)
	if f.has("dialog") || len(homed) != 1 || len(f.requests.Of(tilefloor.RemovePlacementEdit)) != 0 {
		t.Fatalf("a move that breaks nothing happens at once: %+v", homed)
	}
}
