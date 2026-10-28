package pixelview

import (
	"image"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
)

// Locate finds a named part of the window for input scripts: "ui:<id>" for
// a button, a field or the grid toggle, "pixel:<col>,<row>" for a tile pixel,
// "swatch:<colour>" for a colour in the palette panel ("swatch:clear",
// "swatch:grey 3") and "ramp:<name>" for a ramp's name. "state:…" answers
// scripts waiting for the window.
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	shared, _ := palette(docs, art.SharedPaletteID())
	s.layout(docs, shared)
	kind, value, _ := strings.Cut(name, ":")
	if s.guard.Active() {
		return s.guard.Locate(value, s.size)
	}
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

// locatePanel finds a swatch or a ramp's name in the palette panel.
func (s *State) locatePanel(kind, value string) (image.Point, bool) {
	switch kind {
	case "swatch":
		for _, offered := range s.panel.swatches {
			if offered.name == value {
				return ui.Center(offered.rect), true
			}
		}
	case "ramp":
		for _, label := range s.panel.ramps {
			if label.ramp.Name == value {
				return ui.Center(label.rect), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// locateWidget finds a button, a field or the grid toggle by its ID.
func (s *State) locateWidget(widget string) (image.Point, bool) {
	if point, found := s.save.Locate(widget); found {
		return point, true
	}
	buttons := append([]*ui.Button{s.zoomOut, s.zoomIn, s.zoomFit, s.addRamp, s.addShade, s.removeShade,
		s.removeRamp, s.shows}, s.tools...)
	for _, button := range buttons {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}
	for _, field := range s.fields() {
		if field.ID == widget && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	if widget == s.grid.ID {
		return ui.Center(s.grid.Rect), true
	}

	return image.Pt(0, 0), false
}

// hasState answers input scripts waiting for the window: "loaded" once the
// tile and the shared palette are, "tool:<tool>", "color:<colour>",
// "zoom:<whole number>", "pixel:<col>,<row>:<colour>" once a pixel is that
// colour, "saved" once no typed change waits, and "shows:<stand-in>" once
// the sprite shows that stand-in ("shows:" for none).
func (s *State) hasState(state string, docs frame.Docs) bool {
	kind, value, _ := strings.Cut(state, ":")
	switch kind {
	case "loaded":
		_, tiled := s.tileSize(docs)
		_, paletted := palette(docs, art.SharedPaletteID())

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
	case "saved":
		for _, field := range s.fields() {
			if field.Field.Unsaved() || field.Field.Pending() {
				return false
			}
		}

		return true
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
