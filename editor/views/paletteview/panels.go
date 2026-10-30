package paletteview

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Panel geometry, in screen pixels.
const (
	gradientHeight = 176
	copyHeight     = 260
	toggleWidth    = 84
	stepsWidth     = 44
	rampNameWidth  = 140
	previewSize    = 22
	previewStep    = 25
	sourceSize     = 16
	sourceStep     = 19
	sourceName     = 72
	gradientName   = "gradient"
	newRampLabel   = "New ramp"
	defaultSteps   = 5
	maxPreview     = 24
	// The gradient's picker sits right of its fields, or below them when the
	// window is too narrow.
	gradientPicker = 200
	pickerBelow    = 140
)

// gradientPanel fills a ramp from a left colour, a right colour and a number
// of steps, mixing the colour, the opacity or both. Its fields are the tool's
// settings, not the palette's, so they never wait for Save.
type gradientPanel struct {
	open                      bool
	into                      ids.ULID
	mixColor, mixOpacity      *ui.Toggle
	left, right               *ui.TextField
	leftOpacity, rightOpacity *ui.TextField
	steps, name               *ui.TextField
	useLeft, useRight         *ui.Button
	intoButton, apply, close  *ui.Button
	previewTop                int
	// picker sets the left colour, or the right when pickRight is set; each
	// end's swatch, beside its field, chooses which.
	picker                  *ui.ColorPicker
	pickRight               bool
	leftSwatch, rightSwatch image.Rectangle
}

func newGradientPanel(theme *ui.Theme) *gradientPanel {
	field := func(id, text string, maxBytes int) *ui.TextField {
		return ui.NewTextField(id, textedit.NewField(text, false, maxBytes), false, theme)
	}
	panel := &gradientPanel{
		open: false, into: ids.ULID{},
		mixColor:   &ui.Toggle{ID: "mix-colour", Label: "Colour", Rect: ui.Zero(), On: true},
		mixOpacity: &ui.Toggle{ID: "mix-opacity", Label: "Opacity", Rect: ui.Zero(), On: false},
		left:       field("left", "#202020", colorBytes), right: field("right", "#E0E0E0", colorBytes),
		leftOpacity: field("left-opacity", "100", opacityBytes), rightOpacity: field("right-opacity", "100", opacityBytes),
		steps: field("steps", strconv.Itoa(defaultSteps), stepsBytes), name: field("gradient-name", gradientName, nameBytes),
		useLeft: ui.NewButton("use-left", "Use chosen"), useRight: ui.NewButton("use-right", "Use chosen"),
		intoButton: ui.NewButton("gradient-target", newRampLabel), apply: ui.NewButton("apply-gradient", "Apply"),
		close: ui.NewButton("close-gradient", "×"), previewTop: 0, picker: ui.NewColorPicker("gradient-picker"),
		pickRight: false, leftSwatch: ui.Zero(), rightSwatch: ui.Zero(),
	}
	panel.intoButton.Caret = true
	panel.apply.Primary = true

	return panel
}

func (grad *gradientPanel) fields() []*ui.TextField {
	return []*ui.TextField{grad.left, grad.right, grad.leftOpacity, grad.rightOpacity, grad.steps, grad.name}
}

func (grad *gradientPanel) buttons() []*ui.Button {
	return []*ui.Button{grad.useLeft, grad.useRight, grad.intoButton, grad.apply, grad.close}
}

// copyPanel copies colours from another palette: they are picked from the
// source's ramps and copied into a new ramp or one of this palette's.
type copyPanel struct {
	open                     bool
	source                   ids.ULID
	picked                   []ids.ULID
	anchor                   ids.ULID
	into                     ids.ULID
	sourceButton, intoButton *ui.Button
	copyButton, close        *ui.Button
	name                     *ui.TextField
	area                     image.Rectangle
	swatches                 []sourceSwatch
	ramps                    []sourceRamp
	scroll                   int
}

// sourceSwatch is one of the source palette's colours in the copy panel.
type sourceSwatch struct {
	rect  image.Rectangle
	ramp  art.Ramp
	shade art.Shade
	name  string
}

// sourceRamp is one of the source palette's ramp names, which picks it whole.
type sourceRamp struct {
	rect image.Rectangle
	ramp art.Ramp
}

func newCopyPanel(theme *ui.Theme) *copyPanel {
	panel := &copyPanel{
		open: false, source: art.DefaultPaletteID(), picked: nil, anchor: ids.ULID{}, into: ids.ULID{},
		sourceButton: ui.NewButton("copy-source", art.DefaultPaletteName),
		intoButton:   ui.NewButton("copy-target", newRampLabel), copyButton: ui.NewButton("copy", "Copy"),
		close: ui.NewButton("close-copy", "×"),
		name:  ui.NewTextField("copy-name", textedit.NewField("", false, nameBytes), false, theme),
		area:  ui.Zero(), swatches: nil, ramps: nil, scroll: 0,
	}
	panel.sourceButton.Caret = true
	panel.intoButton.Caret = true
	panel.copyButton.Primary = true

	return panel
}

func (copier *copyPanel) buttons() []*ui.Button {
	return []*ui.Button{copier.sourceButton, copier.intoButton, copier.copyButton, copier.close}
}

// panelRect is where the open panel is, at the foot of the content.
func (s *State) panelRect() image.Rectangle {
	content := frame.Content(s.size)
	height := 0
	switch {
	case s.gradient.open && s.gradientPickerBelow():
		height = gradientHeight + pickerBelow + ui.Inset + ui.LineHeight
	case s.gradient.open:
		height = gradientHeight
	case s.copying.open:
		height = copyHeight
	}
	if height == 0 {
		return ui.Zero()
	}

	return image.Rect(ui.Inset, content.Max.Y-ui.Inset-height, content.Max.X-ui.Inset, content.Max.Y-ui.Inset)
}

// toggleGradient opens the gradient, filling a ramp or, with none, a new
// one, or closes it.
func (s *State) toggleGradient(palette *art.Palette, into ids.ULID) {
	if s.gradient.open && into.IsZero() {
		s.gradient.open = false

		return
	}
	s.copying.open = false
	s.gradient.open = true
	s.aimGradient(palette, into)
}

// aimGradient makes the gradient fill a ramp, starting from its ends and its
// length, or a new ramp.
func (s *State) aimGradient(palette *art.Palette, into ids.ULID) {
	s.gradient.into = into
	if ramp, ok := palette.Ramp(into); ok && len(ramp.Shades) > 1 {
		first, last := ramp.Shades[0], ramp.Shades[len(ramp.Shades)-1]
		setText(s.gradient.left, first.Color.String())
		setText(s.gradient.right, last.Color.String())
		setText(s.gradient.leftOpacity, strconv.Itoa(first.Opacity))
		setText(s.gradient.rightOpacity, strconv.Itoa(last.Opacity))
		setText(s.gradient.steps, strconv.Itoa(len(ramp.Shades)))
	}
	setText(s.gradient.name, uniqueRampName(palette, gradientName))
}

// setText sets a tool field's text.
func setText(field *ui.TextField, text string) {
	field.Field.SetSaved(text)
	field.Field.Revert()
}

// toggleCopying opens Copy colours from…, from Default, or closes it.
func (s *State) toggleCopying(ctx *frame.Context) {
	if s.copying.open {
		s.copying.open = false

		return
	}
	s.gradient.open = false
	s.copying.open = true
	s.copying.picked, s.copying.anchor = nil, ids.ULID{}
	s.subscribe(s.copying.source, ctx)
}

// layoutPanels places the open panel's widgets.
func (s *State) layoutPanels(docs frame.Docs) {
	for _, button := range append(s.gradient.buttons(), s.copying.buttons()...) {
		button.Rect = ui.Zero()
	}
	for _, field := range append(s.gradient.fields(), s.copying.name) {
		field.Rect = ui.Zero()
	}
	s.gradient.mixColor.Rect, s.gradient.mixOpacity.Rect = ui.Zero(), ui.Zero()
	s.gradient.picker.Rect, s.gradient.leftSwatch, s.gradient.rightSwatch = ui.Zero(), ui.Zero(), ui.Zero()
	s.copying.area, s.copying.swatches, s.copying.ramps = ui.Zero(), nil, nil
	panel := s.panelRect()
	if panel.Empty() {
		return
	}
	s.place(s.closeButton(), panel.Max.X-ui.Inset-ui.ButtonH, panel.Min.Y+ui.Inset)
	if s.gradient.open {
		s.layoutGradient(docs, panel)

		return
	}
	s.layoutCopying(docs, panel)
}

func (s *State) closeButton() *ui.Button {
	if s.gradient.open {
		return s.gradient.close
	}

	return s.copying.close
}

// layoutGradient places the gradient's two rows: what it mixes and its ends
// and steps; then its preview, where it goes and Apply.
func (s *State) layoutGradient(docs frame.Docs, panel image.Rectangle) {
	grad := s.gradient
	left, top := panel.Min.X+ui.Inset, panel.Min.Y+ui.Inset+ui.LineHeight
	grad.mixColor.Rect = image.Rect(left, top, left+toggleWidth, top+ui.ButtonH)
	across := grad.mixColor.Rect.Max.X + ui.Inset
	grad.leftSwatch = image.Rect(across, top, across+ui.ButtonH, top+ui.ButtonH)
	across = grad.leftSwatch.Max.X + ui.Gap
	grad.left.Rect = image.Rect(across, top, across+colorWidth, top+ui.ButtonH)
	across = s.place(grad.useLeft, grad.left.Rect.Max.X+ui.Gap, top) + ui.Inset
	grad.rightSwatch = image.Rect(across, top, across+ui.ButtonH, top+ui.ButtonH)
	across = grad.rightSwatch.Max.X + ui.Gap
	grad.right.Rect = image.Rect(across, top, across+colorWidth, top+ui.ButtonH)
	across = s.place(grad.useRight, grad.right.Rect.Max.X+ui.Gap, top) + ui.Inset
	stepsLeft := across + s.theme.TextWidth("Steps") + ui.Gap
	grad.steps.Rect = image.Rect(stepsLeft, top, stepsLeft+stepsWidth, top+ui.ButtonH)
	top += ui.ButtonH + ui.Gap
	grad.mixOpacity.Rect = image.Rect(left, top, left+toggleWidth, top+ui.ButtonH)
	grad.leftOpacity.Rect = image.Rect(grad.left.Rect.Min.X, top, grad.left.Rect.Min.X+opacityWidth, top+ui.ButtonH)
	grad.rightOpacity.Rect = image.Rect(grad.right.Rect.Min.X, top, grad.right.Rect.Min.X+opacityWidth, top+ui.ButtonH)
	top += ui.ButtonH + ui.Inset
	grad.previewTop = top
	top += previewSize + ui.Inset
	across = s.place(grad.intoButton, left+s.theme.TextWidth("Into")+ui.Gap, top)
	grad.intoButton.Label = newRampLabel
	if palette, ok := s.palette(docs); ok {
		if ramp, found := palette.Ramp(grad.into); found {
			grad.intoButton.Label = ramp.Name
		}
	}
	if grad.into.IsZero() {
		grad.name.Rect = image.Rect(across, top, across+rampNameWidth, top+ui.ButtonH)
		across = grad.name.Rect.Max.X + ui.Inset
	}
	s.place(grad.apply, across, top)
	grad.leftOpacity.Rect = gate(grad.leftOpacity.Rect, grad.mixOpacity.On)
	grad.rightOpacity.Rect = gate(grad.rightOpacity.Rect, grad.mixOpacity.On)
	pickerTop := panel.Min.Y + ui.Inset + ui.LineHeight
	pickerLeft := s.gradient.close.Rect.Min.X - ui.Inset - gradientPicker
	if s.gradientPickerBelow() {
		pickerTop, pickerLeft = grad.apply.Rect.Max.Y+ui.Inset+ui.LineHeight, left
	}
	grad.picker.Rect = image.Rect(pickerLeft, pickerTop, pickerLeft+gradientPicker, panel.Max.Y-ui.Inset)
	grad.picker.NoOpacity = !grad.mixOpacity.On
	s.syncGradientPicker()
}

// gradientPickerBelow reports whether the window is too narrow for the
// gradient's picker beside its fields.
func (s *State) gradientPickerBelow() bool {
	fields := toggleWidth + ui.Inset + ui.Half*(ui.ButtonH+ui.Gap+colorWidth+ui.Gap+s.buttonWidth(s.gradient.useLeft)+
		ui.Inset) + s.theme.TextWidth("Steps") + ui.Gap + stepsWidth

	return ui.Inset+ui.Inset+fields+ui.Half*ui.Inset+gradientPicker+ui.Inset+ui.ButtonH+ui.Inset+ui.Inset >
		frame.Content(s.size).Max.X
}

// pickedEnd is the gradient's end the picker sets: its colour and opacity
// fields.
func (s *State) pickedEnd() (*ui.TextField, *ui.TextField) {
	if s.gradient.pickRight {
		return s.gradient.right, s.gradient.rightOpacity
	}

	return s.gradient.left, s.gradient.leftOpacity
}

// syncGradientPicker shows in the picker what the picked end's fields hold.
func (s *State) syncGradientPicker() {
	colorField, opacityField := s.pickedEnd()
	value, err := art.ParseRGB(strings.TrimSpace(colorField.Field.Value()))
	if err != nil {
		return
	}
	opacity, ok := parseOpacity(opacityField.Field.Value())
	if !ok || !s.gradient.mixOpacity.On {
		opacity = art.MaxOpacity
	}
	s.gradient.picker.SetColor(color.RGBA(value.NRGBA(art.MaxOpacity)), float64(opacity)/art.MaxOpacity)
}

// updateGradientPicker chooses the end the picker sets, from its swatch or
// its focused field, and puts what is picked into that end's fields.
func (s *State) updateGradientPicker(input *ui.Input) {
	grad := s.gradient
	switch current := s.focus.Current(); {
	case input.Pressed && input.Cursor.In(grad.leftSwatch), current == grad.left, current == grad.leftOpacity:
		grad.pickRight = false
	case input.Pressed && input.Cursor.In(grad.rightSwatch), current == grad.right, current == grad.rightOpacity:
		grad.pickRight = true
	}
	if input.Pressed && input.Cursor.In(grad.picker.Rect) {
		s.focus.Set(nil)
	}
	s.syncGradientPicker()
	if !grad.picker.Update(input) {
		return
	}
	colorField, opacityField := s.pickedEnd()
	colorField.Field.SetDraft(pickedRGB(grad.picker).String())
	if grad.mixOpacity.On {
		opacityField.Field.SetDraft(strconv.Itoa(pickedOpacity(grad.picker)))
	}
}

// gate hides a widget while what it sets is off.
func gate(rect image.Rectangle, on bool) image.Rectangle {
	if on {
		return rect
	}

	return ui.Zero()
}

// gradientShades reads the gradient's typed settings and returns its
// colours, or what is wrong with the settings.
func (s *State) gradientShades() ([]art.Shade, string) {
	grad := s.gradient
	left, leftErr := art.ParseRGB(strings.TrimSpace(grad.left.Field.Value()))
	right, rightErr := art.ParseRGB(strings.TrimSpace(grad.right.Field.Value()))
	if leftErr != nil || rightErr != nil {
		return nil, "Enter both colours as six hexadecimal digits, such as #8A919C"
	}
	leftOpacity, leftOK := parseOpacity(grad.leftOpacity.Field.Value())
	rightOpacity, rightOK := parseOpacity(grad.rightOpacity.Field.Value())
	if !leftOK || !rightOK {
		return nil, fmt.Sprintf("Enter both opacities from %d%% to %d%%", art.MinOpacity, art.MaxOpacity)
	}
	if !grad.mixOpacity.On {
		leftOpacity, rightOpacity = art.MaxOpacity, art.MaxOpacity
	}
	steps, err := strconv.Atoi(strings.TrimSpace(grad.steps.Field.Value()))
	if err != nil || steps < art.MinSteps || steps > art.MaxShades {
		return nil, fmt.Sprintf("Enter from %d to %d steps", art.MinSteps, art.MaxShades)
	}
	shades, err := art.Gradient(
		art.Shade{ID: ids.ULID{}, Color: left, Opacity: leftOpacity, Label: ""},
		art.Shade{ID: ids.ULID{}, Color: right, Opacity: rightOpacity, Label: ""},
		steps, grad.mixColor.On, grad.mixOpacity.On)
	if err != nil {
		return nil, err.Error()
	}

	return shades, ""
}

// applyGradient fills the chosen ramp with the gradient's colours, or adds a
// new ramp of them, at once and as one undo step. Into an existing ramp it
// sets colours 1 to n, keeping their labels, and adds or removes colours at
// the end; it is refused when a colour it would remove is painted with.
func (s *State) applyGradient(ctx *frame.Context, palette *art.Palette) {
	shades, problem := s.gradientShades()
	if problem != "" {
		ctx.Frame.Status(problem)

		return
	}
	ramp, existing := palette.Ramp(s.gradient.into)
	if !existing {
		name := strings.TrimSpace(s.gradient.name.Field.Value())
		if name == "" {
			name = uniqueRampName(palette, gradientName)
		}
		index := len(palette.Ramps())
		args := artplugin.AddRampArgs{Index: index, Ramp: art.Ramp{ID: ids.ULID{}, Name: name, Shades: shades}}
		s.command(ctx, s.target, artplugin.AddRampEdit, "", args, func(protocol.Result) {
			if updated, ok := s.palette(ctx.Docs); ok && index < len(updated.Ramps()) {
				added := updated.Ramps()[index]
				s.choose(choice{ramp: added.ID, shade: added.Shades[0].ID})
				s.reveal = true
			}
			ctx.Frame.Status(fmt.Sprintf("Added “%s”, %s 1 to %s %d", name, name, name, len(shades)))
		}, nil)

		return
	}
	if used := s.usedSurplus(ctx, ramp, len(shades)); used != "" {
		ctx.Frame.Status(used)

		return
	}
	s.fillRamp(ctx, ramp, shades)
}

// usedSurplus says why a gradient of fewer steps cannot shorten a ramp: a
// colour it would remove is painted with, or it is not yet known whether
// one is.
func (s *State) usedSurplus(ctx *frame.Context, ramp art.Ramp, steps int) string {
	for index := steps; index < len(ramp.Shades); index++ {
		uses, known := s.usesOf(ctx, artplugin.ShadeItem(ramp.Shades[index].ID))
		switch {
		case !known:
			return "Checking where the colours it would remove are used; try again"
		case len(uses) > 0:
			return fmt.Sprintf("%s %d is used by %s, so the gradient can't remove it; give it %d steps",
				ramp.Name, index+1, uses[0].Name, len(ramp.Shades))
		}
	}

	return ""
}

// fillRamp sets a ramp's colours to a gradient's, as one undo step.
func (s *State) fillRamp(ctx *frame.Context, ramp art.Ramp, shades []art.Shade) {
	gesture := s.gesture()
	for index, shade := range shades {
		if index < len(ramp.Shades) {
			set := shade
			set.ID, set.Label = ramp.Shades[index].ID, ramp.Shades[index].Label
			s.command(ctx, s.target, artplugin.SetShadeEdit, gesture, artplugin.SetShadeArgs{Ramp: ramp.ID, Shade: set},
				nil, nil)

			continue
		}
		args := artplugin.AddShadeArgs{Ramp: ramp.ID, Index: index, Shade: shade}
		s.command(ctx, s.target, artplugin.AddShadeEdit, gesture, args, nil, nil)
	}
	for index := len(ramp.Shades) - 1; index >= len(shades); index-- {
		args := artplugin.RemoveShadeArgs{Ramp: ramp.ID, Shade: ramp.Shades[index].ID}
		s.command(ctx, s.target, artplugin.RemoveShadeEdit, gesture, args, nil, nil)
	}
	ctx.Frame.Status(fmt.Sprintf("Filled %s with %d colours", ramp.Name, len(shades)))
}

// layoutCopying places Copy colours from…: the source and its colours index
// the left, where they go and Copy beneath.
func (s *State) layoutCopying(docs frame.Docs, panel image.Rectangle) {
	copier := s.copying
	left, top := panel.Min.X+ui.Inset, panel.Min.Y+ui.Inset
	copier.sourceButton.Label = documentName(docs, copier.source, art.DefaultPaletteName)
	s.place(copier.sourceButton, left+s.theme.TextWidth("From")+ui.Gap, top)
	bottom := panel.Max.Y - ui.Inset - ui.ButtonH - ui.Inset
	copier.area = image.Rect(left, top+ui.ButtonH+ui.Gap, panel.Max.X-ui.Inset, bottom)
	source, ok := paletteOf(docs, copier.source)
	if ok {
		s.layoutSource(source)
	}
	row := panel.Max.Y - ui.Inset - ui.ButtonH
	across := s.place(copier.intoButton, left+s.theme.TextWidth("Into")+ui.Gap, row)
	copier.intoButton.Label = newRampLabel
	if palette, found := s.palette(docs); found {
		if ramp, has := palette.Ramp(copier.into); has {
			copier.intoButton.Label = ramp.Name
		}
	}
	if copier.into.IsZero() {
		copier.name.Rect = image.Rect(across, row, across+rampNameWidth, row+ui.ButtonH)
		across = copier.name.Rect.Max.X + ui.Inset
	}
	copier.copyButton.Label = fmt.Sprintf("Copy %d colours", len(copier.picked))
	copier.copyButton.Disabled = len(copier.picked) == 0
	s.place(copier.copyButton, across, row)
}

// layoutSource places the source palette's ramps and colours, scrolled.
func (s *State) layoutSource(source *art.Palette) {
	copier := s.copying
	perRow := max(1, (copier.area.Dx()-sourceName)/sourceStep)
	content := 0
	for _, ramp := range source.Ramps() {
		content += lines(len(ramp.Shades), perRow) * sourceStep
	}
	copier.scroll = min(max(copier.scroll, 0), max(content-copier.area.Dy(), 0))
	top := copier.area.Min.Y - copier.scroll
	for _, ramp := range source.Ramps() {
		copier.ramps = append(copier.ramps, sourceRamp{
			rect: image.Rect(copier.area.Min.X, top, copier.area.Min.X+sourceName-ui.Gap, top+sourceSize), ramp: ramp,
		})
		for index, shade := range ramp.Shades {
			x := copier.area.Min.X + sourceName + index%perRow*sourceStep
			y := top + index/perRow*sourceStep
			copier.swatches = append(copier.swatches, sourceSwatch{
				rect: image.Rect(x, y, x+sourceSize, y+sourceSize), ramp: ramp, shade: shade,
				name: ramp.Name + " " + strconv.Itoa(index+1),
			})
		}
		top += lines(len(ramp.Shades), perRow) * sourceStep
	}
}

// updatePanels handles the open panel.
func (s *State) updatePanels(input *ui.Input, ctx *frame.Context) {
	palette, ok := s.palette(ctx.Docs)
	if !ok {
		return
	}
	switch {
	case s.gradient.open:
		s.updateGradient(input, ctx, palette)
	case s.copying.open:
		s.updateCopying(input, ctx, palette)
	}
}

func (s *State) updateGradient(input *ui.Input, ctx *frame.Context, palette *art.Palette) {
	grad := s.gradient
	grad.mixColor.Update(input)
	grad.mixOpacity.Update(input)
	s.updateGradientPicker(input)
	shade, _, chosen := palette.Lookup(s.chosen.ramp, s.chosen.shade)
	switch {
	case grad.close.Update(input):
		grad.open = false
	case grad.useLeft.Update(input) && chosen:
		setText(grad.left, shade.Color.String())
		setText(grad.leftOpacity, strconv.Itoa(shade.Opacity))
	case grad.useRight.Update(input) && chosen:
		setText(grad.right, shade.Color.String())
		setText(grad.rightOpacity, strconv.Itoa(shade.Opacity))
	case grad.intoButton.Update(input):
		s.openMenu(menuInto, s.rampItems(palette), grad.intoButton.Rect)
	case grad.apply.Update(input):
		s.applyGradient(ctx, palette)
	}
	if ramp, found := palette.Ramp(grad.into); found {
		if shades, problem := s.gradientShades(); problem == "" {
			s.usedSurplus(ctx, ramp, len(shades))
		}
	}
}

// rampItems offers a new ramp and each of the palette's ramps.
func (s *State) rampItems(palette *art.Palette) []ui.MenuItem {
	items := make([]ui.MenuItem, 0, 1+len(palette.Ramps()))
	items = append(items, ui.MenuItem{ID: newRampItem, Label: newRampLabel, Icon: ui.IconNone, Disabled: false,
		Danger: false, Rule: false})
	for at, ramp := range palette.Ramps() {
		items = append(items, ui.MenuItem{
			ID: ramp.ID.String(), Label: ramp.Name, Icon: ui.IconNone, Disabled: false, Danger: false, Rule: at == 0,
		})
	}

	return items
}

func (s *State) updateCopying(input *ui.Input, ctx *frame.Context, palette *art.Palette) {
	copier := s.copying
	switch {
	case copier.close.Update(input):
		copier.open = false
	case copier.sourceButton.Update(input):
		s.openMenu(menuCopySource, s.sourceItems(ctx), copier.sourceButton.Rect)
	case copier.intoButton.Update(input):
		s.openMenu(menuCopyInto, s.rampItems(palette), copier.intoButton.Rect)
	case copier.copyButton.Update(input):
		s.copyIn(ctx, palette)
	case input.Cursor.In(copier.area) && input.WheelY != 0:
		copier.scroll -= input.WheelPan().Y
		input.WheelX, input.WheelY = 0, 0
	case input.Pressed && input.Cursor.In(copier.area):
		s.pickSource(input, palette)
	}
}

// sourceItems offers Default and the project's other palettes to copy from.
func (s *State) sourceItems(ctx *frame.Context) []ui.MenuItem {
	others := s.otherPalettes(ctx)
	items := make([]ui.MenuItem, 0, 1+len(others))
	items = append(items, ui.MenuItem{
		ID: art.DefaultPaletteID().String(), Label: art.DefaultPaletteName, Icon: ui.IconPalette, Disabled: false,
		Danger: false, Rule: false,
	})
	for at, entry := range others {
		items = append(items, ui.MenuItem{
			ID: entry.ID.String(), Label: entry.Name, Icon: ui.IconPalette, Disabled: false, Danger: false, Rule: at == 0,
		})
	}

	return items
}

// pickSource picks source colours: a click picks one, Shift-click the run
// from the last one picked, Cmd-click adds or takes away one, and a ramp's
// name picks the whole ramp.
func (s *State) pickSource(input *ui.Input, palette *art.Palette) {
	copier := s.copying
	for _, label := range copier.ramps {
		if input.Cursor.In(label.rect) {
			copier.picked = nil
			for _, shade := range label.ramp.Shades {
				copier.picked = append(copier.picked, shade.ID)
			}
			copier.anchor = ids.ULID{}
			s.defaultCopyName(palette, label.ramp.Name)
			input.ClearPointer()

			return
		}
	}
	for index, offered := range copier.swatches {
		if !input.Cursor.In(offered.rect) {
			continue
		}
		switch {
		case input.Held.Shift && !copier.anchor.IsZero():
			from := slices.IndexFunc(copier.swatches, func(each sourceSwatch) bool { return each.shade.ID == copier.anchor })
			start, end := min(from, index), max(from, index)
			copier.picked = nil
			for _, each := range copier.swatches[start : end+1] {
				copier.picked = append(copier.picked, each.shade.ID)
			}
		case input.Held.Shortcut:
			if index := slices.Index(copier.picked, offered.shade.ID); index >= 0 {
				copier.picked = slices.Delete(copier.picked, index, index+1)
			} else {
				copier.picked = append(copier.picked, offered.shade.ID)
			}
			copier.anchor = offered.shade.ID
		default:
			copier.picked, copier.anchor = []ids.ULID{offered.shade.ID}, offered.shade.ID
		}
		s.defaultCopyName(palette, offered.ramp.Name)
		input.ClearPointer()

		return
	}
}

// defaultCopyName names the new ramp copies go into after their source ramp,
// until the author types another name.
func (s *State) defaultCopyName(palette *art.Palette, name string) {
	if s.copying.name.Field.Dirty() {
		return
	}
	setText(s.copying.name, uniqueRampName(palette, name))
}

// copyIn copies the picked source colours into a new ramp or one of this
// palette's, at once and as one undo step: colour and opacity are copied,
// labels never are, and the copies have IDs of their own.
func (s *State) copyIn(ctx *frame.Context, palette *art.Palette) {
	if len(s.copying.picked) == 0 {
		return
	}
	var picked []art.Shade
	for _, offered := range s.copying.swatches {
		if slices.Contains(s.copying.picked, offered.shade.ID) {
			picked = append(picked, offered.shade)
		}
	}
	copies := copiesOf(picked)
	from := documentName(ctx.Docs, s.copying.source, art.DefaultPaletteName)
	status := fmt.Sprintf("Copied %d colours from %s, without labels", len(copies), from)
	if ramp, existing := palette.Ramp(s.copying.into); existing {
		gesture := s.gesture()
		for at, shade := range copies {
			args := artplugin.AddShadeArgs{Ramp: ramp.ID, Index: len(ramp.Shades) + at, Shade: shade}
			s.command(ctx, s.target, artplugin.AddShadeEdit, gesture, args, nil, nil)
		}
		ctx.Frame.Status(status + " into " + ramp.Name)

		return
	}
	name := strings.TrimSpace(s.copying.name.Field.Value())
	if name == "" {
		name = uniqueRampName(palette, "copies")
	}
	index := len(palette.Ramps())
	args := artplugin.AddRampArgs{Index: index, Ramp: art.Ramp{ID: ids.ULID{}, Name: name, Shades: copies}}
	s.command(ctx, s.target, artplugin.AddRampEdit, "", args, func(protocol.Result) {
		if updated, found := s.palette(ctx.Docs); found && index < len(updated.Ramps()) {
			added := updated.Ramps()[index]
			s.choose(choice{ramp: added.ID, shade: added.Shades[0].ID})
			s.reveal = true
		}
		ctx.Frame.Status(status + " into the new ramp " + name)
	}, nil)
}

// chosePanelItem handles a choice from a panel's menu: where the gradient
// or the copies go, or the palette copies come from.
func (s *State) chosePanelItem(ctx *frame.Context, purpose, item string) {
	chosen, err := ids.Parse(item)
	if err != nil && item != newRampItem {
		return
	}
	if item == newRampItem {
		chosen = ids.ULID{}
	}
	switch purpose {
	case menuInto:
		if palette, ok := s.palette(ctx.Docs); ok {
			s.aimGradient(palette, chosen)
		}
	case menuCopyInto:
		s.copying.into = chosen
	case menuCopySource:
		s.copying.source, s.copying.picked, s.copying.anchor, s.copying.scroll = chosen, nil, ids.ULID{}, 0
		s.subscribe(chosen, ctx)
	}
}

// drawPanels draws the open panel.
func (s *State) drawPanels(screen *ebiten.Image) {
	panel := s.panelRect()
	if panel.Empty() {
		return
	}
	ui.Box(screen, panel, ui.ColorBackground())
	s.closeButton().Draw(screen, s.theme)
	if s.gradient.open {
		s.drawGradient(screen, panel)

		return
	}
	s.drawCopying(screen, panel)
}

func (s *State) drawGradient(screen *ebiten.Image, panel image.Rectangle) {
	grad := s.gradient
	left := panel.Min.X + ui.Inset
	s.theme.DrawStrong(screen, "Gradient", left, panel.Min.Y+ui.Inset, ui.ColorInk())
	s.theme.DrawText(screen, "fills a ramp from a left colour to a right one, in steps that look evenly spaced",
		left+s.theme.StrongWidth("Gradient")+ui.Inset, panel.Min.Y+ui.Inset, ui.ColorMuted())
	grad.mixColor.Draw(screen, s.theme)
	grad.mixOpacity.Draw(screen, s.theme)
	s.drawGradientEnds(screen)
	for _, field := range grad.fields() {
		if !field.Rect.Empty() {
			field.Draw(screen)
		}
	}
	text := (ui.ButtonH - ui.TextSize) / ui.Half
	s.theme.DrawText(screen, "Steps", grad.steps.Rect.Min.X-s.theme.TextWidth("Steps")-ui.Gap, grad.steps.Rect.Min.Y+text,
		ui.ColorMuted())
	s.theme.DrawText(screen, "Into", left, grad.intoButton.Rect.Min.Y+text, ui.ColorMuted())
	for _, button := range grad.buttons() {
		if !button.Rect.Empty() && button != grad.close {
			button.Draw(screen, s.theme)
		}
	}
	shades, problem := s.gradientShades()
	if problem != "" {
		s.theme.DrawText(screen, problem, left, grad.previewTop+text/ui.Half, ui.ColorWarn())

		return
	}
	for at, shade := range shades[:min(len(shades), maxPreview)] {
		x := left + at*previewStep
		drawSwatch(screen, image.Rect(x, grad.previewTop, x+previewSize, grad.previewTop+previewSize), shade, false)
	}
}

func (s *State) drawCopying(screen *ebiten.Image, panel image.Rectangle) {
	copier := s.copying
	left := panel.Min.X + ui.Inset
	text := (ui.ButtonH - ui.TextSize) / ui.Half
	s.theme.DrawText(screen, "From", left, copier.sourceButton.Rect.Min.Y+text, ui.ColorMuted())
	s.theme.DrawText(screen, "Click a colour, Shift-click a run, Cmd-click to add more; a ramp's name takes it all.",
		copier.sourceButton.Rect.Max.X+ui.Inset, copier.sourceButton.Rect.Min.Y+text, ui.ColorMuted())
	area, clipped := screen.SubImage(copier.area).(*ebiten.Image)
	if clipped {
		for _, label := range copier.ramps {
			s.theme.DrawText(area, s.theme.FitText(label.ramp.Name, label.rect.Dx()), label.rect.Min.X,
				label.rect.Min.Y, ui.ColorInk())
		}
		for _, offered := range copier.swatches {
			drawSwatch(area, offered.rect, offered.shade, false)
			if slices.Contains(copier.picked, offered.shade.ID) {
				ui.StrokeRect(area, offered.rect.Inset(-chosenWidth), chosenWidth, ui.ColorSelect())
			}
		}
	}
	s.theme.DrawText(screen, "Into", left, copier.intoButton.Rect.Min.Y+text, ui.ColorMuted())
	if !copier.name.Rect.Empty() {
		copier.name.Draw(screen)
	}
	for _, button := range copier.buttons() {
		if !button.Rect.Empty() && button != copier.close {
			button.Draw(screen, s.theme)
		}
	}
}

// drawGradientEnds draws each end's swatch, the picked one outlined, and the
// picker, named for its end.
func (s *State) drawGradientEnds(screen *ebiten.Image) {
	grad := s.gradient
	for _, end := range []struct {
		swatch         image.Rectangle
		color, opacity *ui.TextField
		picked         bool
	}{
		{grad.leftSwatch, grad.left, grad.leftOpacity, !grad.pickRight},
		{grad.rightSwatch, grad.right, grad.rightOpacity, grad.pickRight},
	} {
		shade := art.Shade{ID: ids.ULID{}, Color: 0, Opacity: art.MaxOpacity, Label: ""}
		value, err := art.ParseRGB(strings.TrimSpace(end.color.Field.Value()))
		if opacity, ok := parseOpacity(end.opacity.Field.Value()); ok && grad.mixOpacity.On {
			shade.Opacity = opacity
		}
		shade.Color = value
		drawSwatch(screen, end.swatch, shade, err != nil)
		if end.picked {
			ui.StrokeRect(screen, end.swatch.Inset(-chosenWidth), chosenWidth, ui.ColorSelect())
		}
	}
	name := "Left colour"
	if grad.pickRight {
		name = "Right colour"
	}
	s.theme.DrawText(screen, name, grad.picker.Rect.Min.X, grad.picker.Rect.Min.Y-ui.LineHeight, ui.ColorMuted())
	grad.picker.Draw(screen)
}
