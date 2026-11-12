package workflowview

// A Workflow's palette is as wide as the author drags it (increment 53); the
// window keeps the width for every Workflow, and a double-click on its
// divider puts back PaletteWidth.

// paletteFull is the palette's width while it shows.
func (s *State) paletteFull() int {
	if s.left > 0 {
		return s.left
	}

	return PaletteWidth
}

// LeftEdge is where the palette ends, while it shows unfolded; Watch's
// workflow has none.
func (s *State) LeftEdge() (int, bool) {
	return s.paletteFull(), s.live == nil && !s.paletteFolded
}

// SetLeftWidth sets the palette's width; zero is PaletteWidth.
func (s *State) SetLeftWidth(width int) { s.left = width }
