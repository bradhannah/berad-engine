package browser

import (
	"image"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Rows returns the main pane's rows, for tests.
func (s *State) Rows(docs frame.Docs) []ui.Row {
	s.layout(docs)

	return s.list.Rows
}

// Places returns the places column's rows, for tests.
func (s *State) Places(docs frame.Docs) []ui.Row {
	s.layout(docs)

	return s.places.Rows
}

// PlacesLook reports how the places column draws its rows: the selected
// one filled as the place in hand, and each place's count as a cap.
func (s *State) PlacesLook() (bool, bool) { return s.places.InHand, s.places.Caps }

// Show shows a place in the main pane, for tests.
func (s *State) Show(place string) { s.pick(place) }

// SelectedRow returns the selected row, wherever it is listed, for tests: in
// the main pane, among the places, or a document not shown at the moment.
func (s *State) SelectedRow(docs frame.Docs) (ui.Row, bool) {
	if row, ok := s.list.SelectedRow(); ok {
		return row, true
	}
	for _, row := range s.places.Rows {
		if row.ID == s.list.Selected {
			return row, true
		}
	}
	if documentID, err := ids.Parse(s.list.Selected); err == nil {
		if entry, ok := s.index(docs).Entry(documentID); ok {
			return ui.NewRow(entry.ID.String(), entry.Name), true
		}
	}

	return ui.Row{}, false
}

// DialogLines returns the open dialog's lines, for tests.
func (s *State) DialogLines() []string {
	if s.dialog == nil {
		return nil
	}

	return s.dialog.Lines
}

// InspectorScroll returns how far the inspector is scrolled, and the
// furthest it scrolls, for tests.
func (s *State) InspectorScroll() (int, int) { return s.scroll.offset, s.scroll.most }

// LocateInPlace locates an inspector's widget without scrolling to it, for
// tests.
func (s *State) LocateInPlace(name string, docs frame.Docs) (image.Point, bool) {
	s.layout(docs)
	if point, found := s.locateConstant(name); found {
		return point, true
	}

	return s.locateWidget(strings.TrimPrefix(name, "ui:"))
}

// Showing is what the bar says it shows.
func (s *State) Showing() string { return s.showing() }

// Head names the places column, the search, the facets, the strip's path
// and buttons, and the main pane, as laid out.
func (s *State) Head(docs frame.Docs) []frametest.Part {
	s.layout(docs)
	parts := []frametest.Part{
		{Name: "places", Rect: s.places.Rect}, {Name: "search", Rect: s.search.Rect},
		{Name: "path", Rect: s.headPath}, {Name: "main pane", Rect: s.list.Rect},
	}
	for _, def := range barFacets() {
		parts = append(parts, frametest.Part{Name: def.label, Rect: s.facetBns[def.id].Rect})
	}
	for _, button := range s.sizeBns {
		parts = append(parts, frametest.Part{Name: button.Label, Rect: button.Rect})
	}
	for _, view := range []string{viewRows, viewCards, viewPictures, viewPreview} {
		parts = append(parts, frametest.Part{Name: viewLabel(view), Rect: s.viewBns[view].Rect})
	}

	return parts
}

// FacetCross returns the middle of the cross on a set facet's button, and
// whether the facet is set; FacetLook, whether its button is quiet and
// washed as set.
func (s *State) FacetCross(facet string) (image.Point, bool) {
	button := s.facetBns[facet]

	return ui.Center(facetCross(button)), button.On
}

// FacetLook reports whether a facet's button is quiet, and whether it shows
// a triangle to open it.
func (s *State) FacetLook(facet string) (bool, bool) {
	return s.facetBns[facet].Quiet, s.facetBns[facet].Caret
}

// ViewsJoined reports whether the views' buttons stand against one another
// as one control, and how many there are.
func (s *State) ViewsJoined() (int, bool) {
	shown, joined := s.viewsJoined()

	return len(shown), joined
}
