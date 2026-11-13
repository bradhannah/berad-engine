package workflowview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The palette folds when told to (increment 43): by its fold button or
// Cmd+Shift+I, for every Workflow, as the editor window keeps it per kind
// of view. Folded, it is a strip with its name on it; Tab or a double-click
// on empty canvas still offers every node.

// RailWidth is how wide the folded palette's strip is, and foldButton the
// fold button's size.
const (
	RailWidth  = 36
	foldButton = 24
	foldID     = "fold-left"
	foldName   = "NODES"
)

// paletteWidth is how wide the palette is: its strip while folded.
func (s *State) paletteWidth() int {
	if s.paletteFolded {
		return RailWidth
	}

	return s.paletteFull()
}

// layoutFold places the fold button: right of the search while the palette
// shows, at the strip's top while it is folded.
func (s *State) layoutFold() {
	if s.live != nil {
		s.foldBn.Rect = ui.Zero()

		return
	}
	if s.paletteFolded {
		left := (RailWidth - foldButton) / ui.Half
		s.foldBn.Label = "›"
		s.foldBn.Rect = image.Rect(left, paletteTop, left+foldButton, paletteTop+foldButton)

		return
	}
	s.foldBn.Label = "‹"
	right := s.paletteFull() - ui.Inset
	s.foldBn.Rect = image.Rect(right-foldButton, paletteTop, right, paletteTop+foldButton)
}

// updateFold folds or unfolds the palette from its button, and reports
// whether it was pressed.
func (s *State) updateFold(input *ui.Input, ctx *frame.Context) bool {
	if s.foldBn.Rect.Empty() || !s.foldBn.Update(input) {
		return false
	}
	ctx.Frame.ShowLeft.On = !ctx.Frame.ShowLeft.On

	return true
}

// drawFoldedPalette draws the folded palette: its strip, the fold button
// and its name down the strip.
func (s *State) drawFoldedPalette(screen *ebiten.Image) {
	content := frame.Content(s.size)
	strip := image.Rect(0, toolbarHeight, RailWidth, content.Max.Y)
	ui.FillRect(screen, strip, ui.ColorPage())
	ui.Divider(screen, image.Pt(RailWidth-ui.Outline, strip.Min.Y), image.Pt(RailWidth-ui.Outline, strip.Max.Y))
	s.foldBn.Draw(screen, s.theme)
	middle := RailWidth / ui.Half
	top := s.foldBn.Rect.Max.Y + ui.Inset
	for _, letter := range foldName {
		shown := string(letter)
		s.theme.DrawLabel(screen, shown, middle-s.theme.TextWidth(shown)/ui.Half, top, ui.ColorMuted())
		top += ui.LineHeight - ui.Gap/ui.Half
	}
}
