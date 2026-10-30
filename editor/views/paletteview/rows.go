package paletteview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Ramp rows' geometry, in screen pixels.
const (
	swatchSize  = 26
	swatchStep  = 29
	nameColumn  = 112
	countColumn = 28
	rowGap      = 8
	labelDot    = 4
	chosenWidth = 2
	pickedWidth = 1
	dropWidth   = 3
	scrollBar   = 4
	// dragStart is how far a press moves before it drags.
	dragStart = 4
	tipPad    = 6
	tipOffset = 14
)

// rampRow is one ramp's row: its name, which chooses and drags the ramp, and
// its colours, wrapping onto more lines when they do not fit.
type rampRow struct {
	ramp     art.Ramp
	name     image.Rectangle
	top      int
	bottom   int
	swatches []image.Rectangle
}

// movedLine is a shadow record still named by a tile: where the colour was,
// and where it is now.
type movedLine struct {
	record art.Moved
	now    string
	top    int
}

// rowsLayout is where the ramps are this frame: the area they scroll in, the
// clear colour, each ramp's row, how tall they all are, and the moved
// colours below them.
type rowsLayout struct {
	area     image.Rectangle
	clear    image.Rectangle
	rows     []rampRow
	content  int
	movedTop int
	moved    []movedLine
}

func newRowsLayout() rowsLayout {
	return rowsLayout{area: ui.Zero(), clear: ui.Zero(), rows: nil, content: 0, movedTop: 0, moved: nil}
}

// dragKind is what a press on the rows drags.
type dragKind int

const (
	dragNothing dragKind = iota
	dragShade
	dragRamp
)

// drag is a press on a swatch or a ramp's name: a click until it moves far
// enough, then a drag that moves the colour or the ramp where it is dropped.
type drag struct {
	kind   dragKind
	ramp   ids.ULID
	shade  ids.ULID
	from   image.Point
	active bool
}

func noDrag() drag {
	return drag{kind: dragNothing, ramp: ids.ULID{}, shade: ids.ULID{}, from: image.Pt(0, 0), active: false}
}

// rowsArea is where the ramps show: below the toolbar, above any open panel.
func (s *State) rowsArea() image.Rectangle {
	content := frame.Content(s.size)
	top := ui.Inset + ui.ButtonH + ui.Inset
	bottom := content.Max.Y - ui.Inset
	if panel := s.panelRect(); !panel.Empty() {
		bottom = panel.Min.Y - ui.Inset
	}

	return image.Rect(ui.Inset, top, content.Max.X-ui.Inset, max(top, bottom))
}

// layoutRows places the clear colour and each ramp's row, scrolled, and the
// moved colours a tile still names.
func (s *State) layoutRows(palette *art.Palette) {
	area := s.rowsArea()
	perRow := max(1, (area.Dx()-nameColumn-countColumn-scrollBar)/swatchStep)
	ramps := palette.Ramps()
	content := swatchStep + rowGap
	for _, ramp := range ramps {
		content += lines(len(ramp.Shades), perRow)*swatchStep + rowGap
	}
	moved := s.movedLines(palette)
	if len(moved) > 0 {
		content += ui.LineHeight * (len(moved) + 1)
	}
	s.rows = newRowsLayout()
	s.rows.area, s.rows.content = area, content
	s.scrollRows(0)
	top := area.Min.Y - s.scroll
	s.rows.clear = image.Rect(area.Min.X+nameColumn+countColumn, top, area.Min.X+nameColumn+countColumn+swatchSize,
		top+swatchSize)
	top += swatchStep + rowGap
	for _, ramp := range ramps {
		row := rampRow{
			ramp: ramp, name: image.Rect(area.Min.X, top, area.Min.X+nameColumn-ui.Gap, top+swatchSize), top: top,
			bottom: top, swatches: nil,
		}
		for index := range ramp.Shades {
			x := area.Min.X + nameColumn + countColumn + index%perRow*swatchStep
			y := top + index/perRow*swatchStep
			row.swatches = append(row.swatches, image.Rect(x, y, x+swatchSize, y+swatchSize))
		}
		top += lines(len(ramp.Shades), perRow) * swatchStep
		row.bottom = top
		top += rowGap
		s.rows.rows = append(s.rows.rows, row)
	}
	s.rows.movedTop = top
	for at := range moved {
		moved[at].top = top + ui.LineHeight*(at+1)
	}
	s.rows.moved = moved
	if s.reveal {
		s.reveal = false
		if rect, found := s.swatchRect(s.chosen.shade); found {
			s.revealRect(rect)
		}
	}
}

// lines is how many lines a ramp's colours take, at least one.
func lines(count, perRow int) int { return max(1, (count+perRow-1)/perRow) }

// scrollRows scrolls the rows by a distance, kept between the top and the
// end.
func (s *State) scrollRows(distance int) {
	s.scroll = min(max(s.scroll+distance, 0), max(s.rows.content-s.rows.area.Dy(), 0))
}

// revealRect scrolls a part of the rows into view.
func (s *State) revealRect(rect image.Rectangle) image.Rectangle {
	area, before := s.rows.area, s.scroll
	switch {
	case rect.Min.Y < area.Min.Y:
		s.scrollRows(rect.Min.Y - area.Min.Y)
	case rect.Max.Y > area.Max.Y:
		s.scrollRows(rect.Max.Y - area.Max.Y)
	}

	return rect.Sub(image.Pt(0, s.scroll-before))
}

// swatchRect is where a shade's swatch is.
func (s *State) swatchRect(shade ids.ULID) (image.Rectangle, bool) {
	for _, row := range s.rows.rows {
		for index, each := range row.ramp.Shades {
			if each.ID == shade {
				return row.swatches[index], true
			}
		}
	}

	return ui.Zero(), false
}

// rowOf returns a ramp's row.
func (s *State) rowOf(ramp ids.ULID) (rampRow, bool) {
	index := slices.IndexFunc(s.rows.rows, func(row rampRow) bool { return row.ramp.ID == ramp })
	if index < 0 {
		return rampRow{ramp: art.Ramp{ID: ids.ULID{}, Name: "", Shades: nil}, name: ui.Zero(), top: 0, bottom: 0,
			swatches: nil}, false
	}

	return s.rows.rows[index], true
}

// movedLines are the palette's shadow records that a tile still names: both
// the ramp the colour left and the colour itself. Once none does, the record
// is no longer shown.
func (s *State) movedLines(palette *art.Palette) []movedLine {
	var moved []movedLine
	for _, record := range palette.MovedShades() {
		fromRamp, rampKnown := s.uses[artplugin.RampItem(record.Ramp)]
		ofShade, shadeKnown := s.uses[artplugin.ShadeItem(record.Shade)]
		if !rampKnown || !shadeKnown || !slices.ContainsFunc(fromRamp, func(use protocol.Use) bool {
			return slices.ContainsFunc(ofShade, func(other protocol.Use) bool { return other.Document == use.Document })
		}) {
			continue
		}
		now := "a colour that is gone"
		if ramp, placed := palette.Place(record.Shade); placed {
			if name, named := palette.ShadeName(ramp, record.Shade); named {
				now = name
			}
		}
		moved = append(moved, movedLine{record: record, now: now, top: 0})
	}

	return moved
}

// movedUsers names the tiles that still name a moved colour's old place.
func (s *State) movedUsers(record art.Moved) []string {
	var names []string
	for _, use := range s.uses[artplugin.RampItem(record.Ramp)] {
		if slices.ContainsFunc(s.uses[artplugin.ShadeItem(record.Shade)], func(other protocol.Use) bool {
			return other.Document == use.Document
		}) && !slices.Contains(names, use.Name) {
			names = append(names, use.Name)
		}
	}

	return names
}

// updateRows scrolls the rows, and handles a press on a swatch or a ramp's
// name: a click chooses, Shift-click picks a run and Cmd-click adds or takes
// away one; a drag moves the colour or the ramp.
func (s *State) updateRows(input *ui.Input, ctx *frame.Context) {
	if input.Cursor.In(s.rows.area) && input.WheelY != 0 {
		s.scrollRows(-input.WheelPan().Y)
		input.WheelX, input.WheelY = 0, 0
	}
	switch {
	case input.Pressed && input.Cursor.In(s.rows.area):
		s.press(input)
	case s.drag.kind != dragNothing && input.Down && !s.drag.active && !s.ReadOnly():
		moved := input.Cursor.Sub(s.drag.from)
		s.drag.active = abs(moved.X)+abs(moved.Y) >= dragStart
	case s.drag.kind != dragNothing && input.Released:
		if s.drag.active {
			s.drop(ctx, input.Cursor)
		} else {
			s.click(ctx, input.Held)
		}
		s.drag = noDrag()
	}
}

// press starts a press on what is under the cursor.
func (s *State) press(input *ui.Input) {
	if input.Cursor.In(s.rows.clear) {
		s.choose(none())
		input.ClearPointer()

		return
	}
	for _, row := range s.rows.rows {
		if input.Cursor.In(row.name) {
			s.drag = drag{kind: dragRamp, ramp: row.ramp.ID, shade: ids.ULID{}, from: input.Cursor, active: false}
			input.Pressed = false

			return
		}
		for index, rect := range row.swatches {
			if input.Cursor.In(rect) {
				s.drag = drag{
					kind: dragShade, ramp: row.ramp.ID, shade: row.ramp.Shades[index].ID, from: input.Cursor, active: false,
				}
				input.Pressed = false

				return
			}
		}
	}
}

// click chooses what was pressed: a ramp's name chooses its first colour and
// picks the whole ramp; a swatch chooses it, Shift picks the run from the
// chosen colour to it, and Cmd adds it or takes it away.
func (s *State) click(ctx *frame.Context, held ui.Mods) {
	palette, ok := s.palette(ctx.Docs)
	if !ok {
		return
	}
	if s.drag.kind == dragRamp {
		ramp, found := palette.Ramp(s.drag.ramp)
		if !found {
			return
		}
		s.chosen = choice{ramp: ramp.ID, shade: ids.ULID{}}
		s.picked = nil
		for at, shade := range ramp.Shades {
			if at == 0 {
				s.chosen.shade = shade.ID
			}
			s.picked = append(s.picked, shade.ID)
		}

		return
	}
	clicked := choice{ramp: s.drag.ramp, shade: s.drag.shade}
	switch {
	case held.Shift && !s.chosen.shade.IsZero():
		s.picked = run(palette, s.chosen.shade, clicked.shade)
	case held.Shortcut:
		if at := slices.Index(s.picked, clicked.shade); at >= 0 {
			s.picked = slices.Delete(s.picked, at, at+1)
		} else {
			s.picked = append(s.picked, clicked.shade)
			s.chosen = clicked
		}
	default:
		s.choose(clicked)
	}
}

// run returns the shades from one to another, both included, in the
// palette's order.
func run(palette *art.Palette, from, until ids.ULID) []ids.ULID {
	var order []ids.ULID
	for _, ramp := range palette.Ramps() {
		for _, shade := range ramp.Shades {
			order = append(order, shade.ID)
		}
	}
	start, end := slices.Index(order, from), slices.Index(order, until)
	if start < 0 || end < 0 {
		return nil
	}
	if start > end {
		start, end = end, start
	}

	return slices.Clone(order[start : end+1])
}

// dropTarget is where a dragged colour would go: a ramp and the index it
// would have there, counted before the dragged colour leaves its place.
// Dropped on a ramp's name, it goes to the ramp's end.
func (s *State) dropTarget(point image.Point) (ids.ULID, int, bool) {
	for _, row := range s.rows.rows {
		if point.Y < row.top-rowGap/ui.Half || point.Y >= row.bottom+rowGap/ui.Half {
			continue
		}
		index := len(row.swatches)
		if onName(point, row) {
			return row.ramp.ID, index, true
		}
		for at, rect := range row.swatches {
			if point.Y < rect.Max.Y+rowGap && point.X < rect.Min.X+rect.Dx()/ui.Half {
				index = at

				break
			}
		}

		return row.ramp.ID, index, true
	}

	return ids.ULID{}, 0, false
}

// onName reports whether a point is on a ramp's name or count, left of its
// colours.
func onName(point image.Point, row rampRow) bool { return point.X < row.name.Max.X+countColumn }

// rampDropIndex is the place a dragged ramp would take: before the row under
// the cursor, or after the last.
func (s *State) rampDropIndex(point image.Point) int {
	for at, row := range s.rows.rows {
		if point.Y < (row.top+row.bottom)/ui.Half {
			return at
		}
	}

	return len(s.rows.rows)
}

// drop moves the dragged colour or ramp where it was dropped, at once, as
// one undo step.
func (s *State) drop(ctx *frame.Context, point image.Point) {
	if s.ReadOnly() {
		return
	}
	palette, ok := s.palette(ctx.Docs)
	if !ok {
		return
	}
	switch s.drag.kind {
	case dragRamp:
		from := slices.IndexFunc(palette.Ramps(), func(ramp art.Ramp) bool { return ramp.ID == s.drag.ramp })
		index := s.rampDropIndex(point)
		if index > from {
			index--
		}
		if from < 0 || index == from {
			return
		}
		args := artplugin.MoveRampArgs{Ramp: s.drag.ramp, Index: index}
		s.command(ctx, s.target, artplugin.MoveRampEdit, "", args, nil, nil)
	case dragShade:
		s.dropShade(ctx, palette, point)
	case dragNothing:
	}
}

// dropShade moves the dragged colour to the ramp and place it was dropped
// on. The colours after its old place renumber; tiles painted with it there
// find it through the shadow record the move leaves.
func (s *State) dropShade(ctx *frame.Context, palette *art.Palette, point image.Point) {
	ramp, index, ok := s.dropTarget(point)
	if !ok {
		return
	}
	from, _ := palette.Ramp(s.drag.ramp)
	at := slices.IndexFunc(from.Shades, func(shade art.Shade) bool { return shade.ID == s.drag.shade })
	if ramp == s.drag.ramp {
		if index > at {
			index--
		}
		if index == at {
			return
		}
	}
	s.moveShade(ctx, palette, s.drag.ramp, s.drag.shade, ramp, index)
}

// moveShade moves a colour to a place in a ramp, at once, and chooses it
// there.
func (s *State) moveShade(ctx *frame.Context, palette *art.Palette, fromRamp, shade, ramp ids.ULID, index int) {
	oldName, _ := palette.ShadeName(fromRamp, shade)
	args := artplugin.MoveShadeArgs{Shade: shade, Ramp: ramp, Index: index, Moved: nil}
	s.command(ctx, s.target, artplugin.MoveShadeEdit, "", args, func(_ protocol.Result) {
		s.choose(choice{ramp: ramp, shade: shade})
		if moved, found := s.palette(ctx.Docs); found {
			if name, named := moved.ShadeName(ramp, shade); named {
				ctx.Frame.Status("Moved " + oldName + " to " + name)
			}
		}
	}, nil)
}

// drawRows draws the clear colour and each ramp's name, count and colours,
// clipped to their area, the moved colours, a scroll bar when they do not
// fit, and what is being dragged.
func (s *State) drawRows(screen *ebiten.Image, palette *art.Palette) {
	content := frame.Content(s.size)
	ui.FillRect(screen, image.Rect(content.Min.X, s.rows.area.Min.Y-ui.Inset, content.Max.X, content.Max.Y),
		ui.ColorCanvas())
	area, clipped := screen.SubImage(s.rows.area).(*ebiten.Image)
	if !clipped {
		return
	}
	text := (swatchSize - ui.TextSize) / ui.Half
	empty := s.rows.clear
	s.theme.DrawText(area, "clear", s.rows.area.Min.X, empty.Min.Y+text, ui.ColorMuted())
	drawSwatch(area, empty, art.Shade{ID: ids.ULID{}, Color: 0, Opacity: 0, Label: ""}, true)
	s.theme.DrawText(area, "the empty colour, which every palette offers", empty.Max.X+ui.Inset, empty.Min.Y+text,
		ui.ColorMuted())
	for _, row := range s.rows.rows {
		ink := ui.ColorInk()
		if s.chosen.ramp == row.ramp.ID {
			ui.FillRound(area, image.Rect(row.name.Min.X-ui.Gap, row.top-ui.Gap, s.rows.area.Max.X-scrollBar,
				row.bottom+ui.Gap), ui.Radius, ui.ColorSelectWash())
		}
		s.theme.DrawText(area, s.theme.FitText(row.ramp.Name, row.name.Dx()), row.name.Min.X, row.top+text, ink)
		s.theme.DrawText(area, strconv.Itoa(len(row.ramp.Shades)), row.name.Max.X+ui.Gap, row.top+text, ui.ColorMuted())
		for index, rect := range row.swatches {
			shade := row.ramp.Shades[index]
			if s.drag.active && s.drag.shade == shade.ID {
				ui.StrokeRound(area, rect, ui.Radius, pickedWidth, ui.ColorSubdued())

				continue
			}
			drawSwatch(area, rect, shade, false)
			s.outline(area, rect, shade.ID)
			if shade.Label != "" {
				dot := image.Rect(rect.Min.X+rect.Dx()/ui.Half-labelDot/ui.Half, rect.Max.Y+ui.Half,
					rect.Min.X+rect.Dx()/ui.Half+labelDot/ui.Half, rect.Max.Y+ui.Half+labelDot)
				ui.FillRound(area, dot, labelDot/ui.Half, ui.ColorInk())
			}
		}
	}
	s.drawMoved(area)
	if view := s.rows.area.Dy(); s.rows.content > view {
		top := s.rows.area.Min.Y + s.scroll*view/s.rows.content
		bar := image.Rect(s.rows.area.Max.X-scrollBar, top, s.rows.area.Max.X, top+view*view/s.rows.content)
		ui.FillRound(screen, bar, scrollBar/ui.Half, ui.ColorSubdued())
	}
	s.drawDrag(screen, palette)
}

// outline marks the chosen colour and the others picked with it.
func (s *State) outline(screen *ebiten.Image, rect image.Rectangle, shade ids.ULID) {
	switch {
	case s.chosen.shade == shade:
		ui.StrokeRect(screen, rect.Inset(-chosenWidth-1), chosenWidth, ui.ColorSelect())
	case s.picks(shade):
		ui.StrokeRect(screen, rect.Inset(-chosenWidth), pickedWidth, ui.ColorSelect())
	}
}

// drawMoved lists the shadow records tiles still name.
func (s *State) drawMoved(screen *ebiten.Image) {
	if len(s.rows.moved) == 0 {
		return
	}
	left := s.rows.area.Min.X
	s.theme.Heading(screen, "MOVED COLOURS", left, s.rows.movedTop)
	for _, line := range s.rows.moved {
		text := line.record.Name + " → " + line.now
		s.theme.DrawText(screen, text, left, line.top, ui.ColorInk())
		users := "still named by " + strings.Join(s.movedUsers(line.record), ", ") + ", until repainted"
		s.theme.DrawText(screen, users, left+nameColumn+countColumn+ui.Half*nameColumn, line.top, ui.ColorMuted())
	}
}

// drawDrag draws a dragged colour at the cursor and where it would go, or
// where a dragged ramp would go.
func (s *State) drawDrag(screen *ebiten.Image, palette *art.Palette) {
	if !s.drag.active {
		return
	}
	switch s.drag.kind {
	case dragShade:
		s.drawShadeDrop(screen)
		if shade, _, found := palette.Lookup(s.drag.ramp, s.drag.shade); found {
			ghost := image.Rectangle{Min: s.cursor.Add(image.Pt(ui.Gap, ui.Gap)), Max: s.cursor.Add(image.Pt(
				ui.Gap+swatchSize, ui.Gap+swatchSize))}
			drawSwatch(screen, ghost, shade, false)
			ui.StrokeRect(screen, ghost, chosenWidth, ui.ColorSelect())
		}
	case dragRamp:
		index := s.rampDropIndex(s.cursor)
		down := s.rows.movedTop - rowGap/ui.Half
		if index < len(s.rows.rows) {
			down = s.rows.rows[index].top - rowGap/ui.Half
		}
		ui.FillRect(screen, image.Rect(s.rows.area.Min.X, down-1, s.rows.area.Max.X, down-1+dropWidth), ui.ColorSelect())
	case dragNothing:
	}
}

// drawShadeDrop marks where a dragged colour would go: a bar at its place,
// and the ramp's name outlined when it is dropped there.
func (s *State) drawShadeDrop(screen *ebiten.Image) {
	ramp, index, ok := s.dropTarget(s.cursor)
	row, found := s.rowOf(ramp)
	if !ok || !found {
		return
	}
	if onName(s.cursor, row) {
		ui.StrokeRect(screen, row.name, chosenWidth, ui.ColorSelect())
	}
	across, down := row.name.Max.X+countColumn+ui.Gap, row.top
	if index < len(row.swatches) {
		across, down = row.swatches[index].Min.X-ui.Half, row.swatches[index].Min.Y
	} else if count := len(row.swatches); count > 0 {
		across, down = row.swatches[count-1].Max.X+ui.Half, row.swatches[count-1].Min.Y
	}
	ui.FillRect(screen, image.Rect(across-1, down-ui.Gap, across-1+dropWidth, down+swatchSize+ui.Gap), ui.ColorSelect())
}

// drawSwatch draws one colour: see-through colours over a checkerboard, and
// clear as the checkerboard alone.
func drawSwatch(screen *ebiten.Image, rect image.Rectangle, shade art.Shade, empty bool) {
	half := rect.Dx() / ui.Half
	ui.FillRect(screen, rect, ui.ColorBackground())
	ui.FillRect(screen, image.Rect(rect.Min.X+half, rect.Min.Y, rect.Max.X, rect.Min.Y+half), ui.Hex(hexChecker))
	ui.FillRect(screen, image.Rect(rect.Min.X, rect.Min.Y+half, rect.Min.X+half, rect.Max.Y), ui.Hex(hexChecker))
	if !empty {
		ui.FillRect(screen, rect, shade.Color.NRGBA(shade.Opacity))
	}
	ui.StrokeRect(screen, rect, ui.Hairline, ui.ColorRule())
}

// hexChecker is the checkerboard's darker square.
const hexChecker = 0xe4e5e1

// drawTip names the colour under the cursor, with its label and value.
func (s *State) drawTip(screen *ebiten.Image, palette *art.Palette) {
	if !s.cursor.In(s.rows.area) || s.drag.active {
		return
	}
	for _, row := range s.rows.rows {
		for index, rect := range row.swatches {
			if !s.cursor.In(rect) {
				continue
			}
			shade := row.ramp.Shades[index]
			name, _ := palette.ShadeName(row.ramp.ID, shade.ID)
			text := name
			if shade.Label != "" {
				text += " · " + shade.Label
			}
			text += " · " + shade.Color.String() + " · " + strconv.Itoa(shade.Opacity) + "%"
			width := s.theme.TextWidth(text) + ui.Half*tipPad
			corner := image.Pt(min(s.cursor.X+tipOffset, s.size.X-width-ui.Gap), s.cursor.Y+tipOffset)
			tip := image.Rectangle{Min: corner, Max: corner.Add(image.Pt(width, ui.LineHeight+tipPad))}
			ui.FillRound(screen, tip, ui.Radius, ui.ColorInk())
			s.theme.DrawText(screen, text, tip.Min.X+tipPad, tip.Min.Y+tipPad/ui.Half, ui.ColorBackground())

			return
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}
