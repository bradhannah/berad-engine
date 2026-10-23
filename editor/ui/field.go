package ui

import (
	"image"
	"math"

	berad "github.com/bradhannah/berad-sdk/modules/berad-text"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/rivo/uniseg"

	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

const (
	fieldPad    = 6
	caretWidth  = 1.5
	paintInk    = "ink"
	paintMuted  = "muted"
	unsavedMark = 6
)

// TextField draws a textedit.Field and turns input into edits. Text is laid
// out with berad-text, which supplies wrapping, caret positions and selection
// rectangles.
type TextField struct {
	ID          string
	Rect        image.Rectangle
	Field       *textedit.Field
	Placeholder string
	Multiline   bool

	theme       *Theme
	focused     bool
	dragging    bool
	composition string
	scroll      image.Point
	cache       layoutCache
}

type layoutCache struct {
	text   string
	width  float64
	layout *berad.Layout
}

// NewTextField returns a field widget.
func NewTextField(id string, field *textedit.Field, multiline bool, theme *Theme) *TextField {
	return &TextField{
		ID: id, Rect: image.Rect(0, 0, 0, 0), Field: field, Placeholder: "", Multiline: multiline,
		theme: theme, focused: false, dragging: false, composition: "", scroll: image.Pt(0, 0),
		cache: layoutCache{text: "", width: -1, layout: nil},
	}
}

// Focused reports whether the field has keyboard focus.
func (w *TextField) Focused() bool { return w.focused }

// SetFocus gives or takes keyboard focus. Taking it keeps an unsaved draft.
func (w *TextField) SetFocus(focus bool) {
	if focus == w.focused {
		return
	}
	w.focused = focus
	w.composition = ""
	w.dragging = false
	if focus {
		w.Field.Focus()
	} else {
		w.Field.Blur()
	}
}

// Center returns the middle of the field, for input scripts.
func (w *TextField) Center() image.Point {
	return image.Pt(w.Rect.Min.X+w.Rect.Dx()/2, w.Rect.Min.Y+w.Rect.Dy()/2)
}

// Update handles a frame of input. It reports the outcome of any key press
// the field used, and whether the main button was pressed inside it. Presses
// it does not use stay in the input for the window.
func (w *TextField) Update(input *Input, clipboard textedit.Clipboard) (textedit.Outcome, bool) {
	clicked := input.Pressed && input.Cursor.In(w.Rect)
	if clicked {
		w.SetFocus(true)
		w.placeCaret(input)
		input.ClearPointer()
	}
	if w.dragging && input.Down {
		w.Field.Buffer().SetCaret(w.hit(input.Cursor), true)
	}
	if !input.Down {
		w.dragging = false
	}
	if !w.focused {
		return textedit.OutcomeNone, clicked
	}
	w.composition = input.Composition
	if input.Replace != nil {
		w.Field.ReplaceAround(input.Replace.Before, input.Replace.After, input.Replace.Text, input.Replace.Caret)
		input.Replace = nil
	}
	if input.Text != "" {
		w.Field.Type(input.Text)
		input.Text = ""
	}
	outcome := textedit.OutcomeNone
	remaining := input.Presses[:0]
	for _, press := range input.Presses {
		key, ok := fieldKey(press.Key)
		if !ok || outcome != textedit.OutcomeNone {
			remaining = append(remaining, press)

			continue
		}
		result := w.Field.Key(textedit.Press{Key: key, Mods: press.Mods}, w.lines(), clipboard)
		if result == textedit.OutcomePass {
			remaining = append(remaining, press)

			continue
		}
		outcome = result
	}
	input.Presses = remaining
	w.keepCaretVisible()

	return outcome, clicked
}

func (w *TextField) placeCaret(input *Input) {
	buffer := w.Field.Buffer()
	offset := w.hit(input.Cursor)
	if input.DoubleClick {
		buffer.SelectWord(offset)

		return
	}
	buffer.SetCaret(offset, input.Held.Shift)
	w.dragging = true
}

// fieldKey maps a window key to a field key.
func fieldKey(key Key) (textedit.Key, bool) {
	keys := map[Key]textedit.Key{
		KeyEnter: textedit.KeyEnter, KeyEscape: textedit.KeyEscape, KeyBackspace: textedit.KeyBackspace,
		KeyDelete: textedit.KeyDelete, KeyLeft: textedit.KeyLeft, KeyRight: textedit.KeyRight,
		KeyUp: textedit.KeyUp, KeyDown: textedit.KeyDown, KeyHome: textedit.KeyHome, KeyEnd: textedit.KeyEnd,
		KeyA: textedit.KeyA, KeyC: textedit.KeyC, KeyX: textedit.KeyX, KeyV: textedit.KeyV,
		KeyZ: textedit.KeyZ, KeyJ: textedit.KeyJ, KeyTab: textedit.KeyTab,
	}
	mapped, ok := keys[key]

	return mapped, ok
}

// Session returns what an input method needs to start: the caret's screen
// rectangle and the text around the selection on its line.
func (w *TextField) Session() (image.Rectangle, string, string) {
	buffer := w.Field.Buffer()
	if buffer == nil {
		return image.Rect(0, 0, 0, 0), "", ""
	}
	start, end := buffer.Selection()
	text := buffer.Text()
	before := text[buffer.LineStart(start):start]
	after := text[end:buffer.LineEnd(end)]

	return w.caretRect(), before, after
}

func (w *TextField) textWidth() float64 {
	if !w.Multiline {
		return 0
	}

	return float64(max(w.Rect.Dx()-2*fieldPad, 1))
}

// layout returns the layout of the field's current text.
func (w *TextField) layout() *berad.Layout {
	return w.layoutOf(w.Field.Value())
}

func (w *TextField) layoutOf(value string) *berad.Layout {
	width := w.textWidth()
	if w.cache.layout != nil && w.cache.text == value && w.cache.width == width {
		return w.cache.layout
	}
	content, err := berad.NewContent([]berad.Span{{Text: value, Font: w.theme.Font, Paint: paintInk}})
	if err != nil {
		return nil
	}
	var options berad.LayoutOptions
	options.Width = width
	options.DefaultFont = w.theme.Font
	layout, err := berad.NewLayout(content, options)
	if err != nil {
		return nil
	}
	w.cache = layoutCache{text: value, width: width, layout: layout}

	return layout
}

// origin is where the text's top-left corner sits on screen.
func (w *TextField) origin() image.Point {
	return image.Pt(w.Rect.Min.X+fieldPad-w.scroll.X, w.Rect.Min.Y+fieldPad-w.scroll.Y)
}

func (w *TextField) caretRect() image.Rectangle {
	layout := w.layout()
	buffer := w.Field.Buffer()
	if layout == nil || buffer == nil {
		return image.Rect(w.Rect.Min.X, w.Rect.Min.Y, w.Rect.Min.X+1, w.Rect.Min.Y+LineHeight)
	}
	caret, err := layout.CaretAt(buffer.Caret(), false)
	if err != nil {
		return image.Rect(w.Rect.Min.X, w.Rect.Min.Y, w.Rect.Min.X+1, w.Rect.Min.Y+LineHeight)
	}
	origin := w.origin()
	x := origin.X + int(math.Round(caret.Position.X))
	y := origin.Y + int(math.Round(caret.Position.Y))

	return image.Rect(x, y, x+1, y+int(math.Ceil(caret.Height)))
}

func (w *TextField) keepCaretVisible() {
	inner := w.Rect.Inset(fieldPad)
	caret := w.caretRect()
	switch {
	case caret.Min.X < inner.Min.X:
		w.scroll.X -= inner.Min.X - caret.Min.X
	case caret.Max.X > inner.Max.X:
		w.scroll.X += caret.Max.X - inner.Max.X
	}
	switch {
	case caret.Min.Y < inner.Min.Y:
		w.scroll.Y -= inner.Min.Y - caret.Min.Y
	case caret.Max.Y > inner.Max.Y:
		w.scroll.Y += caret.Max.Y - inner.Max.Y
	}
	w.scroll.X = max(w.scroll.X, 0)
	w.scroll.Y = max(w.scroll.Y, 0)
}

// hit returns the text offset nearest a screen point.
func (w *TextField) hit(point image.Point) int {
	layout := w.layout()
	if layout == nil {
		return 0
	}
	origin := w.origin()
	localX, localY := float64(point.X-origin.X), float64(point.Y-origin.Y)
	lines := layout.Lines()
	if len(lines) == 0 {
		return 0
	}
	index := len(lines) - 1
	for candidate, line := range lines {
		if localY < line.Bounds.Max.Y {
			index = candidate

			break
		}
	}

	return w.nearestInLine(layout, lines, index, localX)
}

// nearestInLine returns the offset in a visual line closest to x.
func (w *TextField) nearestInLine(layout *berad.Layout, lines []berad.Line, index int, targetX float64) int {
	line := lines[index]
	text := layout.Content().Text()
	best, bestDistance := line.Start, math.Inf(1)
	softWrap := line.Next == line.End && index+1 < len(lines)
	offset := line.Start
	state := -1
	for {
		upstream := offset == line.End && softWrap
		if caret, err := layout.CaretAt(offset, upstream); err == nil {
			if distance := math.Abs(caret.Position.X - targetX); distance < bestDistance {
				best, bestDistance = offset, distance
			}
		}
		if offset >= line.End {
			return best
		}
		cluster, _, _, newState := uniseg.FirstGraphemeClusterInString(text[offset:line.End], state)
		offset += len(cluster)
		state = newState
	}
}

// fieldLines answers layout questions for the field.
type fieldLines struct {
	widget *TextField
}

func (w *TextField) lines() textedit.Lines { //nolint:ireturn // The field needs the interface.
	return fieldLines{widget: w}
}

// LineBounds returns the visual line holding an offset.
func (l fieldLines) LineBounds(offset int) (int, int) {
	layout := l.widget.layout()
	if layout == nil {
		return 0, 0
	}
	lines := layout.Lines()
	for index, line := range lines {
		if offset <= line.End || index == len(lines)-1 || offset < lines[index+1].Start {
			return line.Start, line.End
		}
	}

	return 0, 0
}

// Vertical returns the offset delta visual lines away, nearest column x.
func (l fieldLines) Vertical(offset, delta int, column *float64) int {
	layout := l.widget.layout()
	if layout == nil {
		return offset
	}
	lines := layout.Lines()
	current := 0
	for index, line := range lines {
		if offset >= line.Start && offset <= line.Next {
			current = index

			break
		}
	}
	if *column < 0 {
		if caret, err := layout.CaretAt(offset, false); err == nil {
			*column = caret.Position.X
		}
	}
	target := current + delta
	switch {
	case target < 0:
		return 0
	case target >= len(lines):
		return len(layout.Content().Text())
	default:
		return l.widget.nearestInLine(layout, lines, target, *column)
	}
}

// Draw renders the field: its box, selection, text, unfinished input method
// text, caret and an unsaved mark.
func (w *TextField) Draw(screen *ebiten.Image) {
	FillRect(screen, w.Rect, ColorBackground())
	edge, width := ColorRule(), float32(Hairline)
	if w.focused {
		edge, width = ColorSelect(), Outline
	}
	StrokeRect(screen, w.Rect, width, edge)
	area, ok := screen.SubImage(w.Rect.Inset(Hairline)).(*ebiten.Image)
	if !ok {
		return
	}
	value := w.Field.Value()
	if value == "" && !w.focused && w.composition == "" {
		w.theme.DrawText(area, w.Placeholder, w.Rect.Min.X+fieldPad, w.Rect.Min.Y+fieldPad, ColorSubdued())
	}
	w.drawText(area, value)
	if w.Field.Dirty() {
		mark := image.Rect(w.Rect.Max.X-unsavedMark-Gap, w.Rect.Min.Y+Gap, w.Rect.Max.X-Gap, w.Rect.Min.Y+Gap+unsavedMark)
		FillRect(screen, mark, ColorWarn())
	}
}

// drawText draws the text with the selection under it and the caret, or the
// input method's unfinished text underlined at the caret.
func (w *TextField) drawText(area *ebiten.Image, value string) {
	buffer := w.Field.Buffer()
	editing := w.focused && buffer != nil
	shown, compositionStart := value, -1
	if editing && w.composition != "" {
		start, end := buffer.Selection()
		shown = value[:start] + w.composition + value[end:]
		compositionStart = start
	}
	layout := w.layoutOf(shown)
	if layout == nil {
		return
	}
	origin := w.origin()
	if editing && compositionStart < 0 {
		w.drawSelection(area, layout, origin, buffer)
	}
	var options berad.DrawOptions
	options.GeoM.Translate(float64(origin.X), float64(origin.Y))
	options.Filter = ebiten.FilterNearest
	palette := berad.Palette{paintInk: ColorInk(), paintMuted: ColorMuted()}
	if err := w.theme.Text.Draw(area, layout, palette, options); err != nil {
		return
	}
	switch {
	case compositionStart >= 0:
		w.underline(area, layout, origin, compositionStart, compositionStart+len(w.composition))
	case editing:
		w.drawCaret(area, layout, origin, buffer)
	}
}

// drawSelection washes the selected text.
func (w *TextField) drawSelection(
	area *ebiten.Image, layout *berad.Layout, origin image.Point, buffer *textedit.Buffer,
) {
	start, end := buffer.Selection()
	coverage, err := layout.RangeRects(start, end)
	if err != nil {
		return
	}
	for _, rect := range coverage.Rects {
		FillRect(area, toScreen(rect, origin), ColorSelectWash())
	}
}

// drawCaret draws the caret when nothing is selected.
func (w *TextField) drawCaret(area *ebiten.Image, layout *berad.Layout, origin image.Point, buffer *textedit.Buffer) {
	if buffer.HasSelection() {
		return
	}
	position, err := layout.CaretAt(buffer.Caret(), false)
	if err != nil {
		return
	}
	x := origin.X + int(position.Position.X)
	y := origin.Y + int(position.Position.Y)
	Line(area, image.Pt(x, y), image.Pt(x, y+int(position.Height)), caretWidth, ColorInk())
}

func (w *TextField) underline(area *ebiten.Image, layout *berad.Layout, origin image.Point, start, end int) {
	coverage, err := layout.RangeRects(start, end)
	if err != nil {
		return
	}
	for _, rect := range coverage.Rects {
		box := toScreen(rect, origin)
		Line(area, image.Pt(box.Min.X, box.Max.Y-1), image.Pt(box.Max.X, box.Max.Y-1), Outline, ColorSelect())
	}
}

func toScreen(rect berad.Rect, origin image.Point) image.Rectangle {
	return image.Rect(
		origin.X+int(math.Floor(rect.Min.X)), origin.Y+int(math.Floor(rect.Min.Y)),
		origin.X+int(math.Ceil(rect.Max.X)), origin.Y+int(math.Ceil(rect.Max.Y)),
	)
}
