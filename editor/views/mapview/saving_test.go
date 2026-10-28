package mapview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/ui"
)

func (f *fixture) cellSize() int {
	return f.at("cell:1,0").X - f.at("cell:0,0").X
}

func (f *fixture) typed(text string) {
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
}

// TestZoomControl steps the zoom with − and +, returns to 100% from the
// label, fits the floor, and never paints the map underneath. Until the art
// settings load, 100% is the default tile size, 32 pixels.
func TestZoomControl(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if f.cellSize() != 32 {
		t.Fatalf("default cell %d", f.cellSize())
	}
	f.click("ui:zoom-in")
	if f.cellSize() != 64 || !f.has("zoom:200") {
		t.Fatalf("zoomed in %d", f.cellSize())
	}
	f.click("ui:zoom-reset")
	f.click("ui:zoom-out")
	if f.cellSize() != 16 || !f.has("zoom:50") {
		t.Fatalf("back to 100%%, then out: %d", f.cellSize())
	}
	f.click("ui:zoom-fit")
	grid := f.state.Grid()
	floorRect := image.Rectangle{Min: f.at("cell:0,0"), Max: f.at("cell:11,9")}
	if f.cellSize() != 32 || !floorRect.In(grid) {
		t.Fatalf("fit: cell %d, floor %v in %v", f.cellSize(), floorRect, grid)
	}
	if len(paints(t, f.requests)) != 0 {
		t.Fatal("clicking the zoom control painted the map")
	}
}

// TestScrollingMatchesTheWorkflowWindow scrolls down: the map moves up, as
// the Workflow window's canvas does.
func TestScrollingMatchesTheWorkflowWindow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	before := f.at("cell:0,0")
	scroll := ui.NewInput(f.at("cell:5,5"))
	scroll.WheelY = 1
	f.frame(scroll)
	if after := f.at("cell:0,0"); after.Y != before.Y-24 || after.X != before.X {
		t.Fatalf("cell 0,0 moved from %v to %v", before, after)
	}
}

// TestAnchorNameSavesAndAsksBeforeLeaving types an anchor's name, then
// clicks away with the Select tool: Cancel keeps the anchor and its name,
// and Save saves it.
func TestAnchorNameSavesAndAsksBeforeLeaving(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("A")
	f.click("cell:4,4")
	f.click("ui:anchor-name")
	f.typed(" two")
	if f.state.Unsaved(f.ctx.Docs) != 1 {
		t.Fatalf("unsaved %d", f.state.Unsaved(f.ctx.Docs))
	}
	f.keys("Tab")
	f.keys("S")
	f.click("cell:9,8")
	f.click("ui:cancel")
	if f.state.SelectedAnchor() != deskID || f.state.Unsaved(f.ctx.Docs) != 1 {
		t.Fatal("Cancel keeps the anchor selected with its unsaved name")
	}
	f.click("ui:save")
	updates := f.requests.Of(tilefloor.UpdateAnchorEdit)
	var args tilefloor.UpdateAnchorArgs
	if len(updates) != 1 || updates[0].Decode(&args) != nil || args.Anchor.Name != "Desk two" {
		t.Fatalf("updates %d %+v", len(updates), args)
	}
}
