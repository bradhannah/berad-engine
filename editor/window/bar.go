package window

import (
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// The window bar is one row of tabs (increment 38): every open document has
// a tab of its own, in the order they opened, the project browser's first.
// Tabs share the row's width, from tabWidth each down to leastTab; once they
// no longer fit at leastTab the last of them go under a "more" tab, which
// lists them.

// Window bar layout, in screen pixels.
const (
	rowHeight = 32
	tabWidth  = 220
	leastTab  = 120
	moreWidth = 72
	tabPad    = 10
	closeSize = 14
	// unsavedDot is the radius of the ink dot on a tab that holds changes
	// waiting for Save.
	unsavedDot = 4
	// liveWord marks a tab that is previewing.
	liveWord = "live"
	// moreTab is the bar tab that stands for the tabs that do not fit.
	moreTab = -1
)

// barTab is one tab drawn on the bar. tab is the index of the shell's tab it
// brings forward, or moreTab for the tab that lists those that do not fit,
// which then counts them in more.
type barTab struct {
	rect   image.Rectangle
	tab    int
	icons  []ui.Icon
	label  string
	more   int
	boxes  bool
	front  bool
	tint   ui.Tint
	tinted bool
	// unsaved is set when the tab holds changes that wait for Save, and
	// live while it previews.
	unsaved bool
	live    bool
}

// barHeight is how tall the bar is: one row.
func (s *shell) barHeight() int { return rowHeight }

// tabRoom is the width the bar's tabs share, left of Launch and the clock.
func (s *shell) tabRoom() int { return max(s.size.X-readoutWidth-launchWidth, 0) }

// fitting reports how many tabs the bar shows, and how wide each is: every
// tab while all fit at leastTab or more, and otherwise as many as fit beside
// the "more" tab.
func (s *shell) fitting() (int, int) {
	count, room := len(s.tabs), s.tabRoom()
	if count == 0 {
		return 0, tabWidth
	}
	if count*leastTab <= room {
		return count, min(tabWidth, room/count)
	}
	shown := max((room-moreWidth)/leastTab, 1)

	return shown, max((room-moreWidth)/shown, 1)
}

// barTabs lays out the bar's tabs, and the "more" tab when some do not fit.
func (s *shell) barTabs() []barTab {
	shown, width := s.fitting()
	labels := s.tabLabels()
	tabs := make([]barTab, 0, shown+1)
	for index := range shown {
		tint, tinted := s.tintOf(index)
		tabs = append(tabs, barTab{
			rect: image.Rect(index*width, 0, (index+1)*width, rowHeight), tab: index, icons: s.icons(index),
			label: labels[index], more: 0, boxes: s.closable(index), front: index == s.active, tint: tint,
			tinted: tinted, unsaved: s.unsaved(index), live: s.live(index),
		})
	}
	if hidden := len(s.tabs) - shown; hidden > 0 {
		left := shown * width
		tabs = append(tabs, barTab{
			rect: image.Rect(left, 0, left+moreWidth, rowHeight), tab: moreTab, icons: nil, label: "more",
			more: hidden, boxes: false, front: s.active >= shown, tint: noTint(), tinted: false, unsaved: false,
			live: false,
		})
	}

	return tabs
}

// noTint is the look of a tab with no tint.
func noTint() ui.Tint {
	return ui.Tint{Name: "", Wash: ui.ColorDesk(), Strong: ui.ColorPage(), Ink: ui.ColorInk()}
}

// icons are the icons a tab shows: its first pane's, and those of any pane
// beside it, such as Watch beside a Map.
func (s *shell) icons(index int) []ui.Icon {
	icons := make([]ui.Icon, 0, len(s.tabs[index].panes))
	for _, shown := range s.tabs[index].panes {
		icons = append(icons, viewIcon(shown.view))
	}

	return icons
}

// tabLabels names every tab by its focused pane's short title, keeping the
// window's ID only where two would read the same, as two views of one map
// do.
func (s *shell) tabLabels() []string {
	labels := make([]string, 0, len(s.tabs))
	times := map[string]int{}
	for index := range s.tabs {
		label := shortTitle(s.tabTitle(index), false)
		labels = append(labels, label)
		times[label]++
	}
	for index, label := range labels {
		if times[label] > 1 {
			labels[index] = shortTitle(s.tabTitle(index), true)
		}
	}

	return labels
}

// closable reports whether a tab closes from the bar: every tab except the
// project browser's, which closes the editor.
func (s *shell) closable(index int) bool {
	for _, shown := range s.tabs[index].panes {
		if shown.host != nil && shown.host.closeQuits {
			return false
		}
	}

	return true
}

// liver is a view that says when it runs a preview, as a Map does.
type liver interface {
	Live() bool
}

// live reports whether any pane of a tab runs a preview.
func (s *shell) live(index int) bool {
	for _, shown := range s.tabs[index].panes {
		if shown.host == nil {
			continue
		}
		if view, ok := shown.host.view.(liver); ok && view.Live() {
			return true
		}
	}

	return false
}

// closeBox is a bar tab's close box, at its right end.
func closeBox(rect image.Rectangle) image.Rectangle {
	top := rect.Min.Y + (rowHeight-closeSize)/ui.Half

	return image.Rect(rect.Max.X-tabPad-closeSize, top, rect.Max.X-tabPad, top+closeSize)
}

// updateBar handles a press on the bar: a tab comes to the front, the "more"
// tab ("3 more") lists the tabs that do not fit, or a tab's panes are asked to close
// from its close box, which asks about unsaved changes first. The bar keeps
// the mouse from the panes, and the "more" list, while open, all input.
func (s *shell) updateBar(input *ui.Input) {
	if s.more.IsOpen() {
		if chosen := s.more.Update(input); chosen != "" {
			if index, err := strconv.Atoi(chosen); err == nil && index < len(s.tabs) {
				s.choose(index)
			}
		}
		input.ClearAll()

		return
	}
	if !input.Cursor.In(image.Rect(0, 0, s.size.X, s.barHeight())) {
		return
	}
	if input.Pressed && !s.pressLaunch(input.Cursor) {
		s.pressBar(input.Cursor)
		s.pressReadout(input.Cursor)
	}
	quietMouse(input)
}

func (s *shell) pressBar(point image.Point) {
	for _, shown := range s.barTabs() {
		if !point.In(shown.rect) {
			continue
		}
		if shown.tab == moreTab {
			s.openMore(shown)

			return
		}
		s.choose(shown.tab)
		if shown.boxes && point.In(closeBox(shown.rect)) {
			for _, closing := range s.tabs[shown.tab].panes {
				closing.host.frame.Closing = true
			}
		}

		return
	}
}

// openMore lists the tabs that do not fit on the bar, under the "more" tab.
func (s *shell) openMore(more barTab) {
	labels := s.tabLabels()
	first := len(s.tabs) - more.more
	items := make([]ui.MenuItem, 0, more.more)
	for index := first; index < len(s.tabs); index++ {
		icon := ui.IconNone
		if icons := s.icons(index); len(icons) > 0 {
			icon = icons[0]
		}
		items = append(items, ui.MenuItem{
			ID: strconv.Itoa(index), Label: labels[index], Icon: icon, Disabled: false, Danger: false, Rule: false,
		})
	}
	s.more.Items = items
	s.more.Width = ui.MenuWidth(items, s.theme)
	s.more.Open(more.rect, image.Rect(0, 0, s.size.X, s.size.Y))
}

// choose brings a tab to the front.
func (s *shell) choose(index int) {
	if index >= 0 && index < len(s.tabs) {
		s.active = index
	}
}

// quietMouse keeps only a frame's keys and typed text.
func quietMouse(input *ui.Input) {
	input.Cursor = outside()
	input.Down, input.Pressed, input.Released, input.DoubleClick = false, false, false, false
	input.Secondary, input.PanDown = false, false
	input.WheelX, input.WheelY = 0, 0
}

// drawBar draws the window bar: its tabs, Launch and the clock's readout.
func (s *shell) drawBar(screen *ebiten.Image) {
	ui.FillRect(screen, image.Rect(0, 0, s.size.X, rowHeight), ui.ColorDesk())
	for _, shown := range s.barTabs() {
		s.drawBarTab(screen, shown)
	}
	s.drawLaunch(screen)
	s.drawReadout(screen)
	ui.Divider(screen, image.Pt(0, rowHeight-ui.Hairline), image.Pt(s.size.X, rowHeight-ui.Hairline))
}

// drawMore draws the "more" tab's list while it is open, over everything.
func (s *shell) drawMore(screen *ebiten.Image) {
	if s.more.IsOpen() {
		s.more.Draw(screen, s.theme)
	}
}

func (s *shell) drawBarTab(screen *ebiten.Image, shown barTab) {
	rect := shown.rect
	switch {
	case shown.tinted:
		drawTint(screen, rect, shown.tint, shown.front)
	case shown.front:
		ui.FillRect(screen, rect, ui.ColorPage())
	}
	if shown.front {
		ui.FillRect(screen, image.Rect(rect.Min.X, rect.Max.Y-ui.Outline, rect.Max.X, rect.Max.Y), ui.ColorSelect())
	}
	left := rect.Min.X + tabPad
	for _, icon := range shown.icons {
		ui.DrawIcon(screen, icon, image.Pt(left, rect.Min.Y+(rowHeight-ui.IconSize)/ui.Half))
		left += ui.IconSize + ui.Gap
	}
	left += ui.Gap
	right := rect.Max.X - tabPad
	if shown.boxes {
		right -= closeSize + ui.Gap
		drawClose(screen, closeBox(rect))
	}
	if shown.unsaved {
		middle := rect.Min.Y + rowHeight/ui.Half
		right -= unsavedDot * ui.Half
		ui.FillRound(screen, image.Rect(right, middle-unsavedDot, right+unsavedDot*ui.Half, middle+unsavedDot),
			unsavedDot, ui.ColorInk())
		right -= ui.Gap
	}
	textY := rect.Min.Y + (rowHeight-ui.TextSize)/ui.Half - 1
	if shown.live {
		right -= s.theme.TextWidth(liveWord)
		s.theme.DrawText(screen, liveWord, right, textY, ui.ColorLive())
		right -= ui.Gap
	}
	label := shown.label
	if shown.more > 0 {
		label = strconv.Itoa(shown.more) + " " + label
	}
	s.theme.DrawText(screen, s.theme.FitText(label, right-left), left, textY, ui.ColorInk())
	ui.Divider(screen, image.Pt(rect.Max.X-ui.Hairline, rect.Min.Y+ui.Gap),
		image.Pt(rect.Max.X-ui.Hairline, rect.Max.Y-ui.Gap))
}

// drawClose draws a close box's cross.
func drawClose(screen *ebiten.Image, rect image.Rectangle) {
	left, top := float32(rect.Min.X+ui.Gap), float32(rect.Min.Y+ui.Gap)
	right, bottom := float32(rect.Max.X-ui.Gap), float32(rect.Max.Y-ui.Gap)
	vector.StrokeLine(screen, left, top, right, bottom, ui.Outline, ui.ColorMuted(), true)
	vector.StrokeLine(screen, left, bottom, right, top, ui.Outline, ui.ColorMuted(), true)
}
