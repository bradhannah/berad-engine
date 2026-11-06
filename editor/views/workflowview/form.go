package workflowview

import (
	"encoding/json"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Inspector layout, in screen pixels.
const (
	formTop      = ui.Inset + ui.LineHeight + ui.TitleSize + ui.Inset
	fieldSpace   = 14
	rowSpace     = 6
	noteField    = 120
	timeWidth    = 64
	dashWidth    = 16
	removeWidth  = 76
	shareWidth   = 48
	maxTextField = 200
	hoursPerDay  = 24
	minuteDigits = 2
	clockDigits  = 4
)

// rangeRow is one time of a time switch: its fields and its Remove button.
type rangeRow struct {
	port   string
	from   *ui.TextField
	to     *ui.TextField
	remove *ui.Button
}

// optionRow is one option of a random choice: its name, its share and its
// Remove button.
type optionRow struct {
	port   string
	label  *ui.TextField
	share  *ui.TextField
	remove *ui.Button
}

// control edits one parameter, built from its declared type.
type control struct {
	param flow.Param
	label int
	// field edits text, tiles or a wait's amount; from and to a time range;
	// rows a list of times; pick and clear an anchor; choices a choice;
	// units and clocks a wait's unit and clock; options a random choice's
	// options.
	field     *ui.TextField
	from, to  *ui.TextField
	rows      []rangeRow
	addRange  *ui.Button
	pick      *ui.Button
	clear     *ui.Button
	anchor    image.Rectangle
	choices   []*ui.Button
	units     []*ui.Button
	clocks    []*ui.Button
	options   []optionRow
	addOption *ui.Button
	// cueBox shows a cue parameter's cue and listen plays it; search and
	// list make up its chooser, shown while open.
	cueBox image.Rectangle
	listen *ui.Button
	search *ui.TextField
	list   *ui.List
	open   bool
	// operands are what an operand's buttons set, in order, operandType
	// the type its value has, and unneeded is set when a Check value asks
	// about a limit, with nothing to compare with; a value parameter uses
	// cueBox, search, list and open as a cue does.
	operands    []json.RawMessage
	operandType worldstate.Type
	unneeded    bool
	// values are what each of a choice's buttons sets, in order. A hidden
	// control takes no room and is not drawn: Check value's comparison for
	// a type with nothing to choose, and an operand its comparison has no
	// use for. note marks an integer's comparison row, drawn as symbols,
	// for the words under it; sources are A number and Another value over
	// an integer's operand, and refers is set while it is another value.
	values  []string
	hidden  bool
	note    bool
	sources []*ui.Button
	refers  bool
	// another is set once Another value is clicked, until a value is picked
	// or A number is clicked, so the box to pick from shows meanwhile.
	another bool
}

// form is the inspector for the selected node. rows counts a time switch's
// times or a random choice's options, so the form is rebuilt when they
// change.
type form struct {
	node     ids.ULID
	kind     flow.Kind
	rows     int
	shape    string
	controls []control
	// call is a Call's own part of the inspector, nil for another node.
	call   *callPanel
	remove *ui.Button
	// bottom is where the last control ends, where the colour rows go.
	bottom int
	// slots are where a constant can stand in for each typed-in number,
	// and makeText makes a constant from words selected in a message.
	slots    map[string]*numberSlot
	makeText *ui.Button
	// words are the words last selected in the message's field, kept as
	// a click on Make a constant… takes the field's focus.
	words *selectedWords
}

// syncForm keeps the inspector built for the selected node and showing its
// saved values. A form rebuilt for the same node, because a time or an
// option was added or removed, keeps what was typed into it. Leaving a node
// whose cue was being heard stops it.
func (s *State) syncForm(ctx *frame.Context) {
	node, ok := s.selectedNode()
	if s.form != nil && (!ok || s.form.node != node.ID) && s.form.hasCue() {
		ctx.Sounds.Stop()
	}
	if !ok {
		if s.form != nil {
			s.form = nil
			s.focus = ui.Focus{}
		}

		return
	}
	drafts := s.rebuildForm(ctx.Docs, node)
	for index := range s.form.controls {
		s.setSaved(&s.form.controls[index], node, ctx)
	}
	if s.form.call != nil {
		s.form.call.setSaved(node)
	}
	for key, field := range s.form.keyed() {
		if draft, kept := drafts[key]; kept {
			field.Field.Restore(draft)
		}
	}
}

// rebuildForm builds the form again for a new node, or when the node's
// times, options or operands have changed, and returns what was typed into
// the form it replaces, for the same node.
func (s *State) rebuildForm(docs frame.Docs, node flow.Node) map[string]string {
	shape := s.formShape(docs, node)
	if s.form != nil && s.form.node == node.ID && s.form.rows == rowCount(node) && s.form.shape == shape {
		return nil
	}
	var drafts map[string]string
	if s.form != nil && s.form.node == node.ID {
		drafts = s.form.drafts()
	}
	s.form = s.buildForm(docs, node)
	s.form.shape = shape
	switch node.Kind {
	case flow.KindCall:
		s.form.call = s.newCallPanel(node)
	case flow.KindGoTo:
		shapeGoTo(s.form, node)
	}

	return drafts
}

// formShape says what a node's operands are made of, so the form is rebuilt
// when the value an operand follows changes type or choices.
func (s *State) formShape(docs frame.Docs, node flow.Node) string {
	kind, _ := flow.KindOf(node.Kind)
	var shape strings.Builder
	for _, param := range kind.Params {
		if param.Type == flow.TypeOperand {
			declared, found := s.operandDeclaration(docs, node, param.Name)
			shape.WriteString(param.Name + ":" + operandShape(node, declared, found) + ";")
		}
	}
	shape.WriteString(identityShape(docs, node))
	switch node.Kind {
	case flow.KindCall:
		shape.WriteString("call=" + s.callShape(node) + ";")
	case flow.KindGoTo:
		shape.WriteString("goes=" + choiceOf(node, flow.ParamGoes) + ";")
	}
	if node.Kind == flow.KindCheckValue {
		_, refers := flow.OperandRef(savedOperand(node, flow.ParamAgainst))
		shape.WriteString("compare=" + choiceOf(node, flow.ParamCompare) + ";refers=" + strconv.FormatBool(refers) + ";")
	}

	return shape.String()
}

// hasCue reports whether the form edits a cue.
func (f *form) hasCue() bool {
	return slices.ContainsFunc(f.controls, func(made control) bool { return made.param.Type == flow.TypeCue })
}

// fields lists the form's text fields, in order.
func (f *form) fields() []*ui.TextField {
	var fields []*ui.TextField
	for key := range f.keyedInOrder() {
		fields = append(fields, key)
	}

	return fields
}

// keyedInOrder yields each text field of the form, in order, with a key that
// stays the same when the form is rebuilt: the field's ID, or for a row of
// times or options, its output's name.
func (f *form) keyedInOrder() func(yield func(*ui.TextField, string) bool) {
	return func(yield func(*ui.TextField, string) bool) {
		for _, each := range f.keyedFields() {
			if !yield(each.field, each.key) {
				return
			}
		}
	}
}

// keyedField is a text field of the form and its lasting key.
type keyedField struct {
	field *ui.TextField
	key   string
}

// keyedFields lists the form's text fields, a Call's first, in order, each
// with its lasting key.
func (f *form) keyedFields() []keyedField {
	var listed []keyedField
	if f.call != nil {
		for _, field := range f.call.fields() {
			listed = append(listed, keyedField{field: field, key: field.ID})
		}
	}
	for _, made := range f.controls {
		for _, field := range []*ui.TextField{made.field, made.from, made.to} {
			if field != nil {
				listed = append(listed, keyedField{field: field, key: field.ID})
			}
		}
		for _, row := range made.rows {
			key := made.param.Name + "/" + row.port
			listed = append(listed, keyedField{field: row.from, key: key + "/from"}, keyedField{field: row.to, key: key + "/to"})
		}
		for _, option := range made.options {
			key := made.param.Name + "/" + option.port
			listed = append(listed, keyedField{field: option.label, key: key + "/label"})
			if option.share != nil {
				listed = append(listed, keyedField{field: option.share, key: key + "/share"})
			}
		}
	}

	return listed
}

// keyed maps each field's lasting key to the field.
func (f *form) keyed() map[string]*ui.TextField {
	fields := map[string]*ui.TextField{}
	for field, key := range f.keyedInOrder() {
		fields[key] = field
	}

	return fields
}

// drafts keeps the unsaved text of each field by its lasting key.
func (f *form) drafts() map[string]string {
	kept := map[string]string{}
	for field, key := range f.keyedInOrder() {
		if field.Field.Unsaved() {
			kept[key] = field.Field.Value()
		}
	}

	return kept
}

func (s *State) selectedNode() (flow.Node, bool) {
	if s.workflow == nil {
		var none flow.Node

		return none, false
	}

	return s.workflow.Node(s.selected)
}

// rowCount counts a time switch's times or a random choice's options.
func rowCount(node flow.Node) int {
	switch node.Kind {
	case flow.KindTimeSwitch:
		ranges, err := node.TimeRanges("ranges")
		if err != nil {
			return 0
		}

		return len(ranges)
	case flow.KindRandom:
		options, err := node.Shares("options")
		if err != nil {
			return 0
		}

		return len(options)
	case flow.KindTogether:
		lanes, err := node.Lanes()
		if err != nil {
			return 0
		}

		return len(lanes)
	default:
		return 0
	}
}

// newForm returns a node's form before its controls: with Delete node but
// for a Start, and Make a constant… for a message.
func newForm(node flow.Node, kind flow.Kind) *form {
	built := &form{
		node: node.ID, kind: kind, rows: rowCount(node), shape: "", controls: nil, call: nil, remove: nil, bottom: 0,
		slots: map[string]*numberSlot{}, makeText: nil, words: nil,
	}
	if node.Kind != flow.KindStart {
		built.remove = deleteNodeButton()
	}
	if hasMessage(node.Kind) {
		built.makeText = ui.NewButton(makeTextID, "Make a constant…")
	}

	return built
}

// buildForm makes a control for each of a node's parameters.
func (s *State) buildForm(docs frame.Docs, node flow.Node) *form {
	kind, _ := flow.KindOf(node.Kind)
	built := newForm(node, kind)
	s.focus = ui.Focus{}
	text := func(id string, multiline bool) *ui.TextField {
		field := ui.NewTextField(id, textedit.NewField("", multiline, maxTextField), multiline, s.theme)
		s.focus.Register(field)

		return field
	}
	for _, param := range kind.Params {
		made := control{param: param, label: 0, field: nil, from: nil, to: nil, rows: nil, addRange: nil, pick: nil,
			clear: nil, anchor: ui.Zero(), choices: nil, units: nil, clocks: nil, options: nil, addOption: nil,
			cueBox: ui.Zero(), listen: nil, search: nil, list: nil, open: false, operands: nil, operandType: "",
			unneeded: false, values: nil, hidden: false, note: false, sources: nil, refers: false, another: false}
		switch param.Type {
		case flow.TypeTiles, flow.TypeSeconds:
			made.field = text("param-"+param.Name, false)
		case flow.TypeChoice:
			s.newChoiceControl(docs, node, &made)
		case flow.TypeDuration:
			made.field = text("param-"+param.Name, false)
			made.units, made.clocks = durationButtons(param.Name)
		case flow.TypeShares:
			newOptionRows(&made, built.rows, text)
		case flow.TypeLanes:
			newLaneRows(&made, built.rows, text)
		case flow.TypeNPC:
			newIdentityControl(docs, &made)
		case flow.TypeText, flow.TypeMessage:
			made.field, made.pick = text("param-"+param.Name, node.Kind == flow.KindNote), insertButton(param.Type)
		case flow.TypeTimeRange:
			made.from, made.to = text(param.Name+"-from", false), text(param.Name+"-to", false)
		case flow.TypeTimeRanges:
			for index := range built.rows {
				prefix := param.Name + "-" + strconv.Itoa(index+1)
				made.rows = append(made.rows, rangeRow{
					port: "", from: text(prefix+"-from", false), to: text(prefix+"-to", false),
					remove: ui.NewButton(prefix+"-remove", "Remove"),
				})
			}
			made.addRange = ui.NewButton("add-time", "Add time")
		case flow.TypeAnchor, flow.TypeCue, flow.TypeValue, flow.TypeOperand, flow.TypeSignal:
			s.newReferenceControl(docs, node, &made)
		case flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort:
			// A call's and a Return's own parameters have no row: the
			// Call's inspector and the Return's name stand for them.
			made.hidden = true
		}
		built.controls = append(built.controls, made)
	}

	return built
}

// controlShown reports whether a control takes room in the inspector: not
// hidden, and not a Seconds setting the selected node doesn't use now, as
// a Say shown for as long as it takes to read.
func (s *State) controlShown(made *control) bool {
	node, _ := s.selectedNode()

	return !made.hidden && (made.param.Type != flow.TypeSeconds || node.UsesSeconds()) && sendShows(node, made)
}

// unitOf is the unit written in a whole-number field.
func unitOf(paramType flow.ParamType) string {
	if paramType == flow.TypeSeconds {
		return "seconds"
	}
	if paramType == flow.TypeTiles {
		return "tiles"
	}

	return ""
}

// newReferenceControl makes the widgets of a parameter that refers to
// something: an anchor, a cue, a value, or an operand that follows one.
func (s *State) newReferenceControl(docs frame.Docs, node flow.Node, made *control) {
	switch made.param.Type {
	case flow.TypeAnchor:
		made.pick, made.clear = anchorButtons()
	case flow.TypeCue:
		s.newCueControl(made)
	case flow.TypeValue:
		s.newValueControl(made)
	case flow.TypeOperand:
		declared, found := s.operandDeclaration(docs, node, made.param.Name)
		s.newOperandControl(made, node, declared, found)
		if node.Kind == flow.KindCheckValue {
			s.shapeCheckOperand(node, made, declared, found)
		}
		if node.Kind == flow.KindSetValue && found && declared.Type == worldstate.TypeText {
			s.newOperandSources(made, node)
			made.sources[0].Label = "Text"
		}
	case flow.TypeTiles, flow.TypeSeconds, flow.TypeChoice, flow.TypeDuration, flow.TypeShares, flow.TypeText,
		flow.TypeTimeRange,
		flow.TypeTimeRanges, flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort,
		flow.TypeMessage, flow.TypeLanes, flow.TypeSignal, flow.TypeNPC:
	}
}

// anchorButtons returns an anchor parameter's Pick on map, the usual choice,
// and Clear.
func anchorButtons() (*ui.Button, *ui.Button) {
	pick := ui.NewButton("pick", "Pick on map")
	pick.Primary = true

	return pick, ui.NewButton("clear", "Clear")
}

// deleteNodeButton returns the inspector's Delete node button, drawn as a
// destructive action.
func deleteNodeButton() *ui.Button {
	button := ui.NewButton("delete-node", "Delete node")
	button.Danger = true

	return button
}

// setSaved shows a parameter's saved value in its fields.
func (s *State) setSaved(made *control, node flow.Node, ctx *frame.Context) {
	switch made.param.Type {
	case flow.TypeTiles, flow.TypeSeconds:
		if number, err := node.Number(made.param.Name); err == nil {
			made.field.Field.SetSaved(strconv.Itoa(number))
		}
	case flow.TypeChoice, flow.TypeDuration, flow.TypeShares, flow.TypeLanes, flow.TypeSignal, flow.TypeNPC:
		setSavedChoices(made, node)
	case flow.TypeText:
		if value, err := node.Text(made.param.Name); err == nil {
			made.field.Field.SetSaved(value)
		}
	case flow.TypeTimeRange:
		if span, err := node.TimeRange(made.param.Name); err == nil {
			made.from.Field.SetSaved(flow.FormatTime(span.From))
			made.to.Field.SetSaved(flow.FormatTime(span.To))
		}
	case flow.TypeTimeRanges:
		spans, err := node.TimeRanges(made.param.Name)
		if err != nil || len(spans) != len(made.rows) {
			return
		}
		for index, span := range spans {
			made.rows[index].port = span.Port
			made.rows[index].from.Field.SetSaved(flow.FormatTime(span.From))
			made.rows[index].to.Field.SetSaved(flow.FormatTime(span.To))
		}
	case flow.TypeAnchor, flow.TypeCue, flow.TypeMessage:
		s.setSavedReference(made, node, ctx)
	case flow.TypeOperand:
		setSavedOperand(made, node)
	case flow.TypeValue, flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort:
	}
}

// setSavedReference loads what a reference parameter names: an anchor's
// floor, or a message's values by their paths. A cue's name comes with the
// audio library.
func (s *State) setSavedReference(made *control, node flow.Node, ctx *frame.Context) {
	if made.param.Type == flow.TypeMessage {
		if segments, err := node.Message(); err == nil {
			made.field.Field.SetSaved(s.messageText(ctx.Docs, segments))
		}

		return
	}
	if made.param.Type != flow.TypeAnchor {
		return
	}
	if destination, err := node.Anchor(made.param.Name); err == nil && !destination.IsZero() {
		s.subscribe(destination.Floor, ctx)
	}
}

// notesTop is where the selected node's problem and hint start, above
// Delete node.
func (s *State) notesTop(panel image.Rectangle, width int) int {
	top := panel.Max.Y - ui.Inset - ui.ButtonH - ui.Half*ui.Gap
	if node, ok := s.selectedNode(); ok {
		top -= s.noteLines(node, width) * ui.LineHeight
	}

	return top
}

// layoutForm places the inspector's fields and buttons.
func (s *State) layoutForm(docs frame.Docs) {
	if s.form == nil {
		return
	}
	panel := frame.Inspector(s.size)
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	top := panel.Min.Y + formTop
	footer := s.notesTop(panel, width)
	for index := range s.form.controls {
		made := &s.form.controls[index]
		if !s.controlShown(made) {
			made.clearRects()

			continue
		}
		made.label = top
		top += ui.LineHeight
		switch made.param.Type {
		case flow.TypeChoice, flow.TypeDuration, flow.TypeShares, flow.TypeLanes, flow.TypeNPC:
			top = s.layoutChoices(made, left, width, top)
		case flow.TypeTiles, flow.TypeSeconds, flow.TypeText:
			top = layoutField(made, left, width, top)
		case flow.TypeTimeRange:
			placeTimes(made.from, made.to, left, top)
			top += ui.ButtonH + fieldSpace
		case flow.TypeTimeRanges:
			for _, row := range made.rows {
				placeTimes(row.from, row.to, left, top)
				row.remove.Rect = image.Rect(left+width-removeWidth, top, left+width, top+ui.ButtonH)
				top += ui.ButtonH + rowSpace
			}
			made.addRange.Rect = image.Rect(left, top, left+width/ui.Half, top+ui.ButtonH)
			top += ui.ButtonH + fieldSpace
		case flow.TypeAnchor, flow.TypeCue, flow.TypeValue, flow.TypeOperand, flow.TypeMessage, flow.TypeSignal:
			top = s.layoutReference(docs, made, image.Rect(left, top, left+width, footer))
		case flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort:
		}
	}
	s.layoutSlots()
	if node, ok := s.selectedNode(); ok && s.form.call != nil {
		top = s.layoutCall(s.form.call, left, width, top)
		s.form.call.hideRefControls(node)
	}
	s.form.bottom = top
	s.layoutFooter(panel.Max.Y-ui.Inset, left, width)
}

// layoutField places a number's or a text's field, and returns where the
// next control goes.
func layoutField(made *control, left, width, top int) int {
	height := ui.ButtonH
	if made.field.Multiline {
		height = noteField
	}
	made.field.Rect = image.Rect(left, top, left+width, top+height)

	return top + height + fieldSpace
}

// layoutReference places a parameter that refers to something, from the
// top of room down to no lower than its bottom, and returns where the next
// control goes.
func (s *State) layoutReference(docs frame.Docs, made *control, room image.Rectangle) int {
	left, width, top := room.Min.X, room.Dx(), room.Min.Y
	switch made.param.Type {
	case flow.TypeAnchor:
		made.anchor = image.Rect(left, top, left+width, top+ui.ButtonH)
		top += ui.ButtonH + rowSpace + ui.LineHeight
		half := (width - rowSpace) / ui.Half
		made.pick.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
		made.clear.Rect = image.Rect(left+half+rowSpace, top, left+width, top+ui.ButtonH)

		return top + ui.ButtonH + fieldSpace
	case flow.TypeCue:
		return s.layoutCue(made, left, width, top, room.Max.Y)
	case flow.TypeValue, flow.TypeSignal:
		return layoutValue(made, left, width, top)
	case flow.TypeOperand:
		return s.layoutOperand(made, left, width, top)
	case flow.TypeMessage:
		return s.layoutMessage(docs, made, left, width, top)
	case flow.TypeTiles, flow.TypeSeconds, flow.TypeChoice, flow.TypeDuration, flow.TypeShares, flow.TypeText,
		flow.TypeTimeRange,
		flow.TypeTimeRanges, flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort,
		flow.TypeLanes, flow.TypeNPC:
	}

	return top
}

// layoutFooter places Delete node at the foot of the inspector and the save
// bar above it, when the node has fields to type into.
func (s *State) layoutFooter(bottom, left, width int) {
	if s.form.remove != nil {
		s.form.remove.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
		bottom = s.form.remove.Rect.Min.Y - ui.Half*ui.Gap
	}
	s.save.Rect = ui.Zero()
	if len(s.form.fields()) > 0 {
		s.save.Rect = image.Rect(left, bottom-ui.SaveBarHeight(width), left+width, bottom)
	}
}

func placeTimes(from, to *ui.TextField, left, top int) {
	from.Rect = image.Rect(left, top, left+timeWidth, top+ui.ButtonH)
	toLeft := left + timeWidth + dashWidth
	to.Rect = image.Rect(toLeft, top, toLeft+timeWidth, top+ui.ButtonH)
}

// updateForm handles the inspector's buttons.
func (s *State) updateForm(input *ui.Input, ctx *frame.Context) {
	if s.form == nil {
		return
	}
	s.save.Unsaved = s.Unsaved(ctx.Docs)
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)

		return
	case ui.RevertID:
		s.discardForm()

		return
	}
	if s.form.remove != nil && s.form.remove.Update(input) {
		s.removeSelected(ctx)

		return
	}
	node, ok := s.selectedNode()
	if !ok || s.updateSlots(input, ctx, node) || s.updateMakeText(input, ctx, node) {
		return
	}
	for index := range s.form.controls {
		s.updateControl(input, ctx, &s.form.controls[index], node)
	}
	if s.form.call != nil {
		s.updateCall(input, ctx, node)
	}
}

// updateControl handles one parameter's buttons and chooser.
func (s *State) updateControl(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	switch made.param.Type {
	case flow.TypeTimeRanges:
		s.updateRanges(input, ctx, made, node)
	case flow.TypeAnchor:
		s.updateAnchor(input, ctx, made, node)
	case flow.TypeChoice, flow.TypeDuration, flow.TypeNPC:
		s.updateChoices(input, ctx, made, node)
	case flow.TypeSignal:
		s.updateSignalBox(input, ctx, made, node)
	case flow.TypeShares:
		s.updateOptions(input, ctx, made, node)
	case flow.TypeLanes:
		s.updateLanes(input, ctx, made, node)
	case flow.TypeCue:
		s.updateCue(input, ctx, made, node)
	case flow.TypeValue:
		s.updateValue(input, ctx, made, node)
	case flow.TypeOperand:
		s.updateOperand(input, ctx, made, node)
	case flow.TypeMessage:
		if made.pick.Update(input) {
			s.openValuePicker(node, made.param.Name, "to put in the message, as its text", "", false)
		}
	case flow.TypeTiles, flow.TypeSeconds, flow.TypeText, flow.TypeTimeRange, flow.TypeWorkflow,
		flow.TypeInputs, flow.TypeKeeps,
		flow.TypeWays, flow.TypePort:
	}
}

// updateRanges adds a time after the last one, or removes one with the
// output it led to.
func (s *State) updateRanges(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	spans, err := node.TimeRanges(made.param.Name)
	if err != nil {
		return
	}
	if made.addRange.Update(input) {
		if len(spans) >= flow.MaxRanges {
			ctx.Frame.Status(fmt.Sprintf("A time switch has at most %d times", flow.MaxRanges))

			return
		}
		from := 0
		if len(spans) > 0 {
			from = spans[len(spans)-1].To
		}
		spans = append(spans, flow.TimeRange{Port: newPort(spans), From: from, To: (from + hour) % flow.MinutesPerDay})
		s.saveParam(ctx, node, made.param.Name, spans)

		return
	}
	for index, row := range made.rows {
		if !row.remove.Update(input) || index >= len(spans) {
			continue
		}
		changed := node.WithLink(spans[index].Port, ids.ULID{})
		spans = slices.Delete(spans, index, index+1)
		s.saveParam(ctx, changed, made.param.Name, spans)

		return
	}
}

const hour = 60

// newPort returns an output name no time of a switch uses yet.
func newPort(spans []flow.TimeRange) string {
	for number := 1; ; number++ {
		name := "r" + strconv.Itoa(number)
		if !slices.ContainsFunc(spans, func(span flow.TimeRange) bool { return span.Port == name }) {
			return name
		}
	}
}

// updateAnchor picks the destination on a map, chooses it from the list of
// places its box opens, or clears it.
func (s *State) updateAnchor(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	target := s.anchorTarget(ctx.Docs, node, made.param.Name)
	switch {
	case s.pickUpdate(made.pick, target.owner, input):
		s.pickOrCancel(ctx, target)
	case input.Pressed && input.Cursor.In(made.anchor):
		s.openPlaces(ctx.Docs, made.anchor, target)
	case made.clear.Update(input):
		s.saveParam(ctx, node, made.param.Name, floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}})
	}
}

// anchorTarget is a Go to's place on a map, as a place control.
func (s *State) anchorTarget(docs frame.Docs, node flow.Node, param string) placeTarget {
	current, err := node.Anchor(param)
	if err != nil {
		current = floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}
	}
	nodeID := node.ID

	return s.goToExtras(s.newPlaceTarget(anchorOwner(nodeID, param), current, "Go to",
		"a Go to in “"+s.name(docs)+"”", func(ctx *frame.Context, ref floor.AnchorRef) {
			if now, ok := s.currentNode(ctx.Docs, nodeID); ok {
				s.saveParam(ctx, now, param, ref)
			}
		}), nodeID)
}

// currentNode reads a node from the window's copy of the workflow as it is
// now, for replies that arrive between frames.
func (s *State) currentNode(docs frame.Docs, nodeID ids.ULID) (flow.Node, bool) {
	var found flow.Node
	loaded := false
	docs.View(s.document, func(document client.Document) {
		if workflowDoc, isWorkflow := document.Model.(*workflowplugin.Document); isWorkflow {
			found, loaded = workflowDoc.Workflow.Node(nodeID)
		}
	})

	return found, loaded
}

// Unsaved counts the selected node's typed changes not saved yet.
func (s *State) Unsaved(frame.Docs) int {
	if s.form == nil {
		if _, ok := s.selectedLocal(); ok && s.localsActive() {
			return s.locals.form.Unsaved()
		}

		return 0
	}

	return frame.UnsavedFields(s.form.fields()...)
}

// Save saves every typed change to the selected node as one changed node: a
// single command, so one undo step. If a field cannot be read, nothing is
// saved, that field takes focus and the status says why.
func (s *State) Save(ctx *frame.Context) bool {
	if _, local := s.selectedLocal(); local && s.localsActive() {
		return s.saveLocal(ctx)
	}
	node, ok := s.selectedNode()
	if !ok || s.form == nil {
		return true
	}
	changed, saving := node, []*ui.TextField{}
	for _, field := range s.form.fields() {
		if !field.Field.Unsaved() {
			continue
		}
		next, problem := s.applyField(ctx.Docs, changed, field)
		if problem != "" {
			s.focus.Set(field)
			ctx.Frame.Status(problem)

			return false
		}
		changed, saving = next, append(saving, field)
	}
	if len(saving) == 0 {
		s.focus.Set(nil)

		return true
	}
	for _, field := range saving {
		field.Field.Commit()
	}
	data, err := json.Marshal(workflowplugin.ReplaceArgs{Node: changed})
	if err != nil {
		ctx.Frame.Status(err.Error())

		return false
	}
	command := protocol.Command{Document: s.document, Edit: workflowplugin.ReplaceEdit, Gesture: "", Args: data}
	ctx.Requests.Command(command, func(result protocol.Result, err error) {
		if err != nil {
			for _, field := range saving {
				field.Field.Failed()
			}
			ctx.Frame.Report("")(result, err)

			return
		}
		s.focus.Set(nil)
	})

	return true
}

// applyField reads a field's typed value into a node, or says what is wrong
// with it.
func (s *State) applyField(docs frame.Docs, node flow.Node, field *ui.TextField) (flow.Node, string) {
	if changed, problem, handled := s.applyCallField(node, field); handled {
		return changed, problem
	}
	if changed, problem, handled := s.applyMessageField(docs, node, field); handled {
		return changed, problem
	}
	for index := range s.form.controls {
		made := &s.form.controls[index]
		value, handled, problem := parseFor(made, field, field.Field.Value(), node)
		if !handled {
			continue
		}
		if problem != "" {
			return node, problem
		}
		if node.Kind == flow.KindReturn && made.param.Name == flow.ParamWayName {
			name, named := value.(string)
			if !named {
				return node, problemWayName
			}

			return s.nameWayOut(node, name)
		}
		changed, err := node.WithParam(made.param.Name, value)
		if err != nil {
			return node, err.Error()
		}

		return changed, ""
	}

	return node, ""
}

// UnsavedName names the node with unsaved changes and its workflow, such as
// "Wait in Workflow 1".
func (s *State) UnsavedName(docs frame.Docs) string {
	return s.formName() + " in " + s.name(docs)
}

// Leave asks about unsaved typed changes before then, such as closing the
// window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), then)
}

// formName names the node the form edits, for the guard.
func (s *State) formName() string {
	if local, ok := s.selectedLocal(); ok && s.form == nil {
		return localPath(local)
	}
	if s.form == nil {
		return "this node"
	}

	return s.form.kind.Label
}

// discardForm drops the selected node's typed changes.
func (s *State) discardForm() {
	s.focus.Set(nil)
	if s.form == nil {
		s.locals.form.Revert()

		return
	}
	for _, field := range s.form.fields() {
		field.Field.Revert()
	}
}

// guardSelection asks about unsaved typed changes before the inspector moves
// to another node, a line or nothing: the selection goes back until the
// author decides, and moves on after Save or Don't save.
func (s *State) guardSelection(ctx *frame.Context) {
	if s.form == nil || s.workflow == nil || s.selected == s.form.node {
		return
	}
	unsaved := s.Unsaved(ctx.Docs)
	if _, exists := s.workflow.Node(s.form.node); unsaved == 0 || !exists {
		return
	}
	wanted, wantedLink := s.selected, s.selectedLink
	s.selected, s.selectedLink = s.form.node, noRoute()
	s.drag, s.link, s.adding, s.resize, s.bending = nil, nil, nil, nil, nil
	s.focus.Set(nil)
	s.guard.Ask(unsaved, s.formName(), func() { s.selected, s.selectedLink = wanted, wantedLink })
}

// parseFor reads a typed value from a field of a control: the new parameter
// value, whether the field belongs to the control, and what is wrong with
// the text.
func parseFor(made *control, field *ui.TextField, value string, node flow.Node) (any, bool, string) {
	switch made.param.Type {
	case flow.TypeTiles, flow.TypeSeconds:
		if field != made.field {
			return nil, false, ""
		}

		limit := flow.MaxTiles
		if made.param.Type == flow.TypeSeconds {
			limit = flow.MaxSeconds
		}

		return parseWhole(value, 1, limit)
	case flow.TypeDuration:
		return parseAmount(made, field, value, node)
	case flow.TypeShares:
		return parseOptionField(made, field, value, node)
	case flow.TypeLanes:
		return parseLaneField(made, field, value, node)
	case flow.TypeChoice, flow.TypeSignal, flow.TypeNPC:
		return nil, false, ""
	case flow.TypeText:
		if field != made.field {
			return nil, false, ""
		}

		return strings.TrimSpace(value), true, ""
	case flow.TypeTimeRange:
		return parseRangeField(made, field, value, node)
	case flow.TypeTimeRanges:
		return parseRangesField(made, field, value, node)
	case flow.TypeOperand:
		return parseOperand(made, field, value)
	case flow.TypeAnchor, flow.TypeCue, flow.TypeValue, flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps,
		flow.TypeWays, flow.TypePort, flow.TypeMessage:
		// A message's values need the project's paths, so applyField
		// reads it.
	}

	return nil, false, ""
}

// parseRangeField reads the start or end of a time range.
func parseRangeField(made *control, field *ui.TextField, value string, node flow.Node) (any, bool, string) {
	if field != made.from && field != made.to {
		return nil, false, ""
	}
	span, err := node.TimeRange(made.param.Name)
	if err != nil {
		return nil, true, err.Error()
	}
	changed, problem := timeInto(span, value, field == made.from)
	if problem != "" {
		return nil, true, problem
	}

	return changed, true, ""
}

// parseRangesField reads the start or end of one time of a time switch.
func parseRangesField(made *control, field *ui.TextField, value string, node flow.Node) (any, bool, string) {
	spans, err := node.TimeRanges(made.param.Name)
	if err != nil {
		return nil, false, ""
	}
	for index, row := range made.rows {
		if (field != row.from && field != row.to) || index >= len(spans) {
			continue
		}
		changed, problem := timeInto(spans[index], value, field == row.from)
		if problem != "" {
			return nil, true, problem
		}
		spans[index] = changed

		return spans, true, ""
	}

	return nil, false, ""
}

// parseWhole reads a whole number from low to high.
func parseWhole(value string, low, high int) (any, bool, string) {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || number < low || number > high {
		return nil, true, fmt.Sprintf("Enter a whole number from %d to %d", low, high)
	}

	return number, true, ""
}

// timeInto sets the start or end of a range from typed text, or says what
// is wrong with it.
func timeInto(span flow.TimeRange, value string, start bool) (flow.TimeRange, string) {
	minute, ok := parseTime(value)
	if !ok {
		return span, "Enter a time of day such as 09:30"
	}
	if start {
		span.From = minute
	} else {
		span.To = minute
	}
	if span.From == span.To {
		return span, "A time needs to end after it starts"
	}

	return span, ""
}

// parseTime reads "9", "9:30", "09:30" or "0930" as minutes from midnight.
func parseTime(value string) (int, bool) {
	value = strings.TrimSpace(value)
	hours, minutes, found := strings.Cut(value, ":")
	if !found {
		hours, minutes = value, "0"
		if len(value) == clockDigits {
			hours, minutes = value[:minuteDigits], value[minuteDigits:]
		}
	}
	hour, hourErr := strconv.Atoi(hours)
	minute, minuteErr := strconv.Atoi(minutes)
	if hourErr != nil || minuteErr != nil || hour < 0 || hour >= hoursPerDay || minute < 0 || minute >= 60 {
		return 0, false
	}

	return hour*60 + minute, true
}

// saveParam sends a node with one parameter changed, for the inspector's
// buttons, which apply at once.
func (s *State) saveParam(ctx *frame.Context, node flow.Node, name string, value any) {
	changed, err := node.WithParam(name, value)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	s.command(ctx, workflowplugin.ReplaceEdit, "", workflowplugin.ReplaceArgs{Node: changed}, nil)
}

// floorName returns a floor's name, or "" for a floor the project no longer
// has.
func (s *State) floorName(docs frame.Docs, floorID ids.ULID) string {
	name := ""
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(floorID, "")
		}
	})

	return name
}

// anchorLabel describes an anchor parameter's value: the anchor's name and
// where it is, or what is wrong with the reference.
func (s *State) anchorLabel(docs frame.Docs, ref floor.AnchorRef) (string, string) {
	if ref.IsZero() {
		return "No destination yet", ""
	}
	floorName := s.floorName(docs, ref.Floor)
	if floorName == "" {
		return "Broken: its floor was deleted", ""
	}
	name, detail := "", ""
	loaded := docs.View(ref.Floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			if anchor, _, found := floorDoc.Floor.Anchor(ref.Anchor); found {
				name, detail = anchor.Name, fmt.Sprintf("Anchor · %s · %d, %d", floorName, anchor.Col, anchor.Row)
			}
		}
	})
	switch {
	case !loaded:
		return "…", floorName
	case name == "":
		return "Broken: the anchor was deleted", floorName
	default:
		return name, detail
	}
}
