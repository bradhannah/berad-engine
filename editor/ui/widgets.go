package ui

import (
	"image"
	"image/color"

	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Button is a labelled push button.
type Button struct {
	ID       string
	Label    string
	Rect     image.Rectangle
	Disabled bool
	// Primary buttons are drawn filled in blue, for the default choice in a
	// dialog. On buttons are drawn filled in ink, for the chosen one of a
	// set.
	Primary bool
	On      bool
	// Danger buttons are drawn in red, for actions such as Delete. Caret
	// buttons open a menu and show a small triangle after their label.
	Danger bool
	Caret  bool
	// Icon, when set, is drawn before the label, as a place's ring.
	Icon Icon
	// Quiet buttons are a panel's secondary actions (increment 66): page
	// tone with a hairline, and desk tone under the pointer, so the panel's
	// one main button and its few plain ones stand out. A quiet button that
	// is On is washed blue, as a toggle that is set.
	Quiet bool
	// Key is the key that presses the button, shown as a small cap after
	// its label.
	Key   string
	hover bool
}

// NewButton returns a plain button, placed later by setting Rect.
func NewButton(id, label string) *Button {
	return &Button{
		ID: id, Label: label, Rect: image.Rect(0, 0, 0, 0), Disabled: false, Primary: false, On: false, Danger: false,
		Caret: false, Icon: IconNone, Quiet: false, Key: "", hover: false,
	}
}

// NewQuietButton returns a quiet button, for a panel's secondary actions.
func NewQuietButton(id, label string) *Button {
	button := NewButton(id, label)
	button.Quiet = true

	return button
}

// Update reports whether the button was clicked this frame.
func (b *Button) Update(input *Input) bool {
	b.hover = input.Cursor.In(b.Rect)
	if b.Disabled || !b.hover || !input.Pressed {
		return false
	}
	input.ClearPointer()

	return true
}

// iconWidth is the room the button's icon takes before its label: none
// without an icon, and no gap after an icon with no label beside it, which
// sits in the button's middle.
func (b *Button) iconWidth() int {
	switch {
	case b.Icon == IconNone:
		return 0
	case b.Label == "":
		return IconSize
	default:
		return IconSize + Gap
	}
}

// Draw renders the button: a rounded box with an ink outline. A button laid
// out nowhere, as a hidden one is, is not drawn.
func (b *Button) Draw(screen *ebiten.Image, theme *Theme) {
	if b.Rect.Empty() {
		return
	}
	fill, ink, edge := b.look()
	stroke := Border
	if b.Quiet && !b.Primary {
		stroke = Hairline
	}
	FillRound(screen, b.Rect, Radius, fill)
	StrokeRound(screen, b.Rect, Radius, stroke, edge)
	width := theme.TextWidth(b.Label) + b.iconWidth()
	if b.Caret {
		width += triangle*Half + labelGap
	}
	if b.Key != "" {
		width += labelGap + CapWidth(theme, b.Key)
	}
	left := b.Rect.Min.X + (b.Rect.Dx()-width)/Half
	if b.Icon != IconNone {
		iconInk := IconColor(b.Icon)
		if b.Disabled || b.Primary || b.On {
			iconInk = ink
		}
		DrawIconInk(screen, b.Icon, image.Pt(left, b.Rect.Min.Y+(b.Rect.Dy()-IconSize)/Half), iconInk)
		left += b.iconWidth()
	}
	theme.DrawText(screen, b.Label, left, b.Rect.Min.Y+(b.Rect.Dy()-TextSize)/Half-1, ink)
	if b.Key != "" {
		DrawCap(screen, theme, b.Key, left+width, b.Rect.Min.Y+b.Rect.Dy()/Half, b.Primary)

		return
	}
	if b.Caret {
		caret := image.Rect(left+width-triangle*Half, b.Rect.Min.Y, left+width, b.Rect.Max.Y)
		DrawTriangle(screen, caret, true, ink)
	}
}

// FitButton is a button's place cut down to the width of its label, from
// its left edge and no wider than it was: for an action that should not
// span its panel, such as Delete at an inspector's foot (increment 66).
func FitButton(theme *Theme, button *Button) image.Rectangle {
	rect := button.Rect
	if rect.Empty() {
		return rect
	}
	rect.Max.X = min(rect.Max.X, rect.Min.X+theme.TextWidth(button.Label)+Half*Inset)

	return rect
}

// look is the button's fill, ink and edge for its state.
func (b *Button) look() (color.RGBA, color.RGBA, color.RGBA) {
	fill, ink, edge := ColorField(), ColorInk(), ColorInk()
	if b.Quiet {
		fill, edge = ColorPage(), ColorRule()
	}
	switch {
	case b.Disabled && b.Quiet:
		ink, edge = ColorSubdued(), ColorRuleSoft()
	case b.Disabled:
		ink, edge = ColorSubdued(), ColorRule()
	case b.Primary:
		fill, ink, edge = ColorSelect(), ColorReverse(), ColorSelect()
	case b.On && b.Quiet:
		fill, edge = ColorSelectWash(), ColorSelect()
	case b.On:
		fill, ink = ColorInk(), ColorReverse()
	case b.Danger:
		ink, edge = ColorWarn(), ColorWarn()
		if b.hover {
			fill = ColorHover()
		}
	case b.hover:
		fill = ColorHover()
	}

	return fill, ink, edge
}

// A small cap is a key or a count on a row or a button, a smaller cousin of
// the key sheet's caps: its height, the room either side of its text, and
// the least it is wide.
const (
	smallCapHeight = 16
	smallCapPad    = 4
	smallCapMin    = 16
)

// CapWidth is how wide a small cap of some text is.
func CapWidth(theme *Theme, text string) int {
	return max(smallCapMin, theme.MonoWidth(text)+Half*smallCapPad)
}

// DrawCap draws a key or a count as a small chip in the engine's own type,
// its right edge and its middle at a point: desk tone with muted text, or,
// on something filled with the selection's blue, a deeper blue with
// reversed text (increment 66).
func DrawCap(screen *ebiten.Image, theme *Theme, text string, right, middle int, onFilled bool) {
	width := CapWidth(theme, text)
	chip := image.Rect(right-width, middle-smallCapHeight/Half, right, middle+smallCapHeight/Half)
	fill, ink := ColorDesk(), ColorMuted()
	if onFilled {
		fill, ink = ColorSelectDeep(), ColorReverse()
	}
	FillRound(screen, chip, capRadius, fill)
	theme.DrawMono(screen, text, chip.Min.X+(width-theme.MonoWidth(text))/Half, middle-MonoSize/Half-1, ink)
}

// Toggle is a labelled on/off box.
type Toggle struct {
	ID    string
	Label string
	Rect  image.Rectangle
	On    bool
}

// Update flips the toggle when clicked and reports whether it changed.
func (t *Toggle) Update(input *Input) bool {
	if !input.Pressed || !input.Cursor.In(t.Rect) {
		return false
	}
	input.ClearPointer()
	t.On = !t.On

	return true
}

// Draw renders the toggle.
func (t *Toggle) Draw(screen *ebiten.Image, theme *Theme) {
	const box = 14
	boxTop := t.Rect.Min.Y + (t.Rect.Dy()-box)/Half
	square := image.Rect(t.Rect.Min.X, boxTop, t.Rect.Min.X+box, boxTop+box)
	FillRound(screen, square, toggleInset, ColorField())
	StrokeRound(screen, square, toggleInset, Border, ColorInk())
	if t.On {
		FillRound(screen, square.Inset(toggleInset), Half, ColorInk())
	}
	theme.DrawText(screen, t.Label, square.Max.X+labelGap, t.Rect.Min.Y+(t.Rect.Dy()-TextSize)/Half-1, ColorInk())
}

// A selected or hovered row is a rounded box just inside the row.
const (
	rowInset     = 1
	rowRadius    = 5
	swatchRadius = 3
)

// highlight is the box drawn behind a selected or hovered row, clear of the
// list's edges so its outline is never clipped.
func highlight(row image.Rectangle) image.Rectangle {
	return image.Rect(row.Min.X+Gap, row.Min.Y+rowInset, row.Max.X-Gap, row.Max.Y-rowInset)
}

// Row is one row of a List. Plain rows show a label, an optional swatch and
// a subdued detail after it. Rows with an icon, a guide or a toggle are
// drawn as a tree: a disclosure column, the icon in its type's colour, the
// label, and the detail at the right edge.
type Row struct {
	ID    string
	Label string
	// Detail is shown subdued after the label, or at the right of a tree row.
	Detail string
	Indent int
	// Swatch, when set, is drawn as a small colour square before the label.
	Swatch *color.RGBA
	// Heading rows cannot be selected.
	Heading bool
	// Marked rows are drawn with a mark, for example maps that show the thing
	// being picked.
	Marked bool
	// Icon is drawn before the label in its type's colour.
	Icon Icon
	// Strong rows have a semi-bold label, for things that hold others.
	Strong bool
	// Guide draws the tree line from the row's parent to it.
	Guide Guide
	// Toggle draws a disclosure triangle for a row whose children can be
	// hidden.
	Toggle Disclosure
	// Section rows are group headings: large, with a divider above, a toggle
	// and, when Add is set, a [+] at the right edge. Like headings they
	// cannot be selected; a click toggles them.
	Section bool
	Add     bool
	// Aside is drawn subdued just after a tree row's label, such as its
	// tags.
	Aside string
	// Tree draws the row as part of a tree, lined up with the rows that have
	// an icon, a guide or a triangle, even when it has none of them.
	Tree bool
	// Muted rows are drawn grey: still listed, but not the ones to choose,
	// such as a place an NPC cannot go to.
	Muted bool
	// Ticked rows are chosen among several at once, such as a facet's
	// values: washed blue, with a tick where a triangle would be.
	Ticked bool
	// Cells are a table row's other columns, drawn at the list's Columns.
	Cells []string
	// Key is the key that chooses the row, or how many it holds, drawn as a
	// cap at the row's right edge. Glyph is a plain mark before a plain
	// row's label, in the label's own ink, where Icon is a document's.
	Key   string
	Glyph Icon
}

// ink is the colour a row's label is drawn in: muted for a muted row.
func (r Row) ink() color.RGBA {
	if r.Muted {
		return ColorMuted()
	}

	return ColorInk()
}

// NewRow returns a plain selectable row; set its other fields as needed.
func NewRow(id, label string) Row {
	return Row{
		ID: id, Label: label, Detail: "", Indent: 0, Swatch: nil, Heading: false, Marked: false, Icon: IconNone,
		Strong: false, Guide: GuideNone, Toggle: DisclosureNone, Section: false, Add: false, Aside: "", Tree: false,
		Muted: false, Ticked: false, Cells: nil, Key: "", Glyph: IconNone,
	}
}

// NewHeading returns a small heading row, which cannot be selected.
func NewHeading(id, label string) Row {
	row := NewRow(id, label)
	row.Heading = true

	return row
}

// Guide is the tree line drawn to a child row.
type Guide int

// Tree lines: none, a child with more below it, or the last child.
const (
	GuideNone Guide = iota
	GuideMiddle
	GuideLast
)

// Disclosure is the state of a row's disclosure triangle.
type Disclosure int

// Disclosure states: no triangle, children shown, or children hidden.
const (
	DisclosureNone Disclosure = iota
	DisclosureOpen
	DisclosureClosed
)

// Tree row geometry, in screen pixels.
const (
	treeToggle   = 16
	treeIndent   = 22
	treeLabelGap = 6
	triangle     = 4
	addSize      = 20
	sectionCount = 6
)

// List is a scrolling list of rows with one selected row. With CanDrop
// set, a row can be dragged onto another row CanDrop accepts. While one is
// dragged, rows Faded names are drawn faded, and DragLabel, when set, says
// what is carried beside the pointer, over a row or none.
type List struct {
	ID       string
	Rect     image.Rectangle
	Rows     []Row
	Selected string
	// DetailAt, when above zero, puts tree rows' details in a column that
	// far from the list's left, close to the names, rather than at the
	// right edge; a name too long for it pushes its detail along.
	DetailAt int
	// Columns, when set, are where each of a row's Cells starts, that far
	// from the list's left; a tree row's label is cut short before the
	// first.
	Columns []int
	// InHand draws the selected row filled with the selection's blue, its
	// text reversed, for the one thing a panel is about: the tool in hand,
	// the place the author is in. Without it the selected row is washed
	// blue with a thin ring, as one choice among others (increment 66).
	InHand bool
	// Caps draws a tree row's detail as a cap at its right edge.
	Caps bool
	// Grid, when above zero, lays the rows out as chips in that many
	// columns, each a swatch and a name on a box of its own, for what a
	// tool paints with.
	Grid      int
	CanDrop   func(dragged, target string) bool
	Faded     func(dragged, row string) bool
	DragLabel func(dragged, over string) string
	// DropElsewhere, when set, finds where a row dragged out of the list
	// would drop, such as a row of another list beside it, or "".
	DropElsewhere func(dragged string, at image.Point) string
	scroll        int
	hover         string
	hoverAdd      bool
	drag          listDrag
}

// NewList returns an empty list with a row selected, or none for "".
func NewList(id, selected string) *List {
	return &List{
		ID: id, Rect: Zero(), Rows: nil, Selected: selected, DetailAt: 0, Columns: nil, InHand: false, Caps: false,
		Grid: 0, CanDrop: nil, Faded: nil,
		DragLabel: nil, DropElsewhere: nil,
		scroll: 0,
		hover:  "", hoverAdd: false, drag: noDrag(),
	}
}

// listDrag follows a row pressed in a list that allows dragging: it becomes
// a drag once the pointer moves far enough, and over is the row it would
// drop on.
type listDrag struct {
	row      string
	from     image.Point
	at       image.Point
	dragging bool
	over     string
	resting  int
}

// dragDistance is how far the pointer moves, in screen pixels, before a
// press on a row becomes a drag; restFrames is how many frames a drag stays
// over one row, about half a second, before it counts as resting there;
// fadeAlpha is how much a faded row is washed out.
const (
	dragDistance = 5
	restFrames   = 30
	fadeAlpha    = 0xa0
)

// ListResult reports what happened to a list this frame.
type ListResult struct {
	// Changed is true when the selection changed.
	Changed bool
	// Activated is true when a row was double-clicked or Enter was pressed
	// while the list had the pointer over it.
	Activated bool
	// Toggled is the row whose disclosure triangle, or whose section
	// heading, was clicked; Added is the section whose [+] was clicked.
	Toggled string
	Added   string
	// Dragged is the row dropped on the row Dropped; Rested is the row a
	// drag has just rested on, as a closed group to open.
	Dragged string
	Dropped string
	Rested  string
	// Context is true when the list was right-clicked, and ContextRow is the
	// row under the pointer, or "" below the rows. The list leaves choosing
	// what a right-click selects to its window.
	Context    bool
	ContextRow string
}

// RowRect returns the screen rectangle of a row, which may lie outside the
// list when scrolled away.
func (l *List) RowRect(index int) image.Rectangle {
	if l.Grid > 0 {
		return l.chipRect(index)
	}
	top := l.Rect.Min.Y + index*RowHeight - l.scroll

	return image.Rect(l.Rect.Min.X, top, l.Rect.Max.X, top+RowHeight)
}

// Locate returns the centre of a row, for input scripts.
func (l *List) Locate(id string) (image.Point, bool) {
	for index, row := range l.Rows {
		if row.ID == id {
			return Center(l.RowRect(index)), true
		}
	}

	return image.Pt(0, 0), false
}

// LocateToggle returns the centre of a row's disclosure triangle, and
// LocateAdd that of a section's [+], for input scripts.
func (l *List) LocateToggle(id string) (image.Point, bool) {
	for index, row := range l.Rows {
		if row.ID == id && row.Toggle != DisclosureNone {
			return Center(toggleRect(l.RowRect(index), row)), true
		}
	}

	return image.Pt(0, 0), false
}

// LocateAdd returns the centre of a section's [+].
func (l *List) LocateAdd(id string) (image.Point, bool) {
	for index, row := range l.Rows {
		if row.ID == id && row.Add {
			return Center(addRect(l.RowRect(index))), true
		}
	}

	return image.Pt(0, 0), false
}

// Update handles scrolling, clicks, right-clicks and dragging a row onto
// another.
func (l *List) Update(input *Input) ListResult {
	result := ListResult{
		Changed: false, Activated: false, Toggled: "", Added: "", Dragged: "", Dropped: "", Rested: "", Context: false,
		ContextRow: "",
	}
	inside := input.Cursor.In(l.Rect)
	if inside && input.WheelY != 0 {
		l.scroll -= int(input.WheelY * RowHeight)
		input.WheelY = 0
	}
	l.scroll = min(max(l.scroll, 0), max(l.Height()-l.Rect.Dy(), 0))
	l.hover, l.hoverAdd = "", false
	if l.updateDrag(input, &result) || !inside {
		return result
	}
	onRow := false
	for index, row := range l.Rows {
		rect := l.RowRect(index)
		if !input.Cursor.In(rect) {
			continue
		}
		onRow = true
		if row.Heading && !row.Section {
			continue
		}
		l.hover = row.ID
		l.hoverAdd = row.Add && input.Cursor.In(addRect(rect))
		if input.Pressed {
			l.press(input, row, rect, &result)
		}
		if input.Secondary {
			result.Context, result.ContextRow = true, row.ID
		}
	}
	if input.Secondary && !onRow {
		result.Context = true
	}

	return result
}

// press handles a press on a row: its [+], its triangle or heading, or the
// row itself, which it selects and, when dragging is allowed, may drag.
func (l *List) press(input *Input, row Row, rect image.Rectangle, result *ListResult) {
	double := input.DoubleClick
	input.ClearPointer()
	switch {
	case l.hoverAdd:
		result.Added = row.ID
	case row.Section:
		result.Toggled = row.ID
	case row.Toggle != DisclosureNone && input.Cursor.In(toggleRect(rect, row)):
		result.Toggled = row.ID
	default:
		result.Changed = l.Selected != row.ID
		l.Selected = row.ID
		result.Activated = double
		if l.CanDrop != nil {
			l.drag = listDrag{row: row.ID, from: input.Cursor, at: input.Cursor, dragging: false, over: "", resting: 0}
		}
	}
}

// noDrag is a list with no row pressed.
func noDrag() listDrag {
	return listDrag{row: "", from: image.Pt(0, 0), at: image.Pt(0, 0), dragging: false, over: "", resting: 0}
}

// updateDrag follows a pressed row: once the pointer has moved far enough
// it is dragged, over whichever row CanDrop accepts, and releasing it there
// reports the drop. It reports whether a drag took the input.
func (l *List) updateDrag(input *Input, result *ListResult) bool {
	if l.drag.row == "" {
		return false
	}
	l.drag.at = input.Cursor
	moved := input.Cursor.Sub(l.drag.from)
	if input.Down && max(moved.X, -moved.X, moved.Y, -moved.Y) >= dragDistance {
		l.drag.dragging = true
	}
	if !l.drag.dragging {
		if !input.Down {
			l.drag = noDrag()
		}

		return false
	}
	l.rest(l.dropTarget(input.Cursor), result)
	if !input.Down {
		if l.drag.over != "" {
			result.Dragged, result.Dropped = l.drag.row, l.drag.over
		}
		l.drag = noDrag()
	}
	input.ClearPointer()

	return true
}

// dropTarget returns the row a drag at a point would drop on: a row of the
// list that takes it, or what DropElsewhere finds outside the list.
func (l *List) dropTarget(point image.Point) string {
	over := ""
	for index, row := range l.Rows {
		if point.In(l.Rect) && point.In(l.RowRect(index)) && row.ID != l.drag.row && l.CanDrop(l.drag.row, row.ID) {
			over = row.ID
		}
	}
	if over == "" && l.DropElsewhere != nil && !point.In(l.Rect) {
		over = l.DropElsewhere(l.drag.row, point)
	}

	return over
}

// rest follows the row a drag is over, reporting it once the drag has
// stayed there long enough.
func (l *List) rest(over string, result *ListResult) {
	if over != l.drag.over {
		l.drag.over, l.drag.resting = over, 0
	}
	if over == "" {
		return
	}
	l.drag.resting++
	if l.drag.resting == restFrames {
		result.Rested = over
	}
}

// Reveal scrolls the list as little as it takes to show a row.
func (l *List) Reveal(id string) {
	for index, row := range l.Rows {
		if row.ID != id {
			continue
		}
		top := l.RowRect(index).Min.Y - l.Rect.Min.Y + l.scroll
		if top < l.scroll {
			l.scroll = top
		}
		if bottom := top + l.RowRect(index).Dy(); bottom > l.scroll+l.Rect.Dy() {
			l.scroll = bottom - l.Rect.Dy()
		}
	}
}

// ScrollToTop scrolls the list back to its first row, as when it lists
// something else.
func (l *List) ScrollToTop() { l.scroll = 0 }

// Dragging reports whether a row is being dragged.
func (l *List) Dragging() bool { return l.drag.dragging }

// SelectedRow returns the selected row.
func (l *List) SelectedRow() (Row, bool) {
	for _, row := range l.Rows {
		if row.ID == l.Selected {
			return row, true
		}
	}

	var none Row

	return none, false
}

// treeRow reports whether a row is drawn as part of a tree.
func treeRow(row Row) bool {
	return row.Tree || row.Icon != IconNone || row.Guide != GuideNone || row.Toggle != DisclosureNone
}

// treeLeft is where a tree row's disclosure column starts.
func treeLeft(rect image.Rectangle, row Row) int {
	return rect.Min.X + Inset/Half + row.Indent*treeIndent
}

// toggleRect is the clickable square round a row's disclosure triangle.
func toggleRect(rect image.Rectangle, row Row) image.Rectangle {
	left := treeLeft(rect, row)
	if !treeRow(row) && !row.Section {
		left = rect.Min.X + Inset/Half
	}

	return image.Rect(left, rect.Min.Y, left+treeToggle, rect.Max.Y)
}

// addRect is a section's [+] at the right edge of its row.
func addRect(rect image.Rectangle) image.Rectangle {
	right := rect.Max.X - Inset/Half - Gap
	top := rect.Min.Y + (rect.Dy()-addSize)/Half

	return image.Rect(right-addSize, top, right, top+addSize)
}

// Draw renders the list.
func (l *List) Draw(screen *ebiten.Image, theme *Theme) {
	area, ok := pen.Clip(screen, l.Rect)
	if !ok {
		return
	}
	for index, row := range l.Rows {
		rect := l.RowRect(index)
		if !rect.Overlaps(l.Rect) {
			continue
		}
		switch {
		case row.Section:
			l.drawSection(area, theme, row, rect, index == 0)

			continue
		case row.Heading:
			theme.Heading(area, row.Label, rect.Min.X+Inset/2+row.Indent*Inset, rect.Min.Y+(RowHeight-TextSize)/2)

			continue
		case l.Grid > 0:
			l.drawChip(area, theme, row, rect)

			continue
		case row.ID == l.Selected && l.InHand:
			FillRound(area, highlight(rect), rowRadius, ColorSelect())
		case row.ID == l.Selected:
			FillRound(area, highlight(rect), rowRadius, ColorSelectWash())
			StrokeRound(area, highlight(rect), rowRadius, Hairline, ColorSelect())
		case row.Ticked:
			FillRound(area, highlight(rect), rowRadius, ColorSelectWash())
		case row.ID == l.hover:
			FillRound(area, highlight(rect), rowRadius, ColorHover())
		}
		if treeRow(row) {
			guided := index+1 < len(l.Rows) && l.Rows[index+1].Guide != GuideNone
			l.drawTreeRow(area, theme, row, rect, guided)
		} else {
			l.drawPlainRow(area, theme, row, rect)
		}
	}
	l.drawDrag(area, theme)
}

// drawDrag outlines the row a dragged row would drop on, and shows what is
// being dragged beside the pointer.
func (l *List) drawDrag(area *ebiten.Image, theme *Theme) {
	if !l.drag.dragging {
		return
	}
	label := ""
	for index, row := range l.Rows {
		if l.Faded != nil && l.Faded(l.drag.row, row.ID) {
			Box(area, l.RowRect(index), HexAlpha(hexPage, fadeAlpha))
		}
		if row.ID == l.drag.over {
			StrokeRound(area, highlight(l.RowRect(index)), rowRadius, Outline, ColorSelect())
		}
		if row.ID == l.drag.row {
			label = row.Label
		}
	}
	if l.DragLabel != nil {
		label = l.DragLabel(l.drag.row, l.drag.over)
	}
	box := image.Rect(0, 0, theme.TextWidth(label)+Half*Inset, RowHeight).Add(l.drag.at.Add(image.Pt(Inset, Gap)))
	FillRound(area, box, rowRadius, ColorSelectWash())
	StrokeRound(area, box, rowRadius, Hairline, ColorSelect())
	theme.DrawText(area, label, box.Min.X+Inset, box.Min.Y+(RowHeight-TextSize)/Half-1, ColorInk())
}

// drawPlainRow draws a row's swatch, label, mark and detail in a line.
func (l *List) drawPlainRow(area *ebiten.Image, theme *Theme, row Row, rect image.Rectangle) {
	left := rect.Min.X + Inset/2 + row.Indent*Inset
	if row.Swatch != nil {
		swatch := image.Rect(left, rect.Min.Y+swatchTop, left+swatchSize, rect.Min.Y+swatchTop+swatchSize)
		FillRound(area, swatch, swatchRadius, *row.Swatch)
		StrokeRound(area, swatch, swatchRadius, Border, ColorInk())
		left += 20
	}
	filled := l.InHand && row.ID == l.Selected
	ink, quiet := row.ink(), ColorSubdued()
	if filled {
		ink, quiet = ColorReverse(), ColorReverse()
	}
	if row.Glyph != IconNone {
		glyph := ColorMuted()
		if filled {
			glyph = ColorReverse()
		}
		DrawIconInk(area, row.Glyph, image.Pt(left, rect.Min.Y+(rect.Dy()-IconSize)/Half), glyph)
		left += IconSize + labelGap
	}
	labelTop := rect.Min.Y + (RowHeight-TextSize)/2 - 1
	if row.ID == l.Selected || row.Strong {
		theme.DrawStrong(area, row.Label, left, labelTop, ink)
	} else {
		theme.DrawText(area, row.Label, left, labelTop, ink)
	}
	after := left + max(theme.TextWidth(row.Label), theme.StrongWidth(row.Label)) + labelGap
	if row.Marked {
		theme.DrawText(area, "●", after, labelTop, ColorSelect())
		after += 14
	}
	if row.Detail != "" {
		theme.DrawText(area, row.Detail, after, labelTop, quiet)
	}
	if row.Key != "" {
		DrawCap(area, theme, row.Key, rect.Max.X-Gap*Half-Gap, rect.Min.Y+rect.Dy()/Half, filled)
	}
}

// Chips of a grid: how tall each is, the gap between them, the room inside
// one's edge and between its parts, and the size of the swatch on one.
const (
	chipHeight = 30
	chipGap    = 6
	chipPad    = 5
	chipSwatch = 16
)

// Height is how tall the list's rows are together.
func (l *List) Height() int {
	if l.Grid > 0 {
		lines := (len(l.Rows) + l.Grid - 1) / l.Grid

		return max(lines*(chipHeight+chipGap)-chipGap, 0)
	}

	return len(l.Rows) * RowHeight
}

// chipRect is where a row of a grid is: a chip in its column and line.
func (l *List) chipRect(index int) image.Rectangle {
	inner := l.Rect.Dx() - Half*Gap
	width := (inner - (l.Grid-1)*chipGap) / l.Grid
	left := l.Rect.Min.X + Gap + (index%l.Grid)*(width+chipGap)
	top := l.Rect.Min.Y + (index/l.Grid)*(chipHeight+chipGap) - l.scroll

	return image.Rect(left, top, left+width, top+chipHeight)
}

// FitsGrid reports whether every row's name fits on a chip when the list,
// as wide as it is now, lays its rows out in some number of columns, so a
// name is never cut short for the sake of a second column.
func (l *List) FitsGrid(theme *Theme, columns int) bool {
	inner := l.Rect.Dx() - Half*Gap
	width := (inner - (columns-1)*chipGap) / columns
	for _, row := range l.Rows {
		room := width - Half*chipPad
		if row.Swatch != nil {
			room -= chipSwatch + chipPad
		}
		if row.Key != "" {
			room -= theme.MonoWidth(row.Key) + chipPad
		}
		if theme.StrongWidth(row.Label) > room {
			return false
		}
	}

	return true
}

// drawChip draws a row of a grid: a box of its own with its swatch, its
// name and its key. The chosen one is washed blue with a ring; the others
// are white, since each is something to choose.
func (l *List) drawChip(area *ebiten.Image, theme *Theme, row Row, rect image.Rectangle) {
	fill, edge, width := ColorField(), ColorRule(), float32(Hairline)
	switch row.ID {
	case l.Selected:
		fill, edge, width = ColorSelectWash(), ColorSelect(), Border
	case l.hover:
		fill = ColorHover()
	}
	FillRound(area, rect, rowRadius, fill)
	StrokeRound(area, rect, rowRadius, width, edge)
	left := rect.Min.X + chipPad
	if row.Swatch != nil {
		top := rect.Min.Y + (rect.Dy()-chipSwatch)/Half
		swatch := image.Rect(left, top, left+chipSwatch, top+chipSwatch)
		FillRound(area, swatch, swatchRadius, *row.Swatch)
		StrokeRound(area, swatch, swatchRadius, Hairline, ColorInk())
		left += chipSwatch + chipPad
	}
	room := rect.Max.X - chipPad - left
	if row.Key != "" {
		room -= theme.MonoWidth(row.Key) + chipPad
		theme.DrawMono(area, row.Key, rect.Max.X-chipPad-theme.MonoWidth(row.Key), rect.Min.Y+(rect.Dy()-MonoSize)/Half-1,
			ColorMuted())
	}
	labelTop := rect.Min.Y + (rect.Dy()-TextSize)/Half - 1
	if row.ID == l.Selected {
		theme.DrawStrong(area, theme.FitText(row.Label, room), left, labelTop, row.ink())
	} else {
		theme.DrawText(area, theme.FitText(row.Label, room), left, labelTop, row.ink())
	}
}

// drawTreeRow draws a tree row: its guide, triangle, icon, label and the
// detail at the right edge. childGuided says the row below hangs from it on
// a tree line, which the row's stub joins.
func (l *List) drawTreeRow(area *ebiten.Image, theme *Theme, row Row, rect image.Rectangle, childGuided bool) {
	left := treeLeft(rect, row)
	middle := rect.Min.Y + rect.Dy()/Half
	iconTop := rect.Min.Y + (rect.Dy()-IconSize)/Half
	if row.Guide != GuideNone {
		// The line comes down from below the parent's icon, one indent left.
		parent := left - treeIndent + treeToggle + IconSize/Half
		bottom := rect.Max.Y
		if row.Guide == GuideLast {
			bottom = middle
		}
		Line(area, image.Pt(parent, rect.Min.Y), image.Pt(parent, bottom), Hairline, ColorRule())
		Line(area, image.Pt(parent, middle), image.Pt(left+treeToggle-Gap, middle), Hairline, ColorRule())
	}
	if row.Toggle != DisclosureNone {
		DrawTriangle(area, toggleRect(rect, row), row.Toggle == DisclosureOpen, ColorMuted())
	}
	if row.Ticked {
		drawTick(area, toggleRect(rect, row))
	}
	if row.Toggle == DisclosureOpen && childGuided {
		// A short stub joins the icon to its first child's guide.
		stub := left + treeToggle + IconSize/Half
		Line(area, image.Pt(stub, iconTop+IconSize+Hairline), image.Pt(stub, rect.Max.Y), Hairline, ColorRule())
	}
	if l.InHand && row.ID == l.Selected && !Chipped(row.Icon) {
		// An icon with no chip of its own is reversed on the row in hand's
		// blue, where its own ink would not show.
		DrawIconInk(area, row.Icon, image.Pt(left+treeToggle, iconTop), ColorReverse())
	} else {
		DrawIcon(area, row.Icon, image.Pt(left+treeToggle, iconTop))
	}
	l.drawTreeLabel(area, theme, row, rect, left+treeToggle+IconSize+treeLabelGap)
}

// drawTreeLabel draws a tree row's label, its aside, its mark and its
// detail: in a column, as a cap at the right edge, or as text there. The
// row in hand has them reversed on its blue.
func (l *List) drawTreeLabel(area *ebiten.Image, theme *Theme, row Row, rect image.Rectangle, labelLeft int) {
	labelTop := rect.Min.Y + (RowHeight-TextSize)/2 - 1
	label := row.Label
	if len(l.Columns) > 0 {
		label = theme.FitText(label, rect.Min.X+l.Columns[0]-labelGap-labelLeft)
		l.drawCells(area, theme, row, rect, labelTop)
	}
	filled := l.InHand && row.ID == l.Selected
	ink, quiet := row.ink(), ColorSubdued()
	if filled {
		ink, quiet = ColorReverse(), ColorReverse()
	}
	if row.Strong || filled {
		theme.DrawStrong(area, label, labelLeft, labelTop, ink)
	} else {
		theme.DrawText(area, label, labelLeft, labelTop, ink)
	}
	if row.Aside != "" {
		asideLeft := labelLeft + theme.StrongWidth(row.Label) + labelGap
		room := rect.Max.X - Inset/Half - Gap*Half - asideLeft
		if row.Detail != "" {
			room -= theme.TextWidth(row.Detail) + labelGap
		}
		theme.DrawText(area, theme.FitText(row.Aside, room), asideLeft, labelTop, quiet)
	}
	if row.Marked {
		theme.DrawText(area, "●", labelLeft+theme.StrongWidth(row.Label)+labelGap, labelTop, ColorSelect())
	}
	switch right := rect.Max.X - Inset/Half - Gap*Half; {
	case row.Detail == "":
	case l.DetailAt > 0:
		at := max(rect.Min.X+l.DetailAt, labelLeft+theme.StrongWidth(row.Label)+labelGap)
		theme.DrawText(area, theme.FitText(row.Detail, rect.Max.X-Inset/Half-at), at, labelTop, quiet)
	case l.Caps:
		DrawCap(area, theme, row.Detail, right+Gap, rect.Min.Y+rect.Dy()/Half, filled)
	default:
		theme.DrawText(area, row.Detail, right-theme.TextWidth(row.Detail), labelTop, quiet)
	}
}

// drawCells draws a table row's cells, each at its column and cut short
// before the next.
func (l *List) drawCells(area *ebiten.Image, theme *Theme, row Row, rect image.Rectangle, top int) {
	for index, cell := range row.Cells {
		if index >= len(l.Columns) {
			return
		}
		left, right := rect.Min.X+l.Columns[index], rect.Max.X-Inset/Half
		if index+1 < len(l.Columns) {
			right = rect.Min.X + l.Columns[index+1] - labelGap
		}
		theme.DrawText(area, theme.FitText(cell, right-left), left, top, ColorSubdued())
	}
}

// drawTick draws a tick in the middle of a box, in the selection's colour.
func drawTick(area *ebiten.Image, box image.Rectangle) {
	centre := Center(box)
	const short, long = 3, 5
	foot := image.Pt(centre.X-1, centre.Y+short)
	Line(area, image.Pt(centre.X-long+1, centre.Y), foot, Outline, ColorSelect())
	Line(area, foot, image.Pt(centre.X+long, centre.Y-long+1), Outline, ColorSelect())
}

// drawSection draws a section heading: a divider above (but not above the
// first), its triangle, its name, its count and its [+].
func (l *List) drawSection(area *ebiten.Image, theme *Theme, row Row, rect image.Rectangle, first bool) {
	left := rect.Min.X + Inset/Half
	if !first {
		Line(area, image.Pt(left, rect.Min.Y), image.Pt(rect.Max.X-Inset/Half, rect.Min.Y), Hairline, ColorRule())
	}
	if row.ID == l.hover && !l.hoverAdd {
		FillRound(area, highlight(rect), rowRadius, ColorHover())
	}
	if row.Toggle != DisclosureNone {
		DrawTriangle(area, toggleRect(rect, row), row.Toggle == DisclosureOpen, ColorInk())
	}
	labelTop := rect.Min.Y + (RowHeight-TextSize)/2 - 1
	theme.DrawSection(area, row.Label, left+treeToggle, labelTop)
	if row.Detail != "" {
		theme.DrawText(area, row.Detail, left+treeToggle+theme.SectionWidth(row.Label)+sectionCount, labelTop,
			ColorSubdued())
	}
	if row.Add {
		plus := addRect(rect)
		if row.ID == l.hover && l.hoverAdd {
			FillRound(area, plus, Radius, ColorHover())
		}
		StrokeRound(area, plus, Radius, Border, ColorRule())
		centre := Center(plus)
		Line(area, image.Pt(centre.X-triangle, centre.Y), image.Pt(centre.X+triangle, centre.Y), Outline, ColorInk())
		Line(area, image.Pt(centre.X, centre.Y-triangle), image.Pt(centre.X, centre.Y+triangle), Outline, ColorInk())
	}
}

// DrawTriangle draws a small triangle centred in a box: pointing down
// when open, as on a disclosure or a menu button, and right when closed.
func DrawTriangle(screen *ebiten.Image, box image.Rectangle, open bool, ink color.Color) {
	centre := Center(box)
	midX, midY, size := float32(centre.X), float32(centre.Y), float32(triangle)
	var path vector.Path
	if open {
		path.MoveTo(midX-size, midY-size/Half)
		path.LineTo(midX+size, midY-size/Half)
		path.LineTo(midX, midY+size)
	} else {
		path.MoveTo(midX-size/Half, midY-size)
		path.LineTo(midX+size, midY)
		path.LineTo(midX-size/Half, midY+size)
	}
	path.Close()
	fillPath(screen, &path, ink)
}
