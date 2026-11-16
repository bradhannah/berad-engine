package ui_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestABodyScrollsBetweenHeadingAndFoot scrolls fields 500 points tall in a
// body 200 tall: the wheel over the panel scrolls them no further than
// their end, a point scrolled under the foot is hidden and revealed in the
// body's middle, and showing something else starts at the top (increment
// 61).
func TestABodyScrollsBetweenHeadingAndFoot(t *testing.T) {
	t.Parallel()
	scroll := ui.NewScroll()
	scroll.For("first")
	scroll.Body = image.Rect(800, 100, 1080, 300)
	if scroll.Fit(600) || scroll.Most != 300 {
		t.Fatalf("fields ending at 600 in a body ending at 300 scroll 300, not %d", scroll.Most)
	}
	input := ui.NewInput(image.Pt(900, 200))
	input.WheelY = -100
	if !scroll.Wheel(&input, image.Rect(800, 0, 1080, 555)) || scroll.Offset != 300 || input.WheelY != 0 {
		t.Fatalf("the wheel scrolls to the end and no further: %d", scroll.Offset)
	}
	if scroll.Fit(400) || scroll.Most != 400 {
		t.Fatalf("laid out at an offset of 300, fields ending at 400 still scroll 400, not %d", scroll.Most)
	}
	scroll.Offset = 0
	under := image.Pt(900, 450)
	if !scroll.Hidden(under) || scroll.Hidden(image.Pt(900, 150)) || scroll.Hidden(image.Pt(100, 450)) {
		t.Fatal("only what lies in the body's column above or below it is hidden")
	}
	if !scroll.Reveal(under) || scroll.Offset != 250 {
		t.Fatalf("a field at 450 is scrolled to the body's middle, 200: %d", scroll.Offset)
	}
	scroll.For("second")
	if scroll.Offset != 0 || scroll.Most != 0 {
		t.Fatal("something else shown starts at the top")
	}
}
