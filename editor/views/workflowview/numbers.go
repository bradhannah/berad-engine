package workflowview

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuepicker"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A number's constant, in screen pixels and for input scripts: the small
// button at the end of a number's field that uses a constant, as
// "ui:constant-<param>" or for a random choice's option
// "ui:constant-options-<n>", and the × that goes back to typing, as
// "ui:typed-<param>".
const (
	slotButton  = 26
	slotUsePre  = "constant-"
	slotClearOn = "typed-"
	slotChipPad = 6
)

// numberSlot is where a constant can stand in for a typed-in number, in the
// inspector: a number's field, or a random choice's option's share. use
// opens the picker for a constant; clear goes back to the number typed;
// chip shows the constant standing in. A constant chosen, or going back to
// the number typed, waits for Save as typing does: draft holds it until
// the node shows it, saving is set once Save has sent it, and lasting
// names the slot across the form being rebuilt.
type numberSlot struct {
	param    string
	option   int
	lasting  string
	field    *ui.TextField
	use      *ui.Button
	clear    *ui.Button
	chip     image.Rectangle
	constant flow.ValueRef
	draft    *flow.ValueRef
	saving   bool
}

// unsaved reports whether a constant was chosen, or the number typed gone
// back to, and no Save has been asked for it yet.
func (n *numberSlot) unsaved() bool { return n.draft != nil && !n.saving }

// slotKey names a slot: its parameter, and an option's number.
func slotKey(param string, option int) string {
	if option < 0 {
		return param
	}

	return param + "-" + strconv.Itoa(option+1)
}

// slotOf returns a slot for a field, made once while the form stands, with
// the constant chosen for it before the form was rebuilt. lasting names it
// across rebuilds: its parameter, and an option's output.
func (s *State) slotOf(param string, option int, lasting string, field *ui.TextField) *numberSlot {
	key := slotKey(param, option)
	if made, found := s.form.slots[key]; found {
		made.field, made.lasting = field, lasting

		return made
	}
	made := &numberSlot{
		param: param, option: option, lasting: lasting, field: field, use: ui.NewButton(slotUsePre+key, "◆"),
		clear: ui.NewButton(slotClearOn+key, "×"), chip: ui.Zero(), constant: flow.NoConstant(), draft: nil,
		saving: false,
	}
	if kept, found := s.form.keptSlots[lasting]; found {
		made.draft = &kept
	}
	s.form.slots[key] = made

	return made
}

// layoutSlots finds the constant standing in for each of the selected
// node's numbers and places its button at the end of the number's field,
// or the constant's chip where the field was.
func (s *State) layoutSlots() {
	node, ok := s.selectedNode()
	if s.form == nil || !ok {
		return
	}
	for _, slot := range s.form.slots {
		slot.use.Rect, slot.clear.Rect, slot.chip = ui.Zero(), ui.Zero(), ui.Zero()
	}
	for index := range s.form.controls {
		s.placeSlots(node, &s.form.controls[index])
	}
}

// placeSlots places the slots of one parameter's numbers: a whole number's,
// a wait's amount's, or each option's share's.
func (s *State) placeSlots(node flow.Node, made *control) {
	name := made.param.Name
	if made.param.Type == flow.TypeTiles || made.param.Type == flow.TypeSeconds {
		amount, _ := node.Amount(name) //nolint:errcheck // One that can't be read has no constant.
		s.placeSlot(s.slotOf(name, -1, name, made.field), amount.Constant)
	}
	if made.param.Type == flow.TypeDuration {
		wait, _ := node.Duration(name) //nolint:errcheck // As above.
		s.placeSlot(s.slotOf(name, -1, name, made.field), wait.Constant)
	}
	if made.param.Type != flow.TypeShares {
		return
	}
	options, _ := node.Shares(name) //nolint:errcheck // As above.
	for option, row := range made.options {
		constant := flow.NoConstant()
		if option < len(options) {
			constant = options[option].Constant
		}
		s.placeSlot(s.slotOf(name, option, name+"/"+row.port, row.share), constant)
	}
}

// placeSlot puts a slot's button at the end of its field, which keeps the
// rest; or, with a constant standing in, saved or chosen, the chip where
// the field was and × at its end. What was chosen stops being a draft once
// the node shows it.
func (s *State) placeSlot(slot *numberSlot, saved flow.ValueRef) {
	if slot.draft != nil && *slot.draft == saved {
		slot.draft, slot.saving = nil, false
	}
	slot.constant = saved
	if slot.draft != nil {
		slot.constant = *slot.draft
	}
	field := slot.field.Rect
	if field.Empty() {
		return
	}
	button := image.Rect(field.Max.X-slotButton, field.Min.Y, field.Max.X, field.Max.Y)
	if slot.constant.IsZero() {
		slot.use.Rect = button
		slot.field.Rect.Max.X = button.Min.X - ui.Half

		return
	}
	slot.clear.Rect = button
	slot.chip = image.Rect(field.Min.X, field.Min.Y, button.Min.X-ui.Half, field.Max.Y)
	slot.field.Rect = ui.Zero()
}

// updateSlots opens the picker for a constant, from a slot's button or its
// chip, or goes back to the number typed; it reports whether it did.
func (s *State) updateSlots(input *ui.Input, node flow.Node) bool {
	for _, slot := range s.form.slots {
		switch {
		case slot.use.Update(input):
			s.openNumberPicker(node, slot)

			return true
		case slot.clear.Update(input):
			s.setSlotDraft(node, slot.param, slot.option, flow.NoConstant())

			return true
		case input.Pressed && input.Cursor.In(slot.chip):
			input.ClearPointer()
			s.openNumberPicker(node, slot)

			return true
		}
	}

	return false
}

// openNumberPicker opens the picker on the whole-number constants, for a
// typed-in number.
func (s *State) openNumberPicker(node flow.Node, slot *numberSlot) {
	source := &valueSource{
		state: s, node: node.ID, param: slot.param, role: roleNumber, operand: false, targets: nil, option: slot.option,
		words: nil,
	}
	selected := ""
	if !slot.constant.IsZero() {
		selected = encodeRef(slot.constant)
	}
	config := valuepicker.Config{
		Title: "Choose a constant", For: "to stand for this number", Placeholder: "Find a whole-number constant",
		Empty: "No whole-number constant matches. Make one in World state's Constants window.", Only: "",
		Kinds: []worldstate.Type{worldstate.TypeInteger}, Start: strconv.Itoa(s.typedNow(node, slot)),
		Rail: pickerRail(roleCheck), Selected: selected, Query: "",
		LastGroup: &s.lastGroup, PickLabel: "Use", PickHint: "uses it", DeclareAs: "Declare and use",
		DeclareLead: "It is used as soon as it is declared.",
	}
	s.values = &valuePicker{
		picker: valuepicker.New(config, source, s.theme), node: node.ID, param: slot.param, role: roleNumber,
		operand: false,
	}
}

// setSlotDraft chooses a constant to stand in for a typed-in number, or with
// none goes back to the number typed: a change that waits for Save, as
// typing does. A number typed that the node could not keep goes as a
// constant stands in for it, so Save never stops at a field the constant
// hides.
func (s *State) setSlotDraft(node flow.Node, param string, option int, ref flow.ValueRef) {
	if s.form == nil || s.form.node != node.ID {
		return
	}
	slot, found := s.form.slots[slotKey(param, option)]
	if !found {
		return
	}
	slot.draft, slot.saving = nil, false
	if ref != savedConstant(node, param, option) {
		slot.draft = &ref
	}
	if _, fits := typedFits(node, param, slot.field.Field.Value()); !ref.IsZero() && !fits {
		slot.field.Field.Revert()
	}
}

// savedConstant is the constant a node has standing in for one of its
// numbers, or none.
func savedConstant(node flow.Node, param string, option int) flow.ValueRef {
	switch paramType(node, param) {
	case flow.TypeDuration:
		if wait, err := node.Duration(param); err == nil {
			return wait.Constant
		}
	case flow.TypeShares:
		if options, err := node.Shares(param); err == nil && option >= 0 && option < len(options) {
			return options[option].Constant
		}
	case flow.TypeTiles, flow.TypeSeconds:
		if amount, err := node.Amount(param); err == nil {
			return amount.Constant
		}
	case flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort, flow.TypeAnchor,
		flow.TypeChoice, flow.TypeTimeRange, flow.TypeTimeRanges, flow.TypeText, flow.TypeCue, flow.TypeMessage,
		flow.TypeSignal, flow.TypeNPC, flow.TypeLanes, flow.TypeValue, flow.TypeOperand:
	}

	return flow.NoConstant()
}

// typedFits reads a number typed for a parameter, reporting whether the
// node could keep it: a whole number within the parameter's limits.
func typedFits(node flow.Node, param, typed string) (int, bool) {
	number, err := strconv.Atoi(strings.TrimSpace(typed))
	if err != nil {
		return 0, false
	}
	low, high := flow.Limits(paramType(node, param))
	if wait, err := node.Duration(param); err == nil && paramType(node, param) == flow.TypeDuration {
		low, high = flow.WaitLimits(wait)
	}

	return number, number >= low && number <= high
}

// typedNow is the number typed for a slot: what its field holds, when the
// node could keep it, or else the number saved.
func (s *State) typedNow(node flow.Node, slot *numberSlot) int {
	if number, fits := typedFits(node, slot.param, slot.field.Field.Value()); fits {
		return number
	}

	return typedNumber(node, slot.param, slot.option)
}

// numberConstantLabel is how a constant standing in for a number reads: its
// path and what it holds, as its default for an NPC's or a zone's.
func (s *State) numberConstantLabel(docs frame.Docs, ref flow.ValueRef) string {
	declared, path, problem := s.resolve(docs, ref)
	if problem != "" {
		return problem
	}
	value := strconv.Itoa(declared.Zero().Integer)
	if declared.Scope.Owned() && ref.Owner.IsZero() {
		value += " by default"
	}

	return path + " · " + value
}

// drawSlots draws each slot's button, or its constant's chip, marked as a
// field is while what was chosen waits for Save.
func (s *State) drawSlots(screen *ebiten.Image, docs frame.Docs) {
	for _, slot := range s.form.slots {
		slot.use.Draw(screen, s.theme)
		slot.clear.Draw(screen, s.theme)
		if slot.chip.Empty() {
			if slot.unsaved() && !slot.field.Field.Dirty() {
				ui.DrawUnsavedMark(screen, slot.field.Rect)
			}

			continue
		}
		ui.FillRound(screen, slot.chip, ui.Radius, ui.ColorConstantWash())
		ui.StrokeRound(screen, slot.chip, ui.Radius, ui.Border, ui.ColorRule())
		inner := slot.chip.Inset(slotChipPad)
		s.theme.DrawTextIn(screen, s.theme.FitText(s.numberConstantLabel(docs, slot.constant), inner.Dx()),
			image.Rect(inner.Min.X, slot.chip.Min.Y, inner.Max.X, slot.chip.Max.Y), ui.ColorConstant())
		if slot.unsaved() {
			ui.DrawUnsavedMark(screen, slot.chip)
		}
	}
}

// locateSlot finds a slot's button for input scripts.
func (s *State) locateSlot(widget string) (image.Point, bool) {
	if s.form == nil {
		return image.Pt(0, 0), false
	}
	for _, slot := range s.form.slots {
		for _, button := range []*ui.Button{slot.use, slot.clear} {
			if button.ID == widget && !button.Rect.Empty() {
				return ui.Center(button.Rect), true
			}
		}
	}

	return image.Pt(0, 0), false
}

// hasSlotState answers "constant:<param>=<path>", the constant standing in
// for a number, by its path, or "constant:<param>=" for none.
func (s *State) hasSlotState(docs frame.Docs, state string) (bool, bool) {
	asked, isSlot := strings.CutPrefix(state, "constant:")
	if !isSlot || s.form == nil {
		return false, false
	}
	key, want, _ := strings.Cut(asked, "=")
	slot, found := s.form.slots[key]
	if !found {
		return false, true
	}
	if slot.constant.IsZero() {
		return want == "", true
	}
	_, path, problem := s.resolve(docs, slot.constant)

	return problem == "" && samePath(path, want), true
}

// numberProblem says what is wrong with a constant standing in for a
// number: one deleted, one not a whole number, or one that is outside the
// number's limits, which the game holds it to.
func (s *State) numberProblem(node flow.Node, docs frame.Docs) string {
	for _, standing := range node.NumberConstants() {
		declared, path, problem := s.resolve(docs, standing.Constant)
		if problem != "" {
			return problem
		}
		if !declared.IsConstant() || declared.Type != worldstate.TypeInteger {
			return path + " is not a whole-number constant."
		}
		kind := paramType(node, standing.Param)
		low, high := flow.Limits(kind)
		if wait, err := node.Duration(standing.Param); err == nil && kind == flow.TypeDuration {
			low, high = flow.WaitLimits(wait)
		}
		for _, number := range s.constantNumbers(docs, declared) {
			if number < low || number > high {
				return path + " is " + strconv.Itoa(number) + ", outside " + strconv.Itoa(low) + " to " +
					strconv.Itoa(high) + ". The game holds it to them."
			}
		}
	}

	return ""
}

// constantNumbers lists every number a constant can be: its default, and
// each NPC's or zone's own.
func (s *State) constantNumbers(docs frame.Docs, declared worldstate.Declaration) []int {
	numbers := []int{declared.Zero().Integer}
	catalog := s.catalog(docs)
	if template, inTemplate := catalog.World().Template(declared.ID); inTemplate {
		for _, holder := range catalog.Holders(template) {
			owner, _ := catalog.Owner(holder)
			if value, own := owner.Owned.ConstantValue(declared); own {
				numbers = append(numbers, value.Integer)
			}
		}
	}

	return numbers
}

// paramType is a node's parameter's type.
func paramType(node flow.Node, param string) flow.ParamType {
	kind, _ := flow.KindOf(node.Kind)
	for _, declared := range kind.Params {
		if declared.Name == param {
			return declared.Type
		}
	}

	return ""
}

// numberLabel is how a typed-in number reads on a node: the number, or the
// path of the constant standing in for it.
func (s *State) numberLabel(docs frame.Docs, number int, constant flow.ValueRef) string {
	if constant.IsZero() {
		return strconv.Itoa(number)
	}
	if _, path, problem := s.resolve(docs, constant); problem == "" {
		return path
	}

	return strconv.Itoa(number)
}
