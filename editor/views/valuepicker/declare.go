package valuepicker

import (
	"encoding/json"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Declaring from the picker: when the search names a value nothing has, or
// on asking while values match, the list gives way to a panel that
// declares it: where it goes, its type, locked where the pick must have
// one, and what it starts at. Its button, or Enter, declares it and hands
// it to the window at once.
const (
	labelWidth      = 90
	declareWidth    = 170
	askWidth        = 220
	startFieldWidth = 240
)

// declarePanel is the picker's panel for declaring a value.
type declarePanel struct {
	// targets are where the name typed can be declared, as where's buttons,
	// and chosen the one chosen, by its ID. matched is how many values the
	// search matches; asked is set when the panel was asked for while some
	// do. lead is what the panel says it does.
	targets []Target
	where   []*ui.Button
	chosen  string
	matched int
	asked   bool
	lead    string
	types   []*ui.Button
	kinds   []worldstate.Type
	kind    worldstate.Type
	// start is what an integer or text starts at, truth a bool's.
	start   *ui.TextField
	truth   []*ui.Button
	truthOn bool
	ask     *ui.Button
	declare *ui.Button
	// whereTop, typeTop and startTop are where each row of the panel
	// starts.
	whereTop, typeTop, startTop int
}

// newDeclarePanel makes the panel, offering every type a saved value can
// have, or only the picker's one, locked.
func newDeclarePanel(config Config, theme *ui.Theme) *declarePanel {
	kinds := worldstate.Types(worldstate.ScopeWorld)
	if config.Only != "" {
		kinds = []worldstate.Type{config.Only}
	}
	types := make([]*ui.Button, 0, len(kinds))
	for _, kind := range kinds {
		label := kind.Label()
		if config.Only != "" {
			label += " · fixed"
		}
		types = append(types, ui.NewButton("declare-type-"+string(kind), label))
	}
	start := ui.NewTextField("declare-start", textedit.NewField("", false, worldstate.MaxText), false, theme)
	start.Unmarked = true
	panel := &declarePanel{
		targets: nil, where: nil, chosen: "", matched: 0, asked: false, lead: config.DeclareLead, types: types,
		kinds: kinds, kind: "", start: start, truth: []*ui.Button{
			ui.NewButton("declare-start-false", "false"), ui.NewButton("declare-start-true", "true"),
		}, truthOn: false,
		ask: ui.NewButton("picker-new", ""), declare: ui.NewButton("picker-declare", config.DeclareAs),
		whereTop: 0, typeTop: 0, startTop: 0,
	}
	panel.declare.Primary = true
	panel.setKind(0)

	return panel
}

// setKind chooses the type at an index, starting at its own zero.
func (d *declarePanel) setKind(index int) {
	d.kind = d.kinds[index]
	for which, button := range d.types {
		button.On = which == index
	}
	d.truthOn = false
	for which, button := range d.truth {
		button.On = which == 0
	}
	switch d.kind {
	case worldstate.TypeInteger:
		d.start.Field.SetSaved("0")
	case worldstate.TypeText:
		d.start.Field.SetSaved("")
	case worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeGroup,
		worldstate.TypeTemplate:
	}
}

// showing reports whether the panel is in the list's place: there is a
// name to declare, and no value matches or the panel was asked for.
func (d *declarePanel) showing() bool { return len(d.targets) > 0 && (d.asked || d.matched == 0) }

// target is where the value will be declared.
func (d *declarePanel) target() (Target, bool) {
	for _, target := range d.targets {
		if target.ID == d.chosen {
			return target, true
		}
	}

	return Target{ID: "", Path: "", Label: "", Heading: "", SetIn: ""}, false
}

// setTargets keeps where the name typed can be declared, with a button for
// each, and the one chosen while it is still offered, else the first.
func (d *declarePanel) setTargets(targets []Target) {
	same := len(targets) == len(d.targets)
	for index := 0; same && index < len(targets); index++ {
		same = targets[index].ID == d.targets[index].ID
	}
	d.targets = targets
	if !same {
		d.where = make([]*ui.Button, 0, len(targets))
		for _, target := range targets {
			d.where = append(d.where, ui.NewButton("declare-where-"+target.ID, target.Label))
		}
	}
	if _, offered := d.target(); !offered && len(targets) > 0 {
		d.chosen = targets[0].ID
	}
	for index, target := range targets {
		d.where[index].On = target.ID == d.chosen
	}
	if len(targets) == 0 {
		d.asked = false
	}
}

// layoutFoot places the foot's buttons from the right: the pick button, or
// the declare button while the panel shows; Cancel; and the button that
// asks for the panel while values match, or goes back to them.
func (p *Picker) layoutFoot(right, foot int) {
	panel := p.declare
	place := func(width int) image.Rectangle {
		rect := image.Rect(right-width, foot, right, foot+ui.ButtonH)
		right -= width + ui.Gap

		return rect
	}
	p.choose.Rect, panel.declare.Rect, panel.ask.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	if panel.showing() {
		panel.declare.Rect = place(declareWidth)
	} else {
		p.choose.Rect = place(buttonW)
	}
	p.cancel.Rect = place(buttonW)
	switch {
	case panel.asked:
		panel.ask.Label = "Back to values"
	case len(panel.targets) > 0 && panel.matched > 0:
		panel.ask.Label = p.theme.FitText(`Declare "`+p.query()+`"…`, askWidth-ui.Half*ui.Inset)
	default:
		return
	}
	panel.ask.Rect = place(min(askWidth, p.theme.TextWidth(panel.ask.Label)+ui.Half*ui.Inset))
}

// layoutDeclare places the panel in the list's place: where, type and what
// it starts at, each after its label.
func (p *Picker) layoutDeclare(area image.Rectangle) {
	panel := p.declare
	for _, button := range slices.Concat(panel.where, panel.types, panel.truth) {
		button.Rect = ui.Zero()
	}
	panel.start.Rect = ui.Zero()
	if !panel.showing() {
		return
	}
	left := area.Min.X + textPad + labelWidth
	width := area.Max.X - textPad - left
	panel.whereTop = area.Min.Y + ui.Gap + ui.Half*ui.LineHeight + ui.Gap
	panel.typeTop = p.buttonRows(panel.where, left, width, panel.whereTop) + ui.Gap
	panel.startTop = p.buttonRows(panel.types, left, width, panel.typeTop) + ui.Gap
	switch panel.kind {
	case worldstate.TypeInteger, worldstate.TypeText:
		panel.start.Rect = image.Rect(left, panel.startTop, left+min(width, startFieldWidth), panel.startTop+ui.ButtonH)
	case worldstate.TypeBool:
		p.buttonRows(panel.truth, left, min(width, startFieldWidth), panel.startTop)
	case worldstate.TypeChoice, worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeGroup,
		worldstate.TypeTemplate:
	}
}

// buttonRows places buttons in rows of equal widths across the width, as
// many to a row as their labels fit, and returns where the next row goes.
func (p *Picker) buttonRows(buttons []*ui.Button, left, width, top int) int {
	for start := 0; start < len(buttons); {
		end, widest := start, 0
		for end < len(buttons) {
			need := max(widest, p.theme.TextWidth(buttons[end].Label)+ui.Half*buttonPad)
			if count := end - start + 1; count > 1 && need*count+(count-1)*rowSpace > width {
				break
			}
			widest, end = need, end+1
		}
		row := buttons[start:end]
		gaps := (len(row) - 1) * rowSpace
		for index, button := range row {
			from := left + index*(width-gaps)/len(row) + index*rowSpace
			to := left + (index+1)*(width-gaps)/len(row) + index*rowSpace
			button.Rect = image.Rect(from, top, to, top+ui.ButtonH)
		}
		top += ui.ButtonH + rowSpace
		start = end
	}

	return top
}

// updateDeclare handles the button that asks for the panel and, while it
// shows, its buttons, and the declare button or Enter. It reports whether
// it took the input from the list.
func (p *Picker) updateDeclare(input *ui.Input, ctx *frame.Context, enter bool) bool {
	panel := p.declare
	if panel.ask.Update(input) {
		panel.asked = !panel.asked

		return true
	}
	if !panel.showing() {
		return false
	}
	for index, button := range panel.where {
		if button.Update(input) {
			panel.chosen = panel.targets[index].ID
		}
	}
	for index, button := range panel.types {
		if button.Update(input) && len(panel.kinds) > 1 {
			panel.setKind(index)
		}
	}
	for index, button := range panel.truth {
		if button.Update(input) {
			panel.truthOn = index == 1
			panel.truth[0].On, panel.truth[1].On = !panel.truthOn, panel.truthOn
		}
	}
	if panel.declare.Update(input) || enter {
		p.declareFrom(ctx)
	}

	return true
}

// declareFrom hands the window the value the panel describes, closing the
// picker, or says what is wrong with what it starts at.
func (p *Picker) declareFrom(ctx *frame.Context) {
	panel := p.declare
	target, found := panel.target()
	if !found {
		return
	}
	start, problem := panel.startValue()
	if problem != "" {
		p.note = problem

		return
	}
	p.closed = true
	p.source.Declare(ctx, target, panel.kind, start)
}

// startValue is what the value starts at, as saved: a typed integer or
// text, or false or true; nil leaves a choice or an NPC at its type's own.
func (d *declarePanel) startValue() (json.RawMessage, string) {
	value := worldstate.NoValue()
	value.Type = d.kind
	switch d.kind {
	case worldstate.TypeInteger:
		number, err := strconv.Atoi(strings.TrimSpace(d.start.Field.Value()))
		if err != nil || number < -worldstate.MaxInteger || number > worldstate.MaxInteger {
			limit := strconv.Itoa(worldstate.MaxInteger)

			return nil, "It starts at a whole number, from -" + limit + " to " + limit + "."
		}
		value.Integer = number
	case worldstate.TypeText:
		value.Text = d.start.Field.Value()
	case worldstate.TypeBool:
		value.Bool = d.truthOn
	case worldstate.TypeChoice, worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeGroup,
		worldstate.TypeTemplate:
		return nil, ""
	}

	return value.Encode(), ""
}

// drawDeclare draws the panel in the list's place: what it declares, then
// where, type and what it starts at, and a note on what is set elsewhere.
func (p *Picker) drawDeclare(screen *ebiten.Image) {
	panel := p.declare
	area := p.list.Rect
	ui.StrokeRound(screen, area.Inset(-ui.Half), ui.Radius, ui.Border, ui.ColorRule())
	target, _ := panel.target()
	left := area.Min.X + textPad
	p.theme.DrawStrong(screen, p.theme.FitText("Declare "+target.Heading, area.Dx()-ui.Half*textPad), left,
		area.Min.Y+ui.Gap, ui.ColorInk())
	lead := "No value is called that yet. " + panel.lead
	if panel.matched > 0 {
		lead = panel.lead
	}
	p.theme.DrawText(screen, lead, left, area.Min.Y+ui.Gap+ui.LineHeight, ui.ColorMuted())
	for _, label := range []struct {
		text string
		top  int
	}{{"Where", panel.whereTop}, {typeLabel, panel.typeTop}, {"Starts at", panel.startTop}} {
		p.theme.DrawStrong(screen, label.text, left, label.top+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorInk())
	}
	for _, button := range slices.Concat(panel.where, panel.types, panel.truth) {
		button.Draw(screen, p.theme)
	}
	noteTop := panel.startTop + ui.ButtonH + ui.Gap
	if panel.start.Rect.Empty() && panel.truth[0].Rect.Empty() {
		noteTop = panel.startTop + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	} else {
		panel.start.Draw(screen)
	}
	p.theme.DrawText(screen, p.theme.FitText(startNote(panel.kind, target.SetIn), area.Max.X-textPad-left-labelWidth),
		left+labelWidth, noteTop, ui.ColorMuted())
}

// startNote says what the panel leaves to be set elsewhere.
func startNote(kind worldstate.Type, setIn string) string {
	switch kind {
	case worldstate.TypeChoice:
		return "It starts at the first of two choices, named in " + setIn + "."
	case worldstate.TypeNPC:
		return "It starts as no NPC."
	case worldstate.TypePlace:
		return "It starts as no place; pick its place in " + setIn + "."
	case worldstate.TypeInteger, worldstate.TypeBool, worldstate.TypeText, worldstate.TypeGroup,
		worldstate.TypeTemplate:
	}

	return "Limits and a description are set in " + setIn + " afterwards."
}

// locateWhere finds where's button for a declare target by its ID.
func (d *declarePanel) locateWhere(value string) (image.Point, bool) {
	for index, target := range d.targets {
		if target.ID == value && index < len(d.where) && !d.where[index].Rect.Empty() {
			return ui.Center(d.where[index].Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// widgets are the panel's buttons and field, for locating them.
func (d *declarePanel) widgets() ([]*ui.Button, *ui.TextField) {
	return slices.Concat([]*ui.Button{d.ask, d.declare}, d.types, d.truth), d.start
}
