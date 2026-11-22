package ui

import (
	"image"
	"image/color"
)

// PaintsOf is each stretch of a text as the field draws it: its text and
// its paint, in order.
func (w *TextField) PaintsOf(value string) [][2]string {
	var painted [][2]string
	for _, span := range w.spans(value, w.theme.Font) {
		painted = append(painted, [2]string{span.Text, span.Paint})
	}

	return painted
}

// SheetRect returns where an open key sheet's card is, and how far its rows
// scroll.
func (k *KeySheet) SheetRect() (image.Rectangle, int, int) { return k.rect, k.rows.Offset, k.rows.Most }

// Look exposes a button's fill, ink and edge for its state to tests.
func (b *Button) Look() (color.RGBA, color.RGBA, color.RGBA) { return b.look() }
