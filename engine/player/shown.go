package player

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/show"
)

// What workflows show the player, the same in the editor's preview, its Play
// tab and a game: a speaker's line in a bubble over its head, fading as its
// time ends, and the message box at the foot of the view, with its name, how
// many boxes wait, and a bar draining as its time runs or how to close it. A
// line's or a box's markup draws bold, italic and in colour.

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

// Box is the message box shown now: its text, the name on it, the share of
// its time left, how many boxes are queued with it, and whether it waits
// for the player to close it.
type Box struct {
	Text    string
	Name    string
	Left    float64
	Queued  int
	Closing bool
}

// BoxOf is the box a stage shows now, or nil for none.
func BoxOf(stage *show.Stage) *Box {
	box, queued, shown := stage.Front()
	if !shown {
		return nil
	}

	return &Box{
		Text: box.Text, Name: box.Name, Left: float64(box.Left) / float64(box.Ticks), Queued: queued,
		Closing: box.Closing,
	}
}

// BubbleRect is where a line's bubble goes over a head, its tail's tip at
// the head, and the line wrapped to fit.
func (w Writer) BubbleRect(head image.Point, line string) (image.Rectangle, []StyledLine) {
	lines := w.Wrap(StyledOf(line, false, fullOpacity), speechWidth-half*bubblePad)
	width := StyledWidth(lines) + half*bubblePad
	height := len(lines)*LineHeight + half*bubblePad
	bottom := head.Y - bubbleTail

	return image.Rect(head.X-width/half, bottom-height, head.X-width/half+width, bottom), lines
}

// DrawBubble draws a line in a bubble over a head, fading as the share of
// its time left comes to an end.
func (w Writer) DrawBubble(screen *ebiten.Image, head image.Point, line string, left float64) {
	alpha := uint8(math.Round(fullOpacity * min(1, left/fadeShare)))
	rect, _ := w.BubbleRect(head, line)
	lines := w.Wrap(StyledOf(line, false, alpha), speechWidth-half*bubblePad)
	ink := hexAlpha(hexInk, alpha)
	w.fillRound(screen, rect, hexAlpha(hexPaper, alpha))
	w.strokeRound(screen, rect, ink)
	middle, tip := (rect.Min.X+rect.Max.X)/half, image.Pt((rect.Min.X+rect.Max.X)/half, rect.Max.Y+bubbleTail)
	w.strokeLine(screen, image.Pt(middle-bubbleTail/half, rect.Max.Y), tip, ink)
	w.strokeLine(screen, image.Pt(middle+bubbleTail/half, rect.Max.Y), tip, ink)
	w.DrawStyled(screen, lines, rect.Min.X+bubblePad, rect.Min.Y+bubblePad, ink)
}

// BoxRect is where a box goes, at the foot of an area, and its text wrapped
// to fit.
func (w Writer) BoxRect(area image.Rectangle, box *Box) (image.Rectangle, []StyledLine) {
	width := min(boxWidth, area.Dx()-half*inset)
	lines := w.Wrap(StyledOf(box.Text, true, fullOpacity), width-half*boxPad)
	rows := len(lines) + 1
	if box.Name != "" {
		rows++
	}
	height := rows*LineHeight + half*boxPad
	left := (area.Min.X+area.Max.X)/half - width/half

	return image.Rect(left, area.Max.Y-inset-height, left+width, area.Max.Y-inset), lines
}

// DrawBox draws a box at the foot of an area: its name, its text, how many
// wait with it, and the bar draining as its time runs, or how to close it.
// A nil box draws nothing.
func (w Writer) DrawBox(screen *ebiten.Image, area image.Rectangle, box *Box) {
	if box == nil {
		return
	}
	rect, lines := w.BoxRect(area, box)
	paper, soft := hexAlpha(hexPaper, fullOpacity), hexAlpha(hexBoxSoft, fullOpacity)
	w.fillRound(screen, rect, hexAlpha(hexBox, fullOpacity))
	w.strokeRound(screen, rect.Inset(half), paper)
	top := rect.Min.Y + boxPad
	if box.Name != "" {
		w.Draw(screen, box.Name, w.Type.Strong, rect.Min.X+boxPad, top, soft)
		top += LineHeight
	}
	w.DrawStyled(screen, lines, rect.Min.X+boxPad, top, paper)
	if box.Queued > 1 {
		count := "1 of " + strconv.Itoa(box.Queued)
		w.Draw(screen, count, w.Type.Small, rect.Max.X-boxPad-w.Width(count, w.Type.Small), rect.Min.Y+boxPad/half, soft)
	}
	if box.Closing {
		w.Draw(screen, CloseHint, w.Type.Small, rect.Max.X-boxPad-w.Width(CloseHint, w.Type.Small), rect.Max.Y-boxPad,
			soft)

		return
	}
	bar := image.Rect(rect.Min.X+boxPad, rect.Max.Y-boxPad/half-boxBar, rect.Max.X-boxPad, rect.Max.Y-boxPad/half)
	w.fillRect(screen, bar, hexAlpha(hexBoxSoft, fullOpacity/half))
	bar.Max.X = bar.Min.X + int(float64(bar.Dx())*box.Left)
	w.fillRect(screen, bar, soft)
}

// StyledOf reads a text's markup into styled runs: on a dark box, or on a
// light bubble, each named colour drawn so it reads there, at an alpha.
func StyledOf(marked string, dark bool, alpha uint8) []Styled {
	runs, _ := show.Parse(marked)
	styled := make([]Styled, 0, len(runs))
	for _, run := range runs {
		look := Styled{Text: run.Text, Bold: run.Bold, Italic: run.Italic, Colour: nil}
		if run.Colour != "" {
			look.Colour = Ink(run.Colour, dark, alpha)
		}
		styled = append(styled, look)
	}

	return styled
}

// Named colours, as each reads on a dark box and on a light bubble: a game
// will draw them from its own palette; these are the engine's.
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
		return hexAlpha(rgb, alpha)
	}
	rgb, err := strconv.ParseUint(strings.TrimPrefix(colour, "#"), hexBase, rgbBits)
	if err != nil {
		return nil
	}

	return hexAlpha(uint32(rgb), alpha)
}

// Reading #rrggbb.
const (
	hexBase = 16
	rgbBits = 24
)

// DrawShown draws over a game, where Draw last placed it in its room, what
// its workflows show: each line said by an NPC in view, in a bubble over its
// head, and the box at the foot. A speaker off screen speaks unseen. The
// game is placed in the room's pixels, and what it shows is written in
// points, each the writer's scale of them. Last, the whole of the game's
// screen is darkened as far as a portal that fades has faded it.
func (s *Screen) DrawShown(room *ebiten.Image, played *game.Game, write Writer) {
	view := played.View()
	points := func(pixels image.Point) image.Point {
		return image.Pt(int(float64(pixels.X)/write.Scale), int(float64(pixels.Y)/write.Scale))
	}
	shown := image.Rectangle{Min: points(s.origin), Max: points(s.origin.Add(image.Pt(
		int(float64(view.Columns*s.tile)*s.scale), int(float64(view.Rows*s.tile)*s.scale))))}
	for _, character := range played.Characters() {
		if !character.Speaking || character.Floor != view.Floor {
			continue
		}
		corner := s.CellAt(view, character.X, character.Y)
		head := points(s.origin.Add(image.Pt(int(float64(corner.X*half+s.tile)*s.scale/half),
			int(float64(corner.Y)*s.scale))))
		if head.In(shown) {
			write.DrawBubble(room, head, character.Line.Text, float64(character.Line.Left)/float64(character.Line.Ticks))
		}
	}
	write.DrawBox(room, shown, BoxOf(played.Stage()))
	if darkness := played.Darkness(); darkness > 0 {
		size := image.Pt(int(math.Ceil(float64(view.Columns*s.tile)*s.scale)),
			int(math.Ceil(float64(view.Rows*s.tile)*s.scale)))
		fillRect(room, image.Rectangle{Min: s.origin, Max: s.origin.Add(size)},
			color.RGBA{R: 0, G: 0, B: 0, A: uint8(math.Round(darkness * opaque))})
	}
}

// opaque is the alpha of a colour nothing shows through.
const opaque = 0xff
