package mapview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// When the floor is wholly out of view, the Map says which way it is and
// how to bring it back, so an empty stage is never left to be puzzled over
// (increment 42).

// floorOffView reports which way the floor lies when none of it is in view:
// "to the left", "to the right", "above" or "below".
func (s *State) floorOffView(docs frame.Docs) (string, bool) {
	grid := s.Grid()
	var floorRect image.Rectangle
	loaded := docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			floorRect = s.camera.rectOf(floorDoc.Floor.Bounds(), grid)
		}
	})
	if !loaded || floorRect.Empty() || grid.Empty() || floorRect.Overlaps(grid) {
		return "", false
	}
	switch {
	case floorRect.Max.X <= grid.Min.X:
		return "to the left", true
	case floorRect.Min.X >= grid.Max.X:
		return "to the right", true
	case floorRect.Max.Y <= grid.Min.Y:
		return "above", true
	default:
		return "below", true
	}
}

// drawOffView says, on a tag in the middle of the canvas, which way the
// floor is when none of it is in view, and that Fit brings it back.
func (s *State) drawOffView(area *ebiten.Image, grid image.Rectangle, docs frame.Docs) {
	way, off := s.floorOffView(docs)
	if !off {
		return
	}
	text := "The floor is out of view, " + way + ". Fit brings it back."
	width := s.theme.TextWidth(text) + ui.Half*ui.Inset
	middle := ui.Center(grid)
	tag := image.Rect(middle.X-width/ui.Half, middle.Y-ui.ButtonH/ui.Half, middle.X+width/ui.Half,
		middle.Y+ui.ButtonH/ui.Half)
	ui.FloatBox(area, tag, ui.ColorPage())
	s.theme.DrawText(area, text, tag.Min.X+ui.Inset, tag.Min.Y+(tag.Dy()-ui.TextSize)/ui.Half-1, ui.ColorInk())
}
