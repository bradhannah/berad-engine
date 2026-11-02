package places

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// PickOnMap is the list's last row, which picks on a map instead.
const PickOnMap = "pick-on-map"

// SearchID is the list's search field, for input scripts.
const SearchID = "place-search"

// The list's size and the longest search.
const (
	listWidth = 300
	listRows  = 8
	maxSearch = 64
	headID    = "head:"
)

// List chooses a place by name: a search over every place, grouped by zone
// and floor, with Pick on map last. It opens under the box it chooses for.
type List struct {
	theme   *ui.Theme
	under   image.Rectangle
	current string
	search  *ui.TextField
	focus   ui.Focus
	list    *ui.List
	rect    image.Rectangle
}

// Open opens the list under a box, the current place marked chosen.
func Open(theme *ui.Theme, under image.Rectangle, current floor.AnchorRef) *List {
	search := ui.NewTextField(SearchID, textedit.NewField("", false, maxSearch), false, theme)
	search.Placeholder, search.Unmarked = "Find a place", true
	made := &List{
		theme: theme, under: under, current: "", search: search, focus: ui.Focus{}, list: ui.NewList("places", ""),
		rect: ui.Zero(),
	}
	if !current.IsZero() {
		made.current = ID(current)
	}
	made.focus.Register(search)
	made.focus.Set(search)

	return made
}

// Under is the box the list opens from.
func (l *List) Under() image.Rectangle { return l.under }

// Focused returns the search, which takes typing while the list is open.
func (l *List) Focused() *ui.TextField { return l.search }

// layout places the list under its box, or over it when there is no room,
// inside the window.
func (l *List) layout(window image.Rectangle) {
	width := max(l.under.Dx(), listWidth)
	height := ui.Gap + ui.ButtonH + ui.Gap + listRows*ui.RowHeight + ui.Gap
	left := min(l.under.Min.X, window.Max.X-ui.Gap-width)
	top := l.under.Max.Y + ui.Gap
	if top+height > window.Max.Y {
		top = max(window.Min.Y, l.under.Min.Y-ui.Gap-height)
	}
	l.rect = image.Rect(left, top, left+width, top+height)
	inner := l.rect.Inset(ui.Gap)
	l.search.Rect = image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, inner.Min.Y+ui.ButtonH)
	l.list.Rect = image.Rect(inner.Min.X, l.search.Rect.Max.Y+ui.Gap, inner.Max.X, inner.Max.Y)
}

// Update runs the list for a frame, taking all input: typing searches, a
// click or Enter chooses, and Escape or a click outside closes it. It
// returns the row chosen, a place's ID or PickOnMap, and whether the list
// is still open.
func (l *List) Update(input *ui.Input, ctx *frame.Context, window image.Rectangle, all []Place) (string, bool) {
	l.layout(window)
	defer input.ClearAll()
	if input.Has(ui.KeyEscape, ui.Plain()) || (input.Pressed && !input.Cursor.In(l.rect)) {
		return "", false
	}
	enter := input.Has(ui.KeyEnter, ui.Plain())
	l.focus.Update(input, ctx.Clipboard)
	if l.focus.Current() == nil {
		l.focus.Set(l.search)
	}
	l.list.Rows = l.rows(all)
	row := rowAt(l.list, input.Cursor)
	pressed := input.Pressed
	l.list.Update(input)
	switch {
	case pressed && row != "":
		return row, false
	case enter:
		for _, each := range l.list.Rows {
			if !each.Heading {
				return each.ID, false
			}
		}
	}

	return "", true
}

// rows lists the places whose names hold the search, under a heading for
// each zone and floor, then Pick on map.
func (l *List) rows(all []Place) []ui.Row {
	query := strings.ToLower(strings.TrimSpace(l.search.Field.Value()))
	var rows []ui.Row
	heading := ""
	for _, place := range all {
		if query != "" && !strings.Contains(strings.ToLower(place.Name), query) {
			continue
		}
		if place.Where != heading {
			heading = place.Where
			rows = append(rows, headingRow(headID+heading, heading))
		}
		row := ui.NewRow(ID(place.Ref), place.Name)
		row.Icon = ui.IconPlace
		if row.ID == l.current {
			row.Detail = "chosen"
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		empty := "No place matches."
		if len(all) == 0 {
			empty = "No places yet: add anchors to a map."
		}
		rows = append(rows, headingRow(headID, empty))
	}
	pick := ui.NewRow(PickOnMap, "Pick on map…")
	pick.Strong = true

	return append(rows, pick)
}

// headingRow is a row that cannot be chosen.
func headingRow(id, label string) ui.Row {
	row := ui.NewRow(id, label)
	row.Heading = true

	return row
}

// rowAt returns the row under a point that can be chosen, or "".
func rowAt(list *ui.List, point image.Point) string {
	if !point.In(list.Rect) {
		return ""
	}
	for index, row := range list.Rows {
		if !row.Heading && point.In(list.RowRect(index)) {
			return row.ID
		}
	}

	return ""
}

// Draw draws the list over the window.
func (l *List) Draw(screen *ebiten.Image) {
	ui.FillRound(screen, l.rect.Add(image.Pt(0, ui.Gap)), ui.Radius, ui.ColorShade())
	ui.Box(screen, l.rect, ui.ColorBackground())
	l.search.Draw(screen)
	l.list.Draw(screen, l.theme)
}

// Locate finds a part of the list for input scripts: "place:<name>", the
// search by its ID, or PickOnMap.
func (l *List) Locate(name string, window image.Rectangle, all []Place) (image.Point, bool) {
	l.layout(window)
	l.list.Rows = l.rows(all)
	if name == SearchID {
		return l.search.Center(), true
	}
	label, isPlace := strings.CutPrefix(name, "place:")
	for index, row := range l.list.Rows {
		if (row.ID == name || isPlace && row.Label == label && !row.Heading) && l.list.RowRect(index).Overlaps(l.list.Rect) {
			return ui.Center(l.list.RowRect(index)), true
		}
	}

	return image.Pt(0, 0), false
}
