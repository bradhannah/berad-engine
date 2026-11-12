package window

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// A split has one status bar (increment 52): the focused pane's, across the
// whole tab, under both panes. Each pane keeps its own messages, History
// and toggles, and the bar shows the focused one's; a press in the bar
// under the other pane focuses that pane first, as any press there does.

// updateStatusBar runs the status bar and History: across the pane alone,
// or, in a split, across the tab, in the tab's coordinates.
func (h *host) updateStatusBar(input *ui.Input) {
	if h.statusArea.Empty() {
		h.updateHistory(input, h.visible)
		h.frame.UpdateStatus(input, h.visible)

		return
	}
	shift := h.statusArea.Min
	input.Cursor = input.Cursor.Sub(shift)
	h.updateHistory(input, h.statusArea.Size())
	h.frame.UpdateStatus(input, h.statusArea.Size())
	input.Cursor = input.Cursor.Add(shift)
}

// shareStatus tells each pane of a split tab that the status bar spans the
// tab, and each pane alone that it has its own.
func (s *shell) shareStatus() {
	area := s.area()
	for _, each := range s.tabs {
		for _, shown := range each.panes {
			if shown.host == nil {
				continue
			}
			shown.host.statusArea = ui.Zero()
			if len(each.panes) >= splitPanes {
				shown.host.statusArea = area.Sub(shown.rect.Min)
			}
		}
	}
}

// drawSharedStatus draws the focused pane's status bar and History across a
// split tab.
func (s *shell) drawSharedStatus(screen *ebiten.Image, front *tab) {
	if len(front.panes) < splitPanes {
		return
	}
	area := s.area()
	if s.statusImage == nil || s.statusImage.Bounds().Size() != area.Size() {
		if s.statusImage != nil {
			s.statusImage.Deallocate()
		}
		s.statusImage = ebiten.NewImage(area.Dx(), area.Dy())
	}
	s.statusImage.Clear()
	focused := front.panes[front.focus].host
	focused.frame.DrawStatus(s.statusImage, focused.theme)
	focused.frame.DrawHistory(s.statusImage, focused.theme)
	var options ebiten.DrawImageOptions
	options.GeoM.Translate(float64(area.Min.X), float64(area.Min.Y))
	screen.DrawImage(s.statusImage, &options)
}

// abovePanesStatus is a split's area above its shared status bar, where the
// divider and the focused pane's outline are drawn.
func abovePanesStatus(rect image.Rectangle) image.Rectangle {
	return image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y-frame.StatusHeight)
}
