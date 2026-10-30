package pixelview

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// PanelScroll returns how far the palette panel's swatches are scrolled.
func (s *State) PanelScroll() int { return s.scroll }

// PanelArea returns the area the palette panel's swatches scroll in.
func (s *State) PanelArea() image.Rectangle { return s.panel.area }

// Warned counts the tile's pixels named by a moved colour, and loose ones.
func (s *State) Warned(docs frame.Docs) (int, int) { return s.warned(docs) }

// Problem is what the readout's second line says.
func (s *State) Problem(docs frame.Docs) string {
	size, _ := s.tileSize(docs)

	return s.problem(docs, size)
}
