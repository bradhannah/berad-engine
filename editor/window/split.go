package window

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/editor/views/watchview"
)

// A tab can show two panes side by side (increment 10): Watch opens beside a
// Map of its zone, so both are seen while a preview runs. The divider between
// them drags, and a press in either focuses it.

// Split layout: a split tab has splitPanes panes, the divider between them is
// dividerWidth wide, and the left pane takes defaultSplit of the tab at first
// and between minSplit and maxSplit of it when the divider is dragged.
const (
	dividerWidth = 8
	splitPanes   = 2
	defaultSplit = 0.58
	minSplit     = 0.25
	maxSplit     = 0.75
)

// place shows a new pane: Watch beside a Map of its zone when one is open
// alone in its tab, as that tab's right pane, and any other pane in a tab of
// its own. The tab comes to the front; in a split the Map keeps the focus, so
// its arrow keys still walk the preview.
func (s *shell) place(shown *pane) {
	if shown.view == watchview.ViewName {
		for index, each := range s.tabs {
			if len(each.panes) == 1 && each.panes[0].view == mapview.ViewName && each.panes[0].target == shown.target {
				each.panes = append(each.panes, shown)
				s.active = index

				return
			}
		}
	}
	s.tabs = append(s.tabs, &tab{panes: []*pane{shown}, focus: 0, split: defaultSplit})
	s.active = len(s.tabs) - 1
}

// layoutTab gives a tab's panes their parts of an area: all of it for one
// pane, or each side of the divider for two.
func layoutTab(each *tab, area image.Rectangle) {
	if len(each.panes) < splitPanes {
		for _, shown := range each.panes {
			shown.rect = area
		}

		return
	}
	divider := dividerRect(each, area)
	each.panes[0].rect = image.Rect(area.Min.X, area.Min.Y, divider.Min.X, area.Max.Y)
	each.panes[1].rect = image.Rect(divider.Max.X, area.Min.Y, area.Max.X, area.Max.Y)
}

// dividerRect is where a split tab's divider is in an area.
func dividerRect(each *tab, area image.Rectangle) image.Rectangle {
	at := area.Min.X + int(float64(area.Dx())*each.split)

	return image.Rect(at-dividerWidth/ui.Half, area.Min.Y, at+dividerWidth/ui.Half, area.Max.Y)
}

// updateSplit drags the divider of the split tab in front, and focuses the
// pane a press lands in. Dragging keeps the mouse from the panes.
func (s *shell) updateSplit(input *ui.Input) {
	front := s.front()
	if front == nil || len(front.panes) < splitPanes {
		s.dragging = false

		return
	}
	area := s.area()
	divider := dividerRect(front, area)
	switch {
	case input.Pressed && input.Cursor.In(divider):
		s.dragging = true
	case s.dragging && !input.Down:
		s.dragging = false
	}
	if s.dragging || input.Cursor.In(divider) {
		ebiten.SetCursorShape(ebiten.CursorShapeEWResize)
	} else {
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
	}
	if s.dragging {
		share := float64(input.Cursor.X-area.Min.X) / float64(max(area.Dx(), 1))
		front.split = max(minSplit, min(maxSplit, share))
		quietMouse(input)

		return
	}
	if input.Pressed {
		for slot, shown := range front.panes {
			if input.Cursor.In(shown.rect) {
				front.focus = slot
			}
		}
	}
}

// drawSplit draws the divider of a split tab, and outlines its focused pane.
func (s *shell) drawSplit(screen *ebiten.Image, front *tab) {
	if len(front.panes) < splitPanes {
		return
	}
	divider := dividerRect(front, s.area())
	ui.FillRect(screen, divider, ui.ColorPanel())
	middle := divider.Min.X + dividerWidth/ui.Half
	grip := divider.Min.Y + divider.Dy()/ui.Half
	ui.Line(screen, image.Pt(middle, grip-ui.RowHeight), image.Pt(middle, grip+ui.RowHeight), ui.Outline, ui.ColorMuted())
	ui.StrokeRect(screen, front.panes[front.focus].rect, ui.Outline, ui.ColorSelect())
}
