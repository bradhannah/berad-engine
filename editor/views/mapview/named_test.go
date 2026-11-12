package mapview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// TestAHistoryLineNamesAPlacementOrAnAnchor finds the NPC placement or the
// anchor a change to one of the zone's floors touched, and shows it when
// asked, with its tool in hand; cells and other documents name nothing
// (increment 45).
func TestAHistoryLineNamesAPlacementOrAnAnchor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withClerk(f)
	ground := groundFloor(t)
	if err := ground.Floor.InsertPlacement(0, floor.Placement{ID: placedID, NPC: clerkID, Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(groundID, "Ground", ground)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	cell := tilefloor.CellKey(tilefloor.LayerGround, 1, 1)
	placed := protocol.Touching(groundID, []string{cell, tilefloor.PlacementKey(placedID)})
	item, name, named := f.state.Named(placed, f.ctx.Docs)
	if !named || name != "Ada Clerk" || item != mapview.PlacementItem+groundID.String()+":"+placedID.String() {
		t.Fatalf("a line names the NPC placed: %q %q %v", item, name, named)
	}
	f.state.Changed(client.Update{Attention: true, Item: item}, f.ctx)
	if f.state.SelectedPlacement() != placedID || f.state.Tool() != mapview.ToolNPC {
		t.Fatal("asked, the Map selects the placement with the NPC tool")
	}
	desk := protocol.Touching(groundID, []string{tilefloor.AnchorKey(deskID)})
	item, name, named = f.state.Named(desk, f.ctx.Docs)
	if !named || name != "the anchor Desk" {
		t.Fatalf("a line names the anchor: %q %q %v", item, name, named)
	}
	f.state.Changed(client.Update{Attention: true, Item: item}, f.ctx)
	if f.state.SelectedAnchor() != deskID || f.state.Tool() != mapview.ToolAnchor {
		t.Fatal("asked, the Map selects the anchor with the Anchors tool")
	}
	if _, _, other := f.state.Named(protocol.Touching(frametest.ID(77), desk.Items), f.ctx.Docs); other {
		t.Fatal("a change to a floor of another zone names nothing here")
	}
	if _, _, cells := f.state.Named(protocol.Touching(groundID, []string{cell}), f.ctx.Docs); cells {
		t.Fatal("painted cells name nothing to select")
	}
}
