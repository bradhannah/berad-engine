package mapview

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

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

// Previewing reports whether the Map is previewing, for the window, which
// keeps a Map's panels for previewing apart from those for editing
// (increment 54).
func (s *State) Previewing() bool { return s.preview != nil }

// fitLists keeps the left panel, as laid out, from running under the
// status bar: when Show floor below and Preview would end past the panel's
// foot, they stay at its foot and the lists above them scroll (increment
// 61). In a window tall enough, nothing moves.
func (s *State) fitLists() {
	bottom := s.size.Y - frame.StatusHeight
	foot := bottom - ui.Inset
	s.lists.Body = image.Rect(0, 0, s.panelFull(), bottom)
	if s.lists.Offset == 0 && s.previewBn.Rect.Max.Y <= foot {
		s.lists.Most = 0

		return
	}
	s.previewBn.Rect = image.Rect(ui.Inset, foot-ui.ButtonH, s.panelFull()-ui.Inset, foot)
	top := s.previewBn.Rect.Min.Y - panelGap - ui.RowHeight
	s.below.Rect = image.Rect(s.below.Rect.Min.X, top, s.below.Rect.Max.X, top+ui.RowHeight)
	s.lists.Body.Max.Y = max(top-panelGap/ui.Half, 0)
	before := s.lists.Offset
	if s.lists.Fit(s.floorList.Rect.Max.Y + ui.Inset) {
		for _, list := range []*ui.List{s.toolList, s.tileList, s.floorList} {
			list.Rect = list.Rect.Add(image.Pt(0, before-s.lists.Offset))
		}
	}
}
