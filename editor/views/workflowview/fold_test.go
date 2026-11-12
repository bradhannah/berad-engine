package workflowview_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestThePaletteFoldsWhenToldTo folds the palette with its button: the
// canvas widens over its place, the palette's nodes can no longer be found,
// Tab still offers every node, and the button unfolds it (increment 43).
func TestThePaletteFoldsWhenToldTo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	wide := f.state.Canvas().Dx()
	f.click("ui:fold-left")
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if f.ctx.Frame.ShowLeft.On || f.state.Canvas().Dx() <= wide {
		t.Fatalf("the palette folds and the canvas takes its place: %d to %d", wide, f.state.Canvas().Dx())
	}
	if _, found := f.state.Locate("palette:"+flow.KindWait, f.docs); found {
		t.Fatal("folded, the palette lists nothing")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.keys("Tab")
	if !f.menuOffers("node-menu") {
		t.Fatal("Tab still offers every node")
	}
	f.keys("Escape")
	f.click("ui:fold-left")
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.ctx.Frame.ShowLeft.On || f.state.Canvas().Dx() != wide {
		t.Fatal("the button unfolds it")
	}
}
