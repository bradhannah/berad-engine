package mapview_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestHoveringOutsideTheFloor names a cell beyond the floor bounds, but not
// a cell on the floor or the zoom control floating over the map; the map
// draws at every zoom, with and without the outside grid.
func TestHoveringOutsideTheFloor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.frame(ui.NewInput(f.at("cell:14,3")))
	if line, outside := f.state.Outside(); !outside || line != "Cell 14, 3 · outside the floor" {
		t.Fatalf("outside: %q %v", line, outside)
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.frame(ui.NewInput(f.at("cell:5,5")))
	if _, outside := f.state.Outside(); outside {
		t.Fatal("a cell on the floor is not outside")
	}
	f.frame(ui.NewInput(f.at("ui:zoom-fit")))
	if _, outside := f.state.Outside(); outside {
		t.Fatal("the zoom control is not a cell outside")
	}
	for range 3 {
		f.click("ui:zoom-out")
		f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	}
	f.frame(ui.NewInput(image.Pt(5, 5)))
	if _, outside := f.state.Outside(); outside {
		t.Fatal("the tools panel is not a cell outside")
	}
}
