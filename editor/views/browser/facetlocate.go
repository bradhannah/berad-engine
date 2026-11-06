package browser

import (
	"image"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// stateKind starts what input scripts ask of the window's state.
const stateKind = "state"

// locateFacets finds, for input scripts, "facet:<Facet>/<value>", a value
// in an open facet's dropdown; "more:<Facet>", its "N more…"; "view:<name>",
// a saved view under the places; and "place:<name>", a place. It answers
// the narrowing's states too. handled is false for anything else.
func (s *State) locateFacets(kind, value string, docs frame.Docs) (image.Point, bool, bool) {
	none := image.Pt(0, 0)
	switch kind {
	case "facet":
		id, found := s.facetValueRow(value)
		if !found {
			return none, false, true
		}
		point, located := s.facets.Locate(id)

		return point, located, true
	case "more":
		point, located := s.facets.Locate(morePrefix + facetID(value))

		return point, located, true
	case "view":
		s.places.Reveal(viewPrefix + value)
		point, located := s.places.Locate(viewPrefix + value)

		return point, located, true
	case "place":
		point, located := s.locatePlace("row", value)

		return point, located, true
	case "sort":
		point, located := s.locateHeading(value)

		return point, located, true
	case "play":
		point, located := s.locatePlay(value)

		return point, located, true
	case stateKind:
		found, handled := s.hasNarrowingState(value, docs)

		return none, found, handled
	}

	return none, false, false
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

// facetValueRow finds the row of a value, "<Facet>/<value>", in an open
// facet's dropdown.
func (s *State) facetValueRow(named string) (string, bool) {
	label, value, _ := strings.Cut(named, "/")
	prefix := tickPrefix + facetID(label) + ":"
	for _, row := range s.facets.Rows {
		if strings.HasPrefix(row.ID, prefix) && row.Label == value {
			return row.ID, true
		}
	}

	return "", false
}

// facetValue finds the key of a facet's value by its label.
func facetValueKey(labels facetLabels, facet, label string) (string, bool) {
	for value, shown := range labels[facet] {
		if shown.label == label {
			return value, true
		}
	}

	return "", false
}

// hasNarrowingState answers "narrowed", while a search or a tick narrows;
// "showing=<n> of <m>"; "count:<Facet>/<value>=<n>", how many things a
// value would list; "ticked:<Facet>/<value>"; "view=<name>", the saved view
// now ticked; "place=<name>", the place shown; and "pane=<view>", the view
// the main pane is in.
func (s *State) hasNarrowingState(state string, docs frame.Docs) (bool, bool) {
	switch {
	case state == "narrowed":
		return s.narrowed(), true
	case strings.HasPrefix(state, "showing="):
		return s.showing() == strings.TrimPrefix(state, "showing="), true
	case strings.HasPrefix(state, "count:"):
		named, count, _ := strings.Cut(strings.TrimPrefix(state, "count:"), "=")
		label, valueLabel, _ := strings.Cut(named, "/")
		things, labels := s.things(docs)
		facet := facetID(label)
		value, known := facetValueKey(labels, facet, valueLabel)

		return known && strconv.Itoa(s.counts(things, facet)[value]) == count, true
	case strings.HasPrefix(state, "ticked:"):
		label, valueLabel, _ := strings.Cut(strings.TrimPrefix(state, "ticked:"), "/")
		_, labels := s.things(docs)
		facet := facetID(label)
		value, known := facetValueKey(labels, facet, valueLabel)

		return known && s.ticks[facet][value], true
	case strings.HasPrefix(state, "view="):
		current, saved := s.currentView(docs)

		return saved && current.Name == strings.TrimPrefix(state, "view="), true
	case strings.HasPrefix(state, "place="):
		return s.placeLabel() == strings.TrimPrefix(state, "place="), true
	case strings.HasPrefix(state, "pane="):
		return s.viewShown() == strings.TrimPrefix(state, "pane="), true
	case strings.HasPrefix(state, "sound:"):
		name, word, _ := strings.Cut(strings.TrimPrefix(state, "sound:"), "=")

		return s.hasSoundState(name, word), true
	}

	return false, false
}
