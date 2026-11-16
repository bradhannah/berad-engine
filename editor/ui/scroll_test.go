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

// TestTabRevealsAFieldOfTheBody scrolls a field below the body into its
// middle as Tab moves to it, and leaves a field elsewhere in the window
// alone (increment 61).
func TestTabRevealsAFieldOfTheBody(t *testing.T) {
	t.Parallel()
	scroll := ui.NewScroll()
	scroll.Body = image.Rect(800, 100, 1080, 300)
	scroll.Fit(600)
	below := &ui.TextField{ID: "below", Rect: image.Rect(814, 430, 1066, 456)}
	elsewhere := &ui.TextField{ID: "elsewhere", Rect: image.Rect(14, 430, 300, 456)}
	scroll.RevealField(elsewhere)
	if scroll.Offset != 0 {
		t.Fatalf("a field outside the body's column scrolled it to %d", scroll.Offset)
	}
	scroll.RevealField(below)
	if scroll.Offset != 243 {
		t.Fatalf("a field at 430 to 456 scrolls to the body's middle, 200: %d", scroll.Offset)
	}
}

// TestAKeySheetFitsAShortWindow opens a sheet of thirty keys in a window
// 555 points high: the card stays inside the window and its rows scroll
// with the wheel under its title (increment 61).
func TestAKeySheetFitsAShortWindow(t *testing.T) {
	t.Parallel()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	sheet := ui.KeySheet{Rows: make([]ui.KeyRow, 30)}
	for index := range sheet.Rows {
		sheet.Rows[index] = ui.KeyRow{Label: "Something", Keys: []string{"Cmd+K"}}
	}
	window := image.Rect(0, 0, 800, 555)
	sheet.Open(window, theme)
	card, _, most := sheet.SheetRect()
	if !card.In(window) || most == 0 {
		t.Fatalf("the card %v is not inside the window %v, or its rows do not scroll: %d", card, window, most)
	}
	wheel := ui.NewInput(ui.Center(card))
	wheel.WheelY = -2
	sheet.Update(&wheel)
	if _, offset, _ := sheet.SheetRect(); offset == 0 || !sheet.IsOpen() {
		t.Fatal("the wheel scrolls the sheet's rows and leaves it open")
	}
}
