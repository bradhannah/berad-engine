package browser

import (
	"image"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// stateKind starts what input scripts ask of the window's state.
const stateKind = "state"

// locateFacets finds, for input scripts, "facet:<Facet>/<value>", a value's
// row in the facets column; "more:<Facet>", its "N more…"; "view:<name>", a
// saved view; and "chip:<Facet>: <value>", a chip in the table's bar. It
// answers the table's states too. handled is false for anything else.
func (s *State) locateFacets(kind, value string, docs frame.Docs) (image.Point, bool, bool) {
	none := image.Pt(0, 0)
	switch kind {
	case "facet":
		row, found := s.facetValueRow(value)
		if !found {
			return none, false, true
		}
		point, located := s.locateFacetRow(row.ID)

		return point, located, true
	case "more":
		point, located := s.locateFacetRow(morePrefix + facetID(value))

		return point, located, true
	case "view":
		point, located := s.locateFacetRow(viewPrefix + value)

		return point, located, true
	case "chip":
		for _, each := range s.chips {
			if each.label == value {
				return ui.Center(each.rect), true, true
			}
		}

		return none, false, true
	case stateKind:
		found, handled := s.hasTableState(value, docs)

		return none, found, handled
	}

	return none, false, false
}

// locateFacetRow scrolls a row of the facets column into view and finds it.
func (s *State) locateFacetRow(id string) (image.Point, bool) {
	s.facets.Reveal(id)

	return s.facets.Locate(id)
}

// facetID returns the key of the facet with a heading, or the heading.
func facetID(label string) string {
	for _, def := range facetDefs() {
		if def.label == label {
			return def.id
		}
	}

	return label
}

// facetValueRow finds the row of a value, "<Facet>/<value>", in the facets
// column.
func (s *State) facetValueRow(named string) (ui.Row, bool) {
	label, value, _ := strings.Cut(named, "/")
	prefix := tickPrefix + facetID(label) + ":"
	for _, row := range s.facets.Rows {
		if strings.HasPrefix(row.ID, prefix) && row.Label == value {
			return row, true
		}
	}

	return ui.NewRow("", ""), false
}

// hasTableState answers "table", true while the list is a table; "facets",
// while the facets column shows; "showing=<n> of <m>"; "count:<Facet>/
// <value>=<n>", a value's count; "ticked:<Facet>/<value>"; "cell:<name>/
// <Column>=<text>", a table row's cell; and "view=<name>", the saved view
// now ticked.
func (s *State) hasTableState(state string, docs frame.Docs) (bool, bool) {
	switch {
	case state == "table":
		return s.narrowed(), true
	case state == "facets":
		return s.withFacets, true
	case strings.HasPrefix(state, "showing="):
		return s.showing() == strings.TrimPrefix(state, "showing="), true
	case strings.HasPrefix(state, "count:"):
		named, count, _ := strings.Cut(strings.TrimPrefix(state, "count:"), "=")
		row, found := s.facetValueRow(named)

		return found && row.Detail == count, true
	case strings.HasPrefix(state, "ticked:"):
		row, found := s.facetValueRow(strings.TrimPrefix(state, "ticked:"))

		return found && row.Ticked, true
	case strings.HasPrefix(state, "cell:"):
		return s.hasCell(strings.TrimPrefix(state, "cell:")), true
	case strings.HasPrefix(state, "view="):
		current, saved := s.currentView(docs)

		return saved && current.Name == strings.TrimPrefix(state, "view="), true
	}

	return false, false
}

// hasCell answers "<name>/<Column>=<text>": the table row named has the
// text in the column.
func (s *State) hasCell(asked string) bool {
	named, text, _ := strings.Cut(asked, "=")
	name, title, _ := strings.Cut(named, "/")
	for position, each := range s.columns {
		if each.title != title {
			continue
		}
		for _, row := range s.list.Rows {
			if row.Label == name && position < len(row.Cells) && row.Cells[position] == text {
				return true
			}
		}
	}

	return false
}
