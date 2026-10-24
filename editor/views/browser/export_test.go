package browser

import (
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Rows returns the list's rows, for tests.
func (s *State) Rows(docs frame.Docs) []ui.Row { return s.rows(docs) }

// SelectedRow returns the selected row, for tests.
func (s *State) SelectedRow() (ui.Row, bool) { return s.list.SelectedRow() }
