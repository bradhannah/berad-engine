package workflowview

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
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Declaring from the picker: when the search names a value nothing has, or
// on asking while values match, the list gives way to a panel that
// declares it: where it goes, its type, locked where the pick must have
// one, and what it starts at. Declare and choose, or Enter, declares it and
// chooses it at once.
const (
	declareLabelWidth = 90
	declareWidth      = 170
	askWidth          = 220
	startFieldWidth   = 240
)

// pickerDeclare is the picker's panel for declaring a value.
type pickerDeclare struct {
	// targets are where the name typed can be declared, as where's buttons,
	// and chosen the one chosen, as input scripts name it. matched is how
	// many values the search matches; asked is set when the panel was asked
	// for while some do.
	targets []declareTarget
	where   []*ui.Button
	chosen  string
	matched int
	asked   bool
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

// newPickerDeclare makes the panel, offering every type a saved value can
// have, or only one, locked.
func newPickerDeclare(only worldstate.Type, theme *ui.Theme) *pickerDeclare {
	kinds := worldstate.Types(worldstate.ScopeWorld)
	if only != "" {
		kinds = []worldstate.Type{only}
	}
	types := make([]*ui.Button, 0, len(kinds))
	for _, kind := range kinds {
		label := kind.Label()
		if only != "" {
			label += " · fixed"
		}
		types = append(types, ui.NewButton("declare-type-"+string(kind), label))
	}
	start := ui.NewTextField("declare-start", textedit.NewField("", false, worldstate.MaxText), false, theme)
	start.Unmarked = true
	panel := &pickerDeclare{
		targets: nil, where: nil, chosen: "", matched: 0, asked: false, types: types, kinds: kinds, kind: "",
		start: start, truth: []*ui.Button{ui.NewButton("declare-start-false", "false"),
			ui.NewButton("declare-start-true", "true")}, truthOn: false,
		ask: ui.NewButton("picker-new", ""), declare: ui.NewButton("picker-declare", "Declare and choose"),
		whereTop: 0, typeTop: 0, startTop: 0,
	}
	panel.declare.Primary = true
	panel.setKind(0)

	return panel
}

// setKind chooses the type at an index, starting at its own zero.
func (d *pickerDeclare) setKind(index int) {
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
	case worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeNPC, worldstate.TypeGroup, worldstate.TypeTemplate:
	}
}

// showing reports whether the panel is in the list's place: there is a
// name to declare, and no value matches or the panel was asked for.
func (d *pickerDeclare) showing() bool { return len(d.targets) > 0 && (d.asked || d.matched == 0) }

// target is where the value will be declared.
func (d *pickerDeclare) target() (declareTarget, bool) {
	for _, target := range d.targets {
		if target.located() == d.chosen {
			return target, true
		}
	}
	var none declareTarget

	return none, false
}

// freeTargets lists where a typed name can be declared that no value
// already has.
func (s *State) freeTargets(docs frame.Docs, role valueRole, query string, taken map[string]bool) []declareTarget {
	var free []declareTarget
	for _, target := range s.declareTargets(docs, role, query) {
		if !taken[target.Path] {
			free = append(free, target)
		}
	}

	return free
}

// setTargets keeps where the name typed can be declared, with a button for
// each, and the one chosen while it is still offered, else the first.
func (s *State) setTargets(docs frame.Docs, targets []declareTarget) {
	panel := s.values.declare
	same := len(targets) == len(panel.targets)
	for index := 0; same && index < len(targets); index++ {
		same = targets[index].located() == panel.targets[index].located()
	}
	panel.targets = targets
	if !same {
		panel.where = make([]*ui.Button, 0, len(targets))
		for _, target := range targets {
			panel.where = append(panel.where, ui.NewButton("declare-where-"+target.located(), s.whereLabel(docs, target)))
		}
	}
	if _, offered := panel.target(); !offered && len(targets) > 0 {
		panel.chosen = targets[0].located()
	}
	for index, target := range targets {
		panel.where[index].On = target.located() == panel.chosen
	}
	if len(targets) == 0 {
		panel.asked = false
	}
}

// whereLabel names where a value can be declared: its scope, one NPC's
// own, or a template.
func (s *State) whereLabel(docs frame.Docs, target declareTarget) string {
	switch {
	case target.For != "":
		return s.ownerName(docs, target.Document) + "'s own"
	case target.Parent != (ids.ULID{}):
		names := strings.Split(target.Path, ".")

		return target.Scope.Label() + " template " + names[1]
	default:
		return target.Scope.Label()
	}
}

// layoutPickerFoot places the foot's buttons from the right: Choose, or
// Declare and choose while the panel shows; Cancel; and the button that
// asks for the panel while values match, or goes back to them.
func (s *State) layoutPickerFoot(right, foot int) {
	picker := s.values
	panel := picker.declare
	place := func(width int) image.Rectangle {
		rect := image.Rect(right-width, foot, right, foot+ui.ButtonH)
		right -= width + ui.Gap

		return rect
	}
	picker.choose.Rect, panel.declare.Rect, panel.ask.Rect = ui.Zero(), ui.Zero(), ui.Zero()
	if panel.showing() {
		panel.declare.Rect = place(declareWidth)
	} else {
		picker.choose.Rect = place(pickerButton)
	}
	picker.cancel.Rect = place(pickerButton)
	switch {
	case panel.asked:
		panel.ask.Label = "Back to values"
	case len(panel.targets) > 0 && panel.matched > 0:
		panel.ask.Label = s.clip(`Declare "`+picker.query()+`"…`, askWidth-ui.Half*ui.Inset)
	default:
		return
	}
	panel.ask.Rect = place(min(askWidth, s.theme.TextWidth(panel.ask.Label)+ui.Half*ui.Inset))
}

// layoutDeclare places the panel in the list's place: where, type and what
// it starts at, each after its label.
func (s *State) layoutDeclare(area image.Rectangle) {
	panel := s.values.declare
	for _, button := range slices.Concat(panel.where, panel.types, panel.truth) {
		button.Rect = ui.Zero()
	}
	panel.start.Rect = ui.Zero()
	if !panel.showing() {
		return
	}
	left := area.Min.X + textPad + declareLabelWidth
	width := area.Max.X - textPad - left
	panel.whereTop = area.Min.Y + ui.Gap + ui.Half*ui.LineHeight + ui.Gap
	panel.typeTop = s.layoutButtonRows(panel.where, left, width, panel.whereTop) + ui.Gap
	panel.startTop = s.layoutButtonRows(panel.types, left, width, panel.typeTop) + ui.Gap
	switch panel.kind {
	case worldstate.TypeInteger, worldstate.TypeText:
		panel.start.Rect = image.Rect(left, panel.startTop, left+min(width, startFieldWidth), panel.startTop+ui.ButtonH)
	case worldstate.TypeBool:
		s.layoutButtonRows(panel.truth, left, min(width, startFieldWidth), panel.startTop)
	case worldstate.TypeChoice, worldstate.TypeNPC, worldstate.TypeGroup, worldstate.TypeTemplate:
	}
}

// updateDeclare handles the button that asks for the panel and, while it
// shows, its buttons, and Declare and choose or Enter. It reports whether
// it took the input from the list.
func (s *State) updateDeclare(input *ui.Input, ctx *frame.Context, enter bool) bool {
	panel := s.values.declare
	if panel.ask.Update(input) {
		panel.asked = !panel.asked

		return true
	}
	if !panel.showing() {
		return false
	}
	for index, button := range panel.where {
		if button.Update(input) {
			panel.chosen = panel.targets[index].located()
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
		s.declareFrom(ctx)
	}

	return true
}

// declareFrom declares the value the panel describes and chooses it, or
// says what is wrong with what it starts at.
func (s *State) declareFrom(ctx *frame.Context) {
	picker := s.values
	panel := picker.declare
	target, found := panel.target()
	if !found {
		return
	}
	start, problem := panel.startValue()
	if problem != "" {
		picker.note = problem

		return
	}
	s.values = nil
	s.declareValue(ctx, picker.node, picker.param,
		declaring{target: target, kind: panel.kind, start: start, operand: picker.operand})
}

// startValue is what the value starts at, as saved: a typed integer or
// text, or false or true; nil leaves a choice or an NPC at its type's own.
func (d *pickerDeclare) startValue() (json.RawMessage, string) {
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
	case worldstate.TypeChoice, worldstate.TypeNPC, worldstate.TypeGroup, worldstate.TypeTemplate:
		return nil, ""
	}

	return value.Encode(), ""
}

// drawDeclare draws the panel in the list's place: what it declares, then
// where, type and what it starts at, and a note on what is set elsewhere.
func (s *State) drawDeclare(screen *ebiten.Image, docs frame.Docs) {
	picker := s.values
	panel := picker.declare
	area := picker.list.Rect
	ui.StrokeRound(screen, area.Inset(-ui.Half), ui.Radius, ui.Border, ui.ColorRule())
	target, _ := panel.target()
	heading := "Declare " + target.Path
	if target.For != "" {
		heading += " for " + s.ownerName(docs, target.Document)
	}
	left := area.Min.X + textPad
	s.theme.DrawStrong(screen, s.clip(heading, area.Dx()-ui.Half*textPad), left, area.Min.Y+ui.Gap, ui.ColorInk())
	lead := "No value is called that yet. It is chosen as soon as it is declared."
	if panel.matched > 0 {
		lead = "It is chosen as soon as it is declared."
	}
	s.theme.DrawText(screen, lead, left, area.Min.Y+ui.Gap+ui.LineHeight, ui.ColorMuted())
	for _, label := range []struct {
		text string
		top  int
	}{{"Where", panel.whereTop}, {typeLabel, panel.typeTop}, {"Starts at", panel.startTop}} {
		s.theme.DrawStrong(screen, label.text, left, label.top+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorInk())
	}
	for _, button := range slices.Concat(panel.where, panel.types, panel.truth) {
		button.Draw(screen, s.theme)
	}
	noteTop := panel.startTop + ui.ButtonH + ui.Gap
	if panel.start.Rect.Empty() && panel.truth[0].Rect.Empty() {
		noteTop = panel.startTop + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	} else {
		panel.start.Draw(screen)
	}
	s.theme.DrawText(screen, s.clip(startNote(panel.kind, target.Scope), area.Max.X-textPad-left-declareLabelWidth),
		left+declareLabelWidth, noteTop, ui.ColorMuted())
}

// startNote says what the panel leaves to be set elsewhere.
func startNote(kind worldstate.Type, scope worldstate.Scope) string {
	switch kind {
	case worldstate.TypeChoice:
		return "It starts at the first of two choices, named in " + declaredWhere(scope) + "."
	case worldstate.TypeNPC:
		return "It starts as no NPC."
	case worldstate.TypeInteger, worldstate.TypeBool, worldstate.TypeText, worldstate.TypeGroup,
		worldstate.TypeTemplate:
	}

	return "Limits and a description are set in " + declaredWhere(scope) + " afterwards."
}

// locateWhere finds where's button for a declare target, "declare:<path>".
func (d *pickerDeclare) locateWhere(value string) (image.Point, bool) {
	for index, target := range d.targets {
		if target.located() == value && index < len(d.where) && !d.where[index].Rect.Empty() {
			return ui.Center(d.where[index].Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// widgets are the panel's buttons and field, for locating them.
func (d *pickerDeclare) widgets() ([]*ui.Button, *ui.TextField) {
	return slices.Concat([]*ui.Button{d.ask, d.declare}, d.types, d.truth), d.start
}
