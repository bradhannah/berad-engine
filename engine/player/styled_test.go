package player_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/bradhannah/berad-engine/engine/player"
	"github.com/bradhannah/berad-engine/engine/show"
)

func writer(t *testing.T, scale float64) player.Writer {
	t.Helper()
	faces, err := player.NewType()
	if err != nil {
		t.Fatal(err)
	}

	return player.Writer{Type: faces, Scale: scale}
}

// TestWrapStyled wraps styled text to a width, keeping its line breaks and
// a word whose look changes partway whole, each piece in its own face.
func TestWrapStyled(t *testing.T) {
	t.Parallel()
	write := writer(t, 1)
	gold := color.RGBA{R: 0xf0, G: 0xc7, B: 0x5e, A: 0xff}
	lines := write.Wrap([]player.Styled{
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
	for _, piece := range append(append([]player.Piece{}, first...), lines[1].Pieces...) {
		bold = bold || (piece.Text == "Mar" && piece.Face == write.Type.Strong)
		italic = italic || (piece.Text == "mackerel" && piece.Face == write.Type.Italic && piece.Colour == gold)
	}
	if !bold || !italic {
		t.Fatalf("each piece in its own face: %+v", lines)
	}
	room := write.Width("one two", write.Type.Face)
	narrow := write.Wrap([]player.Styled{{Text: "one two three four five", Bold: false, Italic: false, Colour: nil}}, room)
	if len(narrow) != 3 || player.StyledWidth(narrow) > room {
		t.Fatalf("words wrap to the width: %d lines, %d wide", len(narrow), player.StyledWidth(narrow))
	}
}

// TestTextMeasuresTheSameAtAnyScale lays text out in points: a line is as
// wide at twice the pixels to a point, give or take a point of rounding.
func TestTextMeasuresTheSameAtAnyScale(t *testing.T) {
	t.Parallel()
	once, twice := writer(t, 1), writer(t, 2)
	at1, at2 := once.Width("Welcome to Marta's Fish!", once.Type.Face), twice.Width("Welcome to Marta's Fish!", twice.Type.Face)
	if at1 == 0 || at2 < at1-1 || at2 > at1+1 {
		t.Fatalf("the same width in points at any scale: %d and %d", at1, at2)
	}
}

// TestStyledOfReadsMarkup reads a line's markup: bold, italic, and a named
// colour as it reads on a dark box and on a light bubble.
func TestStyledOfReadsMarkup(t *testing.T) {
	t.Parallel()
	runs := player.StyledOf("Fresh [b]mackerel[/b], [color=gold]two coins[/color]", true, 0xff)
	var bold, gold bool
	for _, run := range runs {
		bold = bold || (run.Text == "mackerel" && run.Bold)
		gold = gold || (run.Text == "two coins" && run.Colour == color.RGBA{R: 0xf0, G: 0xc7, B: 0x5e, A: 0xff})
	}
	if !bold || !gold {
		t.Fatalf("bold and gold are read: %+v", runs)
	}
	if player.Ink("gold", false, 0xff) == player.Ink("gold", true, 0xff) {
		t.Fatal("a named colour reads differently on light and dark")
	}
	if got := player.Ink("#102030", true, 0xff); got != (color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xff}) {
		t.Fatalf("#rrggbb is itself: %v", got)
	}
	if player.Ink("nonsense", true, 0xff) != nil {
		t.Fatal("an unknown colour is none")
	}
}

// TestBoxOfAndWhereItGoes shows a stage's front box, and places it at the
// foot of its area, wide enough for its text and taller with a name.
func TestBoxOfAndWhereItGoes(t *testing.T) {
	t.Parallel()
	stage := show.NewStage()
	if player.BoxOf(stage) != nil {
		t.Fatal("no box on an empty stage")
	}
	write := writer(t, 1)
	area := image.Rect(0, 0, 800, 600)
	plain, lines := write.BoxRect(area, &player.Box{Text: "Hello there", Name: "", Left: 1, Queued: 0, Closing: true})
	named, _ := write.BoxRect(area, &player.Box{Text: "Hello there", Name: "Marta", Left: 1, Queued: 0, Closing: true})
	if len(lines) != 1 || plain.Max.Y != named.Max.Y || named.Dy() != plain.Dy()+player.LineHeight {
		t.Fatalf("a name adds a line above: %v and %v, %d lines", plain, named, len(lines))
	}
	if plain.Max.Y >= area.Max.Y || plain.Min.X <= area.Min.X || plain.Dx() != 560 {
		t.Fatalf("the box sits at the foot of its area, inside it: %v", plain)
	}
	bubble, _ := write.BubbleRect(image.Pt(400, 300), "Fresh fish!")
	if bubble.Max.Y >= 300 || bubble.Min.X >= 400 || bubble.Max.X <= 400 {
		t.Fatalf("a bubble sits over its speaker's head: %v", bubble)
	}
}
