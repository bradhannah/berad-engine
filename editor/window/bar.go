package window

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/audioview"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/editor/views/textview"
	"github.com/bradhannah/berad-engine/editor/views/watchview"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
)

// Window bar layout, in screen pixels: the bar is barHeight tall, and its
// tabs share its width, up to tabWidth each.
const (
	barHeight = 32
	tabWidth  = 220
	tabPad    = 10
	closeSize = 14
)

// viewIcon is the icon of a kind of window, shown on its tab.
func viewIcon(view string) ui.Icon {
	switch view {
	case browser.ViewName:
		return ui.IconProject
	case mapview.ViewName:
		return ui.IconZone
	case textview.ViewName:
		return ui.IconText
	case workflowview.ViewName:
		return ui.IconWorkflow
	case audioview.ViewName:
		return ui.IconSound
	case clockview.ViewName:
		return ui.IconClock
	case watchview.ViewName:
		return ui.IconWatch
	}

	return ui.IconNone
}

// tabRect returns a tab's place on the bar, left of the clock's readout.
func (s *shell) tabRect(index int) image.Rectangle {
	width := tabWidth
	if count := len(s.tabs); count > 0 && (s.size.X-readoutWidth)/count < width {
		width = (s.size.X - readoutWidth) / count
	}

	return image.Rect(index*width, 0, (index+1)*width, barHeight)
}

// closable reports whether a tab closes from the bar: every tab except the
// project browser's, which closes the editor.
func (s *shell) closable(index int) bool {
	for _, shown := range s.tabs[index].panes {
		if shown.host.closeQuits {
			return false
		}
	}

	return true
}

// closeRect returns a tab's close box.
func (s *shell) closeRect(index int) image.Rectangle {
	rect := s.tabRect(index)
	top := (barHeight - closeSize) / ui.Half

	return image.Rect(rect.Max.X-tabPad-closeSize, top, rect.Max.X-tabPad, top+closeSize)
}

// updateBar handles a press on the bar: a tab comes to the front, or its
// panes are asked to close from its close box, which asks about unsaved
// changes first. The bar keeps the mouse from the panes.
func (s *shell) updateBar(input *ui.Input) {
	if !input.Cursor.In(image.Rect(0, 0, s.size.X, barHeight)) {
		return
	}
	if input.Pressed {
		s.pressTab(input.Cursor)
		s.pressReadout(input.Cursor)
	}
	quietMouse(input)
}

func (s *shell) pressTab(point image.Point) {
	for index, each := range s.tabs {
		if !point.In(s.tabRect(index)) {
			continue
		}
		s.active = index
		if s.closable(index) && point.In(s.closeRect(index)) {
			for _, shown := range each.panes {
				shown.host.frame.Closing = true
			}
		}
	}
}

// quietMouse keeps only a frame's keys and typed text.
func quietMouse(input *ui.Input) {
	input.Cursor = outside()
	input.Down, input.Pressed, input.Released, input.DoubleClick = false, false, false, false
	input.Secondary, input.PanDown = false, false
	input.WheelX, input.WheelY = 0, 0
}

// drawBar draws the window bar: a tab for each window, the one in front
// underlined.
func (s *shell) drawBar(screen *ebiten.Image) {
	ui.FillRect(screen, image.Rect(0, 0, s.size.X, barHeight), ui.ColorPanel())
	for index := range s.tabs {
		s.drawTab(screen, index)
	}
	s.drawReadout(screen)
	ui.Divider(screen, image.Pt(0, barHeight-ui.Hairline), image.Pt(s.size.X, barHeight-ui.Hairline))
}

func (s *shell) drawTab(screen *ebiten.Image, index int) {
	rect := s.tabRect(index)
	if index == s.active {
		ui.FillRect(screen, rect, ui.ColorBackground())
		ui.FillRect(screen, image.Rect(rect.Min.X, rect.Max.Y-ui.Outline, rect.Max.X, rect.Max.Y), ui.ColorSelect())
	}
	textLeft := rect.Min.X + tabPad
	for _, shown := range s.tabs[index].panes {
		ui.DrawIcon(screen, viewIcon(shown.view), image.Pt(textLeft, (barHeight-ui.IconSize)/ui.Half))
		textLeft += ui.IconSize + ui.Gap
	}
	textLeft += ui.Gap
	textRight := rect.Max.X - tabPad
	if s.closable(index) {
		textRight -= closeSize + ui.Gap
		drawClose(screen, s.closeRect(index))
	}
	label := s.theme.FitText(s.label(index), textRight-textLeft)
	s.theme.DrawText(screen, label, textLeft, (barHeight-ui.TextSize)/ui.Half-1, ui.ColorInk())
	ui.Divider(screen, image.Pt(rect.Max.X-ui.Hairline, ui.Gap), image.Pt(rect.Max.X-ui.Hairline, barHeight-ui.Gap))
}

// label names a tab after its focused pane; a split tab shows each pane's
// icon before it.
func (s *shell) label(index int) string {
	each := s.tabs[index]

	return each.panes[each.focus].place.title
}

// drawClose draws a close box's cross.
func drawClose(screen *ebiten.Image, rect image.Rectangle) {
	left, top := float32(rect.Min.X+ui.Gap), float32(rect.Min.Y+ui.Gap)
	right, bottom := float32(rect.Max.X-ui.Gap), float32(rect.Max.Y-ui.Gap)
	vector.StrokeLine(screen, left, top, right, bottom, ui.Outline, ui.ColorMuted(), true)
	vector.StrokeLine(screen, left, bottom, right, top, ui.Outline, ui.ColorMuted(), true)
}
