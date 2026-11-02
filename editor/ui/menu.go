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
//
// An item can open a submenu, set by SetSub, beside it when the pointer
// rests on it or it is clicked; a choice in the submenu is the menu's
// choice.
type Menu struct {
	Items  []MenuItem
	Width  int
	open   bool
	rect   image.Rectangle
	hover  int
	window image.Rectangle
	subs   map[string]subMenu
	child  *Menu
	parent string
}

// subMenu is the items a menu item opens beside it, and their width.
type subMenu struct {
	items []MenuItem
	width int
}

// SetSub sets the items an item opens beside it, and their width, until
// the menu is opened again without them; none clears it.
func (m *Menu) SetSub(id string, items []MenuItem, width int) {
	if m.subs == nil {
		m.subs = map[string]subMenu{}
	}
	m.subs[id] = subMenu{items: items, width: width}
}

// ClearSubs drops every submenu.
func (m *Menu) ClearSubs() { m.subs, m.child, m.parent = nil, nil, "" }

// Open shows the menu below an anchor, kept inside the window.
func (m *Menu) Open(anchor, window image.Rectangle) {
	width := m.width()
	left := min(anchor.Min.X, window.Max.X-width-Gap)
	m.rect = image.Rect(left, anchor.Max.Y+Gap, left+width, anchor.Max.Y+Gap+m.height())
	m.open, m.hover, m.window, m.child, m.parent = true, -1, window, nil, ""
}

// OpenAt shows the menu with its top-left corner at a point, moved left or
// up as needed to stay inside the window.
func (m *Menu) OpenAt(point image.Point, window image.Rectangle) {
	width, height := m.width(), m.height()
	left := max(min(point.X, window.Max.X-width-Gap), window.Min.X)
	top := max(min(point.Y, window.Max.Y-height-Gap), window.Min.Y)
	m.rect = image.Rect(left, top, left+width, top+height)
	m.open, m.hover, m.window, m.child, m.parent = true, -1, window, nil, ""
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
func (m *Menu) Close() { m.open, m.child, m.parent = false, nil, "" }

// itemRect is the rectangle of an item.
func (m *Menu) itemRect(index int) image.Rectangle {
	top := m.rect.Min.Y + menuPad + index*RowHeight

	return image.Rect(m.rect.Min.X+menuPad, top, m.rect.Max.X-menuPad, top+RowHeight)
}

// Update handles a frame of input while open and returns the chosen item's
// ID, or "". Either mouse button chooses; an item with a submenu opens it
// instead, as resting on it does.
func (m *Menu) Update(input *Input) string {
	if !m.open {
		return ""
	}
	defer input.ClearAll()
	if input.Consume(KeyEscape, Plain()) {
		m.Close()

		return ""
	}
	if m.child != nil && input.Cursor.In(m.child.rect) {
		return m.chooseInChild(input)
	}
	m.hoverAt(input.Cursor)
	m.openChild()
	if !input.Pressed && !input.Secondary {
		return ""
	}
	if m.child != nil && m.hover >= 0 && m.Items[m.hover].ID == m.parent {
		return ""
	}
	chosen := ""
	if m.hover >= 0 {
		chosen = m.Items[m.hover].ID
	}
	m.Close()

	return chosen
}

// hoverAt marks the item under the pointer, unless it is disabled.
func (m *Menu) hoverAt(cursor image.Point) {
	m.hover = -1
	for index, item := range m.Items {
		if cursor.In(m.itemRect(index)) && !item.Disabled {
			m.hover = index
		}
	}
}

// chooseInChild handles a frame of input on the open submenu; a click
// there closes both.
func (m *Menu) chooseInChild(input *Input) string {
	chosen := m.child.choose(input)
	if chosen != "" || input.Pressed || input.Secondary {
		m.Close()
	}

	return chosen
}

// openChild opens the submenu of the item the pointer rests on, beside
// it, and closes one whose item it has left for another.
func (m *Menu) openChild() {
	if m.hover < 0 {
		return
	}
	item := m.Items[m.hover]
	sub, has := m.subs[item.ID]
	switch {
	case !has:
		m.child, m.parent = nil, ""
	case m.parent != item.ID:
		child := &Menu{
			Items: sub.items, Width: sub.width, open: false, rect: Zero(), hover: -1, window: m.window,
			subs: nil, child: nil, parent: "",
		}
		row := m.itemRect(m.hover)
		child.OpenAt(image.Pt(m.rect.Max.X-menuPad, row.Min.Y-menuPad), m.window)
		m.child, m.parent = child, item.ID
	}
}

// choose handles a frame of input on an open submenu: it returns the item
// clicked, or "".
func (m *Menu) choose(input *Input) string {
	m.hoverAt(input.Cursor)
	if (!input.Pressed && !input.Secondary) || m.hover < 0 {
		return ""
	}

	return m.Items[m.hover].ID
}

// Locate returns the centre of an item, for input scripts.
func (m *Menu) Locate(wanted string) (image.Point, bool) {
	if m.child != nil && m.open {
		if point, found := m.child.Locate(wanted); found {
			return point, true
		}
	}
	for index, item := range m.Items {
		if item.ID == wanted && m.open {
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
		if _, has := m.subs[item.ID]; has {
			theme.DrawText(screen, "›", rect.Max.X-menuPad*Half-theme.TextWidth("›"), rect.Min.Y+(RowHeight-TextSize)/Half-1,
				ink)
		}
	}
	if m.child != nil {
		m.child.Draw(screen, theme)
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
