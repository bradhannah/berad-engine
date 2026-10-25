package ui

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
)

// Menu geometry, in screen pixels.
const (
	menuWidth = 200
	menuPad   = 4
)

// MenuItem is one choice of a Menu. Danger marks a destructive choice, drawn
// in red; Rule draws a dividing line above the item, to start a group.
type MenuItem struct {
	ID       string
	Label    string
	Icon     Icon
	Disabled bool
	Danger   bool
	Rule     bool
}

// Menu is a small list of choices that opens below a button, such as
// "New ▾", or at the cursor, such as a right-click menu. While open it takes
// all input; a choice, Escape or a click outside closes it. Width is the
// menu's width, or a default when zero.
type Menu struct {
	Items []MenuItem
	Width int
	open  bool
	rect  image.Rectangle
	hover int
}

// Open shows the menu below an anchor, kept inside the window.
func (m *Menu) Open(anchor, window image.Rectangle) {
	width := m.width()
	left := min(anchor.Min.X, window.Max.X-width-Gap)
	m.rect = image.Rect(left, anchor.Max.Y+Gap, left+width, anchor.Max.Y+Gap+m.height())
	m.open, m.hover = true, -1
}

// OpenAt shows the menu with its top-left corner at a point, moved left or
// up as needed to stay inside the window.
func (m *Menu) OpenAt(point image.Point, window image.Rectangle) {
	width, height := m.width(), m.height()
	left := max(min(point.X, window.Max.X-width-Gap), window.Min.X)
	top := max(min(point.Y, window.Max.Y-height-Gap), window.Min.Y)
	m.rect = image.Rect(left, top, left+width, top+height)
	m.open, m.hover = true, -1
}

func (m *Menu) width() int {
	if m.Width > 0 {
		return m.Width
	}

	return menuWidth
}

func (m *Menu) height() int { return len(m.Items)*RowHeight + Half*menuPad }

// IsOpen reports whether the menu is showing.
func (m *Menu) IsOpen() bool { return m.open }

// Close hides the menu without a choice.
func (m *Menu) Close() { m.open = false }

// itemRect is the rectangle of an item.
func (m *Menu) itemRect(index int) image.Rectangle {
	top := m.rect.Min.Y + menuPad + index*RowHeight

	return image.Rect(m.rect.Min.X+menuPad, top, m.rect.Max.X-menuPad, top+RowHeight)
}

// Update handles a frame of input while open and returns the chosen item's
// ID, or "". Either mouse button chooses.
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
	if !input.Pressed && !input.Secondary {
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
// icon. When some items have icons, every label lines up after the icon
// column.
func (m *Menu) Draw(screen *ebiten.Image, theme *Theme) {
	if !m.open {
		return
	}
	FillRound(screen, m.rect.Add(image.Pt(0, shadowDrop/Half)), dialogRadius, HexAlpha(0, shadowAlpha))
	Box(screen, m.rect, ColorBackground())
	icons := slices.ContainsFunc(m.Items, func(item MenuItem) bool { return item.Icon != IconNone })
	for index, item := range m.Items {
		rect := m.itemRect(index)
		if item.Rule && index > 0 {
			Divider(screen, image.Pt(rect.Min.X, rect.Min.Y), image.Pt(rect.Max.X, rect.Min.Y))
		}
		if index == m.hover {
			FillRound(screen, rect, rowRadius, ColorHover())
		}
		left := rect.Min.X + menuPad*Half
		if item.Icon != IconNone {
			DrawIcon(screen, item.Icon, image.Pt(left, rect.Min.Y+(rect.Dy()-IconSize)/Half))
		}
		if icons {
			left += IconSize + treeLabelGap
		}
		ink := ColorInk()
		switch {
		case item.Disabled:
			ink = ColorSubdued()
		case item.Danger:
			ink = ColorWarn()
		}
		theme.DrawText(screen, item.Label, left, rect.Min.Y+(RowHeight-TextSize)/Half-1, ink)
	}
}

// MenuWidth returns a width that fits every item's label, after the icon
// column when any item has an icon, and at least the default width.
func MenuWidth(items []MenuItem, theme *Theme) int {
	icons := slices.ContainsFunc(items, func(item MenuItem) bool { return item.Icon != IconNone })
	width := menuWidth
	for _, item := range items {
		labelWidth := theme.TextWidth(item.Label) + Half*Half*menuPad
		if icons {
			labelWidth += IconSize + treeLabelGap
		}
		width = max(width, labelWidth)
	}

	return width
}
