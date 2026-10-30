package paletteview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Locate finds a named part of the window for input scripts: "ui:<id>" for a
// button, a field or a toggle; "swatch:<ramp> <n>" for a colour and
// "ramp:<name>" for a ramp's name, each scrolled into view;
// "slot:<ramp> <n>" for the place before colour n, or after the last when n
// is one more; "rampslot:<n>" for the place before ramp n; and
// "source:<ramp> <n>" and "sourceramp:<name>" in Copy colours from…; and
// "picker:square <s> <b>", "picker:hue <h>" and "picker:opacity <o>", in
// percent, on the colour picker.
// "state:…" answers scripts waiting for the window.
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layout(docs)
	kind, value, _ := strings.Cut(name, ":")
	switch {
	case kind == "state":
		return image.Pt(0, 0), s.hasState(value, docs)
	case s.guard.Active():
		return s.guard.Locate(value, s.size)
	case kind == "menu":
		return s.menu.Locate(value)
	case kind == "ui":
		return s.locateWidget(value)
	case kind == "picker":
		return s.picker.Locate(value)
	}

	return s.locateRows(kind, value)
}

// locateRows finds a colour, a ramp's name or a place to drop on.
func (s *State) locateRows(kind, value string) (image.Point, bool) {
	switch kind {
	case "swatch":
		rampName, number, ok := splitNumber(value)
		if row, found := s.rowNamed(rampName); found && ok && number >= 1 && number <= len(row.swatches) {
			return ui.Center(s.revealRect(row.swatches[number-1])), true
		}
	case "ramp":
		if row, found := s.rowNamed(value); found {
			return ui.Center(s.revealRect(row.name)), true
		}
	case "slot":
		return s.locateSlot(value)
	case "rampslot":
		place, err := strconv.Atoi(value)
		if err != nil || place < 1 || place > len(s.rows.rows)+1 {
			break
		}
		if place <= len(s.rows.rows) {
			row := s.rows.rows[place-1]

			return image.Pt(row.name.Min.X+ui.Gap, row.top+ui.Gap), true
		}

		return image.Pt(s.rows.area.Min.X+ui.Gap, s.rows.movedTop-ui.Gap), true
	case "source", "sourceramp":
		return s.locateSource(kind, value)
	}

	return image.Pt(0, 0), false
}

// locateSlot finds the place before a ramp's colour n, or after its last.
func (s *State) locateSlot(value string) (image.Point, bool) {
	rampName, number, ok := splitNumber(value)
	row, found := s.rowNamed(rampName)
	if !found || !ok || number < 1 || number > len(row.swatches)+1 {
		return image.Pt(0, 0), false
	}
	if number <= len(row.swatches) {
		rect := s.revealRect(row.swatches[number-1])

		return image.Pt(rect.Min.X+ui.Gap, rect.Min.Y+rect.Dy()/ui.Half), true
	}
	if count := len(row.swatches); count > 0 {
		rect := s.revealRect(row.swatches[count-1])

		return image.Pt(rect.Max.X+swatchStep/ui.Half, rect.Min.Y+rect.Dy()/ui.Half), true
	}

	return image.Pt(row.name.Max.X+countColumn+swatchSize/ui.Half, row.top+swatchSize/ui.Half), true
}

// locateSource finds a colour or a ramp's name in Copy colours from….
func (s *State) locateSource(kind, value string) (image.Point, bool) {
	if kind == "sourceramp" {
		for _, label := range s.copying.ramps {
			if label.ramp.Name == value {
				return ui.Center(s.revealSource(label.rect)), true
			}
		}

		return image.Pt(0, 0), false
	}
	for _, offered := range s.copying.swatches {
		if offered.name == value {
			return ui.Center(s.revealSource(offered.rect)), true
		}
	}

	return image.Pt(0, 0), false
}

// revealSource scrolls a source colour into view in Copy colours from….
func (s *State) revealSource(rect image.Rectangle) image.Rectangle {
	area, shift := s.copying.area, 0
	switch {
	case rect.Min.Y < area.Min.Y:
		shift = rect.Min.Y - area.Min.Y
	case rect.Max.Y > area.Max.Y:
		shift = rect.Max.Y - area.Max.Y
	}
	s.copying.scroll += shift

	return rect.Sub(image.Pt(0, shift))
}

// rowNamed returns the row of a ramp by its name.
func (s *State) rowNamed(name string) (rampRow, bool) {
	for _, row := range s.rows.rows {
		if row.ramp.Name == name {
			return row, true
		}
	}

	return rampRow{ramp: art.Ramp{Name: "", ID: ids.ULID{}, Shades: nil}, name: ui.Zero(), top: 0, bottom: 0,
		swatches: nil}, false
}

// splitNumber reads "<name> <number>".
func splitNumber(value string) (string, int, bool) {
	space := strings.LastIndex(value, " ")
	if space < 0 {
		return value, 0, false
	}
	number, err := strconv.Atoi(value[space+1:])

	return value[:space], number, err == nil
}

// locateWidget finds a button, a field or a toggle by its ID.
func (s *State) locateWidget(widget string) (image.Point, bool) {
	if point, found := s.save.Locate(widget); found && !s.save.Rect.Empty() {
		return point, true
	}
	buttons := []*ui.Button{
		s.bar.addRamp, s.bar.gradient, s.bar.copyFrom, s.bar.remove, s.bar.copyTo, s.bar.newPalette,
		s.inspect.addColor, s.inspect.removeColor, s.inspect.gradientInto, s.inspect.removeRamp,
	}
	buttons = append(buttons, s.gradient.buttons()...)
	buttons = append(buttons, s.copying.buttons()...)
	if s.deleting != nil {
		buttons = append(slices.Clone(s.deleting.removes), s.deleting.all, s.deleting.cancel, s.deleting.confirm)
	}
	for _, button := range buttons {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}
	fields := append(s.fields(), s.gradient.fields()...)
	for _, field := range append(fields, s.copying.name) {
		if field.ID == widget && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	for _, toggle := range []*ui.Toggle{s.gradient.mixColor, s.gradient.mixOpacity} {
		if toggle.ID == widget && !toggle.Rect.Empty() {
			return ui.Center(toggle.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// hasState answers input scripts waiting for the window: "loaded" once the
// palette is; "chosen:<ramp> <n>"; "picked:<count>"; "saved" once no typed
// change waits; "ramps:<name>,<name>…" in order; "colour:<ramp> <n>:<#hex>"
// and "label:<ramp> <n>:<label>" once a colour is so; "moved:<old name>"
// while a moved colour is listed; and "deletable" once the delete panel finds
// nothing that refers to the palette.
func (s *State) hasState(state string, docs frame.Docs) bool {
	kind, value, _ := strings.Cut(state, ":")
	palette, loaded := s.palette(docs)
	if !loaded {
		return false
	}
	switch kind {
	case "loaded":
		return true
	case "chosen":
		name, ok := palette.ShadeName(s.chosen.ramp, s.chosen.shade)

		return ok && name == value
	case "picked":
		return strconv.Itoa(len(s.picked)) == value
	case "saved":
		return !slices.ContainsFunc(s.fields(), func(field *ui.TextField) bool {
			return field.Field.Unsaved() || field.Field.Pending()
		})
	case "ramps":
		names := make([]string, 0, len(palette.Ramps()))
		for _, ramp := range palette.Ramps() {
			names = append(names, ramp.Name)
		}

		return strings.Join(names, ",") == value
	case "colour", "label":
		return colourIs(palette, kind, value)
	case "moved":
		return slices.ContainsFunc(s.rows.moved, func(line movedLine) bool { return line.record.Name == value })
	case "deletable":
		return s.deleting != nil && s.deleting.known && len(s.deleting.uses) == 0
	}

	return false
}

// colourIs reports whether a colour, written "<ramp> <n>:<value>", has that
// value as #RRGGBB or that label.
func colourIs(palette *art.Palette, kind, value string) bool {
	at, want, _ := strings.Cut(value, ":")
	rampName, number, ok := splitNumber(at)
	if !ok {
		return false
	}
	for _, ramp := range palette.Ramps() {
		if ramp.Name != rampName || number < 1 || number > len(ramp.Shades) {
			continue
		}
		shade := ramp.Shades[number-1]
		if kind == "label" {
			return shade.Label == want
		}

		return shade.Color.String() == want
	}

	return false
}
