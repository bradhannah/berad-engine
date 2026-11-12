package mapview

// A Map's left panel is as wide as the author drags it (increment 53); the
// window keeps the width for every Map, and a double-click on its divider
// puts back PanelWidth.

// panelFull is the left panel's width while it shows.
func (s *State) panelFull() int {
	if s.left > 0 {
		return s.left
	}

	return PanelWidth
}

// LeftEdge is where the left panel ends, while it shows unfolded.
func (s *State) LeftEdge() (int, bool) { return s.panelFull(), !s.fold.folded }

// SetLeftWidth sets the left panel's width; zero is PanelWidth.
func (s *State) SetLeftWidth(width int) { s.left = width }
