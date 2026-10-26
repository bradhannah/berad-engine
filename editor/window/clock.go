package window

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
)

// The pinned clock (increment 10) is the Clock view, always running in a pane
// of its own: the window bar reads it out at its right end, on every tab, and
// its controls open in a panel under the readout.

// readoutWidth is the width of the clock's readout on the bar.
const readoutWidth = 210

// readout is a view that sums itself up for the window bar.
type readout interface {
	Readout() (string, bool)
}

// pinClock keeps a connected Clock window as the pinned clock.
func (s *shell) pinClock(conn *client.Client, args Args) error {
	shown, err := s.newPane(conn, args)
	if err != nil {
		return err
	}
	s.clock = shown

	return nil
}

// readoutRect is the clock's place at the right end of the bar.
func (s *shell) readoutRect() image.Rectangle {
	return image.Rect(s.size.X-readoutWidth, 0, s.size.X, barHeight)
}

// panelRect is where the clock's panel shows: under its readout, at the right
// of the window, as large as the Clock window was.
func (s *shell) panelRect() image.Rectangle {
	return image.Rect(s.size.X-clockview.Width, barHeight, s.size.X, barHeight+clockview.Height)
}

// clockReadout returns the pinned clock's readout and whether a preview runs.
func (s *shell) clockReadout() (string, bool) {
	if s.clock == nil {
		return "", false
	}
	if summary, ok := s.clock.host.view.(readout); ok {
		return summary.Readout()
	}

	return "", false
}

// followClock opens the clock's panel when a preview starts, as the Clock
// window opened by itself.
func (s *shell) followClock() {
	_, live := s.clockReadout()
	if live && !s.clockLive {
		s.clockOpen = true
	}
	s.clockLive = live
}

// pressReadout opens or closes the clock's panel.
func (s *shell) pressReadout(point image.Point) {
	if s.clock != nil && point.In(s.readoutRect()) {
		s.clockOpen = !s.clockOpen
	}
}

// drawClock draws the clock's panel when it is open, outlined over the tab in
// front.
func (s *shell) drawClock(screen *ebiten.Image) {
	if !s.clockOpen || s.clock == nil {
		return
	}
	drawPane(screen, s.clock)
	ui.StrokeRect(screen, s.clock.rect, ui.Outline, ui.ColorInk())
}

// drawReadout draws the pinned clock on the bar, in orange while a preview
// runs, and underlined while its panel is open.
func (s *shell) drawReadout(screen *ebiten.Image) {
	if s.clock == nil {
		return
	}
	text, live := s.clockReadout()
	rect := s.readoutRect()
	ink := ui.ColorInk()
	if live {
		ui.FillRect(screen, rect, ui.ColorLiveWash())
		ink = ui.ColorLive()
	}
	if s.clockOpen {
		ui.FillRect(screen, image.Rect(rect.Min.X, rect.Max.Y-ui.Outline, rect.Max.X, rect.Max.Y), ink)
	}
	ui.Divider(screen, image.Pt(rect.Min.X, ui.Gap), image.Pt(rect.Min.X, barHeight-ui.Gap))
	ui.DrawIcon(screen, ui.IconClock, image.Pt(rect.Min.X+tabPad, (barHeight-ui.IconSize)/ui.Half))
	left := rect.Min.X + tabPad + ui.IconSize + ui.Gap*ui.Half
	s.theme.DrawText(screen, s.theme.FitText(text, rect.Max.X-tabPad-left), left, (barHeight-ui.TextSize)/ui.Half-1, ink)
}
