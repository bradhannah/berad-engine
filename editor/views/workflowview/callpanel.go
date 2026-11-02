package workflowview

import (
	"encoding/json"
	"image"
	"maps"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The Call's inspector: the workflow it calls, with Open; each of the
// module's inputs, given a value, a place picked on a map or another value,
// or left to start as it does; and each of its outputs, kept in a value or
// left. Locate names: "ui:call-workflow", the box that lists the workflows
// as "menu:workflow:<id>" or "menu:workflow-named:<name>"; "ui:call-open"; an input's "ui:input-<name>"
// field, "ui:input-<name>-<value>" buttons, "ui:input-<name>-pick",
// "ui:input-<name>-another", "ui:input-<name>-value" box and
// "ui:input-<name>-clear"; an output's "ui:keep-<name>" box and
// "ui:keep-<name>-leave".
const (
	callWorkflowID = "call-workflow"
	callOpenID     = "call-open"
	inputPrefix    = "input-"
	keepPrefix     = "keep-"
	// menuWorkflow prefixes a workflow's item in the list of workflows.
	menuWorkflow = "workflow:"
	// pickInput and pickKeep prefix the parameter a picker chooses for:
	// another value for an input, or where an output is kept.
	pickInput = "input:"
	pickKeep  = "keep:"
	openWidth = 70
	sideWidth = 116
	clearW    = 60
)

// callPanel is the Call's part of the inspector.
type callPanel struct {
	box     image.Rectangle
	label   int
	open    *ui.Button
	inputs  []inputRow
	keeps   []keepRow
	heads   map[string]int
	nothing []string
}

// inputRow gives one input of the module.
type inputRow struct {
	input   worldstate.Declaration
	label   int
	field   *ui.TextField
	choices []*ui.Button
	values  []json.RawMessage
	pick    *ui.Button
	another *ui.Button
	clear   *ui.Button
	box     image.Rectangle
	// place is a place input's box, which opens the list of places.
	place image.Rectangle
}

// keepRow keeps one output of the module.
type keepRow struct {
	output worldstate.Declaration
	label  int
	box    image.Rectangle
	leave  *ui.Button
}

// newCallPanel makes the Call's inspector for its module as loaded.
func (s *State) newCallPanel(node flow.Node) *callPanel {
	open := ui.NewButton(callOpenID, "Open")
	open.Disabled = true
	panel := &callPanel{box: ui.Zero(), label: 0, open: open, inputs: nil, keeps: nil, heads: map[string]int{},
		nothing: nil}
	called, ok := s.moduleOf(node)
	if !ok {
		return panel
	}
	open.Disabled = false
	for _, input := range called.workflow.Inputs() {
		panel.inputs = append(panel.inputs, s.newInputRow(input))
	}
	for _, output := range called.workflow.Outputs() {
		leave := ui.NewButton(keepPrefix+output.Name+"-leave", "Leave it")
		panel.keeps = append(panel.keeps, keepRow{output: output, label: 0, box: ui.Zero(), leave: leave})
	}

	return panel
}

// newInputRow makes the controls an input's type takes.
func (s *State) newInputRow(input worldstate.Declaration) inputRow {
	prefix := inputPrefix + input.Name
	row := inputRow{
		input: input, label: 0, field: nil, choices: nil, values: nil, pick: nil,
		another: ui.NewButton(prefix+"-another", "Another value"), clear: ui.NewButton(prefix+"-clear", "Clear"),
		box: ui.Zero(), place: ui.Zero(),
	}
	switch input.Type {
	case worldstate.TypeInteger, worldstate.TypeText:
		row.field = ui.NewTextField(prefix, textedit.NewField("", false, worldstate.MaxText), false, s.theme)
		row.field.Placeholder = "Starts as " + input.Label(input.Zero())
		s.focus.Register(row.field)
	case worldstate.TypeBool, worldstate.TypeChoice:
		row.choices, row.values = valueButtons(prefix, input)
	case worldstate.TypePlace:
		row.pick = ui.NewButton(prefix+"-pick", "Pick on map")
	case worldstate.TypeNPC, worldstate.TypeGroup, worldstate.TypeTemplate:
	}

	return row
}

// valueButtons are a bool's or a choice's buttons and what each gives.
func valueButtons(prefix string, input worldstate.Declaration) ([]*ui.Button, []json.RawMessage) {
	var buttons []*ui.Button
	var values []json.RawMessage
	given := worldstate.NoValue()
	given.Type = input.Type
	if input.Type == worldstate.TypeBool {
		for _, truth := range []bool{false, true} {
			given.Bool = truth
			label := strconv.FormatBool(truth)
			buttons, values = append(buttons, ui.NewButton(prefix+"-"+label, label)), append(values, given.Encode())
		}

		return buttons, values
	}
	for _, choice := range input.Choices {
		given.Choice = choice.ID
		buttons = append(buttons, ui.NewButton(prefix+"-"+choice.ID, choice.Name))
		values = append(values, given.Encode())
	}

	return buttons, values
}

// callShape says what a Call's inspector is built from: its module's
// inputs and outputs, so it is built again when they change.
func (s *State) callShape(node flow.Node) string {
	called, ok := s.moduleOf(node)
	if !ok {
		return "none"
	}
	var shape strings.Builder
	for _, declared := range append(called.workflow.Inputs(), called.workflow.Outputs()...) {
		shape.WriteString(declared.ID.String() + ":" + declared.Name + ":" + string(declared.Type) + ":" +
			strconv.Itoa(len(declared.Choices)) + ";")
	}

	return shape.String()
}

// setSavedCall shows what the Call gives each input: a field's text and the
// button on.
func (panel *callPanel) setSaved(node flow.Node) {
	given := node.Inputs()
	for _, row := range panel.inputs {
		raw, filled := given[row.input.ID]
		_, refers := flow.OperandRef(raw)
		value, err := row.input.Parse(raw)
		if row.field != nil {
			text := ""
			if filled && !refers && err == nil {
				text = value.Text
				if row.input.Type == worldstate.TypeInteger {
					text = strconv.Itoa(value.Integer)
				}
			}
			row.field.Field.SetSaved(text)
		}
		for index, button := range row.choices {
			button.On = filled && !refers && string(row.values[index]) == string(raw)
		}
	}
}

// layoutCall places the Call's inspector from top down and returns where
// the next part goes.
func (s *State) layoutCall(panel *callPanel, left, width, top int) int {
	panel.label = top
	top += ui.LineHeight
	panel.box = image.Rect(left, top, left+width-openWidth-rowSpace, top+ui.ButtonH)
	panel.open.Rect = image.Rect(left+width-openWidth, top, left+width, top+ui.ButtonH)
	top += ui.ButtonH + fieldSpace
	if panel.open.Disabled {
		return top
	}
	panel.heads["Inputs"] = top
	top += ui.LineHeight
	for index := range panel.inputs {
		top = s.layoutInput(&panel.inputs[index], left, width, top)
	}
	if len(panel.inputs) == 0 {
		top += ui.LineHeight
	}
	panel.heads["Outputs"] = top
	top += ui.LineHeight
	for index := range panel.keeps {
		row := &panel.keeps[index]
		row.label = top
		top += ui.LineHeight
		row.box = image.Rect(left, top, left+width-sideWidth-rowSpace, top+ui.ButtonH)
		row.leave.Rect = image.Rect(left+width-sideWidth, top, left+width, top+ui.ButtonH)
		top += ui.ButtonH + rowSpace
	}

	return top + fieldSpace
}

// layoutInput places one input's controls under its name, with Another
// value at the right; a place's box takes the row, the line naming its zone
// and floor under it, then Pick on map and Another value.
func (s *State) layoutInput(row *inputRow, left, width, top int) int {
	row.label = top
	row.clear.Rect = image.Rect(left+width-clearW, top-ui.Half, left+width, top+ui.ButtonH-ui.Half)
	top += ui.ButtonH
	main := image.Rect(left, top, left+width-sideWidth-rowSpace, top+ui.ButtonH)
	row.another.Rect = image.Rect(left+width-sideWidth, top, left+width, top+ui.ButtonH)
	row.box = main
	if row.field != nil {
		row.field.Rect = main
	}
	if row.pick != nil {
		row.place = image.Rect(left, top, left+width, top+ui.ButtonH)
		row.box = row.place
		top += ui.ButtonH + ui.LineHeight
		half := (width - rowSpace) / ui.Half
		row.pick.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
		row.another.Rect = image.Rect(left+width-half, top, left+width, top+ui.ButtonH)
	}
	if len(row.choices) > 0 {
		top = s.layoutButtonRows(row.choices, main.Min.X, main.Dx(), top) - rowSpace

		return top + ui.Half*rowSpace
	}

	return top + ui.ButtonH + ui.Half*rowSpace
}

// hideRefControls hides an input's own controls while it gives another
// value, whose box shows instead, and Clear while it is unfilled.
func (panel *callPanel) hideRefControls(node flow.Node) {
	given := node.Inputs()
	for index := range panel.inputs {
		row := &panel.inputs[index]
		raw, filled := given[row.input.ID]
		if !filled {
			row.clear.Rect = ui.Zero()
		}
		if _, refers := flow.OperandRef(raw); !refers {
			if row.field != nil || row.pick != nil || len(row.choices) > 0 {
				row.box = ui.Zero()
			}

			continue
		}
		if row.field != nil {
			row.field.Rect = ui.Zero()
		}
		if row.pick != nil {
			row.pick.Rect, row.place = ui.Zero(), ui.Zero()
		}
		for _, button := range row.choices {
			button.Rect = ui.Zero()
		}
	}
}

// fields lists the Call's typed fields.
func (panel *callPanel) fields() []*ui.TextField {
	var fields []*ui.TextField
	for _, row := range panel.inputs {
		if row.field != nil {
			fields = append(fields, row.field)
		}
	}

	return fields
}

// buttons lists the Call's buttons, for input scripts and drawing.
func (panel *callPanel) buttons() []*ui.Button {
	buttons := []*ui.Button{panel.open}
	for _, row := range panel.inputs {
		buttons = append(append(buttons, row.choices...), row.another, row.clear)
		if row.pick != nil {
			buttons = append(buttons, row.pick)
		}
	}
	for _, row := range panel.keeps {
		buttons = append(buttons, row.leave)
	}

	return buttons
}

// updateCall handles the Call's inspector.
func (s *State) updateCall(input *ui.Input, ctx *frame.Context, node flow.Node) {
	panel := s.form.call
	switch {
	case input.Pressed && input.Cursor.In(panel.box) && panel.box != ui.Zero():
		s.openWorkflowMenu(ctx.Docs)
	case panel.open.Update(input):
		if target, ok := node.Called(); ok {
			ctx.Requests.Open(ViewName, target, false)
		}
	}
	for index := range panel.inputs {
		s.updateInput(input, ctx, node, &panel.inputs[index])
	}
	for _, row := range panel.keeps {
		switch {
		case input.Pressed && input.Cursor.In(row.box):
			s.openCallPicker(node, pickKeep, row.output)
		case row.leave.Update(input):
			keeps := node.Keeps()
			delete(keeps, row.output.ID)
			s.saveParam(ctx, node, flow.ParamKeep, keeps)
		}
	}
}

// updateInput handles one input's buttons.
func (s *State) updateInput(input *ui.Input, ctx *frame.Context, node flow.Node, row *inputRow) {
	for index, button := range row.choices {
		if button.Update(input) {
			s.giveInput(ctx, node, row.input.ID, row.values[index])
		}
	}
	if row.pick != nil {
		target := s.inputTarget(ctx.Docs, node, row.input)
		switch {
		case s.pickUpdate(row.pick, target.owner, input):
			s.pickOrCancel(ctx, target)

			return
		case input.Pressed && input.Cursor.In(row.place):
			s.openPlaces(row.place, target)

			return
		}
	}
	switch {
	case row.another.Update(input), input.Pressed && input.Cursor.In(row.box):
		s.openCallPicker(node, pickInput, row.input)
	case row.clear.Update(input):
		s.giveInput(ctx, node, row.input.ID, nil)
	}
}

// giveInput gives one input a value, or, given none, leaves it unfilled.
func (s *State) giveInput(ctx *frame.Context, node flow.Node, input ids.ULID, raw json.RawMessage) {
	inputs := maps.Clone(node.Inputs())
	if raw == nil {
		delete(inputs, input)
	} else {
		inputs[input] = raw
	}
	s.saveParam(ctx, node, flow.ParamInputs, inputs)
}

// inputTarget is a Call's place input, as a place control.
func (s *State) inputTarget(docs frame.Docs, node flow.Node, input worldstate.Declaration) placeTarget {
	current := floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}
	if value, err := input.Parse(node.Inputs()[input.ID]); err == nil && !value.Place.IsZero() {
		current = floor.AnchorRef{Floor: value.Place.Floor, Anchor: value.Place.Anchor}
	}
	nodeID := node.ID
	label := "the input " + input.Name + " of a Call in “" + s.name(docs) + "”"

	return s.newPlaceTarget(inputOwner(nodeID, input.ID), current, input.Name, label,
		func(ctx *frame.Context, ref floor.AnchorRef) {
			now, ok := s.currentNode(ctx.Docs, nodeID)
			if !ok {
				return
			}
			given := worldstate.NoValue()
			given.Type, given.Place = worldstate.TypePlace, worldstate.Place{Floor: ref.Floor, Anchor: ref.Anchor}
			s.giveInput(ctx, now, input.ID, given.Encode())
		})
}

// openCallPicker opens the value picker for another value an input is
// given, or a value an output is kept in, locked to its type.
func (s *State) openCallPicker(node flow.Node, prefix string, declared worldstate.Declaration) {
	title := "for the input " + declared.Name + " · " + declared.Type.Label() + " values only"
	if prefix == pickKeep {
		title = "to keep the output " + declared.Name + " in · " + declared.Type.Label() + " values only"
	}
	s.openValuePicker(node, prefix+declared.ID.String(), title, declared.Type, false)
}

// chooseForCall gives an input another value, or keeps an output in one,
// for the picker's param.
func (s *State) chooseForCall(ctx *frame.Context, node flow.Node, param string, ref flow.ValueRef) bool {
	if rest, ok := strings.CutPrefix(param, pickInput); ok {
		if input, err := ids.Parse(rest); err == nil {
			s.giveInput(ctx, node, input, flow.RefOperand(ref))
		}

		return true
	}
	if rest, ok := strings.CutPrefix(param, pickKeep); ok {
		if output, err := ids.Parse(rest); err == nil {
			keeps := maps.Clone(node.Keeps())
			keeps[output] = ref
			s.saveParam(ctx, node, flow.ParamKeep, keeps)
		}

		return true
	}

	return false
}

// applyCallField reads a typed input into the Call, or says what is wrong
// with it; empty leaves it unfilled. handled is false for another field.
func (s *State) applyCallField(node flow.Node, field *ui.TextField) (flow.Node, string, bool) {
	if s.form.call == nil {
		return node, "", false
	}
	for _, row := range s.form.call.inputs {
		if row.field != field {
			continue
		}
		inputs := maps.Clone(node.Inputs())
		typed := strings.TrimSpace(field.Field.Value())
		if typed == "" {
			delete(inputs, row.input.ID)
		} else {
			given, problem := typedInput(row.input, typed)
			if problem != "" {
				return node, problem, true
			}
			inputs[row.input.ID] = given
		}
		changed, err := node.WithParam(flow.ParamInputs, inputs)
		if err != nil {
			return node, err.Error(), true
		}

		return changed, "", true
	}

	return node, "", false
}

// typedInput reads an integer or text typed for an input.
func typedInput(input worldstate.Declaration, typed string) (json.RawMessage, string) {
	given := worldstate.NoValue()
	given.Type = input.Type
	if input.Type == worldstate.TypeText {
		given.Text = typed

		return given.Encode(), ""
	}
	number, err := strconv.Atoi(typed)
	if err != nil || number < input.Lowest() || number > input.Highest() {
		return nil, "Give " + input.Name + " a whole number from " + strconv.Itoa(input.Lowest()) + " to " +
			strconv.Itoa(input.Highest())
	}
	given.Integer = number

	return given.Encode(), ""
}

// openWorkflowMenu lists the project's other workflows to call.
func (s *State) openWorkflowMenu(docs frame.Docs) {
	var items []ui.MenuItem
	docs.View(project.IndexID(), func(document client.Document) {
		index, ok := document.Model.(*project.Index)
		if !ok {
			return
		}
		for _, entry := range index.OfFormat(flow.FormatName) {
			if entry.ID != s.document {
				items = append(items, ui.MenuItem{
					ID: menuWorkflow + entry.ID.String(), Label: entry.Name, Icon: ui.IconNone, Disabled: false,
					Danger: false, Rule: false,
				})
			}
		}
	})
	if len(items) == 0 {
		items = append(items, ui.MenuItem{ID: "", Label: "No other workflow yet", Icon: ui.IconNone, Disabled: true,
			Danger: false, Rule: false})
	}
	s.contextMenu.Items, s.contextMenu.Width = items, ui.MenuWidth(items, s.theme)
	s.contextMenu.Open(s.form.call.box, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
}

// chooseModule makes the selected Call call a workflow, starting its inputs
// and outputs afresh and keeping only its done line.
func (s *State) chooseModule(ctx *frame.Context, chosen string) {
	target, err := ids.Parse(strings.TrimPrefix(chosen, menuWorkflow))
	node, ok := s.selectedNode()
	if err != nil || !ok || node.Kind != flow.KindCall {
		return
	}
	ways := []flow.WayOut{}
	s.subscribe(target, ctx)
	ctx.Docs.View(target, func(document client.Document) {
		if workflowDoc, loaded := document.Model.(*workflowplugin.Document); loaded {
			ways = workflowDoc.Workflow.WaysOut()
		}
	})
	changed := node.Clone()
	for output := range changed.Next {
		if output != flow.OutputDone {
			changed = changed.WithLink(output, ids.ULID{})
		}
	}
	for name, value := range map[string]any{
		flow.ParamWorkflow: target.String(), flow.ParamInputs: map[string]any{}, flow.ParamKeep: map[string]any{},
		flow.ParamWays: ways,
	} {
		if changed, err = changed.WithParam(name, value); err != nil {
			ctx.Frame.Status(err.Error())

			return
		}
	}
	s.command(ctx, workflowplugin.ReplaceEdit, "", workflowplugin.ReplaceArgs{Node: changed}, nil)
}

// locateMenuItem finds an item of the open menu, a workflow to call also by
// its name, as "menu:workflow-named:<name>", since scripts do not know IDs.
func (s *State) locateMenuItem(value string) (image.Point, bool) {
	if name, named := strings.CutPrefix(value, "workflow-named:"); named {
		for _, item := range s.contextMenu.Items {
			if item.Label == name && strings.HasPrefix(item.ID, menuWorkflow) {
				return s.contextMenu.Locate(item.ID)
			}
		}
	}

	return s.contextMenu.Locate(value)
}

// drawCall draws the Call's inspector.
func (s *State) drawCall(screen *ebiten.Image, docs frame.Docs, node flow.Node, left int) {
	panel := s.form.call
	s.theme.DrawStrong(screen, "Workflow", left, panel.label, ui.ColorInk())
	ink := ui.ColorInk()
	if _, ok := node.Called(); !ok {
		ink = ui.ColorMuted()
	}
	s.drawPickBox(screen, panel.box, s.callTitle(node, docs), ink, s.contextMenu.IsOpen())
	for _, button := range panel.buttons() {
		if !button.Rect.Empty() {
			button.Draw(screen, s.theme)
		}
	}
	if panel.open.Disabled {
		return
	}
	s.theme.DrawStrong(screen, "Inputs", left, panel.heads["Inputs"], ui.ColorInk())
	if len(panel.inputs) == 0 {
		s.theme.DrawText(screen, "It has no inputs.", left, panel.heads["Inputs"]+ui.LineHeight, ui.ColorMuted())
	}
	given := node.Inputs()
	for _, row := range panel.inputs {
		s.drawInput(screen, docs, row, given, left)
	}
	s.theme.DrawStrong(screen, "Outputs", left, panel.heads["Outputs"], ui.ColorInk())
	if len(panel.keeps) == 0 {
		s.theme.DrawText(screen, "It has no outputs.", left, panel.heads["Outputs"]+ui.LineHeight, ui.ColorMuted())
	}
	keeps := node.Keeps()
	for _, row := range panel.keeps {
		s.theme.DrawText(screen, row.output.Name+" · "+row.output.Type.Label(), left, row.label, ui.ColorInk())
		label, kept := "Leave it", keeps[row.output.ID]
		if !kept.IsZero() {
			_, path, problem := s.resolve(docs, kept)
			label = path + problem
		}
		s.drawPickBox(screen, row.box, label, ui.ColorInk(), false)
	}
}

// drawInput draws one input: its name and type, and what it is given.
func (s *State) drawInput(screen *ebiten.Image, docs frame.Docs, row inputRow, given map[ids.ULID]json.RawMessage,
	left int,
) {
	s.theme.DrawMonoStrong(screen, row.input.Name, left, row.label, ui.ColorInk())
	s.drawTypeTag(screen, row.input.Type, left+s.theme.TextWidth(row.input.Name)+ui.Inset, row.label)
	if row.field != nil && !row.field.Rect.Empty() {
		row.field.Draw(screen)
	}
	raw, filled := given[row.input.ID]
	if !row.box.Empty() && row.place.Empty() {
		label := "Starts as " + row.input.Label(row.input.Zero())
		if filled {
			label = s.givenLabel(docs, row.input, raw)
		}
		s.drawPickBox(screen, row.box, label, ui.ColorInk(), false)
	}
	if !row.place.Empty() {
		s.drawPlaceInput(screen, docs, row, raw)
	}
}

// drawPlaceInput draws a place input's box: the place given, or the one it
// starts as, muted, and the zone and floor under it.
func (s *State) drawPlaceInput(screen *ebiten.Image, docs frame.Docs, row inputRow, raw json.RawMessage) {
	place := row.input.Zero().Place
	if value, err := row.input.Parse(raw); err == nil && raw != nil {
		place = value.Place
	}
	label, look, where := s.placeShown(docs, floor.AnchorRef{Floor: place.Floor, Anchor: place.Anchor})
	if raw == nil && look == places.Chosen {
		label, look = "Starts as "+label, places.Falling
	}
	owner := inputOwner(s.selected, row.input.ID)
	places.DrawBox(screen, s.theme, row.place, label, look, s.isLit(owner), s.placesOpen(owner))
	s.theme.DrawText(screen, where, row.place.Min.X, row.place.Max.Y+ui.Gap, ui.ColorMuted())
}

// drawTypeTag draws a value's type after its name, small and muted, with a
// place's ring.
func (s *State) drawTypeTag(screen *ebiten.Image, kind worldstate.Type, left, top int) {
	ink := ui.ColorMuted()
	if kind == worldstate.TypePlace {
		ui.DrawIcon(screen, ui.IconPlace, image.Pt(left, top+(ui.TextSize-ui.IconSize)/ui.Half+1))
		left += ui.IconSize + ui.Gap
		ink = ui.ColorSelect()
	}
	s.theme.DrawText(screen, kind.Label(), left, top, ink)
}

// locateCall finds a part of the Call's inspector.
func (panel *callPanel) locate(widget string) (image.Point, bool) {
	if widget == callWorkflowID && !panel.box.Empty() {
		return ui.Center(panel.box), true
	}
	for _, field := range panel.fields() {
		if field.ID == widget && !field.Rect.Empty() {
			return field.Center(), true
		}
	}
	for _, button := range panel.buttons() {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return panel.locateBox(widget)
}

// locateBox finds an input's place or value box, or an output's box.
func (panel *callPanel) locateBox(widget string) (image.Point, bool) {
	boxes := map[string]image.Rectangle{}
	for _, row := range panel.inputs {
		boxes[inputPrefix+row.input.Name+"-place"] = row.place
		boxes[inputPrefix+row.input.Name+"-value"] = row.box
	}
	for _, row := range panel.keeps {
		boxes[keepPrefix+row.output.Name] = row.box
	}
	if box, found := boxes[widget]; found && !box.Empty() {
		return ui.Center(box), true
	}

	return image.Pt(0, 0), false
}
