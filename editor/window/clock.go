package window

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
)

// The pinned clock (increment 10) is the Clock view, always running in a pane
// of its own: the window bar reads it out at its right end, on every tab, and
// its controls open in a panel under the readout.

// readoutWidth is the width of the clock's readout on the bar; its
// controls, an hour back and on and stopping its clock, sit left of it,
// controlWidth each (increment 41).
const (
	readoutWidth  = 210
	controlWidth  = 52
	clockControls = 3
	controlsWidth = controlWidth*clockControls + ui.Gap
)

// clockAsker is the Clock view's side of the bar's controls.
type clockAsker interface {
	Ask(action string)
	Controls() (bool, bool)
}

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
	return image.Rect(s.size.X-readoutWidth, 0, s.size.X, rowHeight)
}

// panelRect is where the clock's panel shows: under its readout, at the right
// of the window, as large as the Clock window was.
func (s *shell) panelRect() image.Rectangle {
	top := s.barHeight()

	return image.Rect(s.size.X-clockview.Width, top, s.size.X, top+clockview.Height)
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

// followClock keeps whether a preview runs. Starting one opens nothing
// (increment 41): the clock's panel opens only when its readout is pressed.
func (s *shell) followClock() {
	_, live := s.clockReadout()
	s.clockLive = live
}

// pressReadout opens or closes the clock's panel, or carries out one of the
// clock's controls on the bar.
func (s *shell) pressReadout(point image.Point) {
	if s.clock == nil {
		return
	}
	if point.In(s.readoutRect()) {
		s.clockOpen = !s.clockOpen

		return
	}
	asker, ok := s.clock.host.view.(clockAsker)
	if !ok {
		return
	}
	running, _ := asker.Controls()
	for _, control := range s.clockButtons() {
		if point.In(control.Rect) && (running || control.ID != clockview.AskStopStart) {
			asker.Ask(control.ID)
		}
	}
}

// clockButtons are the clock's controls on the bar: an hour back, an hour
// on, and Stop or Start for the preview's clock, which is pale while no
// preview runs.
func (s *shell) clockButtons() []*ui.Button {
	right := s.size.X - readoutWidth - ui.Gap
	labels := [][2]string{{clockview.AskHourBack, "−1h"}, {clockview.AskHourOn, "+1h"}, {clockview.AskStopStart, "Stop"}}
	buttons := make([]*ui.Button, 0, len(labels))
	top := (rowHeight - ui.ButtonH) / ui.Half
	for index, label := range labels {
		left := right - (len(labels)-index)*controlWidth
		button := ui.NewButton("clock-"+label[0], label[1])
		button.Rect = image.Rect(left+ui.Gap/ui.Half, top, left+controlWidth-ui.Gap/ui.Half, top+ui.ButtonH)
		buttons = append(buttons, button)
	}
	if asker, ok := s.clock.host.view.(clockAsker); ok {
		running, stopped := asker.Controls()
		buttons[len(buttons)-1].Disabled = !running
		if stopped {
			buttons[len(buttons)-1].Label = "Start"
		}
	}
	for _, button := range buttons {
		button.ID = strings.TrimPrefix(button.ID, "clock-")
	}

	return buttons
}

// drawClockButtons draws the clock's controls on the bar.
func (s *shell) drawClockButtons(screen *ebiten.Image) {
	if s.clock == nil {
		return
	}
	for _, button := range s.clockButtons() {
		button.Draw(screen, s.theme)
	}
}

// drawClock draws the clock's panel when it is open, outlined over the tab in
// front.
func (s *shell) drawClock(screen *ebiten.Image) {
	if !s.clockOpen || s.clock == nil {
		return
	}
	ui.Float(screen, s.clock.rect, 0)
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
	ui.Divider(screen, image.Pt(rect.Min.X, ui.Gap), image.Pt(rect.Min.X, rowHeight-ui.Gap))
	ui.DrawIcon(screen, ui.IconClock, image.Pt(rect.Min.X+tabPad, (rowHeight-ui.IconSize)/ui.Half))
	left := rect.Min.X + tabPad + ui.IconSize + ui.Gap*ui.Half
	s.theme.DrawText(screen, s.theme.FitText(text, rect.Max.X-tabPad-left), left, (rowHeight-ui.TextSize)/ui.Half-1, ink)
}
