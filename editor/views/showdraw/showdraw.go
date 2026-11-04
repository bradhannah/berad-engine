// Package showdraw draws what workflows show the player, the same in the
// Map's preview and the Play tab: a speaker's line in a bubble over its
// head, fading as its time ends, and the message box at the foot of the
// view, with its name, how many boxes wait, and a bar draining as its time
// runs or how to close it.
package showdraw

import (
	"image"
	"math"
	"strconv"

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
func BubbleRect(theme *ui.Theme, head image.Point, text string) (image.Rectangle, []string) {
	lines := theme.Wrap(text, speechWidth-ui.Half*bubblePad)
	width := 0
	for _, line := range lines {
		width = max(width, theme.TextWidth(line))
	}
	width += ui.Half * bubblePad
	height := len(lines)*ui.LineHeight + ui.Half*bubblePad
	bottom := head.Y - bubbleTail

	return image.Rect(head.X-width/ui.Half, bottom-height, head.X-width/ui.Half+width, bottom), lines
}

// DrawBubble draws a line in a bubble over a head, fading as the share of
// its time left comes to an end.
func DrawBubble(screen *ebiten.Image, theme *ui.Theme, head image.Point, text string, left float64) {
	rect, lines := BubbleRect(theme, head, text)
	alpha := uint8(math.Round(opaque * min(1, left/fadeShare)))
	ink := ui.HexAlpha(hexInk, alpha)
	ui.FillRound(screen, rect, ui.Radius, ui.HexAlpha(hexPaper, alpha))
	ui.StrokeRound(screen, rect, ui.Radius, ui.Border, ink)
	middle, tip := (rect.Min.X+rect.Max.X)/ui.Half, image.Pt((rect.Min.X+rect.Max.X)/ui.Half, rect.Max.Y+bubbleTail)
	ui.Line(screen, image.Pt(middle-bubbleTail/ui.Half, rect.Max.Y), tip, ui.Border, ink)
	ui.Line(screen, image.Pt(middle+bubbleTail/ui.Half, rect.Max.Y), tip, ui.Border, ink)
	for index, line := range lines {
		theme.DrawText(screen, line, rect.Min.X+bubblePad, rect.Min.Y+bubblePad+index*ui.LineHeight, ink)
	}
}

// BoxRect is where a box goes, at the foot of an area, and its text wrapped
// to fit.
func BoxRect(theme *ui.Theme, area image.Rectangle, box *protocol.PreviewBox) (image.Rectangle, []string) {
	width := min(boxWidth, area.Dx()-ui.Half*ui.Inset)
	lines := theme.Wrap(box.Text, width-ui.Half*boxPad)
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
	for _, line := range lines {
		theme.DrawText(screen, line, rect.Min.X+boxPad, top, ui.Hex(hexPaper))
		top += ui.LineHeight
	}
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
