package workflowview

import (
	"encoding/json"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A value parameter shows the chosen value's path, such as npc.mood;
// clicking it opens a chooser under it: a search box and the values the
// node may use, grouped by whose they are. Typing a name no value has
// offers to declare it. An operand follows its value's type: a number to
// type, yes or no, or one of a list's choices.
const (
	maxValueSearch = 40
	// Row IDs: a value's row is its reference, encoded; a heading's starts
	// with valueHeading; a row that declares a value starts with
	// declarePrefix and names its scope.
	valueHeading  = "values:"
	declarePrefix = "declare:"
	// locateValue names a chooser row for input scripts, "valuerow:<path>",
	// and locateDeclare a row that declares one, "declare:<scope>".
	locateValue   = "valuerow"
	locateDeclare = "declare"
	// valueBoxPrefix names a value parameter's box, "ui:pick-<param>".
	valueBoxPrefix = "pick-"
	operandPrefix  = "operand-"
)

// Problems the value nodes can have.
const (
	problemNoValue       = "No value chosen yet."
	problemValueGone     = "Its value was deleted. Choose another, or undo the delete."
	problemReadOnly      = "It can read another NPC's values, but never set them."
	problemNPCValue      = "An NPC value cannot be set or checked here."
	problemOperand       = "Choose what it sets the value to, or compares it with."
	problemAddNumber     = "Only a number can be added to."
	problemCompareNumber = "Only a number can be at least or below."
	problemNoKeep        = "Choose a local NPC value to keep what it finds in."
	problemWhere         = "Only an NPC's own value can be looked for."
	problemNPCGone       = "Its NPC was deleted."
	problemFoundGone     = "The local value holding its NPC was deleted."
)

// valueOption is one value a parameter can choose: its reference, path and
// declaration, under a heading.
type valueOption struct {
	ref      flow.ValueRef
	path     string
	declared worldstate.Declaration
	heading  string
}

// namedNPC is an NPC a workflow can name: its ID, display name and path
// name.
type namedNPC struct {
	id   ids.ULID
	name string
	path string
}

// worldState reads the project's world state, or an empty one before it
// arrives.
func (s *State) worldState(docs frame.Docs) *worldstate.Document {
	found := worldstate.NewDocument()
	docs.View(worldstate.ID(), func(document client.Document) {
		if stateDoc, ok := document.Model.(*stateplugin.Document); ok {
			found = stateDoc.State.Clone()
		}
	})

	return found
}

// namedNPCs lists the project's NPCs that have a path name, read one at a
// time.
func (s *State) namedNPCs(docs frame.Docs) []namedNPC {
	var entries []project.Entry
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = index.OfFormat(npc.FormatName)
		}
	})
	var named []namedNPC
	for _, entry := range entries {
		docs.View(entry.ID, func(document client.Document) {
			if npcDoc, ok := document.Model.(*npcplugin.Document); ok && npcDoc.NPC.Path != "" {
				named = append(named, namedNPC{id: entry.ID, name: entry.Name, path: npcDoc.NPC.Path})
			}
		})
	}

	return named
}

// watchValues keeps the world state and every NPC loaded, for value paths
// and problems.
func (s *State) watchValues(ctx *frame.Context) {
	s.subscribe(worldstate.ID(), ctx)
	var entries []project.Entry
	ctx.Docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = index.OfFormat(npc.FormatName)
		}
	})
	for _, entry := range entries {
		s.subscribe(entry.ID, ctx)
	}
}

// resolve finds what a reference refers to: its declaration and path, or
// what is wrong with it.
func (s *State) resolve(docs frame.Docs, ref flow.ValueRef) (worldstate.Declaration, string, string) {
	var none worldstate.Declaration
	if ref.IsZero() {
		return none, "", problemNoValue
	}
	if ref.Scope == worldstate.ScopeLocal {
		declared, ok := s.workflow.Locals().Value(ref.Value)
		if !ok {
			return none, "", problemValueGone
		}

		return declared, declared.Path(), ""
	}
	declared, ok := s.worldState(docs).Values.Value(ref.Value)
	if !ok || declared.Scope != ref.Scope {
		return none, "", problemValueGone
	}
	switch {
	case !ref.Found.IsZero():
		holder, held := s.workflow.Locals().Value(ref.Found)
		if !held {
			return declared, "", problemFoundGone
		}

		return declared, holder.Path() + "." + declared.Name, ""
	case !ref.NPC.IsZero():
		index := slices.IndexFunc(s.namedNPCs(docs), func(each namedNPC) bool { return each.id == ref.NPC })
		if index < 0 {
			return declared, "", problemNPCGone
		}

		return declared, "npc." + s.namedNPCs(docs)[index].path + "." + declared.Name, ""
	}

	return declared, declared.Path(), ""
}

// valueRole says which values a parameter may choose.
type valueRole int

const (
	roleSet valueRole = iota
	roleCheck
	roleWhere
	roleKeep
)

func roleOf(kind, param string) valueRole {
	switch {
	case kind == flow.KindCheckValue:
		return roleCheck
	case kind == flow.KindFindNPC && param == "into":
		return roleKeep
	case kind == flow.KindFindNPC:
		return roleWhere
	}

	return roleSet
}

// valueOptions lists the values a parameter may choose, in the order the
// chooser shows them.
func (s *State) valueOptions(docs frame.Docs, role valueRole) []valueOption {
	var options []valueOption
	add := func(heading string, ref flow.ValueRef, path string, declared worldstate.Declaration) {
		options = append(options, valueOption{ref: ref, path: path, declared: declared, heading: heading})
	}
	ref := func(declared worldstate.Declaration) flow.ValueRef {
		return flow.ValueRef{Scope: declared.Scope, Value: declared.ID, NPC: ids.ULID{}, Found: ids.ULID{}}
	}
	locals := s.workflow.Locals().Values()
	for _, local := range locals {
		if (role == roleKeep) == (local.Type == worldstate.TypeNPC) && role != roleWhere {
			add("LOCAL", ref(local), local.Path(), local)
		}
	}
	if role == roleKeep {
		return options
	}
	shared := s.worldState(docs).Values.Values()
	for _, scope := range []worldstate.Scope{worldstate.ScopeNPC, worldstate.ScopeZone, worldstate.ScopeWorld} {
		if role == roleWhere && scope != worldstate.ScopeNPC {
			continue
		}
		for _, declared := range shared {
			if declared.Scope == scope {
				add(scopeHeading(scope, role), ref(declared), declared.Path(), declared)
			}
		}
	}
	if role != roleCheck {
		return options
	}

	return append(options, s.othersOptions(docs, locals, shared)...)
}

// othersOptions lists other NPCs' values a Check value can read: those of
// each NPC a local value holds, and of each NPC with a path name.
func (s *State) othersOptions(docs frame.Docs, locals, shared []worldstate.Declaration) []valueOption {
	var options []valueOption
	for _, holder := range locals {
		if holder.Type != worldstate.TypeNPC {
			continue
		}
		for _, declared := range shared {
			if declared.Scope == worldstate.ScopeNPC {
				options = append(options, valueOption{
					ref:  flow.ValueRef{Scope: worldstate.ScopeNPC, Value: declared.ID, NPC: ids.ULID{}, Found: holder.ID},
					path: holder.Path() + "." + declared.Name, declared: declared, heading: "THE NPC IN " + holder.Path(),
				})
			}
		}
	}
	for _, named := range s.namedNPCs(docs) {
		for _, declared := range shared {
			if declared.Scope == worldstate.ScopeNPC {
				options = append(options, valueOption{
					ref:  flow.ValueRef{Scope: worldstate.ScopeNPC, Value: declared.ID, NPC: named.id, Found: ids.ULID{}},
					path: "npc." + named.path + "." + declared.Name, declared: declared, heading: strings.ToUpper(named.name),
				})
			}
		}
	}

	return options
}

func scopeHeading(scope worldstate.Scope, role valueRole) string {
	switch {
	case role == roleWhere:
		return "THE NPC IT FINDS"
	case scope == worldstate.ScopeNPC:
		return "THIS NPC"
	case scope == worldstate.ScopeZone:
		return "THIS ZONE"
	}

	return "WORLD"
}

// encodeRef and decodeRef write a reference as a chooser row's ID.
func encodeRef(ref flow.ValueRef) string {
	data, err := json.Marshal(ref)
	if err != nil {
		return ""
	}

	return string(data)
}

func decodeRef(row string) (flow.ValueRef, bool) {
	var ref flow.ValueRef
	if err := json.Unmarshal([]byte(row), &ref); err != nil || ref.IsZero() {
		return ref, false
	}

	return ref, true
}

// newValueControl makes a value parameter's widgets; the search box takes
// part in focus only while the chooser shows it.
func (s *State) newValueControl(made *control) {
	made.search = ui.NewTextField("value-search", textedit.NewField("", false, maxValueSearch), false, s.theme)
	made.search.Placeholder, made.search.Unmarked = "Find a value, or type a new name", true
	made.list = &ui.List{ID: "values", Rect: ui.Zero(), Rows: nil, Selected: "", CanDrop: nil}
	s.focus.Register(made.search)
}

// layoutValue places the chosen value and, when open, the chooser down to
// bottom. It returns where the next control goes.
func (s *State) layoutValue(docs frame.Docs, made *control, node flow.Node, left, width, top, bottom int) int {
	made.cueBox = image.Rect(left, top, left+width, top+ui.ButtonH)
	top += ui.ButtonH + rowSpace
	made.search.Rect, made.list.Rect = ui.Zero(), ui.Zero()
	if !made.open {
		return top + fieldSpace
	}
	made.search.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	top += ui.ButtonH + rowSpace
	made.list.Rows = s.valueRows(docs, roleOf(node.Kind, made.param.Name), made.search.Field.Value())
	made.list.Rect = image.Rect(left, top, left+width, max(top+ui.RowHeight, bottom-ui.Gap))

	return made.list.Rect.Max.Y + fieldSpace
}

// valueRows lists the values a search matches under their headings, and
// rows that declare a value named by the search where none has that name.
func (s *State) valueRows(docs frame.Docs, role valueRole, query string) []ui.Row {
	query = strings.TrimSpace(query)
	var rows []ui.Row
	heading := ""
	taken := map[string]bool{}
	for _, option := range s.valueOptions(docs, role) {
		taken[option.path] = true
		if query != "" && !strings.Contains(option.path, strings.ToLower(query)) {
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
	if worldstate.CheckName(query) != nil {
		return rows
	}
	for _, scope := range declareScopes(role) {
		if path := string(scope) + "." + query; !taken[path] {
			rows = append(rows, ui.NewRow(declarePrefix+string(scope), "Declare "+path))
		}
	}

	return rows
}

// declareScopes are the scopes a chooser can declare a new value in.
func declareScopes(role valueRole) []worldstate.Scope {
	switch role {
	case roleKeep:
		return []worldstate.Scope{worldstate.ScopeLocal}
	case roleWhere:
		return []worldstate.Scope{worldstate.ScopeNPC}
	case roleSet, roleCheck:
	}

	return []worldstate.Scope{worldstate.ScopeLocal, worldstate.ScopeNPC, worldstate.ScopeZone, worldstate.ScopeWorld}
}

// updateValue opens and closes the chooser and chooses or declares the
// value clicked.
func (s *State) updateValue(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	if input.Pressed && input.Cursor.In(made.cueBox) {
		input.ClearPointer()
		if made.open {
			s.closeChooser(made)

			return
		}
		made.open = true
		if ref, err := node.Ref(made.param.Name); err == nil {
			made.list.Selected = encodeRef(ref)
		}
		s.focus.Set(made.search)

		return
	}
	if !made.open {
		return
	}
	row, _ := rowAt(made.list, input.Cursor)
	pressed := input.Pressed
	made.list.Update(input)
	if !pressed || row == "" {
		return
	}
	name := strings.TrimSpace(made.search.Field.Value())
	s.closeChooser(made)
	if scope, declaring := strings.CutPrefix(row, declarePrefix); declaring {
		s.declareValue(ctx, node.ID, made.param.Name, worldstate.Scope(scope), name)

		return
	}
	if ref, ok := decodeRef(row); ok {
		s.chooseValue(ctx, ctx.Docs, node, made.param.Name, ref)
	}
}

// chooseValue sets a value parameter, starting its operand at the value's
// own default so the two always fit.
func (s *State) chooseValue(ctx *frame.Context, docs frame.Docs, node flow.Node, param string, ref flow.ValueRef) {
	changed, err := node.WithParam(param, ref)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	operand := operandOfValue(node.Kind, param)
	if declared, _, problem := s.resolve(docs, ref); operand != "" && problem == "" {
		changed, err = changed.WithParam(operand, declared.Zero().Encode())
		if err != nil {
			ctx.Frame.Status(err.Error())

			return
		}
	}
	s.command(ctx, workflowplugin.ReplaceEdit, "", workflowplugin.ReplaceArgs{Node: changed}, nil)
}

// operandOfValue names the operand that follows a value parameter.
func operandOfValue(kind, param string) string {
	switch {
	case kind == flow.KindSetValue && param == flow.ParamValue:
		return "to"
	case kind == flow.KindCheckValue && param == flow.ParamValue:
		return "against"
	case kind == flow.KindFindNPC && param == "where":
		return "is"
	}

	return ""
}

// declareValue declares a value named by the chooser's search, in a scope,
// then chooses it: a local on the workflow, anything else in the world
// state. A value to keep an NPC in is an NPC; any other starts as a number.
func (s *State) declareValue(ctx *frame.Context, nodeID ids.ULID, param string, scope worldstate.Scope, name string) {
	kind := worldstate.TypeNumber
	node, ok := s.currentNode(ctx.Docs, nodeID)
	if !ok {
		return
	}
	if roleOf(node.Kind, param) == roleKeep {
		kind = worldstate.TypeNPC
	}
	made := worldstate.New(ids.ULID{}, name, scope, kind)
	chosen := func(result protocol.Result) {
		current, found := s.currentNode(ctx.Docs, nodeID)
		ref := flow.ValueRef{Scope: scope, Value: result.Created, NPC: ids.ULID{}, Found: ids.ULID{}}
		if found {
			s.chooseValue(ctx, ctx.Docs, current, param, ref)
		}
		ctx.Frame.Status("Declared " + made.Path() + ". Change its type in " + declaredWhere(scope) + ".")
	}
	if scope == worldstate.ScopeLocal {
		s.command(ctx, workflowplugin.AddLocalEdit, "", stateplugin.ValueArgs{Value: made}, chosen)

		return
	}
	data, err := json.Marshal(stateplugin.ValueArgs{Value: made})
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	command := protocol.Command{Document: worldstate.ID(), Edit: stateplugin.AddValueEdit, Gesture: "", Args: data}
	ctx.Requests.Command(command, func(result protocol.Result, err error) {
		if err != nil {
			ctx.Frame.Report("")(result, err)

			return
		}
		chosen(result)
	})
}

func declaredWhere(scope worldstate.Scope) string {
	if scope == worldstate.ScopeLocal {
		return "Local values, with no node selected"
	}

	return "the World state window"
}

// operandDeclaration returns the declaration an operand follows, if its
// value is chosen and found.
func (s *State) operandDeclaration(docs frame.Docs, node flow.Node, param string) (worldstate.Declaration, bool) {
	ref, err := node.Ref(flow.OperandOf(node.Kind, param))
	if err != nil {
		return worldstate.Declaration{
			ID: ids.ULID{}, Name: "", Scope: "", Type: "", Choices: nil, Default: nil, Description: "",
		}, false
	}
	declared, _, problem := s.resolve(docs, ref)

	return declared, problem == "" || problem == problemFoundGone || problem == problemNPCGone
}

// operandShape says what an operand's control is made of: its value's type
// and choices. The form is rebuilt when it changes.
func operandShape(declared worldstate.Declaration, found bool) string {
	if !found {
		return "-"
	}
	var shape strings.Builder
	shape.WriteString(string(declared.Type))
	for _, choice := range declared.Choices {
		shape.WriteString("," + choice.ID + "=" + choice.Name)
	}

	return shape.String()
}

// newOperandControl makes an operand's widgets for its value's type: a
// field for a number, no and yes, or a button for each choice.
func (s *State) newOperandControl(made *control, declared worldstate.Declaration, found bool) {
	made.operands = nil
	if !found {
		return
	}
	switch declared.Type {
	case worldstate.TypeNumber:
		made.field = ui.NewTextField(operandPrefix+made.param.Name, textedit.NewField("", false, maxValueSearch), false,
			s.theme)
		s.focus.Register(made.field)
	case worldstate.TypeYesNo:
		for _, yes := range []bool{false, true} {
			value := worldstate.Value{Type: worldstate.TypeYesNo, Number: 0, Yes: yes, Choice: "", NPC: ids.ULID{}}
			label := declared.Label(value)
			made.choices = append(made.choices, ui.NewButton(operandPrefix+made.param.Name+"-"+label, label))
			made.operands = append(made.operands, value.Encode())
		}
	case worldstate.TypeChoice:
		for _, choice := range declared.Choices {
			value := worldstate.Value{Type: worldstate.TypeChoice, Number: 0, Yes: false, Choice: choice.ID, NPC: ids.ULID{}}
			made.choices = append(made.choices, ui.NewButton(operandPrefix+made.param.Name+"-"+choice.ID, choice.Name))
			made.operands = append(made.operands, value.Encode())
		}
	case worldstate.TypeNPC:
	}
}

// setSavedOperand shows an operand's saved value.
func setSavedOperand(made *control, node flow.Node) {
	saved, err := node.Operand(made.param.Name)
	if err != nil {
		return
	}
	if made.field != nil {
		var number int
		if json.Unmarshal(saved, &number) == nil {
			made.field.Field.SetSaved(strconv.Itoa(number))
		}
	}
	for index, button := range made.choices {
		button.On = index < len(made.operands) && string(made.operands[index]) == string(saved)
	}
}

// layoutOperand places an operand's field or buttons, or leaves room for a
// note while its value is not chosen. It returns where the next control
// goes.
func layoutOperand(made *control, left, width, top int) int {
	if made.field != nil {
		made.field.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)

		return top + ui.ButtonH + fieldSpace
	}
	if len(made.choices) == 0 {
		return top + ui.LineHeight + fieldSpace
	}
	const perRow = 3
	for start := 0; start < len(made.choices); start += perRow {
		row := made.choices[start:min(start+perRow, len(made.choices))]
		gaps := (perRow - 1) * rowSpace
		for index, button := range row {
			from := left + index*(width-gaps)/perRow + index*rowSpace
			to := left + (index+1)*(width-gaps)/perRow + index*rowSpace
			button.Rect = image.Rect(from, top, to, top+ui.ButtonH)
		}
		top += ui.ButtonH + rowSpace
	}

	return top + fieldSpace - rowSpace
}

// updateOperand saves an operand's yes, no or choice when its button is
// clicked.
func (s *State) updateOperand(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	for index, button := range made.choices {
		if button.Update(input) && !button.On && index < len(made.operands) {
			s.saveParam(ctx, node, made.param.Name, made.operands[index])
		}
	}
}

// parseOperand reads a number typed into an operand's field.
func parseOperand(made *control, field *ui.TextField, value string) (any, bool, string) {
	if field != made.field || made.field == nil {
		return nil, false, ""
	}

	return parseWhole(value, -worldstate.MaxNumber, worldstate.MaxNumber)
}

// drawValue draws a value parameter: its path, or what is wrong with it,
// and the chooser when open.
func (s *State) drawValue(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node) {
	ref, err := node.Ref(made.param.Name)
	label, ink := "?", ui.ColorWarn()
	if err == nil {
		_, path, problem := s.resolve(docs, ref)
		switch {
		case ref.IsZero() && roleOf(node.Kind, made.param.Name) == roleWhere:
			label, ink = "Any NPC", ui.ColorInk()
		case ref.IsZero():
			label, ink = "Choose a value", ui.ColorMuted()
		case problem != "" && path == "":
			label = "Broken: " + strings.ToLower(strings.TrimSuffix(problem, "."))
		default:
			label, ink = path, ui.ColorInk()
		}
	}
	ui.Box(screen, made.cueBox, ui.ColorBackground())
	if made.open {
		ui.StrokeRound(screen, made.cueBox, ui.Radius, ui.Border, ui.ColorSelect())
	}
	textTop := made.cueBox.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	caret := image.Rect(made.cueBox.Max.X-textPad-caretWidth, made.cueBox.Min.Y, made.cueBox.Max.X-textPad,
		made.cueBox.Max.Y)
	ui.DrawTriangle(screen, caret, true, ui.ColorMuted())
	s.theme.DrawMono(screen, s.clip(label, caret.Min.X-made.cueBox.Min.X-ui.Half*textPad), made.cueBox.Min.X+textPad,
		textTop, ink)
	if !made.open {
		return
	}
	made.search.Draw(screen)
	if len(made.list.Rows) == 0 {
		s.theme.DrawText(screen, "No value matches. Type a name to declare one.", made.list.Rect.Min.X,
			made.list.Rect.Min.Y, ui.ColorMuted())

		return
	}
	made.list.Draw(screen, s.theme)
}

// drawOperand draws an operand's field or buttons, or says to choose its
// value first.
func (s *State) drawOperand(screen *ebiten.Image, made *control, left int) {
	if made.field != nil {
		made.field.Draw(screen)

		return
	}
	for _, button := range made.choices {
		button.Draw(screen, s.theme)
	}
	if len(made.choices) == 0 {
		s.theme.DrawText(screen, "Choose its value first.", left, made.label+ui.LineHeight, ui.ColorMuted())
	}
}

// valueProblem says what is wrong with a Set value, Check value or Find NPC
// nearby.
func (s *State) valueProblem(node flow.Node, docs frame.Docs) string {
	if node.Kind == flow.KindFindNPC {
		return s.findProblem(node, docs)
	}
	ref, err := node.Ref(flow.ParamValue)
	if err != nil {
		return problemNoValue
	}
	declared, _, problem := s.resolve(docs, ref)
	switch {
	case problem != "":
		return problem
	case node.Kind == flow.KindSetValue && !ref.Writable():
		return problemReadOnly
	case declared.Type == worldstate.TypeNPC:
		return problemNPCValue
	case node.Kind == flow.KindSetValue && choiceOf(node, "how") == flow.HowAdd && declared.Type != worldstate.TypeNumber:
		return problemAddNumber
	case node.Kind == flow.KindCheckValue && choiceOf(node, "compare") != flow.CompareIs &&
		declared.Type != worldstate.TypeNumber:
		return problemCompareNumber
	}
	if !operandFits(node, operandOfValue(node.Kind, flow.ParamValue), declared) {
		return problemOperand
	}

	return ""
}

// findProblem says what is wrong with a Find NPC nearby.
func (s *State) findProblem(node flow.Node, docs frame.Docs) string {
	keep, err := node.Ref("into")
	if err != nil || keep.Scope != worldstate.ScopeLocal {
		return problemNoKeep
	}
	if declared, _, problem := s.resolve(docs, keep); problem != "" || declared.Type != worldstate.TypeNPC {
		return problemNoKeep
	}
	where, err := node.Ref("where")
	if err != nil || where.IsZero() {
		return ""
	}
	if where.Scope != worldstate.ScopeNPC || where.Others() {
		return problemWhere
	}
	declared, _, problem := s.resolve(docs, where)
	if problem != "" {
		return problem
	}
	if !operandFits(node, "is", declared) {
		return problemOperand
	}

	return ""
}

// operandFits reports whether an operand holds a value of the type it
// follows.
func operandFits(node flow.Node, operand string, declared worldstate.Declaration) bool {
	saved, err := node.Operand(operand)
	if err != nil || flow.IsNoOperand(saved) {
		return false
	}
	_, err = declared.Parse(saved)

	return err == nil
}

// valueSummary describes a value node in a few words.
func (s *State) valueSummary(node flow.Node, docs frame.Docs) string {
	path := func(param string) string {
		ref, err := node.Ref(param)
		if err != nil || ref.IsZero() {
			return ""
		}
		if _, shown, problem := s.resolve(docs, ref); problem == "" {
			return shown
		}

		return "?"
	}
	operand := func(param string) string {
		declared, found := s.operandDeclaration(docs, node, param)
		saved, err := node.Operand(param)
		if !found || err != nil {
			return "?"
		}
		value, err := declared.Parse(saved)
		if err != nil {
			return "?"
		}

		return declared.Label(value)
	}
	switch node.Kind {
	case flow.KindSetValue:
		if path(flow.ParamValue) == "" {
			return "no value"
		}
		if choiceOf(node, "how") == flow.HowAdd {
			return path(flow.ParamValue) + " + " + operand("to")
		}

		return path(flow.ParamValue) + " to " + operand("to")
	case flow.KindCheckValue:
		if path(flow.ParamValue) == "" {
			return "no value"
		}

		return path(flow.ParamValue) + " " + strings.ReplaceAll(choiceOf(node, "compare"), "-", " ") + " " +
			operand("against")
	case flow.KindFindNPC:
		return s.findSummary(node, path, operand)
	}

	return ""
}

func (s *State) findSummary(node flow.Node, path, operand func(string) string) string {
	tiles, err := node.Number("within")
	if err != nil {
		return "?"
	}
	written := "within " + strconv.Itoa(tiles) + " " + plural(tiles, "tile", "tiles")
	if where := path("where"); where != "" {
		written += " · " + where + " is " + operand("is")
	}
	if keep := path("into"); keep != "" {
		written += " → " + keep
	}

	return written
}

// locateValueRow finds an open value chooser's row, "valuerow:<path>", or
// a row that declares a value, "declare:<scope>", for input scripts.
func (s *State) locateValueRow(kind, value string) (image.Point, bool) {
	made, open := s.openChooser()
	if !open || made.param.Type != flow.TypeValue {
		return image.Pt(0, 0), false
	}
	for index, row := range made.list.Rows {
		rect := made.list.RowRect(index)
		if row.Heading || !rect.In(made.list.Rect) {
			continue
		}
		if (kind == locateValue && row.Label == value) || (kind == locateDeclare && row.ID == declarePrefix+value) {
			return image.Pt(rect.Min.X+textPad, ui.Center(rect).Y), true
		}
	}

	return image.Pt(0, 0), false
}

// hasValueState answers "value:<param>=<path>" for the selected node's
// value parameter and "operand:<param>=<saved>" for its operand.
func (s *State) hasValueState(state string, docs frame.Docs) (bool, bool) {
	node, selected := s.selectedNode()
	switch {
	case strings.HasPrefix(state, "value:"):
		param, want, _ := strings.Cut(strings.TrimPrefix(state, "value:"), "=")
		ref, err := node.Ref(param)
		if !selected || err != nil {
			return false, true
		}
		_, path, problem := s.resolve(docs, ref)

		return problem == "" && path == want, true
	case strings.HasPrefix(state, "operand:"):
		param, want, _ := strings.Cut(strings.TrimPrefix(state, "operand:"), "=")
		saved, err := node.Operand(param)

		return selected && err == nil && string(saved) == want, true
	}

	return false, false
}

// npcIDs lists the project's NPCs.
func (s *State) npcIDs(docs frame.Docs) []ids.ULID {
	var found []ids.ULID
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			for _, entry := range index.OfFormat(npc.FormatName) {
				found = append(found, entry.ID)
			}
		}
	})

	return found
}

// valueTitle names a value node on the canvas as a short sentence, so what
// it sets or asks shows in its width: "Set npc.mood to cross", "Add 1 to
// local.tally", "Is npc.count at least 3?" or "Find NPC within 2 tiles".
func (s *State) valueTitle(node flow.Node, docs frame.Docs) string {
	kind, _ := flow.KindOf(node.Kind)
	path, operand := "", ""
	if ref, err := node.Ref(flow.ParamValue); err == nil && !ref.IsZero() {
		_, path, _ = s.resolve(docs, ref)
		if path == "" {
			path = "?"
		}
		if declared, found := s.operandDeclaration(docs, node, operandOfValue(node.Kind, flow.ParamValue)); found {
			saved, savedErr := node.Operand(operandOfValue(node.Kind, flow.ParamValue))
			if value, parseErr := declared.Parse(saved); savedErr == nil && parseErr == nil {
				operand = declared.Label(value)
			}
		}
	}
	if operand == "" {
		operand = "?"
	}
	switch {
	case node.Kind == flow.KindFindNPC:
		tiles, err := node.Number("within")
		if err != nil {
			return kind.Label
		}

		return "Find NPC within " + strconv.Itoa(tiles) + " " + plural(tiles, "tile", "tiles")
	case path == "":
		return kind.Label + " · no value"
	case node.Kind == flow.KindCheckValue && choiceOf(node, "compare") == flow.CompareIs:
		return "Is " + path + " " + operand + "?"
	case node.Kind == flow.KindCheckValue:
		return "Is " + path + " " + strings.ReplaceAll(choiceOf(node, "compare"), "-", " ") + " " + operand + "?"
	case choiceOf(node, "how") == flow.HowAdd:
		return "Add " + operand + " to " + path
	}

	return "Set " + path + " to " + operand
}
