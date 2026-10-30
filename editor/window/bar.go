package window

import (
	"image"
	"slices"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// Window bar layout, in screen pixels: each of the bar's rows is rowHeight
// tall, and its tabs share its width, up to tabWidth each on the top row and
// documentWidth on the second, whose names are longer.
const (
	rowHeight     = 32
	tabWidth      = 220
	documentWidth = 340
	tabPad        = 10
	closeSize     = 14
)

// barTab is one tab drawn on the bar: a kind on the top row, or a document of
// the kind in front on the second. tab is the index of the shell's tab it
// brings forward; boxes is set when it carries a close box for that tab, and
// count when it stands for several.
type barTab struct {
	rect  image.Rectangle
	tab   int
	icons []ui.Icon
	label string
	count int
	boxes bool
	front bool
}

// barHeight is how tall the bar is: one row, or two while the kind in front
// has several windows open.
func (s *shell) barHeight() int {
	if s.documentRow() {
		return rowHeight * ui.Half
	}

	return rowHeight
}

// barTabs lays out the bar's tabs: a kind's tab on the top row, named after
// its window when it has one open and after the kind with a count when it has
// several, and those several on the second row.
func (s *shell) barTabs() []barTab {
	groups := s.kinds()
	tabs := make([]barTab, 0, len(groups))
	width := min(tabWidth, (s.size.X-readoutWidth)/max(len(groups), 1))
	for at, group := range groups {
		index := s.chooseKind(group)
		shown := barTab{
			rect: image.Rect(at*width, 0, (at+1)*width, rowHeight), tab: index, icons: s.icons(index, true),
			label: kindName(group.view), count: len(group.tabs), boxes: false, front: false,
		}
		if len(group.tabs) == 1 {
			shown.label, shown.count, shown.boxes = shortTitle(s.tabTitle(index), false), 0, s.closable(index)
		}
		shown.front = slices.Contains(group.tabs, s.active)
		tabs = append(tabs, shown)
	}
	if group, found := s.frontKind(); found && len(group.tabs) > 1 {
		tabs = append(tabs, s.documentTabs(group)...)
	}

	return tabs
}

// documentTabs lays out the second row: the open documents of the kind in
// front.
func (s *shell) documentTabs(group kindGroup) []barTab {
	labels := s.documentLabels(group)
	width := min(documentWidth, s.size.X/len(group.tabs))
	tabs := make([]barTab, 0, len(group.tabs))
	for at, index := range group.tabs {
		tabs = append(tabs, barTab{
			rect: image.Rect(at*width, rowHeight, (at+1)*width, rowHeight*ui.Half), tab: index,
			icons: s.icons(index, false), label: labels[at], count: 0, boxes: s.closable(index), front: index == s.active,
		})
	}

	return tabs
}

// icons are the icons a tab shows: its kind's, when asked, and those of any
// pane beside its first, such as Watch beside a Map.
func (s *shell) icons(index int, kind bool) []ui.Icon {
	var icons []ui.Icon
	for slot, shown := range s.tabs[index].panes {
		if slot > 0 || kind {
			icons = append(icons, viewIcon(shown.view))
		}
	}

	return icons
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

// closeBox is a bar tab's close box, at its right end.
func closeBox(rect image.Rectangle) image.Rectangle {
	top := rect.Min.Y + (rowHeight-closeSize)/ui.Half

	return image.Rect(rect.Max.X-tabPad-closeSize, top, rect.Max.X-tabPad, top+closeSize)
}

// updateBar handles a press on the bar: a kind or a document comes to the
// front, or a tab's panes are asked to close from its close box, which asks
// about unsaved changes first. The bar keeps the mouse from the panes.
func (s *shell) updateBar(input *ui.Input) {
	if !input.Cursor.In(image.Rect(0, 0, s.size.X, s.barHeight())) {
		return
	}
	if input.Pressed {
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
		s.active = shown.tab
		if shown.boxes && point.In(closeBox(shown.rect)) {
			for _, closing := range s.tabs[shown.tab].panes {
				closing.host.frame.Closing = true
			}
		}

		return
	}
}

// quietMouse keeps only a frame's keys and typed text.
func quietMouse(input *ui.Input) {
	input.Cursor = outside()
	input.Down, input.Pressed, input.Released, input.DoubleClick = false, false, false, false
	input.Secondary, input.PanDown = false, false
	input.WheelX, input.WheelY = 0, 0
}

// drawBar draws the window bar: the kinds, the documents of the kind in front
// when it has several, and the clock's readout.
func (s *shell) drawBar(screen *ebiten.Image) {
	height := s.barHeight()
	ui.FillRect(screen, image.Rect(0, 0, s.size.X, rowHeight), ui.ColorPanel())
	if height > rowHeight {
		ui.FillRect(screen, image.Rect(0, rowHeight, s.size.X, height), ui.ColorBackground())
		ui.Divider(screen, image.Pt(0, rowHeight-ui.Hairline), image.Pt(s.size.X, rowHeight-ui.Hairline))
	}
	for _, shown := range s.barTabs() {
		s.drawBarTab(screen, shown)
	}
	s.drawReadout(screen)
	ui.Divider(screen, image.Pt(0, height-ui.Hairline), image.Pt(s.size.X, height-ui.Hairline))
}

func (s *shell) drawBarTab(screen *ebiten.Image, shown barTab) {
	rect := shown.rect
	if shown.front {
		ui.FillRect(screen, rect, ui.ColorBackground())
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
	textY := rect.Min.Y + (rowHeight-ui.TextSize)/ui.Half - 1
	if shown.count > 1 {
		count := strconv.Itoa(shown.count)
		right -= s.theme.MonoWidth(count)
		s.theme.DrawMono(screen, count, right, rect.Min.Y+(rowHeight-ui.MonoSize)/ui.Half-1, ui.ColorMuted())
		right -= ui.Gap * ui.Half
	}
	s.theme.DrawText(screen, s.theme.FitText(shown.label, right-left), left, textY, ui.ColorInk())
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
