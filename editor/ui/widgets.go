package ui

import (
	"image"
	"image/color"

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
	hover  bool
}

// NewButton returns a plain button, placed later by setting Rect.
func NewButton(id, label string) *Button {
	return &Button{
		ID: id, Label: label, Rect: image.Rect(0, 0, 0, 0), Disabled: false, Primary: false, On: false, Danger: false,
		Caret: false, hover: false,
	}
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

// Draw renders the button: a rounded box with an ink outline.
func (b *Button) Draw(screen *ebiten.Image, theme *Theme) {
	fill, ink, edge := ColorBackground(), ColorInk(), ColorInk()
	switch {
	case b.Disabled:
		ink, edge = ColorSubdued(), ColorRule()
	case b.Primary:
		fill, ink, edge = ColorSelect(), ColorBackground(), ColorSelect()
	case b.On:
		fill, ink = ColorInk(), ColorBackground()
	case b.Danger:
		ink, edge = ColorWarn(), ColorWarn()
		if b.hover {
			fill = ColorHover()
		}
	case b.hover:
		fill = ColorHover()
	}
	FillRound(screen, b.Rect, Radius, fill)
	StrokeRound(screen, b.Rect, Radius, Border, edge)
	width := theme.TextWidth(b.Label)
	if b.Caret {
		width += triangle*Half + labelGap
	}
	left := b.Rect.Min.X + (b.Rect.Dx()-width)/Half
	theme.DrawText(screen, b.Label, left, b.Rect.Min.Y+(b.Rect.Dy()-TextSize)/Half-1, ink)
	if b.Caret {
		caret := image.Rect(left+width-triangle*Half, b.Rect.Min.Y, left+width, b.Rect.Max.Y)
		drawTriangle(screen, caret, true, ink)
	}
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
	FillRound(screen, square, toggleInset, ColorBackground())
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
}

// NewRow returns a plain selectable row; set its other fields as needed.
func NewRow(id, label string) Row {
	return Row{
		ID: id, Label: label, Detail: "", Indent: 0, Swatch: nil, Heading: false, Marked: false, Icon: IconNone,
		Strong: false, Guide: GuideNone, Toggle: DisclosureNone, Section: false, Add: false,
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

// List is a scrolling list of rows with one selected row.
type List struct {
	ID       string
	Rect     image.Rectangle
	Rows     []Row
	Selected string
	scroll   int
	hover    string
	hoverAdd bool
}

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
}

// RowRect returns the screen rectangle of a row, which may lie outside the
// list when scrolled away.
func (l *List) RowRect(index int) image.Rectangle {
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

// Update handles scrolling and clicks.
func (l *List) Update(input *Input) ListResult {
	result := ListResult{Changed: false, Activated: false, Toggled: "", Added: ""}
	inside := input.Cursor.In(l.Rect)
	if inside && input.WheelY != 0 {
		l.scroll -= int(input.WheelY * RowHeight)
		input.WheelY = 0
	}
	l.scroll = min(max(l.scroll, 0), max(len(l.Rows)*RowHeight-l.Rect.Dy(), 0))
	l.hover, l.hoverAdd = "", false
	if !inside {
		return result
	}
	for index, row := range l.Rows {
		rect := l.RowRect(index)
		if !input.Cursor.In(rect) || (row.Heading && !row.Section) {
			continue
		}
		l.hover = row.ID
		l.hoverAdd = row.Add && input.Cursor.In(addRect(rect))
		if !input.Pressed {
			continue
		}
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
		}
	}

	return result
}

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
	return row.Icon != IconNone || row.Guide != GuideNone || row.Toggle != DisclosureNone
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
	area, ok := screen.SubImage(l.Rect).(*ebiten.Image)
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
		case row.ID == l.Selected:
			FillRound(area, highlight(rect), rowRadius, ColorSelectWash())
			StrokeRound(area, highlight(rect), rowRadius, Outline, ColorSelect())
		case row.ID == l.hover:
			FillRound(area, highlight(rect), rowRadius, ColorHover())
		}
		if treeRow(row) {
			l.drawTreeRow(area, theme, row, rect)
		} else {
			l.drawPlainRow(area, theme, row, rect)
		}
	}
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
	labelTop := rect.Min.Y + (RowHeight-TextSize)/2 - 1
	if row.ID == l.Selected || row.Strong {
		theme.DrawStrong(area, row.Label, left, labelTop, ColorInk())
	} else {
		theme.DrawText(area, row.Label, left, labelTop, ColorInk())
	}
	after := left + max(theme.TextWidth(row.Label), theme.StrongWidth(row.Label)) + labelGap
	if row.Marked {
		theme.DrawText(area, "●", after, labelTop, ColorSelect())
		after += 14
	}
	if row.Detail != "" {
		theme.DrawText(area, row.Detail, after, labelTop, ColorSubdued())
	}
}

// drawTreeRow draws a tree row: its guide, triangle, icon, label and the
// detail at the right edge.
func (l *List) drawTreeRow(area *ebiten.Image, theme *Theme, row Row, rect image.Rectangle) {
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
		drawTriangle(area, toggleRect(rect, row), row.Toggle == DisclosureOpen, ColorMuted())
	}
	if row.Toggle == DisclosureOpen {
		// A short stub joins the icon to its first child's guide.
		stub := left + treeToggle + IconSize/Half
		Line(area, image.Pt(stub, iconTop+IconSize+Hairline), image.Pt(stub, rect.Max.Y), Hairline, ColorRule())
	}
	DrawIcon(area, row.Icon, image.Pt(left+treeToggle, iconTop))
	labelLeft := left + treeToggle + IconSize + treeLabelGap
	labelTop := rect.Min.Y + (RowHeight-TextSize)/2 - 1
	if row.Strong {
		theme.DrawStrong(area, row.Label, labelLeft, labelTop, ColorInk())
	} else {
		theme.DrawText(area, row.Label, labelLeft, labelTop, ColorInk())
	}
	if row.Marked {
		theme.DrawText(area, "●", labelLeft+theme.StrongWidth(row.Label)+labelGap, labelTop, ColorSelect())
	}
	if row.Detail != "" {
		right := rect.Max.X - Inset/Half - Gap*Half
		theme.DrawText(area, row.Detail, right-theme.TextWidth(row.Detail), labelTop, ColorSubdued())
	}
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
		drawTriangle(area, toggleRect(rect, row), row.Toggle == DisclosureOpen, ColorInk())
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

// drawTriangle draws a disclosure triangle centred in a box: pointing down
// when open, right when closed.
func drawTriangle(screen *ebiten.Image, box image.Rectangle, open bool, ink color.Color) {
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
