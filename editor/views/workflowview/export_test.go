package workflowview

// LayoutCount is how many times the window has laid the workflow out.
func (s *State) LayoutCount() int { return s.layouts }

// ZoomPercent is the canvas's zoom, as a percentage.
func (s *State) ZoomPercent() int { return zoomLevels()[s.zoom] }
