package browser

import (
	"image"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/stateview"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A search lists what the game says that has its words: constants, each
// row's ID foundConstant followed by the item that opens the Constants
// window at it.
const (
	foundConstant = "found-constant:"
	// excerptRunes is about how much of a text a row shows around the
	// words found.
	excerptRunes = 44
	// foundConstants is the Matches group of constants found, and
	// constantsLabel its heading, as World state's part is named.
	foundConstants = "found-constants"
	constantsLabel = "Constants"
)

// constantThings are the constants a search finds by their path or their
// words, a row each; none while only tags are searched.
func (s *State) constantThings(docs frame.Docs, match matcher) []thing {
	if !match.active() || match.tags {
		return nil
	}
	var found []thing
	for _, each := range stateview.FindConstants(docs, match.text) {
		row := ui.NewRow(foundConstant+each.Item, each.Path)
		row.Icon, row.Tree, row.Detail = ui.IconSettings, true, foundDetail(each, match.text)
		found = append(found, thing{row: row, matches: true, extra: true, values: nil, cells: [columnCount]string{}})
	}

	return found
}

// foundDetail says what a constant's row found: whose text and the words
// in it, or for a path, its value.
func foundDetail(each stateview.Found, query string) string {
	said := each.Said
	if each.Text {
		said = excerpt(said, query)
	} else if utf8.RuneCountInString(said) > excerptRunes {
		said = quoted(string([]rune(said)[:excerptRunes]) + "…")
	}
	if each.Holder != "" {
		return each.Holder + ": " + said
	}

	return said
}

// excerpt is the part of a text around the words found, in quotes, cut
// with an ellipsis where it starts or ends inside the text, on one line.
func excerpt(text, query string) string {
	runes := []rune(strings.Join(strings.Fields(text), " "))
	lower := []rune(strings.ToLower(string(runes)))
	index := strings.Index(string(lower), strings.ToLower(query))
	if index < 0 || len(runes) <= excerptRunes {
		if len(runes) > excerptRunes {
			return quoted(string(runes[:excerptRunes]) + "…")
		}

		return quoted(string(runes))
	}
	start := max(utf8.RuneCountInString(string(lower)[:index])-excerptRunes/ui.Half, 0)
	end := min(start+excerptRunes, len(runes))
	start = max(end-excerptRunes, 0)
	cut := string(runes[start:end])
	if start > 0 {
		cut = "…" + cut
	}
	if end < len(runes) {
		cut += "…"
	}

	return quoted(cut)
}

// quoted puts text in curly quotes.
func quoted(text string) string { return "“" + text + "”" }

// selectedConstant is the constant found that is selected, as the search
// still finds it.
func (s *State) selectedConstant(docs frame.Docs) (stateview.Found, bool) {
	item, isFound := strings.CutPrefix(s.list.Selected, foundConstant)
	if !isFound {
		return stateview.Found{Item: "", Path: "", Holder: "", Said: "", Text: false}, false
	}
	for _, each := range stateview.FindConstants(docs, newMatcher(s.search.Field.Value()).text) {
		if each.Item == item {
			return each, true
		}
	}

	return stateview.Found{Item: item, Path: "", Holder: "", Said: "", Text: false}, false
}

// openFound opens what a found row stands for, and reports whether the
// selected row was one: a constant in the Constants window, or a line in
// its workflow.
func (s *State) openFound(ctx *frame.Context) bool {
	if item, isConstant := strings.CutPrefix(s.list.Selected, foundConstant); isConstant {
		ctx.Requests.OpenItem(worldConstantsView, worldstate.ID(), item)

		return true
	}

	return s.openLine(ctx)
}

// layoutFound places Open for a found row, and reports whether one is
// selected.
func (s *State) layoutFound(left, width int) bool {
	switch {
	case strings.HasPrefix(s.list.Selected, foundConstant):
		s.open.Label = "Open in Constants"
	case strings.HasPrefix(s.list.Selected, foundLine):
		s.open.Label = "Open in its workflow"
	default:
		return false
	}
	s.open.Disabled = false
	s.open.Rect = image.Rect(left, nameFieldTop, left+width, nameFieldTop+ui.ButtonH)

	return true
}

// drawFound draws a found row's inspector: what it is, Open, and the whole
// text that has the words; and reports whether one is selected.
func (s *State) drawFound(screen *ebiten.Image, docs frame.Docs, left int) bool {
	icon, label := ui.IconSettings, "Constant"
	var title, said string
	switch {
	case strings.HasPrefix(s.list.Selected, foundConstant):
		each, _ := s.selectedConstant(docs)
		title, said = each.Path, each.Said
		if each.Holder != "" {
			label = "Constant · " + each.Holder + "'s own"
		}
	case strings.HasPrefix(s.list.Selected, foundLine):
		each, _ := s.selectedLine(docs)
		icon, label, title, said = ui.IconWorkflow, "Line · "+each.kind, each.name, each.text
	default:
		return false
	}
	s.drawHeader(screen, icon, label, title, left)
	s.open.Draw(screen, s.theme)
	top := s.open.Rect.Max.Y + ui.Gap
	for _, line := range s.theme.Wrap(said, frame.Inspector(s.size).Dx()-ui.Half*ui.Inset) {
		s.theme.DrawText(screen, line, left, top, ui.ColorInk())
		top += ui.LineHeight
	}

	return true
}
