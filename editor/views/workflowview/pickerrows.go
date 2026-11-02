package workflowview

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
// the values it matches, flat. A rail beside them narrows to one scope
// and counts the values there.
const (
	// otherPrefix starts the row ID of a value of another type than the one
	// an operand must match: shown on request, never picked. groupPrefix
	// starts a group's row ID.
	otherPrefix = "other:"
	groupPrefix = "group:"
	railWidth   = 170
)

// railEntry is one scope on the picker's rail.
type railEntry struct {
	id    string
	label string
}

// pickerRail lists the scopes a role's values come from, after Everywhere.
func pickerRail(role valueRole) []railEntry {
	everywhere := railEntry{id: railEverywhere, label: "Everywhere"}
	local := railEntry{id: string(worldstate.ScopeLocal), label: "Local"}
	switch role {
	case roleKeep:
		return []railEntry{everywhere, local}
	case roleWhere:
		return []railEntry{everywhere, {id: string(worldstate.ScopeNPC), label: "The NPC it finds"}}
	case roleSet, roleCheck:
	}
	rail := []railEntry{
		everywhere, local, {id: string(worldstate.ScopeNPC), label: "This NPC"},
		{id: string(worldstate.ScopeZone), label: "This zone"}, {id: string(worldstate.ScopeWorld), label: "World"},
	}
	if role == roleCheck {
		rail = append(rail, railEntry{id: railOthers, label: "Other NPCs"}, railEntry{id: railZones, label: "Zones"})
	}

	return rail
}

// query is the search typed, as values are matched against it.
func (p *valuePicker) query() string {
	return strings.ToLower(strings.TrimSpace(p.search.Field.Value()))
}

// optionRole is whose values the picker offers: a Check value's, for an
// operand.
func (p *valuePicker) optionRole() valueRole {
	if p.operand {
		return roleCheck
	}

	return p.role
}

// pickerRows lists what the picker offers for the search typed: the values
// it matches of the types offered in the scope chosen, as a tree or flat,
// and those of another type, when asked for. It counts the values each
// scope has, keeps the ones the type hides, and where the name typed could
// be declared.
func (s *State) pickerRows(docs frame.Docs) []ui.Row {
	picker := s.values
	query := picker.query()
	var shown []valueOption
	picker.hidden, picker.counts = nil, map[string]int{}
	picker.paths, picker.parents = map[string]string{}, map[string]string{}
	taken := map[string]bool{}
	for _, option := range s.valueOptions(docs, picker.optionRole()) {
		taken[option.path] = true
		if query != "" && !strings.Contains(option.path, query) {
			continue
		}
		inScope := picker.scope == railEverywhere || option.rail == picker.scope
		switch {
		case picker.offers(option):
			picker.counts[option.rail]++
			picker.counts[railEverywhere]++
			if inScope {
				shown = append(shown, option)
			}
		case inScope:
			picker.hidden = append(picker.hidden, option)
		}
	}
	rows := picker.flatRows(shown, false)
	if query == "" {
		rows = picker.treeRows(shown)
	}
	if picker.others && picker.only != "" {
		rows = append(rows, picker.flatRows(picker.hidden, true)...)
	}
	picker.declare.matched = len(shown)
	s.setTargets(docs, s.freeTargets(docs, picker.optionRole(), query, taken))

	return rows
}

// valueRow is a value's row: its label, its type and limits at the right
// and its description after the label. A value of another type is marked
// as never picked.
func (p *valuePicker) valueRow(option valueOption, label string, indent int, other bool) ui.Row {
	rowID, detail := encodeRef(option.ref), typeDetail(option.declared)
	if other {
		rowID, detail = otherPrefix+rowID, detail+" · cannot be picked here"
	}
	row := ui.NewRow(rowID, label)
	row.Tree, row.Indent, row.Detail, row.Aside = true, indent, detail, option.declared.Description
	p.paths[rowID] = option.path

	return row
}

// flatRows lists values under their headings by their paths.
func (p *valuePicker) flatRows(options []valueOption, other bool) []ui.Row {
	var rows []ui.Row
	heading := ""
	prefix, suffix := "", ""
	if other {
		prefix, suffix = otherPrefix, " · ANOTHER TYPE"
	}
	for _, option := range options {
		if option.heading != heading {
			heading = option.heading
			rows = append(rows, ui.NewHeading(valueHeading+prefix+heading, heading+suffix))
		}
		rows = append(rows, p.valueRow(option, option.path, 0, other))
	}

	return rows
}

// treeRows lists values under their headings as a tree: a value at the top
// of its root by its path, and the values of a group under the group's row
// by their names from there, once the group is opened.
func (p *valuePicker) treeRows(options []valueOption) []ui.Row {
	var rows []ui.Row
	heading := ""
	listed := map[string]bool{}
	for index, option := range options {
		if option.heading != heading {
			heading = option.heading
			rows = append(rows, ui.NewHeading(valueHeading+heading, heading))
		}
		group := option.group()
		if group == "" {
			rows = append(rows, p.valueRow(option, option.path, 0, false))

			continue
		}
		key := groupPrefix + heading + "|" + group
		if listed[key] {
			continue
		}
		listed[key] = true
		var members []valueOption
		for _, member := range options[index:] {
			if member.heading == heading && member.group() == group {
				members = append(members, member)
			}
		}
		rows = append(rows, groupRow(key, group, len(members), p.open[key]))
		for _, member := range members {
			if p.open[key] {
				row := p.valueRow(member, strings.TrimPrefix(member.path, group+"."), 1, false)
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

// foldGroup opens or closes a group; the last one opened is remembered,
// while the window is open, and opened again in the next picker.
func (s *State) foldGroup(key string, open bool) {
	s.values.open[key] = open
	switch {
	case open:
		s.lastGroup = key
	case s.lastGroup == key:
		s.lastGroup = ""
	}
}

// moveBy moves the selection by the arrow keys: up and down a row; with
// nothing typed, right opens the selected group and left closes it, or goes
// from a value to its group. It reports whether a group was folded.
func (s *State) moveBy(keys pickerKeys) bool {
	picker := s.values
	selected := picker.list.Selected
	isGroup := strings.HasPrefix(selected, groupPrefix)
	switch {
	case keys.down:
		picker.move(1)
	case keys.up:
		picker.move(-1)
	case picker.query() != "":
	case keys.right && isGroup:
		s.foldGroup(selected, true)

		return true
	case keys.left && isGroup:
		s.foldGroup(selected, false)

		return true
	case keys.left && picker.parents[selected] != "":
		picker.list.Selected = picker.parents[selected]
	}

	return false
}

// layoutRail places the rail's scopes down the left of the list.
func (p *valuePicker) layoutRail(left, top int) {
	p.railRects = p.railRects[:0]
	for index := range p.rail {
		row := top + index*ui.RowHeight
		p.railRects = append(p.railRects, image.Rect(left, row, left+railWidth, row+ui.RowHeight))
	}
}

// updateRail narrows to the scope clicked, and reports whether one was.
func (p *valuePicker) updateRail(input *ui.Input) bool {
	for index, rect := range p.railRects {
		if input.Pressed && input.Cursor.In(rect) {
			input.ClearPointer()
			p.scope = p.rail[index].id

			return true
		}
	}

	return false
}

// drawRail draws the rail: each scope with how many values of the types
// offered match the search there, the one chosen washed.
func (s *State) drawRail(screen *ebiten.Image) {
	picker := s.values
	for index, entry := range picker.rail {
		rect := picker.railRects[index]
		if entry.id == picker.scope {
			ui.FillRound(screen, rect, ui.Radius, ui.ColorSelectWash())
			ui.StrokeRound(screen, rect, ui.Radius, ui.Hairline, ui.ColorSelect())
		}
		top := rect.Min.Y + (ui.RowHeight-ui.TextSize)/ui.Half - 1
		s.theme.DrawText(screen, entry.label, rect.Min.X+textPad, top, ui.ColorInk())
		count := strconv.Itoa(picker.counts[entry.id])
		s.theme.DrawText(screen, count, rect.Max.X-textPad-s.theme.TextWidth(count), top, ui.ColorMuted())
	}
}

// hiddenLine says what the line under the list shows: a note on the last
// row that could not be picked, or how many values the search matches that
// the type hides, and which.
func (p *valuePicker) hiddenLine() string {
	count := len(p.hidden)
	switch {
	case p.note != "":
		return p.note
	case count == 0:
		return ""
	case p.others && p.only != "":
		return "Values of another type are listed last, to see; they cannot be picked here."
	}
	paths := make([]string, 0, count)
	for _, option := range p.hidden {
		paths = append(paths, option.path)
	}

	return strconv.Itoa(count) + " more " + plural(count, "value", "values") + " of another type " +
		plural(count, "matches", "match") + ": " + strings.Join(paths, ", ")
}

// locatePart finds a part of the picker for input scripts: a type chip,
// "type:<type>", a group's row, "group:<path>", or a scope on the rail by
// its name, as "world" or "everywhere".
func (p *valuePicker) locatePart(value string) (image.Point, bool) {
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
			if strings.HasPrefix(row.ID, groupPrefix) && row.Label == named && p.list.RowRect(index).In(p.list.Rect) {
				return image.Pt(p.list.Rect.Min.X+railWidth/ui.Half, ui.Center(p.list.RowRect(index)).Y), true
			}
		}
	default:
		for index, entry := range p.rail {
			if entry.id == kind && index < len(p.railRects) {
				return ui.Center(p.railRects[index]), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// locateRow finds a value's row by its path, while the list shows.
func (p *valuePicker) locateRow(value string) (image.Point, bool) {
	for index, row := range p.list.Rows {
		rect := p.list.RowRect(index)
		if path, isValue := p.paths[row.ID]; isValue && path == value && rect.In(p.list.Rect) && !p.declare.showing() {
			return image.Pt(rect.Min.X+railWidth/ui.Half, ui.Center(rect).Y), true
		}
	}

	return image.Pt(0, 0), false
}
