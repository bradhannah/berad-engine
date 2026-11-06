package ui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// A scrolling panel's thumb, in screen pixels: how wide it is, how far it
// sits from the panel's right edge, and how short it can get.
const (
	thumbWidth = 4
	thumbInset = 3
	thumbLeast = 24
)

// DrawScrollThumb draws, while a panel's body scrolls, a thumb at its right
// edge: how much of what scrolls shows, and where. offset is how far it is
// scrolled, and most the furthest it can be.
func DrawScrollThumb(screen *ebiten.Image, body image.Rectangle, offset, most int) {
	if most <= 0 {
		return
	}
	shown := body.Dy()
	length := max(shown*shown/(shown+most), thumbLeast)
	top := body.Min.Y + (shown-length)*offset/most
	right := body.Max.X - thumbInset
	FillRound(screen, image.Rect(right-thumbWidth, top, right, top+length), thumbWidth/Half, ColorRule())
}
