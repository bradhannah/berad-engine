package ui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

// Dialog button IDs with keyboard meanings: Enter chooses the primary button,
// Escape chooses Cancel.
const (
	DialogCancel = "cancel"
	dialogWidth  = 420
	listRows     = 6
	buttonWidth  = 110
	buttonGap    = 8
)

// Dialog is a modal box inside a window: a title, lines of explanation, an
// optional list to choose from and a row of buttons. While shown it takes all
// input.
type Dialog struct {
	Title   string
	Lines   []string
	List    *List
	Buttons []*Button
	rect    image.Rectangle
}

// NewDialog returns a dialog. The first button is the primary one.
func NewDialog(title string, lines []string, buttons ...*Button) *Dialog {
	if len(buttons) > 0 {
		buttons[0].Primary = true
	}

	return &Dialog{Title: title, Lines: lines, List: nil, Buttons: buttons, rect: image.Rect(0, 0, 0, 0)}
}

// DialogButton returns a button for a dialog.
func DialogButton(id, label string) *Button {
	return &Button{ID: id, Label: label, Rect: image.Rect(0, 0, 0, 0), Disabled: false, Primary: false, hover: false}
}

// layout centres the dialog in the window and places its parts.
func (d *Dialog) layout(window image.Rectangle) {
	height := Inset*2 + LineHeight + len(d.Lines)*LineHeight + Inset + ButtonH
	if d.List != nil {
		height += listRows*RowHeight + Inset
	}
	width := min(dialogWidth, window.Dx()-2*Inset)
	left := window.Min.X + (window.Dx()-width)/Half
	top := window.Min.Y + max((window.Dy()-height)/third, Inset)
	d.rect = image.Rect(left, top, left+width, top+height)
	rowY := top + Inset + LineHeight + len(d.Lines)*LineHeight
	if d.List != nil {
		d.List.Rect = image.Rect(left+Inset, rowY, left+width-Inset, rowY+listRows*RowHeight)
		rowY += listRows*RowHeight + Inset
	}
	right := left + width - Inset
	for index := len(d.Buttons) - 1; index >= 0; index-- {
		button := d.Buttons[index]
		button.Rect = image.Rect(right-buttonWidth, rowY, right, rowY+ButtonH)
		right -= buttonWidth + buttonGap
	}
}

// Update handles a frame of input and returns the ID of the chosen button, or
// "". Enter chooses the primary button and Escape chooses Cancel; a double
// click in the list chooses the primary button.
func (d *Dialog) Update(input *Input, window image.Rectangle) string {
	d.layout(window)
	defer input.ClearAll()
	if d.List != nil && d.List.Update(input).Activated && len(d.Buttons) > 0 && !d.Buttons[0].Disabled {
		return d.Buttons[0].ID
	}
	for _, button := range d.Buttons {
		if button.Update(input) {
			return button.ID
		}
	}
	if input.Has(KeyEnter, Plain()) && len(d.Buttons) > 0 && !d.Buttons[0].Disabled {
		return d.Buttons[0].ID
	}
	if input.Has(KeyEscape, Plain()) {
		for _, button := range d.Buttons {
			if button.ID == DialogCancel {
				return DialogCancel
			}
		}
	}

	return ""
}

// Locate returns the centre of a button or list row, for input scripts.
func (d *Dialog) Locate(name string, window image.Rectangle) (image.Point, bool) {
	d.layout(window)
	for _, button := range d.Buttons {
		if button.ID == name {
			return image.Pt(button.Rect.Min.X+button.Rect.Dx()/2, button.Rect.Min.Y+button.Rect.Dy()/2), true
		}
	}
	if d.List != nil {
		return d.List.Locate(name)
	}

	return image.Pt(0, 0), false
}

// Draw renders the dialog over a shaded window.
func (d *Dialog) Draw(screen *ebiten.Image, theme *Theme) {
	d.layout(screen.Bounds())
	FillRect(screen, screen.Bounds(), ColorShade())
	FillRect(screen, d.rect, ColorBackground())
	StrokeRect(screen, d.rect, Hairline, ColorMuted())
	x, y := d.rect.Min.X+Inset, d.rect.Min.Y+Inset
	theme.DrawText(screen, d.Title, x, y, ColorInk())
	y += LineHeight
	for _, line := range d.Lines {
		theme.DrawText(screen, line, x, y, ColorMuted())
		y += LineHeight
	}
	if d.List != nil {
		StrokeRect(screen, d.List.Rect, Hairline, ColorRule())
		d.List.Draw(screen, theme)
	}
	for _, button := range d.Buttons {
		button.Draw(screen, theme)
	}
}

// Focus tracks which text field in a window has keyboard focus.
type Focus struct {
	fields  []*TextField
	current *TextField
}

// Register adds the window's fields, in Tab order.
func (f *Focus) Register(fields ...*TextField) {
	f.fields = append(f.fields, fields...)
}

// Current returns the focused field, or nil.
func (f *Focus) Current() *TextField { return f.current }

// Set focuses a field, or none.
func (f *Focus) Set(field *TextField) {
	if f.current == field {
		return
	}
	if f.current != nil {
		f.current.SetFocus(false)
	}
	f.current = field
	if field != nil {
		field.SetFocus(true)
	}
}

// Update runs every registered field. It returns the field whose key press
// produced an outcome other than none, with that outcome. A press outside
// every field removes focus.
func (f *Focus) Update(input *Input, clipboard textedit.Clipboard) (*TextField, textedit.Outcome) {
	pressedOutside := input.Pressed
	for _, field := range f.fields {
		if field != f.current && input.Pressed && input.Cursor.In(field.Rect) {
			f.Set(field)
		}
	}
	var acted *TextField
	outcome := textedit.OutcomeNone
	for _, field := range f.fields {
		result, clicked := field.Update(input, clipboard)
		if clicked {
			pressedOutside = false
		}
		if result != textedit.OutcomeNone {
			acted, outcome = field, result
		}
	}
	if pressedOutside {
		f.Set(nil)
	}
	if outcome == textedit.OutcomeBlur {
		f.Set(nil)
	}

	return acted, outcome
}
