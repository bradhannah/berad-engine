package workflowview_test

import (
	"image"
	"strconv"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
)

// The middle button held on the canvas moves it with the pointer, at 100% and
// at 150% for text and controls.
//
//nolint:paralleltest // It sets the window's scale.
func TestTheMiddleButtonPansTheWorkflow(t *testing.T) {
	t.Cleanup(func() { pen.SetScale(1, 1) })
	for _, author := range []float64{1, 1.5} {
		pen.SetScale(1, author)
		f := newFixture(t)
		from, moved := f.at("node:start:1"), image.Pt(40, 30)
		for _, at := range []image.Point{from, from.Add(moved)} {
			input := ui.NewInput(at)
			input.PanDown = true
			f.frame(input)
		}
		if got := f.at("node:start:1"); got != from.Add(moved) {
			t.Fatalf("at %s%%, the node went from %v to %v, want %v",
				strconv.Itoa(int(author*100)), from, got, from.Add(moved))
		}
	}
}
