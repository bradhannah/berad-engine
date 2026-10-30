package mapview

import (
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The inspector lists the palettes the zone uses while nothing on the map is
// selected (increment 12), below the zone's name: the same list as the
// project browser's inspector.

// zonePalettesShown reports whether the inspector shows the zone's palettes:
// nothing is selected, picked or being resized.
func (s *State) zonePalettesShown() bool {
	return s.pick == nil && s.anchor.IsZero() && s.placement.IsZero() && len(s.selected) == 0 && s.tool != ToolResize
}

// zonePalettesTop is where the list starts: below the inspector's heading
// and the zone's name.
func zonePalettesTop() int { return ui.Inset + ui.Half*ui.LineHeight }

// layoutZonePalettes places the zone's palettes, or hides them.
func (s *State) layoutZonePalettes(docs frame.Docs) {
	inspector := frame.Inspector(s.size)
	left, width := inspector.Min.X+ui.Inset, inspector.Dx()-ui.Half*ui.Inset
	zoneID := ids.ULID{}
	if s.zonePalettesShown() {
		zoneID = s.zone
	}
	s.zoneListFoot = s.zonePalettes.Layout(docs, zoneID, left, inspector.Min.Y+zonePalettesTop(), width, s.size)
}
