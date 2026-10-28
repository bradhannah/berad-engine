package pixelview

import "image"

// PanelScroll returns how far the palette panel's swatches are scrolled.
func (s *State) PanelScroll() int { return s.scroll }

// PanelArea returns the area the palette panel's swatches scroll in.
func (s *State) PanelArea() image.Rectangle { return s.panel.area }
