package stateview

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// InspectorParts returns where the inspector's fields scroll, and what stays
// at its foot, as laid out.
func (s *State) InspectorParts() (image.Rectangle, []frametest.Part) {
	s.layout()

	return s.scroll.Body, []frametest.Part{
		{Name: "save bar", Rect: s.save.Rect}, {Name: "Delete", Rect: s.form.Delete.Rect},
		{Name: "Delete signal", Rect: s.signal.remove.Rect},
	}
}

// InspectorScroll returns how far the inspector's fields are scrolled, and
// the furthest they go.
func (s *State) InspectorScroll() (int, int) { return s.scroll.Offset, s.scroll.Most }

// HeaderParts names the Add buttons, Find a value and the list, as laid
// out.
func (s *State) HeaderParts() []frametest.Part {
	s.layout()
	parts := []frametest.Part{{Name: "Find", Rect: s.findButton.Rect}, {Name: "list", Rect: s.list.Rect}}
	for _, button := range s.adds {
		parts = append(parts, frametest.Part{Name: button.Label, Rect: button.Rect})
	}

	return parts
}
