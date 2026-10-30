package paletteview

// Chosen returns the chosen ramp and shade, for tests.
func (s *State) Chosen() (string, string) { return s.chosen.ramp.String(), s.chosen.shade.String() }

// Guarding reports whether the window asks about unsaved changes.
func (s *State) Guarding() bool { return s.guard.Active() }
