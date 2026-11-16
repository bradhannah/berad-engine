package paletteview

import "github.com/bradhannah/berad-engine/editor/views/frame/frametest"

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
