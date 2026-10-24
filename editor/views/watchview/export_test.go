package watchview

import "github.com/bradhannah/berad-engine/editor/views/frame"

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
