package ui

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui/textedit"
)

// Dialog button IDs with keyboard meanings: Enter chooses the primary button,
// Escape chooses Cancel.
const (
	DialogCancel = "cancel"
	dialogWidth  = 420
	listRows     = 9
	buttonWidth  = 110
	buttonGap    = 8
	dialogRadius = 10
	shadowDrop   = 6
	shadowAlpha  = 0x26
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
	// wide is how wide each button's label needs it to be, as last drawn,
	// so no label is cut off at its button's edges.
	wide []int
}

// NewDialog returns a dialog. The first button is the primary one.
func NewDialog(title string, lines []string, buttons ...*Button) *Dialog {
	if len(buttons) > 0 {
		buttons[0].Primary = true
	}

	return &Dialog{Title: title, Lines: lines, List: nil, Buttons: buttons, rect: image.Rect(0, 0, 0, 0), wide: nil}
}

// DialogButton returns a button for a dialog.
func DialogButton(id, label string) *Button { return NewButton(id, label) }

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
		button, width := d.Buttons[index], buttonWidth
		if index < len(d.wide) {
			width = max(width, d.wide[index])
		}
		button.Rect = image.Rect(right-width, rowY, right, rowY+ButtonH)
		right -= width + buttonGap
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

// Draw renders the dialog over a shaded window: a rounded box with an ink
// outline and a soft shadow.
func (d *Dialog) Draw(screen *ebiten.Image, theme *Theme) {
	d.wide = d.wide[:0]
	for _, button := range d.Buttons {
		d.wide = append(d.wide, theme.TextWidth(button.Label)+Inset)
	}
	d.layout(screen.Bounds())
	FillRect(screen, screen.Bounds(), ColorShade())
	FillRound(screen, d.rect.Add(image.Pt(0, shadowDrop)), dialogRadius, HexAlpha(hexInk, shadowAlpha))
	Float(screen, d.rect, dialogRadius)
	FillRound(screen, d.rect, dialogRadius, ColorPage())
	StrokeRound(screen, d.rect, dialogRadius, Border, ColorInk())
	x, y := d.rect.Min.X+Inset, d.rect.Min.Y+Inset
	theme.DrawStrong(screen, d.Title, x, y, ColorInk())
	y += LineHeight
	for _, line := range d.Lines {
		theme.DrawText(screen, line, x, y, ColorMuted())
		y += LineHeight
	}
	if d.List != nil {
		StrokeRound(screen, d.List.Rect.Inset(-Half), Radius, Border, ColorInk())
		d.List.Draw(screen, theme)
	}
	for _, button := range d.Buttons {
		button.Draw(screen, theme)
	}
}

// Focus tracks which text field in a window has keyboard focus. reveal,
// when set, brings a field Tab moved to into view, as a panel that scrolls
// does.
type Focus struct {
	fields  []*TextField
	current *TextField
	reveal  func(*TextField)
}

// SetReveal sets what brings a field Tab moved to into view.
func (f *Focus) SetReveal(reveal func(*TextField)) { f.reveal = reveal }

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
	if f.current != nil {
		if step := f.current.Field.TakeTab(); step != 0 {
			f.move(step)
		}
	}

	return acted, outcome
}

// move gives the keyboard to the next field shown, or with Shift the one
// before, in Tab order, and Reveal brings it into view; at the last or the
// first it stays, and says so (increment 59). What was typed in the field
// left waits for Save.
func (f *Focus) move(step int) {
	at := slices.Index(f.fields, f.current)
	for next := at + step; at >= 0 && next >= 0 && next < len(f.fields); next += step {
		if f.fields[next].Rect.Empty() {
			continue
		}
		f.Set(f.fields[next])
		if f.reveal != nil {
			f.reveal(f.fields[next])
		}

		return
	}
	if step > 0 {
		f.current.Field.Say("This is the last field. Esc leaves it.")
	} else {
		f.current.Field.Say("This is the first field. Esc leaves it.")
	}
}
