package ui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Key sheet geometry, in screen pixels.
const (
	capHeight = 20
	capPad    = 6
	capGap    = 4
	capRadius = 4
	rowGap    = 6
	columnGap = 24
)

// KeyRow is one row of a key sheet: what the keys do, and each key or
// chord that does it, such as "Cmd+Z" or "Right-click".
type KeyRow struct {
	Label string
	Keys  []string
}

// KeySheet lists a window's keys in a card over the window, titled "Keys",
// each key drawn as a cap. While open it takes all input; Escape or a click
// outside the card closes it.
type KeySheet struct {
	Rows []KeyRow
	open bool
	rect image.Rectangle
	// labelWidth is the width of the label column.
	labelWidth int
}

// Open shows the sheet in the middle of the window.
func (k *KeySheet) Open(window image.Rectangle, theme *Theme) {
	k.labelWidth = 0
	keysWidth := 0
	for _, row := range k.Rows {
		k.labelWidth = max(k.labelWidth, theme.TextWidth(row.Label))
		keysWidth = max(keysWidth, capsWidth(row.Keys, theme))
	}
	width := Half*Inset + k.labelWidth + columnGap + keysWidth
	height := Half*Inset + LineHeight + len(k.Rows)*(capHeight+rowGap)
	middle := Center(window)
	k.rect = image.Rect(middle.X-width/Half, middle.Y-height/Half, middle.X-width/Half+width, middle.Y-height/Half+height)
	k.open = true
}

// capsWidth is the width of a row of caps.
func capsWidth(keys []string, theme *Theme) int {
	width := 0
	for index, key := range keys {
		if index > 0 {
			width += capGap
		}
		width += theme.MonoWidth(key) + Half*capPad
	}

	return width
}

// IsOpen reports whether the sheet is showing.
func (k *KeySheet) IsOpen() bool { return k.open }

// Close hides the sheet.
func (k *KeySheet) Close() { k.open = false }

// Update handles a frame of input while open: Escape or a click outside the
// card closes it, and nothing else reaches the window.
func (k *KeySheet) Update(input *Input) {
	if !k.open {
		return
	}
	defer input.ClearAll()
	if input.Consume(KeyEscape, Plain()) || ((input.Pressed || input.Secondary) && !input.Cursor.In(k.rect)) {
		k.open = false
	}
}

// Draw renders the sheet when open: the window shaded, and the card with a
// row per action, its label at the left and its keys as caps.
func (k *KeySheet) Draw(screen *ebiten.Image, theme *Theme) {
	if !k.open {
		return
	}
	FillRect(screen, screen.Bounds(), ColorShade())
	FillRound(screen, k.rect.Add(image.Pt(0, shadowDrop)), dialogRadius, HexAlpha(hexInk, shadowAlpha))
	FillRound(screen, k.rect, dialogRadius, ColorBackground())
	StrokeRound(screen, k.rect, dialogRadius, Border, ColorInk())
	left, top := k.rect.Min.X+Inset, k.rect.Min.Y+Inset
	theme.DrawStrong(screen, "Keys", left, top, ColorInk())
	hint := "Escape closes"
	theme.DrawSmall(screen, hint, k.rect.Max.X-Inset-theme.SmallWidth(hint), top+Half, ColorSubdued())
	top += LineHeight + rowGap
	for _, row := range k.Rows {
		theme.DrawText(screen, row.Label, left, top+(capHeight-TextSize)/Half-1, ColorMuted())
		capLeft := left + k.labelWidth + columnGap
		for _, key := range row.Keys {
			width := theme.MonoWidth(key) + Half*capPad
			keyCap := image.Rect(capLeft, top, capLeft+width, top+capHeight)
			FillRound(screen, keyCap, capRadius, ColorPanel())
			StrokeRound(screen, keyCap, capRadius, Hairline, ColorRule())
			theme.DrawMono(screen, key, capLeft+capPad, top+(capHeight-MonoSize)/Half-1, ColorInk())
			capLeft += width + capGap
		}
		top += capHeight + rowGap
	}
}
