package audioview

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// Toolbar names the toolbar's buttons, as laid out, and the content they
// lie in.
func (s *State) Toolbar() (image.Rectangle, []frametest.Part) {
	s.layout()

	return frame.Content(s.size), append(named(s.addSound, s.newCue, s.checkFiles, s.loud.toggle, s.soundscape.toggle,
		s.stopAll), frametest.Part{Name: "title", Rect: s.titleAt})
}

// Panel names the panel's parts, as laid out, and the panel's area: its
// buttons, the recordings it plays, the waveform and its controls, and what
// is written beside them.
func (s *State) Panel() (image.Rectangle, []frametest.Part) {
	s.layout()
	trim := s.trim
	parts := named(append(s.panel.all(), trim.buttons()...)...)
	parts = append(parts, frametest.Part{Name: "recordings", Rect: s.played.Rect},
		frametest.Part{Name: "waveform", Rect: trim.wave.Rect}, frametest.Part{Name: "start", Rect: trim.start.Rect},
		frametest.Part{Name: "end", Rect: trim.end.Rect}, frametest.Part{Name: "Loop", Rect: trim.loop.Rect},
		frametest.Part{Name: "waveform's label", Rect: trim.labelAt},
		frametest.Part{Name: "Play it", Rect: trim.playItAt}, frametest.Part{Name: "ringed", Rect: trim.ringedAt},
		frametest.Part{Name: "note", Rect: trim.noteAt})

	return s.panelArea(), parts
}

// named names buttons by their labels.
func named(buttons ...*ui.Button) []frametest.Part {
	parts := make([]frametest.Part, 0, len(buttons))
	for _, button := range buttons {
		parts = append(parts, frametest.Part{Name: button.ID, Rect: button.Rect})
	}

	return parts
}

// InspectorParts returns where the inspector's fields scroll, how far and
// the furthest, and Delete at its foot.
func (s *State) InspectorParts() (image.Rectangle, int, int, frametest.Part) {
	s.layout()
	insp := s.inspector

	return insp.body.Body, insp.body.Offset, insp.body.Most, frametest.Part{Name: "Delete cue", Rect: insp.removeCue.Rect}
}
