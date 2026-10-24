package ui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Menu geometry, in screen pixels.
const (
	menuWidth = 200
	menuPad   = 4
)

// MenuItem is one choice of a Menu.
type MenuItem struct {
	ID       string
	Label    string
	Icon     Icon
	Disabled bool
}

// Menu is a small list of choices that opens below a button, such as
// "New ▾". While open it takes all input; a choice, Escape or a click
// outside closes it.
type Menu struct {
	Items []MenuItem
	open  bool
	rect  image.Rectangle
	hover int
}

// Open shows the menu below an anchor, kept inside the window.
func (m *Menu) Open(anchor, window image.Rectangle) {
	height := len(m.Items)*RowHeight + Half*menuPad
	left := min(anchor.Min.X, window.Max.X-menuWidth-Gap)
	m.rect = image.Rect(left, anchor.Max.Y+Gap, left+menuWidth, anchor.Max.Y+Gap+height)
	m.open, m.hover = true, -1
}

// IsOpen reports whether the menu is showing.
func (m *Menu) IsOpen() bool { return m.open }

// itemRect is the rectangle of an item.
func (m *Menu) itemRect(index int) image.Rectangle {
	top := m.rect.Min.Y + menuPad + index*RowHeight

	return image.Rect(m.rect.Min.X+menuPad, top, m.rect.Max.X-menuPad, top+RowHeight)
}

// Update handles a frame of input while open and returns the chosen item's
// ID, or "".
func (m *Menu) Update(input *Input) string {
	if !m.open {
		return ""
	}
	defer input.ClearAll()
	if input.Consume(KeyEscape, Plain()) {
		m.open = false

		return ""
	}
	m.hover = -1
	for index, item := range m.Items {
		if input.Cursor.In(m.itemRect(index)) && !item.Disabled {
			m.hover = index
		}
	}
	if !input.Pressed {
		return ""
	}
	m.open = false
	if m.hover < 0 {
		return ""
	}

	return m.Items[m.hover].ID
}

// Locate returns the centre of an item, for input scripts.
func (m *Menu) Locate(id string) (image.Point, bool) {
	for index, item := range m.Items {
		if item.ID == id && m.open {
			return Center(m.itemRect(index)), true
		}
	}

	return image.Pt(0, 0), false
}

// Draw renders the menu when open: a card with a shadow, each item with its
// icon.
func (m *Menu) Draw(screen *ebiten.Image, theme *Theme) {
	if !m.open {
		return
	}
	FillRound(screen, m.rect.Add(image.Pt(0, shadowDrop/Half)), dialogRadius, HexAlpha(0, shadowAlpha))
	Box(screen, m.rect, ColorBackground())
	for index, item := range m.Items {
		rect := m.itemRect(index)
		if index == m.hover {
			FillRound(screen, rect, rowRadius, ColorHover())
		}
		left := rect.Min.X + menuPad*Half
		if item.Icon != IconNone {
			DrawIcon(screen, item.Icon, image.Pt(left, rect.Min.Y+(rect.Dy()-IconSize)/Half))
			left += IconSize + treeLabelGap
		}
		ink := ColorInk()
		if item.Disabled {
			ink = ColorSubdued()
		}
		theme.DrawText(screen, item.Label, left, rect.Min.Y+(RowHeight-TextSize)/Half-1, ink)
	}
}
