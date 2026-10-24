package mapview_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestKeySheet opens the list of keys from its button and with ?, and closes
// it with Escape and with a click outside; while open, keys and clicks never
// reach the map.
func TestKeySheet(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:keys")
	if !f.has("keys") {
		t.Fatal("the Keys button opens the sheet")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.keys("W")
	if f.state.Tool() == "walls" {
		t.Fatal("a key reached the map under the sheet")
	}
	f.keys("Escape")
	if f.has("keys") {
		t.Fatal("Escape closes the sheet")
	}
	f.keys("Shift+Slash")
	if !f.has("keys") {
		t.Fatal("? opens the sheet")
	}
	f.frame(pointer(image.Pt(2, 2), true, true, false))
	f.frame(pointer(image.Pt(2, 2), false, false, true))
	if f.has("keys") {
		t.Fatal("a click outside closes the sheet")
	}
}
