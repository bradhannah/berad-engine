package paletteview

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// Chosen returns the chosen ramp and shade, for tests.
func (s *State) Chosen() (string, string) { return s.chosen.ramp.String(), s.chosen.shade.String() }

// Guarding reports whether the window asks about unsaved changes.
func (s *State) Guarding() bool { return s.guard.Active() }

// Toolbar names the toolbar's parts' places, and the ramps' area below it.
func (s *State) Toolbar() []frametest.Part {
	return []frametest.Part{
		{Name: "name", Rect: s.name.Rect}, {Name: "Default's name", Rect: s.bar.title},
		{Name: "+ Ramp", Rect: s.bar.addRamp.Rect}, {Name: "Gradient…", Rect: s.bar.gradient.Rect},
		{Name: "Copy colours from…", Rect: s.bar.copyFrom.Rect}, {Name: "Delete palette…", Rect: s.bar.remove.Rect},
		{Name: "Copy to palette", Rect: s.bar.copyTo.Rect}, {Name: "New palette from these", Rect: s.bar.newPalette.Rect},
		{Name: "ramps", Rect: s.rowsArea()},
	}
}

// InspectorParts returns where the inspector's fields scroll, how far and
// the furthest, and the save bar at its foot.
func (s *State) InspectorParts() (image.Rectangle, int, int, frametest.Part) {
	return s.body.Body, s.body.Offset, s.body.Most, frametest.Part{Name: "save bar", Rect: s.save.Rect}
}

// GradientParts names the open gradient panel's parts, as laid out, and the
// panel.
func (s *State) GradientParts() (image.Rectangle, []frametest.Part) {
	grad := s.gradient
	parts := []frametest.Part{
		{Name: "Colour", Rect: grad.mixColor.Rect}, {Name: "Opacity", Rect: grad.mixOpacity.Rect},
		{Name: "left swatch", Rect: grad.leftSwatch}, {Name: "right swatch", Rect: grad.rightSwatch},
		{Name: "picker", Rect: grad.picker.Rect},
	}
	for _, field := range grad.fields() {
		parts = append(parts, frametest.Part{Name: field.ID, Rect: field.Rect})
	}
	for _, button := range grad.buttons() {
		parts = append(parts, frametest.Part{Name: button.ID, Rect: button.Rect})
	}

	return s.panelRect(), parts
}
