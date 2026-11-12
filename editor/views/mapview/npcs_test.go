package mapview_test

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var (
	clerkID   = frametest.ID(60)
	routineID = frametest.ID(61)
	placedID  = frametest.ID(70)
)

// withClerk adds an NPC called "Ada Clerk", who follows "Clerk day", to the
// fixture's project.
func withClerk(f *fixture) {
	clerk := npcplugin.NewDocument()
	clerk.NPC.Behaviour = routineID
	f.docs.Put(clerkID, "Ada Clerk", clerk)
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"}, {ID: groundID, Name: "Ground", Format: floor.FormatName},
		{ID: upperID, Name: "Upper", Format: floor.FormatName}, {ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName},
		{ID: routineID, Name: "Clerk day", Format: flow.FormatName},
	}))
}

// TestPlacingAnNPC chooses an NPC, places it, drags it to another cell,
// opens its behaviour and removes it, each through the floor's placement
// edits.
func TestPlacingAnNPC(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withClerk(f)
	f.keys("N")
	if f.state.Tool() != mapview.ToolNPC {
		t.Fatalf("N chose %q", f.state.Tool())
	}
	f.click("cell:9,8")
	if len(f.requests.Of(tilefloor.AddPlacementEdit)) != 0 {
		t.Fatal("nothing is placed before an NPC is chosen")
	}
	f.click("npc:Ada Clerk")
	f.click("cell:9,8")
	f.answerUses(nil)
	added := f.requests.Of(tilefloor.AddPlacementEdit)
	if len(added) != 1 {
		t.Fatalf("add placement commands %d", len(added))
	}
	var args tilefloor.AddPlacementArgs
	if err := added[0].Decode(&args); err != nil || args.Placement.NPC != clerkID || args.Placement.Col != 9 ||
		args.Placement.Row != 8 || !args.Placement.ID.IsZero() {
		t.Fatalf("args %+v %v", args, err)
	}
	ground := groundFloor(t)
	if err := ground.Floor.InsertPlacement(0, floor.Placement{ID: placedID, NPC: clerkID, Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(groundID, "Ground", ground)
	added[0].Succeed(placedID)
	if f.state.SelectedPlacement() != placedID {
		t.Fatal("the new placement is not selected")
	}
	if _, ok := f.state.Locate("state:placement:9,8=Ada Clerk", f.ctx.Docs); !ok {
		t.Fatal("the window does not see the placement")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.frame(pointer(f.at("cell:9,8"), true, true, false))
	f.frame(pointer(f.at("cell:10,8"), false, true, false))
	f.frame(pointer(f.at("cell:10,8"), false, false, true))
	moved := f.requests.Of(tilefloor.UpdatePlacementEdit)
	var update tilefloor.UpdatePlacementArgs
	if len(moved) != 1 || moved[0].Decode(&update) != nil || update.Placement.Col != 10 || update.Placement.ID != placedID {
		t.Fatalf("drag sent %d moves: %+v", len(moved), update)
	}
	f.click("ui:open-behaviour")
	if opened := f.requests.Opened; len(opened) != 1 || opened[0].Target != routineID || opened[0].View != "workflow" {
		t.Fatalf("opened %+v", opened)
	}
	f.keys("Backspace")
	removed := f.requests.Of(tilefloor.RemovePlacementEdit)
	var removal tilefloor.RemovePlacementArgs
	if len(removed) != 1 || removed[0].Decode(&removal) != nil || removal.ID != placedID {
		t.Fatalf("remove commands %d %+v", len(removed), removal)
	}
	if !f.state.SelectedPlacement().IsZero() {
		t.Fatal("the removed placement is still selected")
	}
}

func TestInitials(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"Ada Clerk": "AC", "NPC 1": "N1", "guard": "G", "  ": "", "Old Tom the Smith": "OT", "(Ma) Jones": "MJ",
	} {
		if got := mapview.Initials(name); got != want {
			t.Errorf("initials of %q: %q, want %q", name, got, want)
		}
	}
}

// answerUses answers every where-used question still waiting with the
// uses given.
func (f *fixture) answerUses(uses []protocol.Use) {
	for index, done := range f.requests.Uses {
		if done != nil {
			done(uses, nil)
			f.requests.Uses[index] = nil
		}
	}
}

// TestOnePlacementInTheProject refuses to place an NPC that another zone's
// floor already places, naming the floor.
func TestOnePlacementInTheProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withClerk(f)
	f.keys("N")
	f.click("npc:Ada Clerk")
	f.click("cell:3,3")
	f.answerUses([]protocol.Use{{Document: frametest.ID(80), Name: "Harbour ground", Format: floor.FormatName,
		Label: "placement"}})
	if added := f.requests.Of(tilefloor.AddPlacementEdit); len(added) != 0 {
		t.Fatalf("an NPC placed elsewhere is refused: %d sent", len(added))
	}
	if !strings.Contains(f.status(), "already placed on Harbour ground") {
		t.Fatalf("status %q", f.status())
	}
}

// TestOnePlacementPerZone refuses to place an NPC a second time in a zone,
// saying why.
func TestOnePlacementPerZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withClerk(f)
	ground := groundFloor(t)
	if err := ground.Floor.InsertPlacement(0, floor.Placement{ID: placedID, NPC: clerkID, Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(groundID, "Ground", ground)
	f.keys("N")
	f.click("npc:Ada Clerk")
	f.click("cell:3,3")
	if added := f.requests.Of(tilefloor.AddPlacementEdit); len(added) != 0 {
		t.Fatalf("a second placement in one zone is refused: %d sent", len(added))
	}
	if !strings.Contains(f.status(), "already placed in this zone") {
		t.Fatalf("status %q", f.status())
	}
}

// TestPlacingInItsZone places an NPC that lives nowhere, which then lives
// in the map's zone, in one undo step, and refuses one that lives in
// another zone, saying where to change it.
func TestPlacingInItsZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withClerk(f)
	harbourID, customerID := frametest.ID(81), frametest.ID(82)
	customer := npcplugin.NewDocument()
	customer.NPC.Zone = harbourID
	f.docs.Put(customerID, "Customer", customer)
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"}, {ID: groundID, Name: "Ground", Format: floor.FormatName},
		{ID: upperID, Name: "Upper", Format: floor.FormatName}, {ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName},
		{ID: customerID, Name: "Customer", Format: npc.FormatName}, {ID: harbourID, Name: "Harbour", Format: "zone"},
		{ID: routineID, Name: "Clerk day", Format: flow.FormatName},
	}))
	f.keys("N")
	f.click("npc:Customer")
	f.click("cell:3,3")
	if added := f.requests.Of(tilefloor.AddPlacementEdit); len(added) != 0 ||
		f.status() != "Refused: Customer lives in Harbour. To place it here, change its zone in its details." {
		t.Fatalf("an NPC living elsewhere is refused: %d sent, %q", len(added), f.status())
	}
	f.click("npc:Ada Clerk")
	f.click("cell:3,3")
	f.answerUses(nil)
	added, homed := f.requests.Of(tilefloor.AddPlacementEdit), f.requests.Of(npcplugin.SetZoneEdit)
	var home npcplugin.SetZoneArgs
	if len(added) != 1 || len(homed) != 1 || homed[0].Decode(&home) != nil || home.Zone != zoneID ||
		homed[0].Command.Document != clerkID || added[0].Command.Gesture == "" ||
		added[0].Command.Gesture != homed[0].Command.Gesture {
		t.Fatalf("placing an NPC that lives nowhere gives it the zone, in one step: %+v %+v", added, homed)
	}
	added[0].Succeed(placedID)
	if f.status() != "Placed Ada Clerk at 3, 3. It lives in Town now." {
		t.Fatalf("status %q", f.status())
	}
}

// TestPlacingAnNPCWhoseZoneWasDeleted places an NPC whose zone is gone as
// one that lives nowhere: it takes the map's zone.
func TestPlacingAnNPCWhoseZoneWasDeleted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withClerk(f)
	clerk := npcplugin.NewDocument()
	clerk.NPC.Zone = frametest.ID(98)
	f.docs.Put(clerkID, "Ada Clerk", clerk)
	f.keys("N")
	f.click("npc:Ada Clerk")
	f.click("cell:3,3")
	f.answerUses(nil)
	homed := f.requests.Of(npcplugin.SetZoneEdit)
	var home npcplugin.SetZoneArgs
	if len(homed) != 1 || homed[0].Decode(&home) != nil || home.Zone != zoneID {
		t.Fatalf("an NPC whose zone was deleted takes the map's zone: %+v %q", homed, f.status())
	}
}

// TestNoNPCsHintClearsTheFloors chooses Place NPC in a project with no
// NPCs: the hint's two lines end above the heading of the floor list.
func TestNoNPCsHintClearsTheFloors(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("N")
	f.keys()
	hint, heading := f.state.NPCHint()
	if hint.Empty() || hint.Max.Y > heading {
		t.Fatalf("the hint ends at %d and the floors' heading starts at %d", hint.Max.Y, heading)
	}
}
