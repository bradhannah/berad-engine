package ui

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
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
	hover   bool
}

// NewButton returns a plain button, placed later by setting Rect.
func NewButton(id, label string) *Button {
	return &Button{
		ID: id, Label: label, Rect: image.Rect(0, 0, 0, 0), Disabled: false, Primary: false, On: false, hover: false,
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
	case b.hover:
		fill = ColorHover()
	}
	FillRound(screen, b.Rect, Radius, fill)
	StrokeRound(screen, b.Rect, Radius, Border, edge)
	width := theme.TextWidth(b.Label)
	theme.DrawText(screen, b.Label, b.Rect.Min.X+(b.Rect.Dx()-width)/2, b.Rect.Min.Y+(b.Rect.Dy()-TextSize)/2-1, ink)
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

// Row is one row of a List.
type Row struct {
	ID    string
	Label string
	// Detail is shown subdued after the label.
	Detail string
	Indent int
	// Swatch, when set, is drawn as a small colour square before the label.
	Swatch *color.RGBA
	// Heading rows cannot be selected.
	Heading bool
	// Marked rows are drawn with a mark, for example maps that show the thing
	// being picked.
	Marked bool
}

// List is a scrolling list of rows with one selected row.
type List struct {
	ID       string
	Rect     image.Rectangle
	Rows     []Row
	Selected string
	scroll   int
	hover    string
}

// ListResult reports what happened to a list this frame.
type ListResult struct {
	// Changed is true when the selection changed.
	Changed bool
	// Activated is true when a row was double-clicked or Enter was pressed
	// while the list had the pointer over it.
	Activated bool
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
			rect := l.RowRect(index)

			return image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2), true
		}
	}

	return image.Pt(0, 0), false
}

// Update handles scrolling and clicks.
func (l *List) Update(input *Input) ListResult {
	result := ListResult{Changed: false, Activated: false}
	inside := input.Cursor.In(l.Rect)
	if inside && input.WheelY != 0 {
		l.scroll -= int(input.WheelY * RowHeight)
		input.WheelY = 0
	}
	l.scroll = min(max(l.scroll, 0), max(len(l.Rows)*RowHeight-l.Rect.Dy(), 0))
	l.hover = ""
	if !inside {
		return result
	}
	for index, row := range l.Rows {
		if !input.Cursor.In(l.RowRect(index)) || row.Heading {
			continue
		}
		l.hover = row.ID
		if input.Pressed {
			result.Changed = l.Selected != row.ID
			l.Selected = row.ID
			result.Activated = input.DoubleClick
			input.ClearPointer()
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
		left := rect.Min.X + Inset/2 + row.Indent*Inset
		switch {
		case row.Heading:
			theme.Heading(area, row.Label, left, rect.Min.Y+(RowHeight-TextSize)/2)

			continue
		case row.ID == l.Selected:
			FillRound(area, highlight(rect), rowRadius, ColorSelectWash())
			StrokeRound(area, highlight(rect), rowRadius, Outline, ColorSelect())
		case row.ID == l.hover:
			FillRound(area, highlight(rect), rowRadius, ColorHover())
		}
		if row.Swatch != nil {
			swatch := image.Rect(left, rect.Min.Y+swatchTop, left+swatchSize, rect.Min.Y+swatchTop+swatchSize)
			FillRound(area, swatch, swatchRadius, *row.Swatch)
			StrokeRound(area, swatch, swatchRadius, Border, ColorInk())
			left += 20
		}
		labelTop := rect.Min.Y + (RowHeight-TextSize)/2 - 1
		if row.ID == l.Selected {
			theme.DrawStrong(area, row.Label, left, labelTop, ColorInk())
		} else {
			theme.DrawText(area, row.Label, left, labelTop, ColorInk())
		}
		after := left + max(theme.TextWidth(row.Label), theme.StrongWidth(row.Label)) + labelGap
		if row.Marked {
			theme.DrawText(area, "●", after, rect.Min.Y+(RowHeight-TextSize)/2-1, ColorSelect())
			after += 14
		}
		if row.Detail != "" {
			theme.DrawText(area, row.Detail, after, rect.Min.Y+(RowHeight-TextSize)/2-1, ColorSubdued())
		}
	}
}
