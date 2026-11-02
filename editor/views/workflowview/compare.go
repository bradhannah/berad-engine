package workflowview

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Check value's inspector follows its value's type: an integer compares
// with symbols, against a number or another value; a bool only picks true
// or false; a choice ticks a list; text is or is not. The buttons a type
// cannot use are never made.

// compareSymbol is how an integer's comparison is drawn: a symbol, or
// short words for the ones that have none.
func compareSymbol(compare string) string {
	switch compare {
	case flow.CompareIs:
		return "="
	case flow.CompareNot:
		return "≠"
	case flow.CompareBelow:
		return "<"
	case flow.CompareMost:
		return "≤"
	case flow.CompareAbove:
		return ">"
	case flow.CompareLeast:
		return "≥"
	case flow.CompareMax:
		return "at its max"
	case flow.CompareMin:
		return "at its min"
	default:
		return compare
	}
}

// compareNote reads an integer's comparison back in words, under the row
// of symbols.
func compareNote(compare string) string {
	switch compare {
	case flow.CompareIs:
		return "= is equal to"
	case flow.CompareNot:
		return "≠ is not equal to"
	case flow.CompareBelow:
		return "< is below, less than"
	case flow.CompareMost:
		return "≤ is at most, that or less"
	case flow.CompareAbove:
		return "> is above, more than"
	case flow.CompareLeast:
		return "≥ is at least, that or more"
	case flow.CompareBetween:
		return "between, both ends included"
	case flow.CompareMax, flow.CompareMin:
		return compareSymbol(compare) + ", nothing to compare with"
	default:
		return ""
	}
}

// checkPhrase says what a Check value asks after its value's path, and
// whether "is" goes between: an integer by its symbol, "≥ 3" or "between 1
// and 5", or at its limit; a bool "is true"; a choice "is any of calm,
// glad"; text "is not "Stranger"".
func (s *State) checkPhrase(docs frame.Docs, node flow.Node) (string, bool) {
	compare := choiceOf(node, flow.ParamCompare)
	declared, found := s.operandDeclaration(docs, node, flow.ParamAgainst)
	against := s.operandLabel(docs, node, flow.ParamAgainst)
	switch {
	case flow.Unlimited(compare):
		return compareWords(node), false
	case found && declared.Type == worldstate.TypeInteger && compare == flow.CompareBetween:
		return "between " + against + " and " + s.operandLabel(docs, node, flow.ParamTo), false
	case found && declared.Type == worldstate.TypeInteger:
		return compareSymbol(compare) + " " + against, false
	case compare == flow.CompareAnyOf || compare == flow.CompareNoneOf:
		return strings.TrimPrefix(compareWords(node), "is ") + " " + s.operandLabel(docs, node, flow.ParamAmong), true
	case found && declared.Type == worldstate.TypeText && compare == flow.CompareNot:
		return "not " + against, true
	case compare == flow.CompareNot:
		return "not " + against, true
	default:
		return against, true
	}
}

// checkProblem says what a Check value's operands lack: a choice ticked, a
// value of the right type to compare with, or one at all.
func (s *State) checkProblem(docs frame.Docs, node flow.Node, declared worldstate.Declaration) string {
	if unlimited(node) || s.operandsFit(docs, node, declared) {
		return ""
	}
	compare := choiceOf(node, flow.ParamCompare)
	if compare == flow.CompareAnyOf || compare == flow.CompareNoneOf {
		return "Tick at least one choice."
	}
	if ref, refers := flow.OperandRef(savedOperand(node, flow.ParamAgainst)); refers {
		other, _, problem := s.resolve(docs, ref)
		if problem != "" {
			return "The value it compares with was deleted. Choose another."
		}
		if other.Type != declared.Type {
			return "The value it compares with must have the same type, " + declared.Type.Label() + "."
		}
	}

	return problemOperand
}

// hasOperandState answers "refers:<param>=<path>", whether the selected
// node's operand refers to the value at a path, and "another:<param>",
// whether its Another value button is on: the operand refers to a value,
// or its chooser is open to pick one.
func (s *State) hasOperandState(state string, docs frame.Docs) (bool, bool) {
	node, selected := s.selectedNode()
	switch {
	case strings.HasPrefix(state, "refers:"):
		param, want, _ := strings.Cut(strings.TrimPrefix(state, "refers:"), "=")
		saved, err := node.Operand(param)
		ref, refers := flow.OperandRef(saved)
		_, path, problem := s.resolve(docs, ref)

		return selected && err == nil && refers && problem == "" && path == want, true
	case strings.HasPrefix(state, "another:"):
		return s.anotherValueLit(strings.TrimPrefix(state, "another:")), true
	default:
		return false, false
	}
}

// clearRects takes every widget of a control off the screen, for a control
// that is hidden or whose parts are not all laid out this frame, so a stale
// rectangle never takes a click meant for what is drawn there now.
func (made *control) clearRects() {
	fields, buttons := made.widgets()
	for _, field := range fields {
		if field != nil {
			field.Rect = ui.Zero()
		}
	}
	for _, button := range slices.Concat(buttons, made.sources) {
		if button != nil {
			button.Rect = ui.Zero()
		}
	}
	made.cueBox, made.anchor = ui.Zero(), ui.Zero()
	if made.list != nil {
		made.list.Rect = ui.Zero()
	}
}

// picksValue reports whether an operand shows the box to pick a value
// from: it refers to one, or Another value was clicked.
func (made *control) picksValue() bool { return made.refers || made.another }

// anotherValueLit reports whether an operand's Another value button is on.
func (s *State) anotherValueLit(param string) bool {
	if s.form == nil {
		return false
	}
	for _, made := range s.form.controls {
		if made.param.Name == param && len(made.sources) == ui.Half {
			return made.sources[1].On
		}
	}

	return false
}

// savedOperand is an operand as saved, or none when the node lacks it.
func savedOperand(node flow.Node, name string) json.RawMessage {
	saved, err := node.Operand(name)
	if err != nil {
		return flow.NoOperand()
	}

	return saved
}

// isBox reports whether a widget name is this control's pick box: a value's
// or an operand's "pick-<param>", or a cue's "cue".
func (made control) isBox(widget string) bool {
	switch made.param.Type {
	case flow.TypeValue, flow.TypeOperand:
		return widget == valueBoxPrefix+made.param.Name
	case flow.TypeCue:
		return widget == "cue"
	case flow.TypeAnchor, flow.TypeChoice, flow.TypeDuration, flow.TypeShares, flow.TypeText, flow.TypeTiles,
		flow.TypeTimeRange, flow.TypeTimeRanges:
	}

	return false
}

// newChoiceControl makes a choice's buttons, one per declared choice; Check
// value's comparison is then shaped to its value's type.
func (s *State) newChoiceControl(docs frame.Docs, node flow.Node, made *control) {
	for _, choice := range made.param.Choices {
		made.choices = append(made.choices, ui.NewButton(made.param.Name+"-"+choice.Value, choice.Label))
		made.values = append(made.values, choice.Value)
	}
	if node.Kind == flow.KindCheckValue && made.param.Name == flow.ParamCompare {
		s.shapeCompare(docs, node, made)
	}
}

// declaredLabel is the label a parameter declares for one of its choices.
func declaredLabel(choices []flow.Choice, value string) string {
	for _, choice := range choices {
		if choice.Value == value {
			return choice.Label
		}
	}

	return value
}

// shapeCompare keeps only the comparisons the value's type takes on Check
// value's Compare row, drawn as symbols for an integer; a type with nothing
// to choose, or no value chosen yet, hides the row.
func (s *State) shapeCompare(docs frame.Docs, node flow.Node, made *control) {
	declared, found := s.operandDeclaration(docs, node, flow.ParamAgainst)
	allowed := flow.Comparisons(declared.Type)
	if !found || len(allowed) < ui.Half {
		made.hidden = true

		return
	}
	made.choices, made.values = nil, nil
	for _, compare := range allowed {
		label := declaredLabel(made.param.Choices, compare)
		if declared.Type == worldstate.TypeInteger {
			label = compareSymbol(compare)
		}
		made.choices = append(made.choices, ui.NewButton(made.param.Name+"-"+compare, label))
		made.values = append(made.values, compare)
	}
	made.note = declared.Type == worldstate.TypeInteger
}

// drawCompareNote draws the chosen comparison in words under the symbols.
func (s *State) drawCompareNote(screen *ebiten.Image, made *control, node flow.Node) {
	if len(made.choices) == 0 {
		return
	}
	last := made.choices[len(made.choices)-1].Rect
	s.theme.DrawText(screen, compareNote(choiceOf(node, flow.ParamCompare)), made.choices[0].Rect.Min.X,
		last.Max.Y+rowSpace, ui.ColorMuted())
}

// saveCompare saves Check value's comparison. Any of or none of chosen on a
// choice that an older is compared with one choice takes that choice as its
// list, so nothing is lost.
func (s *State) saveCompare(ctx *frame.Context, node flow.Node, compare string) {
	changed, err := node.WithParam(flow.ParamCompare, compare)
	if err == nil && (compare == flow.CompareAnyOf || compare == flow.CompareNoneOf) {
		among, against := savedOperand(node, flow.ParamAmong), savedOperand(node, flow.ParamAgainst)
		if flow.IsNoOperand(among) && !flow.IsNoOperand(against) {
			changed, err = changed.WithParam(flow.ParamAmong, flow.OperandList([]json.RawMessage{against}))
		}
	}
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	s.replace(ctx, changed, "")
}

// shapeCheckOperand fits one of Check value's operands to its value's type
// and comparison. An integer's With gets A number and Another value, with
// a chooser for the value; a bool's is called Is; a choice ticks Among, as
// toggles, in place of With; "and" shows only for between. An operand the
// comparison has no use for is hidden.
func (s *State) shapeCheckOperand(node flow.Node, made *control, declared worldstate.Declaration, found bool) {
	compare, kind := choiceOf(node, flow.ParamCompare), declared.Type
	switch made.param.Name {
	case flow.ParamAgainst:
		made.hidden = found && kind == worldstate.TypeChoice
		if found && kind == worldstate.TypeBool {
			made.param.Label = "Is"
		}
		if found && kind == worldstate.TypeInteger && !made.unneeded {
			s.newOperandSources(made, node)
		}
	case flow.ParamTo:
		made.hidden = !found || kind != worldstate.TypeInteger || compare != flow.CompareBetween
	case flow.ParamAmong:
		made.hidden = !found || kind != worldstate.TypeChoice
		made.param.Label = "With"
	}
}

// newOperandSources makes A number and Another value over an integer's
// operand, and the chooser Another value opens.
func (s *State) newOperandSources(made *control, node flow.Node) {
	made.sources = []*ui.Button{ui.NewButton("source-number", "A number"), ui.NewButton("source-value", "Another value")}
	_, made.refers = flow.OperandRef(savedOperand(node, made.param.Name))
}

// operandRows lists the values an operand may refer to: every value a
// Check value can read that is of the same type, under the chooser's
// headings, narrowed by a search.
func (s *State) operandRows(docs frame.Docs, kind worldstate.Type, query string) []ui.Row {
	query = strings.ToLower(strings.TrimSpace(query))
	var rows []ui.Row
	heading := ""
	for _, option := range s.valueOptions(docs, roleCheck) {
		if option.declared.Type != kind || (query != "" && !strings.Contains(option.path, query)) {
			continue
		}
		if option.heading != heading {
			heading = option.heading
			rows = append(rows, ui.NewHeading(valueHeading+heading, heading))
		}
		row := ui.NewRow(encodeRef(option.ref), option.path)
		row.Detail = option.declared.Type.Label()
		rows = append(rows, row)
	}

	return rows
}

// updateOperandSource handles A number and Another value: a number drops
// the reference for a zero to type over; another value puts up the box to
// pick one from, which opens its chooser when clicked, as the Value box
// above does.
func (s *State) updateOperandSource(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) bool {
	if len(made.sources) < ui.Half {
		return false
	}
	switch {
	case made.sources[0].Update(input):
		made.another, made.open = false, false
		if made.refers {
			s.saveParam(ctx, node, made.param.Name, json.RawMessage("0"))
		}

		return true
	case made.sources[1].Update(input):
		made.another = true

		return true
	default:
		return false
	}
}

// updateOperandChooser opens the picker, offering values of the operand's
// type, when its box is clicked.
func (s *State) updateOperandChooser(input *ui.Input, _ *frame.Context, made *control, node flow.Node) {
	if input.Pressed && input.Cursor.In(made.cueBox) {
		input.ClearPointer()
		s.openValuePicker(node, made.param.Name, pickerTitle(made.param, made.operandType), made.operandType, true)
	}
}

// drawOperandChooser draws the value an operand refers to, or asks for one,
// as a box that opens the picker, with the value's detail under it.
func (s *State) drawOperandChooser(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node) {
	label, ink, detail := chooseValueLabel, ui.ColorMuted(), ""
	if saved, err := node.Operand(made.param.Name); err == nil {
		if ref, refers := flow.OperandRef(saved); refers {
			declared, path, problem := s.resolve(docs, ref)
			label, ink, detail = path, ui.ColorInk(), valueDetail(declared)
			if problem != "" {
				label, ink, detail = "Broken: "+strings.ToLower(strings.TrimSuffix(problem, ".")), ui.ColorWarn(), ""
			}
		}
	}
	open := s.values != nil && s.values.operand && s.values.param == made.param.Name
	s.drawPickBox(screen, made.cueBox, label, ink, open)
	if detail != "" {
		s.theme.DrawText(screen, s.clip(detail, made.cueBox.Dx()), made.cueBox.Min.X, made.cueBox.Max.Y+ui.Gap,
			ui.ColorMuted())
	}
}

// saveAgainst saves what a Check value compares with; a comparison its
// value's type no longer takes, left over from another type, becomes the
// first the type does, so a bool whose row is hidden never sticks.
func (s *State) saveAgainst(ctx *frame.Context, node flow.Node, made *control, value json.RawMessage) {
	changed, err := node.WithParam(made.param.Name, value)
	allowed := flow.Comparisons(made.operandType)
	if err == nil && len(allowed) > 0 && !flow.CanCompare(choiceOf(node, flow.ParamCompare), made.operandType) {
		changed, err = changed.WithParam(flow.ParamCompare, allowed[0])
	}
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	s.replace(ctx, changed, "")
}

// saveAmong ticks or unticks one choice in any of's or none of's list and
// saves the list; a comparison still saved as is becomes any of.
func (s *State) saveAmong(ctx *frame.Context, node flow.Node, made *control, index int) {
	var items []json.RawMessage
	for which, button := range made.choices {
		if button.On != (which == index) && which < len(made.operands) {
			items = append(items, made.operands[which])
		}
	}
	changed, err := node.WithParam(flow.ParamAmong, flow.OperandList(items))
	compare := choiceOf(node, flow.ParamCompare)
	if err == nil && compare != flow.CompareAnyOf && compare != flow.CompareNoneOf {
		changed, err = changed.WithParam(flow.ParamCompare, flow.CompareAnyOf)
	}
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	s.replace(ctx, changed, "")
}

// operandsFit reports whether a Set value's or Check value's operands hold
// what it needs: a value of its value's type or a reference to one, both
// ends of a range, or at least one ticked choice.
func (s *State) operandsFit(docs frame.Docs, node flow.Node, declared worldstate.Declaration) bool {
	if node.Kind != flow.KindCheckValue {
		return operandFits(node, operandOfValue(node.Kind, flow.ParamValue), declared)
	}
	switch choiceOf(node, flow.ParamCompare) {
	case flow.CompareBetween:
		return s.refFits(docs, node, flow.ParamAgainst, declared) && s.refFits(docs, node, flow.ParamTo, declared)
	case flow.CompareAnyOf, flow.CompareNoneOf:
		among, err := node.Operand(flow.ParamAmong)
		items := flow.Operands(among)
		if err != nil || len(items) == 0 {
			return false
		}

		return !slices.ContainsFunc(items, func(item json.RawMessage) bool {
			_, parseErr := declared.Parse(item)

			return parseErr != nil
		})
	default:
		return s.refFits(docs, node, flow.ParamAgainst, declared)
	}
}

// refFits reports whether an operand holds a value of the declaration's
// type, or refers to a value of that type that still exists.
func (s *State) refFits(docs frame.Docs, node flow.Node, param string, declared worldstate.Declaration) bool {
	saved, err := node.Operand(param)
	if err != nil {
		return false
	}
	if ref, refers := flow.OperandRef(saved); refers {
		other, _, problem := s.resolve(docs, ref)

		return problem == "" && other.Type == declared.Type
	}

	return operandFits(node, param, declared)
}
