package pixelview

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
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

// Note is what the readout's second line says when nothing is wrong.
func (s *State) Note(docs frame.Docs) string { return s.templateNote(docs) }

// EditTemplateItem is the Template menu's Edit template… item.
func EditTemplateItem() string { return editTemplateItem }

// Toolbar names the toolbar's controls' places, and the canvas's.
func (s *State) Toolbar() []frametest.Part {
	parts := make([]frametest.Part, 0, len(s.tools)+5)
	for _, button := range s.tools {
		parts = append(parts, frametest.Part{Name: button.Label, Rect: button.Rect})
	}
	zoom := image.Rect(s.zoomOut.Rect.Min.X, s.zoomOut.Rect.Min.Y, s.zoomIn.Rect.Max.X, s.zoomIn.Rect.Max.Y)

	return append(parts, frametest.Part{Name: "zoom", Rect: zoom}, frametest.Part{Name: "Fit", Rect: s.zoomFit.Rect},
		frametest.Part{Name: "template", Rect: s.overlay.Rect}, frametest.Part{Name: "Grid", Rect: s.grid.Rect},
		frametest.Part{Name: "canvas", Rect: s.canvas()})
}
