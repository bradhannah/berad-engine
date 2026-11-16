package mapview_test

import (
	"image"
	"strconv"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
)

// panAt is a frame with the middle button held at a point.
func panAt(at image.Point) ui.Input {
	input := ui.NewInput(at)
	input.PanDown = true

	return input
}

// The middle button held on the floor moves it with the pointer, at 100% and
// at 150% for text and controls.
//
//nolint:paralleltest // It sets the window's scale.
func TestTheMiddleButtonPansTheMap(t *testing.T) {
	t.Cleanup(func() { pen.SetScale(1, 1) })
	for _, author := range []float64{1, 1.5} {
		pen.SetScale(1, author)
		f := newFixture(t)
		from, moved := f.at("cell:5,5"), image.Pt(40, 30)
		f.frame(panAt(from))
		f.frame(panAt(from.Add(moved)))
		f.frame(ui.NewInput(from.Add(moved)))
		if got := f.at("cell:5,5"); got != from.Add(moved) {
			t.Fatalf("at %s%%, the cell went from %v to %v, want %v",
				strconv.Itoa(int(author*100)), from, got, from.Add(moved))
		}
	}
}
