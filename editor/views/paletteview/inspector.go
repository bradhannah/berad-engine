package paletteview

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Toolbar and inspector geometry, in screen pixels.
const (
	buttonPad    = 12
	nameWidth    = 180
	labelColumn  = 64
	colorWidth   = 96
	opacityWidth = 56
	pickerHeight = 150
	counterWidth = 44
	preview      = 40
	// A new ramp is named newRampName, numbered after the first, and starts
	// with one colour of newShade.
	newRampName  = "new ramp"
	newShade     = art.RGB(0x808080)
	firstNumber  = 2
	singlesRamp  = "default"
	newPaletteAs = "New palette"
)

// Menu purposes, for menuFor.
const (
	menuCopyTo     = "copy-to"
	menuInto       = "gradient-into"
	menuCopyInto   = "copy-into"
	menuCopySource = "copy-source"
	menuMoveTo     = "move-to"
	newRampItem    = "new-ramp"
)

// buttonWidth is how wide a button is for its label.
func (s *State) buttonWidth(button *ui.Button) int {
	width := s.theme.TextWidth(button.Label) + ui.Half*buttonPad
	if button.Caret {
		width += buttonPad
	}

	return width
}

// place puts a button at a point and returns where the next one goes.
func (s *State) place(button *ui.Button, left, top int) int {
	button.Rect = image.Rect(left, top, left+s.buttonWidth(button), top+ui.ButtonH)

	return button.Rect.Max.X + ui.Gap
}

// layoutToolbar places the palette's name and the buttons above the ramps:
// for Default, only copying out; for any other palette, its name, + Ramp,
// Gradient…, Copy colours from… and Delete palette….
func (s *State) layoutToolbar() {
	for _, button := range []*ui.Button{
		s.bar.addRamp, s.bar.gradient, s.bar.copyFrom, s.bar.remove, s.bar.copyTo, s.bar.newPalette,
	} {
		button.Rect = ui.Zero()
	}
	s.name.Rect = ui.Zero()
	top, left := ui.Inset, ui.Inset
	if s.ReadOnly() {
		left += s.theme.StrongWidth(art.DefaultPaletteName) + ui.Inset + s.theme.TextWidth(readOnlyNote) + ui.Inset
		left = s.place(s.bar.copyTo, left, top)
		s.place(s.bar.newPalette, left, top)
		s.bar.copyTo.Disabled = len(s.picked) == 0
		s.bar.newPalette.Disabled = len(s.picked) == 0

		return
	}
	s.name.Rect = image.Rect(left, top, left+nameWidth, top+ui.ButtonH)
	left = s.name.Rect.Max.X + ui.Inset
	left = s.place(s.bar.addRamp, left, top)
	left = s.place(s.bar.gradient, left, top)
	s.place(s.bar.copyFrom, left, top)
	s.bar.gradient.On = s.gradient.open
	s.bar.copyFrom.On = s.copying.open
	right := frame.Content(s.size).Max.X - ui.Inset
	s.place(s.bar.remove, right-s.buttonWidth(s.bar.remove), top)
}

// readOnlyNote follows Default's name in its toolbar.
const readOnlyNote = "built in · read only · the same in every game"

// drawToolbar draws the palette's name and the toolbar's buttons.
func (s *State) drawToolbar(screen *ebiten.Image) {
	text := ui.Inset + (ui.ButtonH-ui.TextSize)/ui.Half
	if s.ReadOnly() {
		s.theme.DrawStrong(screen, art.DefaultPaletteName, ui.Inset, text, ui.ColorInk())
		s.theme.DrawText(screen, readOnlyNote, ui.Inset+s.theme.StrongWidth(art.DefaultPaletteName)+ui.Inset, text,
			ui.ColorMuted())
	} else {
		s.name.Draw(screen)
	}
	for _, button := range []*ui.Button{
		s.bar.addRamp, s.bar.gradient, s.bar.copyFrom, s.bar.remove, s.bar.copyTo, s.bar.newPalette,
	} {
		if !button.Rect.Empty() {
			button.Draw(screen, s.theme)
		}
	}
}

// updateButtons handles the toolbar's and the inspector's buttons.
func (s *State) updateButtons(input *ui.Input, ctx *frame.Context) {
	palette, ok := s.palette(ctx.Docs)
	if !ok {
		return
	}
	switch {
	case s.bar.addRamp.Update(input):
		s.addRamp(ctx, palette)
	case s.bar.gradient.Update(input):
		s.toggleGradient(palette, ids.ULID{})
	case s.bar.copyFrom.Update(input):
		s.toggleCopying(ctx)
	case s.bar.remove.Update(input):
		s.openDeleting(ctx)
	case s.bar.copyTo.Update(input):
		s.openCopyTo(ctx)
	case s.bar.newPalette.Update(input):
		s.newPalette(ctx, palette)
	case s.inspect.duplicate.Update(input):
		s.duplicateColor(ctx, palette)
	case s.inspect.moveTo.Update(input):
		s.openMenu(menuMoveTo, s.otherRamps(palette), s.inspect.moveTo.Rect)
	case s.inspect.removeColor.Update(input):
		s.removeColor(ctx, palette)
	case s.inspect.gradientInto.Update(input):
		s.toggleGradient(palette, s.chosen.ramp)
	case s.inspect.removeRamp.Update(input):
		s.removeRamp(ctx, palette)
	}
}

// addRamp adds a ramp at the end with one mid grey, and chooses it.
func (s *State) addRamp(ctx *frame.Context, palette *art.Palette) {
	name := uniqueRampName(palette, newRampName)
	index := len(palette.Ramps())
	ramp := art.Ramp{ID: ids.ULID{}, Name: name, Shades: []art.Shade{
		{ID: ids.ULID{}, Color: newShade, Opacity: art.MaxOpacity, Label: ""},
	}}
	s.command(ctx, s.target, artplugin.AddRampEdit, "", artplugin.AddRampArgs{Index: index, Ramp: ramp},
		func(protocol.Result) {
			if updated, found := s.palette(ctx.Docs); found && index < len(updated.Ramps()) {
				added := updated.Ramps()[index]
				s.choose(choice{ramp: added.ID, shade: added.Shades[0].ID})
				s.reveal = true
			}
			ctx.Frame.Status("Added the ramp “" + name + "”. Name it for its use, such as grass.")
		}, nil)
}

// uniqueRampName is the first of a name, then the name numbered from 2, that
// no ramp of the palette has.
func uniqueRampName(palette *art.Palette, base string) string {
	taken := map[string]bool{}
	for _, ramp := range palette.Ramps() {
		taken[ramp.Name] = true
	}
	name := base
	for number := firstNumber; taken[name]; number++ {
		name = base + " " + strconv.Itoa(number)
	}

	return name
}

// duplicateColor adds a copy of the chosen colour after it, without its
// label, and chooses the copy.
func (s *State) duplicateColor(ctx *frame.Context, palette *art.Palette) {
	shade, number, found := palette.Lookup(s.chosen.ramp, s.chosen.shade)
	ramp, rampFound := palette.Ramp(s.chosen.ramp)
	if !rampFound {
		return
	}
	index := len(ramp.Shades)
	added := art.Shade{ID: ids.ULID{}, Color: newShade, Opacity: art.MaxOpacity, Label: ""}
	if found {
		index, added.Color, added.Opacity = number, shade.Color, shade.Opacity
	}
	args := artplugin.AddShadeArgs{Ramp: ramp.ID, Index: index, Shade: added}
	s.command(ctx, s.target, artplugin.AddShadeEdit, "", args, func(protocol.Result) {
		if updated, ok := s.palette(ctx.Docs); ok {
			if grown, ok := updated.Ramp(ramp.ID); ok && index < len(grown.Shades) {
				s.choose(choice{ramp: ramp.ID, shade: grown.Shades[index].ID})
			}
		}
		ctx.Frame.Status(fmt.Sprintf("Duplicated the colour as %s %d", ramp.Name, index+1))
	}, nil)
}

// removeColor removes the chosen colour and chooses its neighbour. A colour
// a tile is painted with is refused, naming the tiles.
func (s *State) removeColor(ctx *frame.Context, palette *art.Palette) {
	name, ok := palette.ShadeName(s.chosen.ramp, s.chosen.shade)
	if !ok {
		return
	}
	_, number, _ := palette.Lookup(s.chosen.ramp, s.chosen.shade)
	ramp := s.chosen.ramp
	args := artplugin.RemoveShadeArgs{Ramp: ramp, Shade: s.chosen.shade}
	s.command(ctx, s.target, artplugin.RemoveShadeEdit, "", args, func(protocol.Result) {
		s.choose(choice{ramp: ramp, shade: ids.ULID{}})
		if updated, found := s.palette(ctx.Docs); found {
			if shrunk, has := updated.Ramp(ramp); has && len(shrunk.Shades) > 0 {
				s.choose(choice{ramp: ramp, shade: shrunk.Shades[min(number-1, len(shrunk.Shades)-1)].ID})
			}
		}
		ctx.Frame.Status("Removed " + name)
	}, nil)
}

// removeRamp removes the chosen ramp and chooses nothing. A ramp a tile is
// painted with is refused, naming the tiles.
func (s *State) removeRamp(ctx *frame.Context, palette *art.Palette) {
	ramp, ok := palette.Ramp(s.chosen.ramp)
	if !ok {
		return
	}
	s.command(ctx, s.target, artplugin.RemoveRampEdit, "", artplugin.RemoveRampArgs{Ramp: ramp.ID},
		func(protocol.Result) {
			s.choose(none())
			ctx.Frame.Status("Removed the ramp “" + ramp.Name + "”")
		}, nil)
}

// syncFields keeps the typed fields showing the palette's name and the
// chosen colour and ramp. A choice that is gone, removed in this window or
// another, falls back to its ramp, or to nothing.
func (s *State) syncFields(docs frame.Docs) {
	s.name.Field.SetSaved(s.paletteName(docs))
	if s.chosen != s.named {
		s.revertChoiceFields()
		s.named = s.chosen
	}
	palette, ok := s.palette(docs)
	if !ok {
		return
	}
	ramp, found := palette.Ramp(s.chosen.ramp)
	if !found {
		s.chosen, s.named, s.picked = none(), none(), nil
		s.revertChoiceFields()

		return
	}
	s.rampName.Field.SetSaved(ramp.Name)
	shade, _, has := palette.Lookup(ramp.ID, s.chosen.shade)
	if !has {
		if !s.chosen.shade.IsZero() {
			s.chosen.shade, s.named = ids.ULID{}, choice{ramp: ramp.ID, shade: ids.ULID{}}
			s.revertChoiceFields()
		}

		return
	}
	s.label.Field.SetSaved(shade.Label)
	s.hex.Field.SetSaved(shade.Color.String())
	s.opacity.Field.SetSaved(strconv.Itoa(shade.Opacity))
}

// choiceFields are the fields that belong to the chosen colour and ramp.
func (s *State) choiceFields() []*ui.TextField {
	return []*ui.TextField{s.label, s.hex, s.opacity, s.rampName}
}

func (s *State) fields() []*ui.TextField { return append(s.choiceFields(), s.name) }

func (s *State) revertChoiceFields() {
	if slices.Contains(s.choiceFields(), s.focus.Current()) {
		s.focus.Set(nil)
	}
	for _, field := range s.choiceFields() {
		field.Field.Revert()
	}
}

// guardChoice asks about unsaved changes before another colour or ramp is
// chosen: the choice goes back until the author decides.
func (s *State) guardChoice() {
	unsaved := frame.UnsavedFields(s.choiceFields()...)
	if s.chosen == s.named || unsaved == 0 {
		return
	}
	wanted, picked := s.chosen, s.picked
	s.chosen = s.named
	s.focus.Set(nil)
	s.guard.Ask(unsaved, s.namedLabel(), func() { s.chosen, s.picked = wanted, picked })
}

// namedLabel names what the typed fields belong to.
func (s *State) namedLabel() string {
	if s.label.Field.Unsaved() || s.hex.Field.Unsaved() || s.opacity.Field.Unsaved() || s.rampName.Field.Unsaved() {
		return s.rampName.Field.Saved()
	}

	return s.name.Field.Saved()
}

// updateFields handles the typed fields and the save bar: Enter in a field
// saves.
func (s *State) updateFields(input *ui.Input, ctx *frame.Context) {
	if field, outcome := s.focus.Update(input, ctx.Clipboard); field != nil && outcome == textedit.OutcomeCommit {
		if slices.Contains(s.fields(), field) {
			s.Save(ctx)
		}
	}
	s.save.Unsaved = s.Unsaved(ctx.Docs)
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)
	case ui.RevertID:
		s.discard()
	}
}

// Unsaved counts the window's typed changes: the palette's name, and the
// chosen colour's and ramp's fields.
func (s *State) Unsaved(frame.Docs) int { return frame.UnsavedFields(s.fields()...) }

// UnsavedName names what has unsaved changes.
func (s *State) UnsavedName(frame.Docs) string { return s.namedLabel() }

// Leave asks about unsaved changes before then, such as closing the window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.namedLabel(), then)
}

func (s *State) discard() {
	s.revertChoiceFields()
	s.name.Field.Revert()
	if s.focus.Current() == s.name {
		s.focus.Set(nil)
	}
}

// Save saves the palette's name, the ramp's name and the colour's label,
// value and opacity, from the save bar, Enter or Cmd+S. It reports false when
// a typed value cannot be saved, saying why in the status bar.
func (s *State) Save(ctx *frame.Context) bool {
	if s.ReadOnly() {
		return true
	}
	renamed := s.saveName(ctx)
	ramped := s.saveRampName(ctx)
	shaded := s.saveShade(ctx)

	return renamed && ramped && shaded
}

// saveName renames the palette, as the project browser does.
func (s *State) saveName(ctx *frame.Context) bool {
	typed, changed := s.name.Field.Commit()
	if !changed {
		return true
	}
	name := strings.TrimSpace(typed)
	if name == "" {
		s.name.Field.Failed()
		ctx.Frame.Status("A palette needs a name, such as gloomy")

		return false
	}
	args := project.RenameArgs{ID: s.target, Name: name}
	s.command(ctx, project.IndexID(), project.RenameEdit, "", args, func(protocol.Result) {
		s.name.Field.Revert()
		ctx.Frame.Status("Renamed the palette “" + name + "”")
	}, s.name.Field.Failed)

	return true
}

// saveRampName renames the chosen ramp.
func (s *State) saveRampName(ctx *frame.Context) bool {
	typed, changed := s.rampName.Field.Commit()
	if !changed || s.named.ramp.IsZero() {
		return true
	}
	name := strings.TrimSpace(typed)
	if name == "" {
		s.rampName.Field.Failed()
		ctx.Frame.Status("A ramp needs a name, such as grass")

		return false
	}
	args := artplugin.RenameRampArgs{Ramp: s.named.ramp, Name: name}
	s.command(ctx, s.target, artplugin.RenameRampEdit, "", args, func(protocol.Result) {
		s.rampName.Field.Revert()
		ctx.Frame.Status("Renamed the ramp “" + name + "”")
	}, s.rampName.Field.Failed)

	return true
}

// saveShade sets the chosen colour's label, value and opacity, all at once.
func (s *State) saveShade(ctx *frame.Context) bool {
	if s.named.shade.IsZero() {
		return true
	}
	texts := make([]string, 0, len(s.choiceFields()))
	changed := false
	for _, field := range []*ui.TextField{s.label, s.hex, s.opacity} {
		text, typed := field.Field.Commit()
		if !typed {
			text = field.Field.Saved()
		}
		changed = changed || typed
		texts = append(texts, text)
	}
	if !changed {
		return true
	}
	failed := func() {
		for _, field := range []*ui.TextField{s.label, s.hex, s.opacity} {
			field.Field.Failed()
		}
	}
	shade, problem := parseShade(s.named.shade, texts[0], texts[1], texts[2])
	if problem != "" {
		failed()
		ctx.Frame.Status(problem)

		return false
	}
	args := artplugin.SetShadeArgs{Ramp: s.named.ramp, Shade: shade}
	s.command(ctx, s.target, artplugin.SetShadeEdit, "", args, func(protocol.Result) {
		for _, field := range []*ui.TextField{s.label, s.hex, s.opacity} {
			field.Field.Revert()
		}
		ctx.Frame.Status("Saved the colour")
	}, failed)

	return true
}

// parseShade reads a typed label, colour, as #RRGGBB, and opacity, as a
// percentage, or says what is wrong with them.
func parseShade(shadeID ids.ULID, labelText, colorText, opacityText string) (art.Shade, string) {
	shade := art.Shade{ID: shadeID, Color: 0, Opacity: 0, Label: strings.TrimSpace(labelText)}
	if utf8.RuneCountInString(shade.Label) > art.MaxLabelLength {
		return shade, fmt.Sprintf("A label is at most %d characters", art.MaxLabelLength)
	}
	value, err := art.ParseRGB(strings.TrimSpace(colorText))
	if err != nil {
		return shade, "Enter a colour as six hexadecimal digits, such as #8A919C"
	}
	opacity, ok := parseOpacity(opacityText)
	if !ok {
		return shade, fmt.Sprintf("Enter an opacity from %d%% to %d%%", art.MinOpacity, art.MaxOpacity)
	}
	shade.Color, shade.Opacity = value, opacity

	return shade, ""
}

// parseOpacity reads a percentage, with or without "%".
func parseOpacity(text string) (int, bool) {
	opacity, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "%")))

	return opacity, err == nil && opacity >= art.MinOpacity && opacity <= art.MaxOpacity
}

// askUses asks where the chosen colour is used, and where each moved
// colour's old place is.
func (s *State) askUses(ctx *frame.Context) {
	if !s.chosen.shade.IsZero() {
		s.usesOf(ctx, artplugin.ShadeItem(s.chosen.shade))
	}
	palette, ok := s.palette(ctx.Docs)
	if !ok {
		return
	}
	for _, record := range palette.MovedShades() {
		s.usesOf(ctx, artplugin.RampItem(record.Ramp))
		s.usesOf(ctx, artplugin.ShadeItem(record.Shade))
	}
}

// layoutInspector places the inspector's fields and buttons for the chosen
// colour and ramp, and the save bar at its foot.
func (s *State) layoutInspector(palette *art.Palette) {
	panel := frame.Inspector(s.size)
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	for _, field := range s.choiceFields() {
		field.Rect = ui.Zero()
	}
	s.picker.Rect = ui.Zero()
	for _, button := range []*ui.Button{
		s.inspect.duplicate, s.inspect.moveTo, s.inspect.removeColor, s.inspect.gradientInto, s.inspect.removeRamp,
	} {
		button.Rect = ui.Zero()
	}
	s.slider.Rect = ui.Zero()
	bottom := panel.Max.Y - ui.Inset
	s.save.Rect = ui.Zero()
	if !s.ReadOnly() {
		s.save.Rect = image.Rect(left, bottom-ui.SaveBarHeight(width), left+width, bottom)
	}
	ramp, found := palette.Ramp(s.chosen.ramp)
	if !found || s.ReadOnly() {
		return
	}
	top := ui.Inset + ui.LineHeight + preview + ui.Inset
	if shade, _, has := palette.Lookup(ramp.ID, s.chosen.shade); has {
		top = s.layoutColor(palette, shade, left, width, top)
	}
	s.rampTop = top
	top += ui.LineHeight
	s.rampName.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	top = s.rampName.Rect.Max.Y + ui.LineHeight + ui.Gap
	half := (width - ui.Gap) / ui.Half
	s.inspect.gradientInto.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
	s.inspect.removeRamp.Rect = image.Rect(left+half+ui.Gap, top, left+width, top+ui.ButtonH)
	s.save.Unsaved = frame.UnsavedFields(s.fields()...)
}

// layoutColor places the chosen colour's fields, slider, picker and buttons
// from top down, and returns where the ramp's part starts.
func (s *State) layoutColor(palette *art.Palette, shade art.Shade, left, width, top int) int {
	s.label.Rect = image.Rect(left+labelColumn, top, left+width-counterWidth, top+ui.ButtonH)
	top = s.label.Rect.Max.Y + ui.Gap
	s.hex.Rect = image.Rect(left+labelColumn, top, left+labelColumn+colorWidth, top+ui.ButtonH)
	top = s.hex.Rect.Max.Y + ui.Gap
	s.opacity.Rect = image.Rect(left+labelColumn, top, left+labelColumn+opacityWidth, top+ui.ButtonH)
	s.slider.Rect = image.Rect(s.opacity.Rect.Max.X+ui.Inset, top, left+width, top+ui.ButtonH)
	top = s.opacity.Rect.Max.Y + ui.Inset
	s.picker.Rect = image.Rect(left, top, left+width, top+pickerHeight)
	s.syncPicker(shade)
	top = s.picker.Rect.Max.Y + ui.Inset + ui.LineHeight
	half := (width - ui.Gap) / ui.Half
	s.inspect.duplicate.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
	s.inspect.moveTo.Rect = image.Rect(left+half+ui.Gap, top, left+width, top+ui.ButtonH)
	s.inspect.moveTo.Disabled = len(palette.Ramps()) < ui.Half
	top = s.inspect.duplicate.Rect.Max.Y + ui.Gap
	s.inspect.removeColor.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)

	return s.inspect.removeColor.Rect.Max.Y + ui.Half*ui.Inset
}

// drawInspector draws the chosen colour and its ramp: typed fields for a
// palette of a game's own, or what Default holds, read only.
func (s *State) drawInspector(screen *ebiten.Image, docs frame.Docs, palette *art.Palette) {
	frame.DrawInspector(screen)
	panel := frame.Inspector(s.size)
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	ramp, found := palette.Ramp(s.chosen.ramp)
	if !found {
		s.theme.Heading(screen, "COLOUR", left, ui.Inset)
		s.theme.DrawText(screen, "Choose a colour or a ramp's name.", left, ui.Inset+ui.LineHeight, ui.ColorMuted())
		s.drawReadOnlyNote(screen, left, width)
		s.save.Draw(screen, s.theme)

		return
	}
	shade, _, has := palette.Lookup(ramp.ID, s.chosen.shade)
	name, _ := palette.ShadeName(ramp.ID, shade.ID)
	if !has {
		name = ramp.Name
	}
	s.theme.Heading(screen, "COLOUR · "+strings.ToUpper(name), left, ui.Inset)
	if has {
		s.drawChosen(screen, docs, shade, left, width)
	}
	if !s.ReadOnly() {
		s.theme.Heading(screen, "RAMP", left, s.rampTop)
		s.rampName.Draw(screen)
		count := strconv.Itoa(len(ramp.Shades)) + " colours"
		s.theme.DrawText(screen, s.theme.FitText(count, width), left, s.rampName.Rect.Max.Y+ui.Gap, ui.ColorMuted())
		s.inspect.gradientInto.Draw(screen, s.theme)
		s.inspect.removeRamp.Draw(screen, s.theme)
	}
	s.drawReadOnlyNote(screen, left, width)
	s.save.Draw(screen, s.theme)
}

// drawChosen draws the chosen colour: its swatch and name, then its fields,
// or for Default its value, and where it is used.
func (s *State) drawChosen(screen *ebiten.Image, docs frame.Docs, shade art.Shade, left, width int) {
	top := ui.Inset + ui.LineHeight
	box := image.Rect(left, top, left+preview, top+preview)
	drawSwatch(screen, box, shade, false)
	if after := s.typedShade(shade); after != shade {
		drawSwatch(screen, image.Rect(box.Min.X+preview/ui.Half, box.Min.Y, box.Max.X, box.Max.Y), after, false)
	}
	label := "no label"
	if shade.Label != "" {
		label = "“" + shade.Label + "”"
	}
	s.theme.DrawText(screen, label, box.Max.X+ui.Inset, top+ui.Gap, ui.ColorMuted())
	s.theme.DrawText(screen, fmt.Sprintf("%s · %d%%", shade.Color, shade.Opacity), box.Max.X+ui.Inset,
		top+ui.Gap+ui.LineHeight, ui.ColorMuted())
	usedTop := top + preview + ui.Inset
	if !s.ReadOnly() {
		for _, field := range []struct {
			widget *ui.TextField
			label  string
		}{{s.label, "Label"}, {s.hex, "Colour"}, {s.opacity, "Opacity"}} {
			s.theme.DrawText(screen, field.label, left, field.widget.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1,
				ui.ColorMuted())
			field.widget.Draw(screen)
		}
		counter := fmt.Sprintf("%d / %d", utf8.RuneCountInString(s.label.Field.Value()), art.MaxLabelLength)
		s.theme.DrawText(screen, counter, s.label.Rect.Max.X+ui.Gap,
			s.label.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorMuted())
		s.picker.Draw(screen)
		usedTop = s.picker.Rect.Max.Y + ui.Inset
		s.slider.Draw(screen)
		s.inspect.duplicate.Draw(screen, s.theme)
		s.inspect.moveTo.Draw(screen, s.theme)
		s.inspect.removeColor.Draw(screen, s.theme)
	}
	s.theme.DrawText(screen, s.theme.FitText(s.usedLine(docs, shade.ID), width), left, usedTop, ui.ColorMuted())
}

// typedShade is the chosen colour with the value and opacity its fields
// hold, typed or picked; a field that does not read keeps the saved one.
func (s *State) typedShade(shade art.Shade) art.Shade {
	if typed, err := art.ParseRGB(strings.TrimSpace(s.hex.Field.Value())); err == nil {
		shade.Color = typed
	}
	if opacity, ok := parseOpacity(s.opacity.Field.Value()); ok {
		shade.Opacity = opacity
	}

	return shade
}

// usedLine says which documents are painted with a colour.
func (s *State) usedLine(_ frame.Docs, shade ids.ULID) string {
	uses, known := s.uses[artplugin.ShadeItem(shade)]
	switch {
	case !known:
		return "Looking for where it is used…"
	case len(uses) == 0:
		return "No tile is painted with it."
	}
	names := make([]string, 0, len(uses))
	for _, use := range uses {
		if !slices.Contains(names, use.Name) {
			names = append(names, use.Name)
		}
	}

	return "Used by " + strings.Join(names, ", ")
}

// drawReadOnlyNote says, for Default, why nothing can be changed.
func (s *State) drawReadOnlyNote(screen *ebiten.Image, left, width int) {
	if !s.ReadOnly() {
		return
	}
	bottom := frame.Inspector(s.size).Max.Y - ui.Inset
	lines := []string{
		"Default can't be changed.",
		"It is the engine's own palette, the same",
		"in every game. To change a colour, copy it",
		"into a palette of your own and edit the copy.",
	}
	top := bottom - len(lines)*ui.LineHeight
	for at, line := range lines {
		ink := ui.ColorMuted()
		if at == 0 {
			ink = ui.ColorInk()
		}
		s.theme.DrawText(screen, s.theme.FitText(line, width), left, top+at*ui.LineHeight, ink)
	}
}

// openCopyTo offers the project's palettes, other than this one and
// Default, to copy the picked colours into.
func (s *State) openCopyTo(ctx *frame.Context) {
	others := s.otherPalettes(ctx)
	items := make([]ui.MenuItem, 0, len(others))
	for _, entry := range others {
		items = append(items, ui.MenuItem{
			ID: entry.ID.String(), Label: entry.Name, Icon: ui.IconPalette, Disabled: false, Danger: false, Rule: false,
		})
	}
	if len(items) == 0 {
		ctx.Frame.Status("There is no palette of the game's own to copy into yet")

		return
	}
	s.openMenu(menuCopyTo, items, s.bar.copyTo.Rect)
}

// otherPalettes are the project's palettes other than this one and Default,
// A to Z, each subscribed to so its ramps are known.
func (s *State) otherPalettes(ctx *frame.Context) []project.Entry {
	var entries []project.Entry
	ctx.Docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = index.OfFormat(art.PaletteFormatName)
		}
	})
	entries = slices.DeleteFunc(entries, func(entry project.Entry) bool {
		return entry.ID == s.target || entry.ID == art.DefaultPaletteID()
	})
	slices.SortFunc(entries, func(left, right project.Entry) int {
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	for _, entry := range entries {
		s.subscribe(entry.ID, ctx)
	}

	return entries
}

// openMenu opens the window's menu below a button, for a purpose.
func (s *State) openMenu(purpose string, items []ui.MenuItem, anchor image.Rectangle) {
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	s.menu.Items = items
	s.menu.Width = min(ui.MenuWidth(items, s.theme), window.Dx()-ui.Half*ui.Inset)
	s.menu.Open(anchor, window)
	s.menuFor = purpose
}

// updateMenu handles the open menu, which takes all input.
func (s *State) updateMenu(input *ui.Input, ctx *frame.Context) {
	chosen := s.menu.Update(input)
	input.ClearAll()
	if chosen == "" {
		return
	}
	switch s.menuFor {
	case menuCopyTo:
		if target, err := ids.Parse(chosen); err == nil {
			s.copyPicked(ctx, target)
		}
	case menuInto, menuCopyInto, menuCopySource:
		s.chosePanelItem(ctx, s.menuFor, chosen)
	case menuMoveTo:
		palette, ok := s.palette(ctx.Docs)
		ramp, err := ids.Parse(chosen)
		if !ok || err != nil {
			return
		}
		if to, found := palette.Ramp(ramp); found {
			s.moveShade(ctx, palette, s.chosen.ramp, s.chosen.shade, ramp, len(to.Shades))
		}
	}
}

// copyPicked copies the picked colours into another palette, as one undo
// step: each source ramp's colours go to the ramp of that name there, or to
// a new ramp of that name. Colour and opacity are copied; labels never are.
func (s *State) copyPicked(ctx *frame.Context, target ids.ULID) {
	source, ok := s.palette(ctx.Docs)
	into, found := paletteOf(ctx.Docs, target)
	if !ok || !found {
		ctx.Frame.Status("That palette is still loading; try again")

		return
	}
	groups := pickedByRamp(source, s.picked)
	gesture := s.gesture()
	count, added := 0, 0
	for _, group := range groups {
		count += len(group.shades)
		copies := copiesOf(group.shades)
		if ramp, exists := rampNamed(into, group.name); exists {
			for at, shade := range copies {
				args := artplugin.AddShadeArgs{Ramp: ramp.ID, Index: len(ramp.Shades) + at, Shade: shade}
				s.command(ctx, target, artplugin.AddShadeEdit, gesture, args, nil, nil)
			}

			continue
		}
		ramp := art.Ramp{ID: ids.ULID{}, Name: group.name, Shades: copies}
		args := artplugin.AddRampArgs{Index: len(into.Ramps()) + added, Ramp: ramp}
		s.command(ctx, target, artplugin.AddRampEdit, gesture, args, nil, nil)
		added++
	}
	name := documentName(ctx.Docs, target, "the palette")
	ctx.Frame.Status(fmt.Sprintf("Copied %d colours into %s, without labels", count, name))
}

// rampGroup is picked colours of one source ramp, in order.
type rampGroup struct {
	name   string
	shades []art.Shade
}

// pickedByRamp groups picked colours by their ramp, in the palette's order.
func pickedByRamp(palette *art.Palette, picked []ids.ULID) []rampGroup {
	var groups []rampGroup
	for _, ramp := range palette.Ramps() {
		group := rampGroup{name: ramp.Name, shades: nil}
		for _, shade := range ramp.Shades {
			if slices.Contains(picked, shade.ID) {
				group.shades = append(group.shades, shade)
			}
		}
		if len(group.shades) > 0 {
			groups = append(groups, group)
		}
	}

	return groups
}

// copiesOf returns independent copies of colours: no IDs yet, and no labels.
func copiesOf(shades []art.Shade) []art.Shade {
	copies := make([]art.Shade, 0, len(shades))
	for _, shade := range shades {
		copies = append(copies, art.Shade{ID: ids.ULID{}, Color: shade.Color, Opacity: shade.Opacity, Label: ""})
	}

	return copies
}

// rampNamed returns a palette's ramp of a name.
func rampNamed(palette *art.Palette, name string) (art.Ramp, bool) {
	for _, ramp := range palette.Ramps() {
		if ramp.Name == name {
			return ramp, true
		}
	}

	return art.Ramp{ID: ids.ULID{}, Name: "", Shades: nil}, false
}

// newPalette makes a palette of the game's own from the picked colours and
// opens it: a ramp picked whole keeps its name, and single colours go
// together into the first ramp, named "default". Labels are not copied.
func (s *State) newPalette(ctx *frame.Context, palette *art.Palette) {
	var ramps []art.Ramp
	singles := art.Ramp{ID: ids.ULID{}, Name: singlesRamp, Shades: nil}
	for _, group := range pickedByRamp(palette, s.picked) {
		whole, _ := rampNamed(palette, group.name)
		if len(group.shades) == len(whole.Shades) {
			ramps = append(ramps, art.Ramp{ID: ids.ULID{}, Name: group.name, Shades: copiesOf(group.shades)})

			continue
		}
		singles.Shades = append(singles.Shades, copiesOf(group.shades)...)
	}
	if len(singles.Shades) > 0 {
		ramps = append([]art.Ramp{singles}, ramps...)
	}
	data, err := json.Marshal(struct {
		Ramps []art.Ramp `json:"ramps"`
	}{Ramps: ramps})
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	name := uniqueDocumentName(ctx.Docs, newPaletteAs)
	args := project.DocumentArgs{
		ID: ids.ULID{}, Format: art.PaletteFormatName, Name: name, Version: 0, Revision: 0, Scope: ids.ULID{},
		Folder: "", Tags: nil, Data: data, Layout: nil,
	}
	count := len(s.picked)
	s.command(ctx, project.IndexID(), project.CreateEdit, "", args, func(result protocol.Result) {
		ctx.Requests.Open(ViewName, result.Created, false)
		ctx.Frame.Status(fmt.Sprintf("Made “%s” from %d colours of %s", name, count, s.paletteName(ctx.Docs)))
	}, nil)
}

// uniqueDocumentName is the first of a name, then the name numbered from 2,
// that no palette of the project has.
func uniqueDocumentName(docs frame.Docs, base string) string {
	taken := map[string]bool{}
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			for _, entry := range index.OfFormat(art.PaletteFormatName) {
				taken[strings.ToLower(entry.Name)] = true
			}
		}
	})
	name := base
	for number := firstNumber; taken[strings.ToLower(name)]; number++ {
		name = base + " " + strconv.Itoa(number)
	}

	return name
}

// otherRamps offers the ramps the chosen colour can move to: every ramp but
// its own.
func (s *State) otherRamps(palette *art.Palette) []ui.MenuItem {
	items := make([]ui.MenuItem, 0, len(palette.Ramps()))
	for _, ramp := range palette.Ramps() {
		if ramp.ID != s.chosen.ramp {
			items = append(items, ui.MenuItem{
				ID: ramp.ID.String(), Label: ramp.Name, Icon: ui.IconNone, Disabled: false, Danger: false, Rule: false,
			})
		}
	}

	return items
}

// syncPicker shows in the picker what the colour and opacity fields hold.
func (s *State) syncPicker(shade art.Shade) {
	typed := s.typedShade(shade)
	s.picker.SetColor(color.RGBA(typed.Color.NRGBA(art.MaxOpacity)), float64(typed.Opacity)/art.MaxOpacity)
	if !s.slider.Dragging() {
		s.slider.Value = typed.Opacity
	}
}

// updatePicker puts what is picked into the colour and opacity fields, where
// it waits for Save as typing does.
func (s *State) updatePicker(input *ui.Input) {
	if input.Pressed && input.Cursor.In(s.picker.Rect) {
		s.focus.Set(nil)
	}
	if input.Pressed && input.Cursor.In(s.slider.Rect) {
		s.focus.Set(nil)
	}
	if s.slider.Update(input) {
		s.opacity.Field.SetDraft(strconv.Itoa(s.slider.Value))
	}
	if !s.picker.Update(input) {
		return
	}
	s.hex.Field.SetDraft(pickedRGB(s.picker).String())
	s.opacity.Field.SetDraft(strconv.Itoa(pickedOpacity(s.picker)))
}

// pickedRGB is the colour a picker shows.
func pickedRGB(picker *ui.ColorPicker) art.RGB {
	rgb := picker.RGB()

	return art.RGB(uint32(rgb.R)<<redShift | uint32(rgb.G)<<greenShift | uint32(rgb.B))
}

// pickedOpacity is the opacity a picker shows, in percent.
func pickedOpacity(picker *ui.ColorPicker) int {
	opacity := int(math.Round(picker.Opacity() * art.MaxOpacity))

	return min(max(opacity, art.MinOpacity), art.MaxOpacity)
}

// Where red and green sit in a colour's value.
const (
	redShift   = 16
	greenShift = 8
)
