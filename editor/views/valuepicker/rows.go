package valuepicker

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The picker's rows: with nothing typed, its values as a tree, a group's
// values folded under the group's row until it is opened; with a search,
// the values it matches, flat. A rail beside them narrows to one scope and
// counts the values there.
const (
	// headingPrefix starts a heading's row ID; otherPrefix starts the row ID
	// of a value of another type than the one the picker must offer, shown
	// on request, never picked; groupPrefix starts a group's.
	headingPrefix = "values:"
	otherPrefix   = "other:"
	groupPrefix   = "group:"
	railWidth     = 170
)

// query is the search typed, as values are matched against it.
func (p *Picker) query() string {
	return strings.ToLower(strings.TrimSpace(p.search.Field.Value()))
}

// rows lists what the picker offers for the search typed: the values it
// matches of the types offered in the scope chosen, as a tree or flat, and
// those of another type, when asked for. It counts the values each scope
// has, keeps the ones the type hides, and where the name typed could be
// declared.
func (p *Picker) rows(docs frame.Docs) []ui.Row {
	query := p.query()
	var shown []Option
	p.hidden, p.counts = nil, map[string]int{}
	p.options, p.paths, p.parents = map[string]Option{}, map[string]string{}, map[string]string{}
	taken := map[string]bool{}
	for _, option := range p.source.Options(docs) {
		taken[option.Path] = true
		if query != "" && !strings.Contains(option.Path, query) &&
			!strings.Contains(strings.ToLower(option.Words), strings.ToLower(query)) {
			continue
		}
		inScope := p.scope == Everywhere || option.Rail == p.scope
		switch {
		case p.offers(option):
			p.counts[option.Rail]++
			p.counts[Everywhere]++
			if inScope {
				shown = append(shown, option)
			}
		case inScope:
			p.hidden = append(p.hidden, option)
		}
	}
	rows := p.flatRows(shown, false)
	if query == "" {
		rows = p.treeRows(shown)
	}
	if p.others && p.config.Only != "" {
		rows = append(rows, p.flatRows(p.hidden, true)...)
	}
	p.declare.matched = len(shown)
	var free []Target
	if query != "" {
		for _, target := range p.source.Targets(docs, query) {
			if !taken[target.Path] {
				free = append(free, target)
			}
		}
	}
	p.declare.setTargets(free)

	return rows
}

// valueRow is a value's row: its label, its type and limits at the right
// and its description after the label. A value of another type is marked
// as never picked.
func (p *Picker) valueRow(option Option, label string, indent int, other bool) ui.Row {
	rowID, detail := option.ID, TypeDetail(option.Declared)
	if option.Detail != "" {
		detail += " · " + option.Detail
	}
	if other {
		rowID, detail = otherPrefix+rowID, detail+" · cannot be picked here"
	}
	row := ui.NewRow(rowID, label)
	row.Tree, row.Indent, row.Detail, row.Aside = true, indent, detail, option.Declared.Description
	p.paths[rowID] = option.Path
	if !other {
		p.options[rowID] = option
	}

	return row
}

// TypeDetail says a value's type and its limits, as integer · 0 to 5.
func TypeDetail(declared worldstate.Declaration) string {
	parts := []string{declared.Type.Label()}
	switch {
	case declared.Min != nil && declared.Max != nil:
		parts = append(parts, strconv.Itoa(*declared.Min)+" to "+strconv.Itoa(*declared.Max))
	case declared.Min != nil:
		parts = append(parts, "at least "+strconv.Itoa(*declared.Min))
	case declared.Max != nil:
		parts = append(parts, "at most "+strconv.Itoa(*declared.Max))
	}

	return strings.Join(parts, " · ")
}

// flatRows lists values under their headings by their paths.
func (p *Picker) flatRows(options []Option, other bool) []ui.Row {
	var rows []ui.Row
	heading := ""
	prefix, suffix := "", ""
	if other {
		prefix, suffix = otherPrefix, " · ANOTHER TYPE"
	}
	for _, option := range options {
		if option.Heading != heading {
			heading = option.Heading
			rows = append(rows, ui.NewHeading(headingPrefix+prefix+heading, heading+suffix))
		}
		rows = append(rows, p.valueRow(option, option.Path, 0, other))
	}

	return rows
}

// treeRows lists values under their headings as a tree: a value at the top
// of its root by its path, and the values of a group under the group's row
// by their names from there, once the group is opened.
func (p *Picker) treeRows(options []Option) []ui.Row {
	var rows []ui.Row
	heading := ""
	listed := map[string]bool{}
	for index, option := range options {
		if option.Heading != heading {
			heading = option.Heading
			rows = append(rows, ui.NewHeading(headingPrefix+heading, heading))
		}
		group := option.group()
		if group == "" {
			rows = append(rows, p.valueRow(option, option.Path, 0, false))

			continue
		}
		key := groupPrefix + heading + "|" + group
		if listed[key] {
			continue
		}
		listed[key] = true
		var members []Option
		for _, member := range options[index:] {
			if member.Heading == heading && member.group() == group {
				members = append(members, member)
			}
		}
		rows = append(rows, groupRow(key, group, len(members), p.open[key]))
		for _, member := range members {
			if p.open[key] {
				row := p.valueRow(member, strings.TrimPrefix(member.Path, group+"."), 1, false)
				p.parents[row.ID] = key
				rows = append(rows, row)
			}
		}
	}

	return rows
}

// groupRow is a group's row: its path, strong, with a triangle and how many
// values it holds.
func groupRow(key, group string, count int, open bool) ui.Row {
	row := ui.NewRow(key, group)
	row.Tree, row.Strong, row.Toggle = true, true, ui.DisclosureClosed
	row.Detail = strconv.Itoa(count) + " " + plural(count, "value", "values")
	if open {
		row.Toggle = ui.DisclosureOpen
	}

	return row
}

// plural picks the singular or plural of a word for a count.
func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}

// foldGroup opens or closes a group; the last one opened is kept by the
// window, and opened again in the next picker.
func (p *Picker) foldGroup(key string, open bool) {
	p.open[key] = open
	if p.config.LastGroup == nil {
		return
	}
	switch {
	case open:
		*p.config.LastGroup = key
	case *p.config.LastGroup == key:
		*p.config.LastGroup = ""
	}
}

// moveBy moves the selection by the arrow keys: up and down a row; with
// nothing typed, right opens the selected group and left closes it, or goes
// from a value to its group. It reports whether a group was folded.
func (p *Picker) moveBy(pressed keys) bool {
	selected := p.list.Selected
	isGroup := strings.HasPrefix(selected, groupPrefix)
	switch {
	case pressed.down:
		p.move(1)
	case pressed.up:
		p.move(-1)
	case p.query() != "":
	case pressed.right && isGroup:
		p.foldGroup(selected, true)

		return true
	case pressed.left && isGroup:
		p.foldGroup(selected, false)

		return true
	case pressed.left && p.parents[selected] != "":
		p.list.Selected = p.parents[selected]
	}

	return false
}

// layoutRail places the rail's scopes down the left of the list.
func (p *Picker) layoutRail(left, top int) {
	p.railRects = p.railRects[:0]
	for index := range p.config.Rail {
		row := top + index*ui.RowHeight
		p.railRects = append(p.railRects, image.Rect(left, row, left+railWidth, row+ui.RowHeight))
	}
}

// updateRail narrows to the scope clicked, and reports whether one was.
func (p *Picker) updateRail(input *ui.Input) bool {
	for index, rect := range p.railRects {
		if input.Pressed && input.Cursor.In(rect) {
			input.ClearPointer()
			p.scope = p.config.Rail[index].ID

			return true
		}
	}

	return false
}

// drawRail draws the rail: each scope with how many values of the types
// offered match the search there, the one chosen washed.
func (p *Picker) drawRail(screen *ebiten.Image) {
	for index, entry := range p.config.Rail {
		rect := p.railRects[index]
		if entry.ID == p.scope {
			ui.FillRound(screen, rect, ui.Radius, ui.ColorSelectWash())
			ui.StrokeRound(screen, rect, ui.Radius, ui.Hairline, ui.ColorSelect())
		}
		top := rect.Min.Y + (ui.RowHeight-ui.TextSize)/ui.Half - 1
		p.theme.DrawText(screen, entry.Label, rect.Min.X+textPad, top, ui.ColorInk())
		count := strconv.Itoa(p.counts[entry.ID])
		p.theme.DrawText(screen, count, rect.Max.X-textPad-p.theme.TextWidth(count), top, ui.ColorMuted())
	}
}

// hiddenLine says what the line under the list shows: a note on the last
// row that could not be picked, or how many values the search matches that
// the type hides, and which.
func (p *Picker) hiddenLine() string {
	count := len(p.hidden)
	switch {
	case p.note != "":
		return p.note
	case count == 0:
		return ""
	case p.others && p.config.Only != "":
		return "Values of another type are listed last, to see; they cannot be picked here."
	}
	paths := make([]string, 0, count)
	for _, option := range p.hidden {
		paths = append(paths, option.Path)
	}

	return strconv.Itoa(count) + " more " + plural(count, "value", "values") + " of another type " +
		plural(count, "matches", "match") + ": " + strings.Join(paths, ", ")
}

// locatePart finds a part of the picker for input scripts: a type chip,
// "type:<type>", a group's row, "group:<path>", or a scope on the rail by
// its ID, as "world" or "everywhere".
func (p *Picker) locatePart(value string) (image.Point, bool) {
	kind, named, _ := strings.Cut(value, ":")
	switch kind {
	case "type":
		for _, chip := range p.chips {
			if chip.ID == chipPrefix+named && !chip.Rect.Empty() {
				return ui.Center(chip.Rect), true
			}
		}
	case "group":
		for index, row := range p.list.Rows {
			if strings.HasPrefix(row.ID, groupPrefix) && (row.Label == named || row.Label == worldstate.ValuePrefix+named) &&
				p.list.RowRect(index).In(p.list.Rect) {
				return image.Pt(p.list.Rect.Min.X+railWidth/ui.Half, ui.Center(p.list.RowRect(index)).Y), true
			}
		}
	default:
		for index, entry := range p.config.Rail {
			if entry.ID == kind && index < len(p.railRects) {
				return ui.Center(p.railRects[index]), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// locateRow finds a value's row by its path, while the list shows.
func (p *Picker) locateRow(value string) (image.Point, bool) {
	for index, row := range p.list.Rows {
		rect := p.list.RowRect(index)
		if path, isValue := p.paths[row.ID]; isValue && path == value && rect.In(p.list.Rect) && !p.declare.showing() {
			return image.Pt(rect.Min.X+railWidth/ui.Half, ui.Center(rect).Y), true
		}
	}

	return image.Pt(0, 0), false
}
