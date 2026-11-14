package pixelview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui/pen"
)

// At 125% for text and controls, a sprite pixel is as large on the screen at
// a magnification as at 100%, four fifths as many of the window's points,
// and the pencil paints the pixel under the pointer (increment 56).
//
//nolint:paralleltest // It sets the window's scale.
func TestTheScaleLeavesTheSpritesPixelsAlone(t *testing.T) {
	t.Cleanup(func() { pen.SetScale(1, 1) })
	pen.SetScale(1, 1.25)
	f := newFixture(t)
	f.keys("Cmd+Minus")
	f.keys("Cmd+Minus")
	if f.state.Zoom() != 40 {
		t.Fatalf("zoom %d", f.state.Zoom())
	}
	if across := f.at("pixel:5,1").X - f.at("pixel:1,1").X; across != 4*40*4/5 {
		t.Fatalf("four pixels at 40× are %d of the window's points at 125%%, want %d", across, 4*40*4/5)
	}
	f.click("swatch:gloomy / grey 3")
	f.drag("pixel:1,1", "pixel:4,1")
	for _, want := range []image.Point{{1, 1}, {2, 1}, {3, 1}, {4, 1}} {
		if f.pixel(want.X, want.Y) != grey(3) {
			t.Fatalf("pixel %v is %+v", want, f.pixel(want.X, want.Y))
		}
	}
	if f.pixel(0, 1) == grey(3) || f.pixel(5, 1) == grey(3) {
		t.Fatal("the pencil painted past the pixels dragged over")
	}
}
