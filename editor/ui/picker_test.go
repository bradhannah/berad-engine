package ui_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

func pick(picker *ui.ColorPicker, at image.Point, pressed, down bool) bool {
	input := ui.NewInput(at)
	input.Pressed, input.Down = pressed, down

	return picker.Update(&input)
}

func locate(t *testing.T, picker *ui.ColorPicker, name string) image.Point {
	t.Helper()
	at, ok := picker.Locate(name)
	if !ok {
		t.Fatalf("no %q on the picker", name)
	}

	return at
}

func rgb(value uint32) color.RGBA {
	return color.RGBA{R: uint8(value >> 16), G: uint8(value >> 8), B: uint8(value), A: 255}
}

// TestColorPickerShowsWhatItIsGiven turns colours into hue, saturation and
// brightness and back without changing them.
func TestColorPickerShowsWhatItIsGiven(t *testing.T) {
	t.Parallel()
	picker := ui.NewColorPicker("picker")
	picker.Rect = image.Rect(0, 0, 256, 150)
	for _, value := range []uint32{
		0x000000, 0xFFFFFF, 0x5D5A63, 0xFF0000, 0x00FF00, 0x0000FF, 0xE07A30, 0x1D2620, 0x8A919C, 0xC7C9CC, 0x090D17,
	} {
		picker.SetColor(rgb(value), 0.6)
		if picker.RGB() != rgb(value) || picker.Opacity() != 0.6 {
			t.Errorf("#%06X shows as %v at %v", value, picker.RGB(), picker.Opacity())
		}
	}
}

// TestColorPickerPicksByEye presses the hue strip and the square, keeps the
// hue through grey, drags past the edges, and lets the colour be set again
// once released.
func TestColorPickerPicksByEye(t *testing.T) {
	t.Parallel()
	picker := ui.NewColorPicker("picker")
	picker.Rect = image.Rect(10, 20, 251, 170)
	picker.SetColor(rgb(0x808080), 1)
	if !pick(picker, locate(t, picker, "hue 50"), true, true) || picker.Dragging() != true {
		t.Fatal("a press on the hue strip sets the hue")
	}
	pick(picker, locate(t, picker, "hue 50"), false, false)
	if picker.RGB() != rgb(0x808080) {
		t.Fatalf("a grey stays grey with any hue: %v", picker.RGB())
	}
	pick(picker, locate(t, picker, "square 100 100"), true, true)
	pick(picker, locate(t, picker, "square 100 100"), false, false)
	if picker.RGB() != rgb(0x00FFFF) {
		t.Fatalf("full saturation and brightness of hue 50%% is cyan: %v", picker.RGB())
	}
	picker.SetColor(rgb(0x000000), 1)
	pick(picker, locate(t, picker, "square 100 100"), true, true)
	if picker.RGB() != rgb(0x00FFFF) {
		t.Fatalf("the hue is kept through black: %v", picker.RGB())
	}
	pick(picker, image.Pt(-50, 500), false, true)
	picker.SetColor(rgb(0xFF0000), 1)
	if picker.RGB() != rgb(0x000000) {
		t.Fatalf("dragging past the edges stops at them, and a drag is not pulled back: %v", picker.RGB())
	}
	pick(picker, image.Pt(-50, 500), false, false)
	picker.SetColor(rgb(0xFF0000), 1)
	if picker.RGB() != rgb(0xFF0000) || picker.Dragging() {
		t.Fatalf("once released the colour is set again: %v", picker.RGB())
	}
	pick(picker, locate(t, picker, "opacity 100"), true, true)
	pick(picker, locate(t, picker, "opacity 25"), false, true)
	pick(picker, locate(t, picker, "opacity 25"), false, false)
	if picker.Opacity() < 0.24 || picker.Opacity() > 0.26 || picker.RGB() != rgb(0xFF0000) {
		t.Fatalf("the opacity strip sets only the opacity: %v %v", picker.Opacity(), picker.RGB())
	}
	if pick(picker, image.Pt(5, 5), true, true) {
		t.Fatal("a press outside changes nothing")
	}
}
