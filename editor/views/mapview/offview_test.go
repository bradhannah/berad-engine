package mapview_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestTheMapSaysWhenTheFloorIsOutOfView pans the floor wholly out of view:
// the Map says so and which way, and Fit brings it back (increment 42).
func TestTheMapSaysWhenTheFloorIsOutOfView(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if f.has("floor-off-view") {
		t.Fatal("the floor starts in view")
	}
	for range 20 {
		wheel := ui.NewInput(f.at("cell:2,3"))
		wheel.WheelX = 40
		f.frame(wheel)
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	if !f.has("floor-off-view") {
		t.Fatal("panned far away, the Map says the floor is out of view")
	}
	f.press("ui:zoom-fit", false)
	if f.has("floor-off-view") {
		t.Fatal("Fit brings it back")
	}
}
