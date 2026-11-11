package places

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// Look is how a place box shows what it holds.
type Look int

const (
	// Chosen is a place given, in ink with its ring.
	Chosen Look = iota
	// Falling is a place it falls back to when none is given, muted.
	Falling
	// Empty is no place, grey and dashed.
	Empty
)

// Box geometry and the glow of a place just picked.
const (
	boxPad   = 8
	caretW   = 8
	dash     = 4
	litFill  = 0xfff4cf
	litEdge  = 0xb8860b
	ringSize = ui.IconSize
)

// DrawBox draws a place box: the ring, the label and a caret for its list,
// outlined blue while the list is open and glowing while lit, just picked.
func DrawBox(screen *ebiten.Image, theme *ui.Theme, box image.Rectangle, label string, look Look, lit, open bool) {
	fill, edge := ui.ColorField(), ui.ColorInk()
	switch {
	case lit:
		fill, edge = ui.Hex(litFill), ui.Hex(litEdge)
	case open:
		edge = ui.ColorSelect()
	}
	ui.FillRound(screen, box, ui.Radius, fill)
	ink := ui.ColorInk()
	if look == Empty && !lit {
		ink = ui.ColorMuted()
		dashed(screen, box, ink)
	} else {
		ui.StrokeRound(screen, box, ui.Radius, ui.Border, edge)
	}
	ring := ui.ColorSelect()
	if look != Chosen {
		ring, ink = ui.ColorMuted(), ui.ColorMuted()
	}
	centre := image.Pt(box.Min.X+boxPad+ringSize/ui.Half, box.Min.Y+box.Dy()/ui.Half)
	ui.DrawRing(screen, centre, ring, false)
	caret := image.Rect(box.Max.X-boxPad-caretW, box.Min.Y, box.Max.X-boxPad, box.Max.Y)
	ui.DrawTriangle(screen, caret, open, ui.ColorMuted())
	left := centre.X + ringSize/ui.Half + boxPad
	text := clip(theme, label, caret.Min.X-left-boxPad)
	textTop := box.Min.Y + (box.Dy()-ui.TextSize)/ui.Half - 1
	if look == Chosen {
		theme.DrawStrong(screen, text, left, textTop, ink)

		return
	}
	theme.DrawText(screen, text, left, textTop, ink)
}

// dashed outlines a box with a dashed line.
func dashed(screen *ebiten.Image, box image.Rectangle, ink color.Color) {
	line := func(x0, y0, x1, y1 int) {
		vector.StrokeLine(screen, float32(x0), float32(y0), float32(x1), float32(y1), ui.Border, ink, true)
	}
	for x := box.Min.X; x < box.Max.X; x += ui.Half * dash {
		end := min(x+dash, box.Max.X)
		line(x, box.Min.Y, end, box.Min.Y)
		line(x, box.Max.Y, end, box.Max.Y)
	}
	for y := box.Min.Y; y < box.Max.Y; y += ui.Half * dash {
		end := min(y+dash, box.Max.Y)
		line(box.Min.X, y, box.Min.X, end)
		line(box.Max.X, y, box.Max.X, end)
	}
}

// clip shortens a label to a width, ending it with an ellipsis.
func clip(theme *ui.Theme, label string, width int) string {
	if theme.TextWidth(label) <= width {
		return label
	}
	runes := []rune(label)
	for len(runes) > 0 && theme.TextWidth(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}

	return string(runes) + "…"
}
