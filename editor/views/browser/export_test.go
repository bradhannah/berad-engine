package browser

import (
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
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
