package ui_test

import (
	"image/color"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestWrapStyled wraps styled text to a width, keeping its line breaks and
// a word whose look changes partway whole, each piece in its own face.
func TestWrapStyled(t *testing.T) {
	t.Parallel()
	loaded := theme(t)
	gold := color.RGBA{R: 0xf0, G: 0xc7, B: 0x5e, A: 0xff}
	lines := loaded.WrapStyled([]ui.Styled{
		{Text: "Welcome to ", Bold: false, Italic: false, Colour: nil},
		{Text: "Mar", Bold: true, Italic: false, Colour: nil},
		{Text: "ta's Fish!\nToday: ", Bold: false, Italic: false, Colour: nil},
		{Text: "mackerel", Bold: false, Italic: true, Colour: gold},
	}, 1000)
	if len(lines) != 2 {
		t.Fatalf("a line break starts a line: %+v", lines)
	}
	first := lines[0].Pieces
	var bold, italic bool
	for _, piece := range append(append([]ui.Piece{}, first...), lines[1].Pieces...) {
		bold = bold || (piece.Text == "Mar" && piece.Face == loaded.Strong)
		italic = italic || (piece.Text == "mackerel" && piece.Face == loaded.Italic && piece.Colour == gold)
	}
	if !bold || !italic {
		t.Fatalf("each piece in its own face: %+v", lines)
	}
	narrow := loaded.WrapStyled([]ui.Styled{{Text: "one two three four five", Bold: false, Italic: false,
		Colour: nil}}, loaded.TextWidth("one two"))
	if len(narrow) != 3 || ui.StyledWidth(narrow) > loaded.TextWidth("one two") {
		t.Fatalf("words wrap to the width: %d lines, %d wide", len(narrow), ui.StyledWidth(narrow))
	}
}
