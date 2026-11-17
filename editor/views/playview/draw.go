package playview

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/player"
)

// hexMessage is the colour of what the tab says while there is no game.
const hexMessage = 0xd8dce3

// Draw draws the game's screen, filling the tab while keeping its shape, or
// why there is no game yet. The engine's player draws the game, in its own
// pixels, and places it on the screen (increments 56 and 62).
func (s *State) Draw(screen *ebiten.Image, _ frame.Docs) {
	screen.Fill(ui.ColorStage())
	if s.game == nil {
		message := "Loading the game…"
		if s.problem != "" {
			message = s.problem
		}
		bounds := pen.Bounds(screen)
		width := s.theme.TextWidth(message)
		s.theme.DrawText(screen, message, (bounds.Dx()-width)/ui.Half, bounds.Dy()/ui.Half, ui.Hex(hexMessage))

		return
	}
	s.screen.Draw(screen, s.game)
	// What the game shows is part of the game, written without the author's
	// scale for text and controls (increment 56): each point is as many of
	// the screen's pixels as the screen has to a point.
	s.screen.DrawShown(screen, s.game, player.Writer{Type: s.theme.Shown, Scale: pen.Device()})
}
