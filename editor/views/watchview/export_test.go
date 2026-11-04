package watchview

import (
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Way returns the middle line of the footer, its label and its text fitted
// to a width, for tests.
func (s *State) Way(docs frame.Docs, width int) (string, string) {
	entity, ok := s.watched()
	if !ok {
		return "", ""
	}
	label, parts := s.way(docs, entity)

	return label, s.fit(parts, width)
}

// StateNow returns the preview state the window holds, for tests.
func (s *State) StateNow() *protocol.PreviewState { return s.state }

// LiveLanes returns the nodes the shown workflow marks for a Together's
// lanes, for tests.
func (s *State) LiveLanes() []ids.ULID {
	if s.flow == nil {
		return nil
	}

	return s.flow.LiveLanes()
}
