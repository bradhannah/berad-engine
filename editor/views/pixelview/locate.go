package pixelview

import (
	"image"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Locate finds a named part of the window for input scripts: "ui:<id>" for
// a button or a toggle, "pixel:<col>,<row>" for a tile pixel,
// "swatch:<colour>" for a colour in the palette panel ("swatch:clear",
// "swatch:Default / grey 3"), "ramp:<palette> / <name>" for a ramp's name and
// "palette:<name>" for a palette's heading, each scrolled into view.
// "state:…" answers scripts waiting for the window.
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layout(docs)
	kind, value, _ := strings.Cut(name, ":")
	if kind == "menu" {
		return s.showsMenu.Locate(value)
	}
	switch kind {
	case "ui":
		return s.locateWidget(value)
	case "pixel":
		pixel, ok := parsePixel(value)
		size, loaded := s.tileSize(docs)
		if !ok || !loaded || pixel.X < 0 || pixel.Y < 0 || pixel.X >= size || pixel.Y >= size {
			return image.Pt(0, 0), false
		}

		return ui.Center(s.pixelRect(pixel, size)), true
	case "state":
		return image.Pt(0, 0), s.hasState(value, docs)
	}

	return s.locatePanel(kind, value)
}

// locatePanel finds a swatch, a ramp's name or a palette's heading in the
// palette panel.
func (s *State) locatePanel(kind, value string) (image.Point, bool) {
	switch kind {
	case "swatch":
		for _, offered := range s.panel.swatches {
			if offered.name == value {
				return ui.Center(s.reveal(offered.rect)), true
			}
		}
	case "ramp":
		paletteName, rampName, _ := strings.Cut(value, " / ")
		for _, label := range s.panel.ramps {
			if label.ramp.Name == rampName && s.headerName(label.palette) == paletteName {
				return ui.Center(s.reveal(label.rect)), true
			}
		}
	case "palette":
		for _, header := range s.panel.headers {
			if header.name == value {
				return image.Pt(header.rect.Min.X+foldColumn, s.reveal(header.rect).Min.Y+swatchSize/ui.Half), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// headerName is a palette's name as its heading shows it.
func (s *State) headerName(paletteID ids.ULID) string {
	for _, header := range s.panel.headers {
		if header.palette == paletteID {
			return header.name
		}
	}

	return ""
}

// locateWidget finds a button, a field or the grid toggle by its ID.
func (s *State) locateWidget(widget string) (image.Point, bool) {
	buttons := append([]*ui.Button{s.zoomOut, s.zoomIn, s.zoomFit, s.edit, s.shows}, s.tools...)
	for _, button := range buttons {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}
	for _, toggle := range []*ui.Toggle{s.grid, s.marks} {
		if widget == toggle.ID && !toggle.Rect.Empty() {
			return ui.Center(toggle.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// hasState answers input scripts waiting for the window: "loaded" once the
// tile and Default are, "tool:<tool>", "color:<colour>",
// "zoom:<whole number>", "pixel:<col>,<row>:<colour>" once a pixel is that
// colour, and "shows:<stand-in>" once the sprite shows that stand-in
// ("shows:" for none).
func (s *State) hasState(state string, docs frame.Docs) bool {
	kind, value, _ := strings.Cut(state, ":")
	switch kind {
	case "loaded":
		_, tiled := s.tileSize(docs)
		_, paletted := palette(docs, art.DefaultPaletteID())

		return tiled && paletted
	case "tool":
		return string(s.tool) == value
	case "color":
		name, ok := s.colorName(docs, s.color)

		return ok && name == value
	case "zoom":
		return strconv.Itoa(s.zoom) == value
	case "pixel":
		return s.hasPixel(value, docs)
	case "shows":
		return s.shownStandIn(docs) == value
	}

	return false
}

// hasPixel reports whether a pixel, written "<col>,<row>:<colour>", is that
// colour.
func (s *State) hasPixel(value string, docs frame.Docs) bool {
	at, colour, _ := strings.Cut(value, ":")
	pixel, ok := parsePixel(at)
	if !ok {
		return false
	}
	ref, found := s.pixelColor(docs, pixel)
	name, named := s.colorName(docs, ref)

	return found && named && name == colour
}

// parsePixel reads a pixel written as "<col>,<row>".
func parsePixel(value string) (image.Point, bool) {
	colText, rowText, found := strings.Cut(value, ",")
	col, colErr := strconv.Atoi(colText)
	row, rowErr := strconv.Atoi(rowText)

	return image.Pt(col, row), found && colErr == nil && rowErr == nil
}
