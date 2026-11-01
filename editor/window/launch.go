package window

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// Launch (increment 14) sits on the window bar left of the pinned clock: it
// opens the Play tab, or launches the open one again, as Cmd+R does in any
// tab.

// launchWidth is the width of Launch on the bar.
const launchWidth = 110

// shower is a view that stops while its tab is behind another, as the Play
// tab does.
type shower interface {
	SetShown(shown bool)
}

// launchRect is Launch's place on the bar, left of the clock's readout.
func (s *shell) launchRect() image.Rectangle {
	right := s.size.X - readoutWidth

	return image.Rect(right-launchWidth, 0, right, rowHeight)
}

// pressLaunch launches the game when Launch is pressed, through the project
// browser's pane, which is always open.
func (s *shell) pressLaunch(point image.Point) bool {
	if !point.In(s.launchRect()) {
		return false
	}
	for _, shown := range s.panes() {
		if shown.host != nil && shown.host.closeQuits {
			shown.host.launch()

			return true
		}
	}

	return false
}

// tellShown tells each view that stops while hidden whether its tab is in
// front.
func (s *shell) tellShown() {
	for index, each := range s.tabs {
		for _, shown := range each.panes {
			if view, ok := shown.host.view.(shower); ok {
				view.SetShown(index == s.active)
			}
		}
	}
}

// drawLaunch draws Launch on the bar.
func (s *shell) drawLaunch(screen *ebiten.Image) {
	rect := s.launchRect()
	ui.Divider(screen, image.Pt(rect.Min.X, ui.Gap), image.Pt(rect.Min.X, rowHeight-ui.Gap))
	ui.DrawIcon(screen, ui.IconPlay, image.Pt(rect.Min.X+tabPad, (rowHeight-ui.IconSize)/ui.Half))
	left := rect.Min.X + tabPad + ui.IconSize + ui.Gap*ui.Half
	s.theme.DrawText(screen, "Launch", left, (rowHeight-ui.TextSize)/ui.Half-1, ui.ColorInk())
}
