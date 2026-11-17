package playview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/showdraw"
	"github.com/bradhannah/berad-engine/engine/game"
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
	// What the game shows is part of the game, drawn without the author's
	// scale for text and controls (increment 56).
	pen.OnCanvas(func() { s.drawShown(screen, s.game.View()) })
}

// drawShown draws over the game what its workflows show: each line said by
// an NPC in view, in a bubble over its head, and the box at the foot. A
// speaker off screen speaks unseen. The game is placed in the screen's
// pixels, and what it shows in the canvas's points.
func (s *State) drawShown(screen *ebiten.Image, view game.View) {
	device := pen.Device()
	points := func(pixels image.Point) image.Point {
		return image.Pt(int(float64(pixels.X)/device), int(float64(pixels.Y)/device))
	}
	origin, scale, tile := s.screen.Origin(), s.screen.Scale(), s.screen.Tile()
	shown := image.Rectangle{Min: points(origin), Max: points(origin.Add(image.Pt(
		int(float64(view.Columns*tile)*scale), int(float64(view.Rows*tile)*scale))))}
	for _, character := range s.game.Characters() {
		if !character.Speaking || character.Floor != view.Floor {
			continue
		}
		corner := s.screen.CellAt(view, character.X, character.Y)
		head := points(origin.Add(image.Pt(int(float64(corner.X*ui.Half+tile)*scale/ui.Half),
			int(float64(corner.Y)*scale))))
		if head.In(shown) {
			showdraw.DrawBubble(screen, s.theme, head, character.Line.Text,
				float64(character.Line.Left)/float64(character.Line.Ticks))
		}
	}
	showdraw.DrawBox(screen, s.theme, shown, showdraw.BoxOf(s.game.Stage()))
}
