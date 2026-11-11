package window

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// tintBand is how tall the band of a document tab's tint is, along its top.
const tintBand = 3

// tintTabs gives each open tab that has none a tint, numbered from one: the
// first no other open tab of its kind has, or, once every tint is taken,
// the one fewest have. A tab keeps its tint while it stays open, so the
// tabs of one kind are told apart on the bar's second row.
func (s *shell) tintTabs() {
	count := len(ui.Tints())
	for _, group := range s.kinds() {
		used := make([]int, count+1)
		for _, index := range group.tabs {
			used[s.tabs[index].tint]++
		}
		for _, index := range group.tabs {
			if s.tabs[index].tint != 0 {
				continue
			}
			fewest := slices.Min(used[1:])
			chosen := 1 + slices.Index(used[1:], fewest)
			s.tabs[index].tint = chosen
			used[chosen]++
		}
	}
}

// tintOf returns a tab's tint, and whether it has one yet.
func (s *shell) tintOf(index int) (ui.Tint, bool) {
	tints := ui.Tints()
	number := s.tabs[index].tint
	if number < 1 || number > len(tints) {
		return ui.Tint{Name: "", Wash: ui.ColorDesk(), Strong: ui.ColorPage(), Ink: ui.ColorInk()}, false
	}

	return tints[number-1], true
}

// tintNamed names the tint of the tab with a title as the bar shows it, or
// "" when no tab has the title or it has no tint yet; it answers
// "state:tab-tint:<title>=<tint>".
func (s *shell) tintNamed(title string) string {
	for index := range s.tabs {
		if shortTitle(s.tabTitle(index), false) == title {
			tint, _ := s.tintOf(index)

			return tint.Name
		}
	}

	return ""
}

// drawTint washes a document tab in its tint and draws the tint's band
// along its top.
func drawTint(screen *ebiten.Image, rect image.Rectangle, tint ui.Tint) {
	ui.FillRect(screen, rect, tint.Wash)
	ui.FillRect(screen, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+tintBand), tint.Ink)
}
