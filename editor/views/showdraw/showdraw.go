// Package showdraw draws what workflows show the player, the same in the
// Map's preview and the Play tab: a speaker's line in a bubble over its
// head, fading as its time ends, and the message box at the foot of the
// view, with its name, how many boxes wait, and a bar draining as its time
// runs or how to close it. A line's or a box's markup draws bold, italic
// and in colour.
package showdraw

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/show"
)

// Sizes and colours: how wide speech and the box may be, a bubble's padding
// and tail, how much of a line's time it takes to fade, and the box.
const (
	speechWidth = 200
	bubblePad   = 6
	bubbleTail  = 8
	fadeShare   = 0.25
	boxWidth    = 560
	boxPad      = 14
	boxBar      = 4
	opaque      = 0xff
	hexInk      = 0x000000
	hexPaper    = 0xffffff
	hexBox      = 0x1c2541
	hexBoxSoft  = 0xc9d1e6
	// CloseHint says how to close a box that waits for the player.
	CloseHint = "Enter, Space or a click closes it"
	// BoxLines is how many lines one box holds before it would need paging,
	// which is not built yet.
	BoxLines = 8
)

// BoxOf is the box a stage shows now, as a preview's state carries it.
func BoxOf(stage *show.Stage) *protocol.PreviewBox {
	box, queued, shown := stage.Front()
	if !shown {
		return nil
	}

	return &protocol.PreviewBox{
		Text: box.Text, Name: box.Name, Left: float64(box.Left) / float64(box.Ticks), Queued: queued,
		Closing: box.Closing,
	}
}

// BubbleRect is where a line's bubble goes over a head, its tail's tip at
// the head, and the line wrapped to fit.
func BubbleRect(theme *ui.Theme, head image.Point, text string) (image.Rectangle, []ui.StyledLine) {
	lines := theme.WrapStyled(Styled(text, false, opaque), speechWidth-ui.Half*bubblePad)
	width := ui.StyledWidth(lines) + ui.Half*bubblePad
	height := len(lines)*ui.LineHeight + ui.Half*bubblePad
	bottom := head.Y - bubbleTail

	return image.Rect(head.X-width/ui.Half, bottom-height, head.X-width/ui.Half+width, bottom), lines
}

// DrawBubble draws a line in a bubble over a head, fading as the share of
// its time left comes to an end.
func DrawBubble(screen *ebiten.Image, theme *ui.Theme, head image.Point, text string, left float64) {
	alpha := uint8(math.Round(opaque * min(1, left/fadeShare)))
	rect, _ := BubbleRect(theme, head, text)
	lines := theme.WrapStyled(Styled(text, false, alpha), speechWidth-ui.Half*bubblePad)
	ink := ui.HexAlpha(hexInk, alpha)
	ui.FillRound(screen, rect, ui.Radius, ui.HexAlpha(hexPaper, alpha))
	ui.StrokeRound(screen, rect, ui.Radius, ui.Border, ink)
	middle, tip := (rect.Min.X+rect.Max.X)/ui.Half, image.Pt((rect.Min.X+rect.Max.X)/ui.Half, rect.Max.Y+bubbleTail)
	ui.Line(screen, image.Pt(middle-bubbleTail/ui.Half, rect.Max.Y), tip, ui.Border, ink)
	ui.Line(screen, image.Pt(middle+bubbleTail/ui.Half, rect.Max.Y), tip, ui.Border, ink)
	theme.DrawStyled(screen, lines, rect.Min.X+bubblePad, rect.Min.Y+bubblePad, ink)
}

// BoxRect is where a box goes, at the foot of an area, and its text wrapped
// to fit.
func BoxRect(theme *ui.Theme, area image.Rectangle, box *protocol.PreviewBox) (image.Rectangle, []ui.StyledLine) {
	width := min(boxWidth, area.Dx()-ui.Half*ui.Inset)
	lines := theme.WrapStyled(Styled(box.Text, true, opaque), width-ui.Half*boxPad)
	rows := len(lines) + 1
	if box.Name != "" {
		rows++
	}
	height := rows*ui.LineHeight + ui.Half*boxPad
	left := (area.Min.X+area.Max.X)/ui.Half - width/ui.Half

	return image.Rect(left, area.Max.Y-ui.Inset-height, left+width, area.Max.Y-ui.Inset), lines
}

// DrawBox draws a box at the foot of an area: its name, its text, how many
// wait with it, and the bar draining as its time runs, or how to close it.
func DrawBox(screen *ebiten.Image, theme *ui.Theme, area image.Rectangle, box *protocol.PreviewBox) {
	if box == nil {
		return
	}
	rect, lines := BoxRect(theme, area, box)
	ui.FillRound(screen, rect, ui.Radius, ui.Hex(hexBox))
	ui.StrokeRound(screen, rect.Inset(ui.Half), ui.Radius, ui.Border, ui.Hex(hexPaper))
	top := rect.Min.Y + boxPad
	if box.Name != "" {
		theme.DrawStrong(screen, box.Name, rect.Min.X+boxPad, top, ui.Hex(hexBoxSoft))
		top += ui.LineHeight
	}
	theme.DrawStyled(screen, lines, rect.Min.X+boxPad, top, ui.Hex(hexPaper))
	if box.Queued > 1 {
		count := "1 of " + strconv.Itoa(box.Queued)
		theme.DrawSmall(screen, count, rect.Max.X-boxPad-theme.SmallWidth(count), rect.Min.Y+boxPad/ui.Half,
			ui.Hex(hexBoxSoft))
	}
	if box.Closing {
		theme.DrawSmall(screen, CloseHint, rect.Max.X-boxPad-theme.SmallWidth(CloseHint), rect.Max.Y-boxPad,
			ui.Hex(hexBoxSoft))

		return
	}
	bar := image.Rect(rect.Min.X+boxPad, rect.Max.Y-boxPad/ui.Half-boxBar, rect.Max.X-boxPad, rect.Max.Y-boxPad/ui.Half)
	ui.FillRect(screen, bar, ui.HexAlpha(hexBoxSoft, opaque/ui.Half))
	bar.Max.X = bar.Min.X + int(float64(bar.Dx())*box.Left)
	ui.FillRect(screen, bar, ui.Hex(hexBoxSoft))
}

// Styled reads a text's markup into styled runs: on a dark box, or on a
// light bubble, each named colour drawn so it reads there, at an alpha.
func Styled(text string, dark bool, alpha uint8) []ui.Styled {
	runs, _ := show.Parse(text)
	styled := make([]ui.Styled, 0, len(runs))
	for _, run := range runs {
		look := ui.Styled{Text: run.Text, Bold: run.Bold, Italic: run.Italic, Colour: nil}
		if run.Colour != "" {
			look.Colour = Ink(run.Colour, dark, alpha)
		}
		styled = append(styled, look)
	}

	return styled
}

// Named colours, as each reads on a dark box and on a light bubble: a game
// draws them from its own palette; these are the editor's.
const (
	darkRed, lightRed       = 0xff8a7a, 0xb5452b
	darkOrange, lightOrange = 0xffb36b, 0xc2621a
	darkGold, lightGold     = 0xf0c75e, 0x9a6700
	darkGreen, lightGreen   = 0x8fd694, 0x3d7a4b
	darkBlue, lightBlue     = 0x8ab4ff, 0x1f5fbf
	darkPurple, lightPurple = 0xc9a7ff, 0x6a4f9e
	darkGrey, lightGrey     = 0xb8bcc6, 0x5a6270
	darkWhite, lightWhite   = 0xffffff, 0x9aa0a6
)

// named returns a colour markup names, as it reads on a dark box or a light
// bubble.
func named(colour string, dark bool) (uint32, bool) {
	pairs := map[string][2]uint32{
		"red": {darkRed, lightRed}, "orange": {darkOrange, lightOrange}, "gold": {darkGold, lightGold},
		"green": {darkGreen, lightGreen}, "blue": {darkBlue, lightBlue}, "purple": {darkPurple, lightPurple},
		"grey": {darkGrey, lightGrey}, "white": {darkWhite, lightWhite},
	}
	pair, found := pairs[colour]
	if dark {
		return pair[0], found
	}

	return pair[1], found
}

// Ink is the colour markup names, on a dark box or a light bubble, at an
// alpha: one of the named colours, or #rrggbb as it is.
func Ink(colour string, dark bool, alpha uint8) color.Color {
	if rgb, found := named(colour, dark); found {
		return ui.HexAlpha(rgb, alpha)
	}
	rgb, err := strconv.ParseUint(strings.TrimPrefix(colour, "#"), hexBase, rgbBits)
	if err != nil {
		return nil
	}

	return ui.HexAlpha(uint32(rgb), alpha)
}

// Reading #rrggbb.
const (
	hexBase = 16
	rgbBits = 24
)
