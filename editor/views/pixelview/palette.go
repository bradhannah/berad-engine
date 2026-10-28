package pixelview

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Palette panel geometry, in screen pixels.
const (
	swatchSize   = 18
	swatchStep   = 21
	rampColumn   = 76
	labelColumn  = 64
	colorWidth   = 96
	opacityWidth = 56
	buttonWidth  = 112
	chosenWidth  = 2
	// A new ramp is named newRampName, numbered after the first, and starts
	// with one shade of newShade.
	newRampName = "new ramp"
	newShade    = art.RGB(0x808080)
	firstNumber = 2
)

// swatch is one colour the panel offers, clear or a shade, where it is
// drawn, and what scripts call it: "clear" or a shade's name.
type swatch struct {
	rect  image.Rectangle
	color art.ColorRef
	name  string
}

// rampLabel is a ramp's name in the panel, which chooses the ramp.
type rampLabel struct {
	rect image.Rectangle
	ramp art.Ramp
}

// panelLayout is where the palette panel's parts are this frame: the
// swatches and ramp names, and the tops of its headings.
type panelLayout struct {
	swatches  []swatch
	ramps     []rampLabel
	rampTop   int
	shadeTop  int
	shadeName string
	showsTop  int
}

func newPanelLayout() panelLayout {
	return panelLayout{swatches: nil, ramps: nil, rampTop: 0, shadeTop: 0, shadeName: "", showsTop: 0}
}

// chosenRamp returns the shared palette's ramp the panel shows, and its
// shade when one is chosen.
func (s *State) chosenRamp(shared *art.Palette) (art.Ramp, art.Shade, int, bool) {
	var none art.Shade
	if shared == nil || s.color.Palette != art.SharedPaletteID() {
		return art.Ramp{ID: ids.ULID{}, Name: "", Shades: nil}, none, 0, false
	}
	ramp, ok := shared.Ramp(s.color.Ramp)
	if !ok {
		return ramp, none, 0, false
	}
	shade, number, found := shared.Lookup(ramp.ID, s.color.Shade)
	if !found {
		return ramp, none, 0, true
	}

	return ramp, shade, number, true
}

// layoutPanel places the palette panel: the clear colour and each ramp's
// shades, + Ramp, the chosen ramp's name, the chosen shade's colour and
// opacity with their buttons, and at the foot Shows and the save bar.
func (s *State) layoutPanel(docs frame.Docs, shared *art.Palette) {
	inspector := frame.Inspector(s.size)
	left, width := inspector.Min.X+ui.Inset, inspector.Dx()-ui.Half*ui.Inset
	for _, widget := range []*ui.TextField{s.rampName, s.shadeColor, s.shadeOpacity} {
		widget.Rect = ui.Zero()
	}
	for _, button := range []*ui.Button{s.addRamp, s.addShade, s.removeShade, s.removeRamp} {
		button.Rect = ui.Zero()
	}
	s.panel = newPanelLayout()
	bottom := inspector.Max.Y - ui.Inset
	s.save.Rect = image.Rect(left, bottom-ui.SaveBarHeight(width), left+width, bottom)
	s.save.Unsaved = s.Unsaved(docs)
	s.layoutShows(docs, left, width)
	if shared == nil {
		return
	}
	top := s.layoutSwatches(shared, left, width)
	s.addRamp.Rect = image.Rect(left, top, left+buttonWidth, top+ui.ButtonH)
	ramp, shade, number, chosen := s.chosenRamp(shared)
	if !chosen {
		return
	}
	top = s.addRamp.Rect.Max.Y + ui.Inset
	s.panel.rampTop = top
	s.rampName.Rect = image.Rect(left, top+ui.LineHeight, left+width, top+ui.LineHeight+ui.ButtonH)
	top = s.rampName.Rect.Max.Y + ui.Inset
	if !shade.ID.IsZero() {
		s.panel.shadeTop, s.panel.shadeName = top, ramp.Name+" "+strconv.Itoa(number)
		top += ui.LineHeight
		s.shadeColor.Rect = image.Rect(left+labelColumn, top, left+labelColumn+colorWidth, top+ui.ButtonH)
		top = s.shadeColor.Rect.Max.Y + ui.Gap
		s.shadeOpacity.Rect = image.Rect(left+labelColumn, top, left+labelColumn+opacityWidth, top+ui.ButtonH)
		top = s.shadeOpacity.Rect.Max.Y + ui.Inset
	}
	s.addShade.Rect = image.Rect(left, top, left+buttonWidth, top+ui.ButtonH)
	if !shade.ID.IsZero() {
		s.removeShade.Rect = image.Rect(left+buttonWidth+ui.Gap, top, left+ui.Half*buttonWidth+ui.Gap, top+ui.ButtonH)
	}
	top = s.addShade.Rect.Max.Y + ui.Gap
	s.removeRamp.Rect = image.Rect(left, top, left+buttonWidth, top+ui.ButtonH)
}

// layoutSwatches places the clear colour and every ramp's name and shades,
// a ramp's shades wrapping onto further rows, and returns the top of what
// follows.
func (s *State) layoutSwatches(shared *art.Palette, left, width int) int {
	top := ui.Inset + ui.LineHeight
	s.panel.swatches = append(s.panel.swatches, swatch{
		rect: image.Rect(left, top, left+swatchSize, top+swatchSize), color: clearColor(), name: "clear",
	})
	top += swatchStep
	perRow := max(1, (width-rampColumn)/swatchStep)
	for _, ramp := range shared.Ramps() {
		s.panel.ramps = append(s.panel.ramps, rampLabel{
			rect: image.Rect(left, top, left+rampColumn-ui.Gap, top+swatchSize), ramp: ramp,
		})
		for index, shade := range ramp.Shades {
			if index > 0 && index%perRow == 0 {
				top += swatchStep
			}
			x := left + rampColumn + index%perRow*swatchStep
			ref := art.ColorRef{Palette: art.SharedPaletteID(), Ramp: ramp.ID, Shade: shade.ID}
			s.panel.swatches = append(s.panel.swatches, swatch{
				rect: image.Rect(x, top, x+swatchSize, top+swatchSize), color: ref,
				name: ramp.Name + " " + strconv.Itoa(index+1),
			})
		}
		top += swatchStep
	}

	return top + ui.Gap
}

// updatePanel handles the panel's buttons and a click on a swatch or a ramp's
// name.
func (s *State) updatePanel(input *ui.Input, ctx *frame.Context, shared *art.Palette) {
	switch {
	case s.addRamp.Update(input):
		s.addRampTo(ctx, shared)
	case s.addShade.Update(input):
		s.addShadeTo(ctx, shared)
	case s.removeShade.Update(input):
		s.removeChosenShade(ctx, shared)
	case s.removeRamp.Update(input):
		s.removeChosenRamp(ctx, shared)
	case s.shows.Update(input):
		s.openShows(ctx.Docs)
	case input.Pressed:
		s.choosePanel(input)
	}
}

// choosePanel chooses the colour of a clicked swatch, or a clicked ramp:
// its first shade, or the ramp alone when it has none.
func (s *State) choosePanel(input *ui.Input) {
	for _, offered := range s.panel.swatches {
		if input.Cursor.In(offered.rect) {
			s.color = offered.color
			input.ClearPointer()

			return
		}
	}
	for _, label := range s.panel.ramps {
		if input.Cursor.In(label.rect) {
			s.chooseShade(label.ramp, 0)
			input.ClearPointer()

			return
		}
	}
}

// chooseShade chooses a ramp's shade by its index, the nearest one when the
// index is past its end, or the ramp alone when it has no shades.
func (s *State) chooseShade(ramp art.Ramp, index int) {
	s.color = art.ColorRef{Palette: art.SharedPaletteID(), Ramp: ramp.ID, Shade: ids.ULID{}}
	if len(ramp.Shades) > 0 {
		s.color.Shade = ramp.Shades[min(max(index, 0), len(ramp.Shades)-1)].ID
	}
}

// addRampTo adds a ramp at the end of the shared palette with one shade, and
// chooses it.
func (s *State) addRampTo(ctx *frame.Context, shared *art.Palette) {
	name := uniqueRampName(shared)
	index := len(shared.Ramps())
	ramp := art.Ramp{
		ID: ids.ULID{}, Name: name,
		Shades: []art.Shade{{ID: ids.ULID{}, Color: newShade, Opacity: art.MaxOpacity}},
	}
	s.command(ctx, art.SharedPaletteID(), artplugin.AddRampEdit, "", artplugin.AddRampArgs{Index: index, Ramp: ramp},
		func() {
			if updated, ok := palette(ctx.Docs, art.SharedPaletteID()); ok && index < len(updated.Ramps()) {
				s.chooseShade(updated.Ramps()[index], 0)
			}
			ctx.Frame.Status("Added the ramp “" + name + "”. Name it for its use, such as grass.")
		}, nil)
}

// uniqueRampName is the first of "new ramp", "new ramp 2"… no ramp has.
func uniqueRampName(shared *art.Palette) string {
	taken := map[string]bool{}
	for _, ramp := range shared.Ramps() {
		taken[ramp.Name] = true
	}
	name := newRampName
	for number := firstNumber; taken[name]; number++ {
		name = newRampName + " " + strconv.Itoa(number)
	}

	return name
}

// addShadeTo adds a shade to the chosen ramp: after the chosen shade, as a
// copy of it, or at the end when no shade is chosen. The new shade is chosen.
func (s *State) addShadeTo(ctx *frame.Context, shared *art.Palette) {
	ramp, shade, number, chosen := s.chosenRamp(shared)
	if !chosen {
		return
	}
	index := len(ramp.Shades)
	added := art.Shade{ID: ids.ULID{}, Color: newShade, Opacity: art.MaxOpacity}
	if !shade.ID.IsZero() {
		index, added.Color, added.Opacity = number, shade.Color, shade.Opacity
	}
	args := artplugin.AddShadeArgs{Ramp: ramp.ID, Index: index, Shade: added}
	s.command(ctx, art.SharedPaletteID(), artplugin.AddShadeEdit, "", args, func() {
		if updated, ok := palette(ctx.Docs, art.SharedPaletteID()); ok {
			if grown, found := updated.Ramp(ramp.ID); found {
				s.chooseShade(grown, index)
			}
		}
		ctx.Frame.Status(fmt.Sprintf("Added %s %d", ramp.Name, index+1))
	}, nil)
}

// removeChosenShade removes the chosen shade and chooses its neighbour. A
// shade a tile is painted with is refused, naming the tiles.
func (s *State) removeChosenShade(ctx *frame.Context, shared *art.Palette) {
	ramp, shade, number, chosen := s.chosenRamp(shared)
	if !chosen || shade.ID.IsZero() {
		return
	}
	name := ramp.Name + " " + strconv.Itoa(number)
	args := artplugin.RemoveShadeArgs{Ramp: ramp.ID, Shade: shade.ID}
	s.command(ctx, art.SharedPaletteID(), artplugin.RemoveShadeEdit, "", args, func() {
		if updated, ok := palette(ctx.Docs, art.SharedPaletteID()); ok {
			if shrunk, found := updated.Ramp(ramp.ID); found {
				s.chooseShade(shrunk, number-1)
			}
		}
		ctx.Frame.Status("Removed " + name)
	}, nil)
}

// removeChosenRamp removes the chosen ramp and chooses clear. A ramp a tile
// is painted with is refused, naming the tiles.
func (s *State) removeChosenRamp(ctx *frame.Context, shared *art.Palette) {
	ramp, _, _, chosen := s.chosenRamp(shared)
	if !chosen {
		return
	}
	args := artplugin.RemoveRampArgs{Ramp: ramp.ID}
	s.command(ctx, art.SharedPaletteID(), artplugin.RemoveRampEdit, "", args, func() {
		s.color = clearColor()
		ctx.Frame.Status("Removed the ramp “" + ramp.Name + "”")
	}, nil)
}

// syncFields keeps the typed fields showing the chosen ramp and shade. A
// choice that is gone, removed in this window or another, falls back to its
// ramp, or to clear. The guard has already asked about unsaved changes
// before the choice moved.
func (s *State) syncFields(shared *art.Palette) {
	if s.color != s.named {
		s.revertFields()
		s.named = s.color
	}
	if shared == nil || s.color.Palette != art.SharedPaletteID() {
		return
	}
	ramp, shade, _, chosen := s.chosenRamp(shared)
	switch {
	case !chosen:
		s.color, s.named = clearColor(), clearColor()
		s.revertFields()

		return
	case shade.ID.IsZero() && !s.color.Shade.IsZero():
		s.color.Shade = ids.ULID{}
		s.named = s.color
		s.revertFields()
	}
	s.rampName.Field.SetSaved(ramp.Name)
	if !shade.ID.IsZero() {
		s.shadeColor.Field.SetSaved(shade.Color.String())
		s.shadeOpacity.Field.SetSaved(strconv.Itoa(shade.Opacity))
	}
}

func (s *State) fields() []*ui.TextField {
	return []*ui.TextField{s.rampName, s.shadeColor, s.shadeOpacity}
}

func (s *State) revertFields() {
	if slices.Contains(s.fields(), s.focus.Current()) {
		s.focus.Set(nil)
	}
	for _, field := range s.fields() {
		field.Field.Revert()
	}
}

// guardShade asks about unsaved changes before the panel moves to another
// colour: the choice goes back until the author decides.
func (s *State) guardShade() {
	if s.color == s.named || frame.UnsavedFields(s.fields()...) == 0 {
		return
	}
	wanted := s.color
	s.color = s.named
	s.stroke.active = false
	s.focus.Set(nil)
	s.guard.Ask(frame.UnsavedFields(s.fields()...), s.namedLabel(), func() { s.color = wanted })
}

// namedLabel names what the typed fields belong to: the shade, or the ramp.
func (s *State) namedLabel() string {
	if s.panel.shadeName != "" {
		return s.panel.shadeName
	}

	return s.rampName.Field.Saved()
}

// updateFields handles the typed fields and the save bar: Enter in a field
// saves.
func (s *State) updateFields(input *ui.Input, ctx *frame.Context) {
	if field, outcome := s.focus.Update(input, ctx.Clipboard); field != nil && outcome == textedit.OutcomeCommit {
		s.Save(ctx)
	}
	s.save.Unsaved = s.Unsaved(ctx.Docs)
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)
	case ui.RevertID:
		s.discard()
	}
}

// Unsaved counts the panel's typed changes.
func (s *State) Unsaved(frame.Docs) int { return frame.UnsavedFields(s.fields()...) }

// UnsavedName names the shade or ramp with unsaved changes.
func (s *State) UnsavedName(frame.Docs) string { return s.namedLabel() }

// Leave asks about unsaved changes before then, such as closing the window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.namedLabel(), then)
}

// discard drops the panel's typed changes.
func (s *State) discard() { s.revertFields() }

// Save saves the ramp's name and the shade's colour and opacity, from the
// save bar, Enter or Cmd+S. It reports false when a typed value cannot be
// saved, saying why in the status bar.
func (s *State) Save(ctx *frame.Context) bool {
	if s.named.Palette != art.SharedPaletteID() || s.named.Ramp.IsZero() {
		return true
	}
	named := s.saveRampName(ctx)
	shaded := s.saveShade(ctx)

	return named && shaded
}

// saveRampName renames the chosen ramp.
func (s *State) saveRampName(ctx *frame.Context) bool {
	typed, changed := s.rampName.Field.Commit()
	if !changed {
		return true
	}
	name := strings.TrimSpace(typed)
	if name == "" {
		s.rampName.Field.Failed()
		ctx.Frame.Status("A ramp needs a name, such as grass")

		return false
	}
	args := artplugin.RenameRampArgs{Ramp: s.named.Ramp, Name: name}
	s.command(ctx, art.SharedPaletteID(), artplugin.RenameRampEdit, "", args, func() {
		s.focus.Set(nil)
		s.rampName.Field.Revert()
		ctx.Frame.Status("Renamed the ramp “" + name + "”")
	}, s.rampName.Field.Failed)

	return true
}

// saveShade sets the chosen shade's colour and opacity, both at once.
func (s *State) saveShade(ctx *frame.Context) bool {
	if s.named.Shade.IsZero() {
		return true
	}
	colorText, colorChanged := s.shadeColor.Field.Commit()
	opacityText, opacityChanged := s.shadeOpacity.Field.Commit()
	if !colorChanged && !opacityChanged {
		return true
	}
	if !colorChanged {
		colorText = s.shadeColor.Field.Saved()
	}
	if !opacityChanged {
		opacityText = s.shadeOpacity.Field.Saved()
	}
	failed := func() {
		s.shadeColor.Field.Failed()
		s.shadeOpacity.Field.Failed()
	}
	shade, problem := parseShade(s.named.Shade, colorText, opacityText)
	if problem != "" {
		failed()
		ctx.Frame.Status(problem)

		return false
	}
	args := artplugin.SetShadeArgs{Ramp: s.named.Ramp, Shade: shade}
	saved := s.named
	s.command(ctx, art.SharedPaletteID(), artplugin.SetShadeEdit, "", args, func() {
		s.focus.Set(nil)
		s.shadeColor.Field.Revert()
		s.shadeOpacity.Field.Revert()
		if name, ok := s.colorName(ctx.Docs, saved); ok {
			ctx.Frame.Status("Saved " + name)
		}
	}, failed)

	return true
}

// parseShade reads a typed colour, as #RRGGBB, and opacity, as a percentage,
// or says what is wrong with them.
func parseShade(shadeID ids.ULID, colorText, opacityText string) (art.Shade, string) {
	shade := art.Shade{ID: shadeID, Color: 0, Opacity: 0}
	value, err := art.ParseRGB(strings.TrimSpace(colorText))
	if err != nil {
		return shade, "Enter a colour as six hexadecimal digits, such as #8A919C"
	}
	opacity, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(opacityText), "%")))
	if err != nil || opacity < art.MinOpacity || opacity > art.MaxOpacity {
		return shade, fmt.Sprintf("Enter an opacity from %d%% to %d%%", art.MinOpacity, art.MaxOpacity)
	}
	shade.Color, shade.Opacity = value, opacity

	return shade, ""
}

// drawPanel draws the palette panel.
func (s *State) drawPanel(screen *ebiten.Image, shared *art.Palette) {
	frame.DrawInspector(screen)
	inspector := frame.Inspector(s.size)
	left := inspector.Min.X + ui.Inset
	s.theme.Heading(screen, "PALETTE · "+strings.ToUpper(art.SharedPaletteName), left, ui.Inset)
	s.drawShows(screen, left)
	if shared == nil {
		s.theme.DrawText(screen, "Loading…", left, ui.Inset+ui.LineHeight, ui.ColorMuted())

		return
	}
	s.theme.DrawText(screen, "clear", left+swatchSize+ui.Gap*ui.Half, ui.Inset+ui.LineHeight+ui.Gap/ui.Half,
		ui.ColorMuted())
	for _, label := range s.panel.ramps {
		s.theme.DrawText(screen, s.theme.FitText(label.ramp.Name, label.rect.Dx()), label.rect.Min.X,
			label.rect.Min.Y+ui.Gap/ui.Half, ui.ColorInk())
		if s.color.Ramp == label.ramp.ID && s.color.Shade.IsZero() {
			ui.StrokeRound(screen, label.rect.Inset(-chosenWidth), ui.Radius, chosenWidth, ui.ColorSelect())
		}
	}
	for _, offered := range s.panel.swatches {
		s.drawSwatch(screen, shared, offered)
	}
	s.addRamp.Draw(screen, s.theme)
	if !s.rampName.Rect.Empty() {
		s.theme.Heading(screen, "RAMP", left, s.panel.rampTop)
		s.rampName.Draw(screen)
	}
	if !s.shadeColor.Rect.Empty() {
		s.theme.Heading(screen, "SHADE · "+strings.ToUpper(s.panel.shadeName), left, s.panel.shadeTop)
		for _, field := range []struct {
			widget *ui.TextField
			label  string
		}{{s.shadeColor, "Colour"}, {s.shadeOpacity, "Opacity"}} {
			s.theme.DrawText(screen, field.label, left, field.widget.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1,
				ui.ColorMuted())
			field.widget.Draw(screen)
		}
		s.theme.DrawText(screen, "%", s.shadeOpacity.Rect.Max.X+ui.Gap,
			s.shadeOpacity.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorMuted())
	}
	for _, button := range []*ui.Button{s.addShade, s.removeShade, s.removeRamp} {
		button.Draw(screen, s.theme)
	}
	s.save.Draw(screen, s.theme)
}

// drawSwatch draws one colour: see-through shades over a checkerboard, and
// the chosen colour outlined in blue.
func (s *State) drawSwatch(screen *ebiten.Image, shared *art.Palette, offered swatch) {
	rect := offered.rect
	half := rect.Dx() / ui.Half
	ui.FillRect(screen, rect, ui.ColorBackground())
	ui.FillRect(screen, image.Rect(rect.Min.X+half, rect.Min.Y, rect.Max.X, rect.Min.Y+half), ui.Hex(hexChecker))
	ui.FillRect(screen, image.Rect(rect.Min.X, rect.Min.Y+half, rect.Min.X+half, rect.Max.Y), ui.Hex(hexChecker))
	if shade, _, known := shared.Lookup(offered.color.Ramp, offered.color.Shade); known && !offered.color.IsClear() {
		var ink color.Color = shade.Color.NRGBA(shade.Opacity)
		ui.FillRect(screen, rect, ink)
	}
	ui.StrokeRect(screen, rect, ui.Hairline, ui.ColorRule())
	if offered.color == s.color {
		ui.StrokeRect(screen, rect.Inset(-chosenWidth), chosenWidth, ui.ColorSelect())
	}
}
