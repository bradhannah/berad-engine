package mapview_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
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
