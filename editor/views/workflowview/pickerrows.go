package workflowview

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// otherPrefix starts the row ID of a value of another type than the one an
// operand must match: shown on request, never picked.
const otherPrefix = "other:"

// pickerRows lists what the picker offers for the search typed: the values
// it matches of the types offered, under their headings; those of another
// type, when asked for; and, for a parameter, the rows that would declare
// the name typed. It keeps the values the type hides, to count them.
func (s *State) pickerRows(docs frame.Docs) []ui.Row {
	picker := s.values
	query := strings.ToLower(strings.TrimSpace(picker.search.Field.Value()))
	role := picker.role
	if picker.operand {
		role = roleCheck
	}
	var shown []valueOption
	picker.hidden = nil
	taken := map[string]bool{}
	for _, option := range s.valueOptions(docs, role) {
		taken[option.path] = true
		switch {
		case query != "" && !strings.Contains(option.path, query):
		case picker.offers(option):
			shown = append(shown, option)
		default:
			picker.hidden = append(picker.hidden, option)
		}
	}
	rows := optionRows(shown, false)
	if picker.others && picker.only != "" {
		rows = append(rows, optionRows(picker.hidden, true)...)
	}
	if !picker.operand {
		rows = append(rows, s.declareRows(docs, picker.role, query, taken)...)
	}

	return rows
}

// optionRows lists values under their headings with their type; values of
// another type are marked as never picked.
func optionRows(options []valueOption, other bool) []ui.Row {
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
		row := ui.NewRow(prefix+encodeRef(option.ref), option.path)
		row.Detail = option.declared.Type.Label()
		if other {
			row.Detail += " · cannot be picked here"
		}
		rows = append(rows, row)
	}

	return rows
}

// declareRows lists the rows that declare a value named by the search,
// where no value already has that path.
func (s *State) declareRows(docs frame.Docs, role valueRole, query string, taken map[string]bool) []ui.Row {
	var rows []ui.Row
	for _, target := range s.declareTargets(docs, role, query) {
		if taken[target.Path] {
			continue
		}
		label := "Declare " + target.Path
		if target.For != "" {
			label += " for " + s.ownerName(docs, target.Document)
		}
		rows = append(rows, ui.NewRow(declarePrefix+encodeTarget(target), label))
	}

	return rows
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
