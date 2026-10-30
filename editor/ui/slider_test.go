package ui_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

func slide(slider *ui.Slider, at image.Point, pressed, down bool) bool {
	input := ui.NewInput(at)
	input.Pressed, input.Down = pressed, down

	return slider.Update(&input)
}

// TestSliderSetsAValue presses and drags the slider, which stops at its ends
// and ignores presses outside it.
func TestSliderSetsAValue(t *testing.T) {
	t.Parallel()
	slider := ui.NewSlider("opacity", 1, 100)
	slider.Rect = image.Rect(10, 0, 210, 26)
	at, ok := slider.Locate("40")
	if !ok || !slide(slider, at, true, true) || slider.Value != 40 || !slider.Dragging() {
		t.Fatalf("a press at 40 sets 40: %d", slider.Value)
	}
	slide(slider, image.Pt(500, 13), false, true)
	slide(slider, image.Pt(500, 13), false, false)
	if slider.Value != 100 || slider.Dragging() {
		t.Fatalf("dragging past the end stops at 100: %d", slider.Value)
	}
	if slide(slider, image.Pt(5, 13), true, true) || slider.Value != 100 {
		t.Fatal("a press outside changes nothing")
	}
	if _, ok := slider.Locate("101"); ok {
		t.Fatal("101 is past the end")
	}
}
