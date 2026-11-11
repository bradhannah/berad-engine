package window

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// tintBand is how tall the band of a document tab's tint is, along its top,
// and frontBand how tall it is on the tab in front.
const (
	tintBand  = 3
	frontBand = 5
)

// tintTabs gives each open document's tab that has none a tint, numbered
// from one: the first no other open tab has, or, once every tint is taken,
// the one fewest have. A tab keeps its tint while it stays open, so open
// documents are told apart wherever their tabs are drawn. The project
// browser's tab is not a document and has none.
func (s *shell) tintTabs() {
	used := make([]int, len(ui.Tints())+1)
	for index := range s.tabs {
		if s.closable(index) {
			used[s.tabs[index].tint]++
		}
	}
	for index := range s.tabs {
		if s.tabs[index].tint != 0 || !s.closable(index) {
			continue
		}
		fewest := slices.Min(used[1:])
		chosen := 1 + slices.Index(used[1:], fewest)
		s.tabs[index].tint = chosen
		used[chosen]++
	}
}

// unsaved reports whether any pane of a tab holds changes that wait for
// Save, as it last told the orchestrator.
func (s *shell) unsaved(index int) bool {
	for _, shown := range s.tabs[index].panes {
		if shown.host != nil && shown.host.reported.Count > 0 {
			return true
		}
	}

	return false
}

// unsavedNamed reports whether the tab with a title as the bar shows it
// holds changes that wait for Save; it answers "state:tab-unsaved:<title>".
func (s *shell) unsavedNamed(title string) bool {
	for index := range s.tabs {
		if shortTitle(s.tabTitle(index), false) == title {
			return s.unsaved(index)
		}
	}

	return false
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
// along its top: the stronger wash and a thicker band on the tab in front.
func drawTint(screen *ebiten.Image, rect image.Rectangle, tint ui.Tint, front bool) {
	wash, band := tint.Wash, tintBand
	if front {
		wash, band = tint.Strong, frontBand
	}
	ui.FillRect(screen, rect, wash)
	ui.FillRect(screen, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+band), tint.Ink)
}
