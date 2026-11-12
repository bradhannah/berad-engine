package workflowview

import (
	"encoding/json"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuepicker"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A value parameter shows the chosen value's path, such as npc.guards.on_duty;
// clicking it opens a chooser under it: a search box and the values the
// node may use, grouped by whose they are. Typing a name no value has
// offers to declare it, dots putting it in groups, as world.quests.dragon_slain,
// or in a template, as npc.guards.on_duty. An operand follows its value's
// type: an integer or text to type, false or true, or one of a list's
// choices.
const (
	// locateValue names a picker's row for input scripts, "valuerow:<path>",
	// and locateDeclare where a value can be declared, "declare:<path>".
	locateValue   = valuepicker.LocateValue
	locateDeclare = valuepicker.LocateDeclare
	// valueBoxPrefix names a value parameter's box, "ui:pick-<param>".
	valueBoxPrefix = "pick-"
	operandPrefix  = "operand-"
	// textShown bounds text shown on a node, in characters.
	textShown = 24
)

// Problems the value nodes can have.
const (
	problemNoValue   = "No value chosen yet."
	problemValueGone = "Its value was deleted. Choose another, or undo the delete."
	problemReadOnly  = "It can read another NPC's or zone's values, but never set them."
	// problemBuiltInFixed is a Set value on an NPC's zone or behaviour.
	problemBuiltInFixed = "An NPC's zone and behaviour are set in its details, not by a workflow."
	problemNPCValue     = "An NPC value cannot be set or checked here."
	problemOperand      = "Choose what it sets the value to, or compares it with."
	problemAddInteger   = "Only an integer can be added to."
	problemNoKeep       = "Choose a local NPC value to keep what it finds in."
	problemWhere        = "Only an NPC's own value can be looked for."
	problemOwnerGone    = "Its NPC or zone was deleted."
	problemFoundGone    = "The local value holding its NPC was deleted."
)

// valueOption is one value a parameter can choose: its reference, path and
// declaration, and where it is listed.
type valueOption struct {
	ref      flow.ValueRef
	path     string
	declared worldstate.Declaration
	place
}

// place is where a value is listed: under a heading, in one of the
// picker's scopes on its rail, and with the root its path starts from
// there, so what its path has between the root and its own name is its
// group.
type place struct {
	heading string
	rail    string
	root    string
}

// The picker's scopes beyond the scopes' own: every scope, other NPCs' and
// zones' values.
const (
	railOthers = "others"
	railZones  = "zones"
)

// scopePlace lists a value under a heading, in a scope of its own name.
func scopePlace(heading string, scope worldstate.Scope) place {
	return place{heading: heading, rail: string(scope), root: string(scope)}
}

// declareTarget is where a chooser's row declares a value: a scope, the
// document and list it goes in, the group or template it goes under, its
// dotted name from there, its whole path, as the row shows it, and for one
// NPC's own value the NPC's path name.
type declareTarget struct {
	Scope    worldstate.Scope `json:"scope"`
	Document ids.ULID         `json:"document"`
	Parent   ids.ULID         `json:"parent"`
	Name     string           `json:"name"`
	Path     string           `json:"path"`
	For      string           `json:"for,omitempty"`
}

// located is how input scripts name a declare row: its path, and for one
// NPC's own value, "for" and the NPC's path name, as npc.mood for ada.
func (t declareTarget) located() string {
	if t.For == "" {
		return t.Path
	}

	return t.Path + " for " + t.For
}

// samePath reports whether a path shown is the one a script names, which
// may leave a value's var. off, as paths were named before constants.
func samePath(shown, want string) bool { return shown == want || shown == worldstate.ValuePrefix+want }

// catalog is every value the project declares, as far as the window holds
// it; empty before the world state arrives.
func (s *State) catalog(docs frame.Docs) *worldstate.Catalog {
	if catalog, ready := valuetree.Catalog(docs); ready {
		return catalog
	}

	return worldstate.NewCatalog(worldstate.NewDeclarations(worldstate.HoldsWorldState), nil)
}

// followers lists the NPCs whose behaviour is this workflow.
func (s *State) followers(docs frame.Docs, catalog *worldstate.Catalog) []ids.ULID {
	var found []ids.ULID
	for _, npcID := range catalog.Owners(worldstate.ScopeNPC) {
		docs.View(npcID, func(document client.Document) {
			if npcDoc, ok := document.Model.(*npcplugin.Document); ok && npcDoc.NPC.Behaviour == s.document {
				found = append(found, npcID)
			}
		})
	}

	return found
}

// watchValues keeps the world state, every NPC and every zone loaded, for
// value paths and problems.
func (s *State) watchValues(ctx *frame.Context) {
	for _, document := range valuetree.Documents(ctx.Docs) {
		s.subscribe(document, ctx)
	}
}

// ownerPath names an NPC or zone in a path: its path name, or its ID before
// it has one.
func ownerPath(owner worldstate.Owner, id ids.ULID) string {
	if owner.Path == "" {
		return "#" + id.Short(shortID)
	}

	return owner.Path
}

// resolve finds what a reference refers to: its declaration and path, or
// what is wrong with it.
func (s *State) resolve(docs frame.Docs, ref flow.ValueRef) (worldstate.Declaration, string, string) {
	none := worldstate.New(ids.ULID{}, "", "", "")
	if ref.IsZero() {
		return none, "", problemNoValue
	}
	if ref.Scope == worldstate.ScopeLocal {
		declared, ok := s.workflow.Locals().Value(ref.Value)
		if !ok {
			return none, "", problemValueGone
		}

		return declared, localPath(declared), ""
	}
	catalog := s.catalog(docs)
	declared, path, found := catalog.Declared(ref.Value)
	if !found || declared.Scope != ref.Scope || !declared.Type.IsValue() {
		return none, "", problemValueGone
	}
	switch {
	case !ref.Found.IsZero():
		holder, held := s.workflow.Locals().Value(ref.Found)
		if !held {
			return declared, "", problemFoundGone
		}

		return declared, worldstate.Prefixed(localPath(holder)+strings.TrimPrefix(path, string(worldstate.ScopeNPC)),
			declared), ""
	case !ref.Owner.IsZero():
		owner, known := catalog.Owner(ref.Owner)
		if !known {
			return declared, "", problemOwnerGone
		}
		named := worldstate.Prefixed(worldstate.NamedPath(path, ownerPath(owner, ref.Owner)), declared)
		if !holds(catalog, ref.Scope, ref.Owner, ref.Value) {
			return declared, named, owner.Name + " has no " + worldstate.Prefixed(path, declared) + "."
		}

		return declared, named, ""
	}

	return declared, worldstate.Prefixed(path, declared), ""
}

// missing says which NPC following this workflow lacks one of its own
// values that a node uses, or "".
func (s *State) missing(docs frame.Docs, ref flow.ValueRef, path string) string {
	if ref.Scope != worldstate.ScopeNPC || ref.Others() {
		return ""
	}
	catalog := s.catalog(docs)
	for _, follower := range s.followers(docs, catalog) {
		if !holds(catalog, worldstate.ScopeNPC, follower, ref.Value) {
			owner, _ := catalog.Owner(follower)

			return owner.Name + " follows this workflow but has no " + path + "."
		}
	}

	return ""
}

// valueRole says which values a parameter may choose.
type valueRole int

const (
	roleSet valueRole = iota
	roleCheck
	roleWhere
	roleKeep
	// roleGive is another value a Call gives an input, or a value a Log
	// writes, and roleKept a value a Call keeps an output in: of any type,
	// an NPC or a place among them.
	roleGive
	roleKept
	// rolePlace is a Go to's place value: a place local.
	rolePlace
	// roleNumber is the constant a typed-in number can be: a whole-number
	// constant, never a value; and roleMakeText the text constant words
	// typed in a message become.
	roleNumber
	roleMakeText
)

func roleOf(kind, param string) valueRole {
	switch {
	case kind == flow.KindCall && strings.HasPrefix(param, pickInput):
		return roleGive
	case kind == flow.KindCall:
		return roleKept
	case hasMessage(kind):
		return roleGive
	case kind == flow.KindGoTo:
		return rolePlace
	case kind == flow.KindCheckValue:
		return roleCheck
	case kind == flow.KindFindNPC && param == "into":
		return roleKeep
	case kind == flow.KindFindNPC:
		return roleWhere
	}

	return roleSet
}

// options gathers a chooser's values, each once.
type options struct {
	listed []valueOption
	seen   map[string]bool
}

func (o *options) add(where place, ref flow.ValueRef, path string, declared worldstate.Declaration) {
	key := encodeRef(ref)
	if o.seen[key] {
		return
	}
	o.seen[key] = true
	where.root = worldstate.Prefixed(where.root, declared)
	o.listed = append(o.listed, valueOption{
		ref: ref, path: worldstate.Prefixed(path, declared), declared: declared, place: where,
	})
}

func refTo(declared worldstate.Declaration, owner, found ids.ULID) flow.ValueRef {
	return flow.ValueRef{Scope: declared.Scope, Value: declared.ID, Owner: owner, Found: found}
}

// valueOptions lists the values a parameter may choose, in the order the
// chooser shows them: its locals, its NPC's values, its zone's and the
// world's; a Check value also reads the NPCs its locals hold, and named
// NPCs and zones.
func (s *State) valueOptions(docs frame.Docs, role valueRole) []valueOption {
	found := &options{listed: nil, seen: map[string]bool{}}
	locals := s.workflow.Locals().Values()
	if makesConstant(role) {
		s.constantOptions(found, docs, s.catalog(docs), role, nil)

		return found.listed
	}
	for _, local := range locals {
		if offersLocal(role, local) {
			found.add(scopePlace("LOCAL", worldstate.ScopeLocal), refTo(local, ids.ULID{}, ids.ULID{}), localPath(local),
				local)
		}
	}
	if role == roleKeep || role == rolePlace {
		return found.listed
	}
	catalog := s.catalog(docs)
	heading := "THIS NPC"
	if role == roleWhere {
		heading = "THE NPC IT FINDS"
	}
	for _, held := range s.ownNPCValues(docs, catalog, role) {
		found.add(scopePlace(heading, worldstate.ScopeNPC), refTo(held.Declaration, ids.ULID{}, ids.ULID{}),
			held.Path, held.Declaration)
	}
	if role == roleWhere {
		s.constantOptions(found, docs, catalog, role, locals)

		return found.listed
	}
	zoneValues := catalog.Templates(worldstate.ScopeZone)
	for _, zone := range catalog.Owners(worldstate.ScopeZone) {
		zoneValues = append(zoneValues, catalog.Held(worldstate.ScopeZone, zone)...)
	}
	for _, held := range zoneValues {
		found.add(scopePlace("THIS ZONE", worldstate.ScopeZone), refTo(held.Declaration, ids.ULID{}, ids.ULID{}),
			held.Path, held.Declaration)
	}
	for _, held := range catalog.Held(worldstate.ScopeWorld, ids.ULID{}) {
		found.add(scopePlace("WORLD", worldstate.ScopeWorld), refTo(held.Declaration, ids.ULID{}, ids.ULID{}),
			held.Path, held.Declaration)
	}
	if role == roleCheck || role == roleGive {
		s.othersOptions(found, catalog, locals)
	}
	s.constantOptions(found, docs, catalog, role, locals)

	return found.listed
}

// offersLocal reports whether a role's chooser offers a local: an NPC to
// keep what Find NPC nearby finds, a place to go to, any local a Call gives
// or keeps, and otherwise any but an NPC or a place.
func offersLocal(role valueRole, local worldstate.Declaration) bool {
	switch role {
	case roleKeep:
		return local.Type == worldstate.TypeNPC
	case rolePlace:
		return local.Type == worldstate.TypePlace
	case roleGive, roleKept:
		return true
	case roleWhere, roleNumber, roleMakeText:
		return false
	case roleSet, roleCheck:
	}

	return local.Type != worldstate.TypeNPC && local.Type != worldstate.TypePlace
}

// npcValues lists the values a workflow's own NPC can have: its built-in
// values, and what the NPCs following it have, or, while none does, every
// template's.
func (s *State) npcValues(docs frame.Docs, catalog *worldstate.Catalog, role valueRole) []worldstate.Held {
	followers := s.followers(docs, catalog)
	if len(followers) == 0 || role == roleWhere {
		return append(builtInsHeld(), catalog.Templates(worldstate.ScopeNPC)...)
	}
	var values []worldstate.Held
	for _, follower := range followers {
		values = append(values, catalog.Held(worldstate.ScopeNPC, follower)...)
	}

	return values
}

// ownNPCValues lists the values of the workflow's own NPC a role offers:
// none in a zone loop, which runs for no NPC, and for a Set value none it
// cannot set.
func (s *State) ownNPCValues(docs frame.Docs, catalog *worldstate.Catalog, role valueRole) []worldstate.Held {
	if role != roleWhere && s.workflow.Purpose() == flow.PurposeZoneLoop {
		return nil
	}
	var offered []worldstate.Held
	for _, held := range s.npcValues(docs, catalog, role) {
		if role != roleSet || !worldstate.ReadOnly(held.Declaration.ID) {
			offered = append(offered, held)
		}
	}

	return offered
}

// builtInsHeld lists every NPC's built-in values as values it holds.
func builtInsHeld() []worldstate.Held {
	builtIns := worldstate.BuiltIns(worldstate.Owner{
		Scope: worldstate.ScopeNPC, Path: "", Name: "", Owned: worldstate.NewOwned(worldstate.ScopeNPC), Zone: ids.ULID{},
		Behaviour: ids.ULID{}, Sprite: ids.ULID{},
	})
	held := make([]worldstate.Held, 0, len(builtIns))
	for _, declared := range builtIns {
		held = append(held, worldstate.Held{Declaration: declared, Path: string(worldstate.ScopeNPC) + "." + declared.Name})
	}

	return held
}

// othersOptions adds other NPCs' and zones' values a Check value can read:
// those of each NPC a local value holds, and of each NPC and zone with a
// path name.
func (s *State) othersOptions(found *options, catalog *worldstate.Catalog, locals []worldstate.Declaration) {
	shared := append(builtInsHeld(), catalog.Templates(worldstate.ScopeNPC)...)
	for _, holder := range locals {
		if holder.Type != worldstate.TypeNPC {
			continue
		}
		where := place{heading: "THE NPC IN " + localPath(holder), rail: railOthers, root: localPath(holder)}
		for _, held := range shared {
			found.add(where, refTo(held.Declaration, ids.ULID{}, holder.ID),
				localPath(holder)+strings.TrimPrefix(held.Path, string(worldstate.ScopeNPC)), held.Declaration)
		}
	}
	for _, scope := range []worldstate.Scope{worldstate.ScopeNPC, worldstate.ScopeZone} {
		rail := railOthers
		if scope == worldstate.ScopeZone {
			rail = railZones
		}
		for _, ownerID := range catalog.Owners(scope) {
			owner, _ := catalog.Owner(ownerID)
			if owner.Path == "" {
				continue
			}
			where := place{heading: strings.ToUpper(owner.Name), rail: rail, root: string(scope) + "." + owner.Path}
			for _, held := range catalog.Held(scope, ownerID) {
				found.add(where, refTo(held.Declaration, ownerID, ids.ULID{}), worldstate.NamedPath(held.Path, owner.Path),
					held.Declaration)
			}
		}
	}
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
func (s *State) newValueControl(*control) {}

// layoutValue places the chosen value's box, which opens the picker, with
// a line under it for the value's type, limits and description. It returns
// where the next control goes.
func layoutValue(made *control, left, width, top int) int {
	made.cueBox = image.Rect(left, top, left+width, top+ui.ButtonH)

	return top + ui.ButtonH + ui.LineHeight + fieldSpace
}

// ownerName is an NPC's display name.
func (s *State) ownerName(docs frame.Docs, id ids.ULID) string {
	owner, _ := s.catalog(docs).Owner(id)

	return owner.Name
}

// declareTargets lists where a typed name can be declared, for the scopes a
// parameter may use; a name starting with a scope, such as world., only in
// that scope. A local has one name; a world value can be in groups, as
// world.quests.dragon_slain. An NPC or zone value goes in the template its
// first name names, as npc.guards.on_duty, or is one NPC's own, for each NPC
// following this workflow.
func (s *State) declareTargets(docs frame.Docs, role valueRole, query string) []declareTarget {
	var targets []declareTarget
	query, _ = worldstate.Unprefixed(query)
	for _, scope := range declareScopes(role) {
		dotted := query
		for _, other := range []worldstate.Scope{
			worldstate.ScopeLocal, worldstate.ScopeNPC, worldstate.ScopeZone, worldstate.ScopeWorld,
		} {
			if rest, named := strings.CutPrefix(query, string(other)+"."); named {
				dotted = rest
				if other != scope {
					dotted = ""
				}
			}
		}
		names, err := worldstate.SplitName(dotted)
		if err != nil {
			continue
		}
		target := declareTarget{Scope: scope, Document: worldstate.ID(), Parent: ids.ULID{}, Name: dotted,
			Path: worldstate.Prefixed(string(scope)+"."+dotted, worldstate.New(ids.ULID{}, "", scope, "")), For: ""}
		switch scope {
		case worldstate.ScopeLocal:
			if len(names) == 1 {
				targets = append(targets, declareTarget{Scope: scope, Document: s.document, Parent: ids.ULID{},
					Name: dotted, Path: target.Path, For: ""})
			}
		case worldstate.ScopeWorld:
			targets = append(targets, target)
		case worldstate.ScopeNPC, worldstate.ScopeZone:
			targets = append(targets, s.ownedTargets(docs, target, names)...)
		}
	}

	return targets
}

// ownedTargets lists where an NPC or zone value can be declared: in the
// template its first name names, or else as each following NPC's own, so
// no NPC has two values of one path.
func (s *State) ownedTargets(docs frame.Docs, target declareTarget, names []string) []declareTarget {
	var targets []declareTarget
	catalog := s.catalog(docs)
	template, found := catalog.World().Find(target.Scope, ids.ULID{}, names[0])
	if found && template.Type == worldstate.TypeTemplate {
		if len(names) > 1 {
			target.Parent, target.Name = template.ID, strings.Join(names[1:], ".")
			targets = append(targets, target)
		}

		return targets
	}
	if target.Scope != worldstate.ScopeNPC {
		return targets
	}
	for _, follower := range s.followers(docs, catalog) {
		held, _ := catalog.Owner(follower)
		own := target
		own.Document, own.Parent, own.Name, own.For = follower, ids.ULID{}, strings.Join(names, "."), ownerPath(held,
			follower)
		targets = append(targets, own)
	}

	return targets
}

// declareScopes are the scopes a chooser can declare a new value in.
func declareScopes(role valueRole) []worldstate.Scope {
	switch role {
	case roleKeep, rolePlace:
		return []worldstate.Scope{worldstate.ScopeLocal}
	case roleWhere:
		return []worldstate.Scope{worldstate.ScopeNPC}
	case roleSet, roleCheck, roleGive, roleKept, roleNumber, roleMakeText:
	}

	return []worldstate.Scope{worldstate.ScopeLocal, worldstate.ScopeNPC, worldstate.ScopeZone, worldstate.ScopeWorld}
}

// updateValue opens the picker when the value's box is clicked.
func (s *State) updateValue(input *ui.Input, _ *frame.Context, made *control, node flow.Node) {
	if input.Pressed && input.Cursor.In(made.cueBox) {
		input.ClearPointer()
		s.openValueBox(node, made)
	}
}

// openValueBox opens the picker for a value's box: locked to NPC values for
// one to keep an NPC in.
func (s *State) openValueBox(node flow.Node, made *control) {
	only := worldstate.Type("")
	switch roleOf(node.Kind, made.param.Name) {
	case roleKeep:
		only = worldstate.TypeNPC
	case rolePlace:
		only = worldstate.TypePlace
	case roleSet, roleCheck, roleWhere, roleGive, roleKept, roleNumber, roleMakeText:
	}
	s.openValuePicker(node, made.param.Name, pickerTitle(made.param, only, false), only, false)
}

// openFirstBox opens the picker for the selected node's first value box.
// The inspector's boxes take no keyboard focus, so Enter with a value node
// selected stands for Enter on its box.
func (s *State) openFirstBox() {
	node, selected := s.selectedNode()
	if !selected || s.form == nil {
		return
	}
	for index := range s.form.controls {
		if made := &s.form.controls[index]; made.param.Type == flow.TypeValue && !made.cueBox.Empty() {
			s.openValueBox(node, made)

			return
		}
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

// declaring is a value to declare from the picker: where, its type and what
// it starts at, nil for its type's own; operand is set when it is then
// chosen as an operand's reference rather than as a value parameter. A
// constant made from a typed-in value has constant set, the random
// choice's option it stands in for or -1, the gesture its edits share, and
// for words in a message, the words.
type declaring struct {
	target   declareTarget
	kind     worldstate.Type
	start    json.RawMessage
	operand  bool
	constant bool
	option   int
	gesture  string
	text     *selectedWords
}

// declareValue declares a value where the picker says, with the groups its
// name needs, then chooses it for a node's parameter.
func (s *State) declareValue(ctx *frame.Context, nodeID ids.ULID, param string, made declaring) {
	if _, ok := s.currentNode(ctx.Docs, nodeID); !ok {
		return
	}
	target := made.target
	list, listed := s.declareList(ctx.Docs, target)
	if !listed {
		return
	}
	declare := valuetree.Declare
	if made.constant {
		declare = valuetree.DeclareConstant
	}
	change, err := declare(list.Values, target.Scope, target.Parent, target.Name, made.kind)
	if err != nil {
		ctx.Frame.Status(strings.TrimPrefix(err.Error(), valuetree.ErrName.Error()+": "))

		return
	}
	if made.start != nil {
		change.Add[len(change.Add)-1].Default = made.start
	}
	data, err := json.Marshal(change)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	command := protocol.Command{Document: list.Document, Edit: list.Edit, Gesture: made.gesture, Args: data, Says: ""}
	ctx.Requests.Command(command, func(result protocol.Result, err error) {
		if err != nil {
			ctx.Frame.Report("")(result, err)

			return
		}
		// A local is declared in this workflow, so it is read again before
		// the value is chosen and its operand started at it.
		s.load(ctx.Docs)
		current, found := s.currentNode(ctx.Docs, nodeID)
		ref := flow.ValueRef{Scope: target.Scope, Value: result.Created, Owner: ids.ULID{}, Found: ids.ULID{}}
		switch {
		case !found:
		case made.constant:
			s.madeConstant(ctx, current, param, made, ref)
		case s.chooseForCall(ctx, current, param, ref), s.insertForLog(ctx, current, ref):
		case made.operand:
			s.saveParam(ctx, current, param, flow.RefOperand(ref))
		default:
			s.chooseValue(ctx, ctx.Docs, current, param, ref)
		}
		ctx.Frame.Status("Declared " + target.Path + ". Change it in " + declaredWhere(target.Scope, made.constant) +
			".")
	})
}

// declareList is the list a value is declared in: the workflow's locals,
// the world state's, or an NPC's own.
func (s *State) declareList(docs frame.Docs, target declareTarget) (valuetree.List, bool) {
	switch {
	case target.Scope == worldstate.ScopeLocal:
		return valuetree.List{
			Document: s.document, Edit: workflowplugin.ChangeLocalsEdit, Values: s.workflow.Locals().Clone(),
		}, true
	case target.Document == worldstate.ID():
		return valuetree.World(docs)
	}

	return valuetree.Owned(docs, target.Document)
}

// declaredWhere names where a value or constant just declared is changed.
func declaredWhere(scope worldstate.Scope, constant bool) string {
	switch {
	case scope == worldstate.ScopeLocal:
		return "Local values, with no node selected"
	case constant:
		return "World state's Constants window"
	}

	return "World state's Values window"
}

// operandDeclaration returns the declaration an operand follows, if its
// value is chosen and found.
func (s *State) operandDeclaration(docs frame.Docs, node flow.Node, param string) (worldstate.Declaration, bool) {
	ref, err := node.Ref(flow.OperandOf(node.Kind, param))
	if err != nil {
		return worldstate.New(ids.ULID{}, "", "", ""), false
	}
	declared, _, problem := s.resolve(docs, ref)

	return declared, declared.Type.IsValue() && declared.Type != "" && problem != problemNoValue &&
		problem != problemValueGone
}

// unlimited reports whether a Check value asks about a limit, with nothing
// to compare with.
func unlimited(node flow.Node) bool {
	return node.Kind == flow.KindCheckValue && flow.Unlimited(choiceOf(node, "compare"))
}

// operandShape says what an operand's control is made of: its value's type
// and choices, or nothing when a Check value asks about a limit. The form
// is rebuilt when it changes.
func operandShape(node flow.Node, declared worldstate.Declaration, found bool) string {
	if unlimited(node) {
		return "limit"
	}
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
// field for an integer or text, false and true, or a button for each
// choice; none when a Check value asks about a limit.
func (s *State) newOperandControl(made *control, node flow.Node, declared worldstate.Declaration, found bool) {
	made.operands, made.operandType, made.unneeded = nil, "", unlimited(node)
	if !found || made.unneeded {
		return
	}
	made.operandType = declared.Type
	switch declared.Type {
	case worldstate.TypeInteger, worldstate.TypeText:
		made.field = ui.NewTextField(operandPrefix+made.param.Name, textedit.NewField("", false, worldstate.MaxText), false,
			s.theme)
		s.focus.Register(made.field)
	case worldstate.TypeBool:
		for _, truth := range []bool{false, true} {
			value := worldstate.NoValue()
			value.Type, value.Bool = worldstate.TypeBool, truth
			label := declared.Label(value)
			made.choices = append(made.choices, ui.NewButton(operandPrefix+made.param.Name+"-"+label, label))
			made.operands = append(made.operands, value.Encode())
		}
	case worldstate.TypeChoice:
		for _, choice := range declared.Choices {
			value := worldstate.NoValue()
			value.Type, value.Choice = worldstate.TypeChoice, choice.ID
			made.choices = append(made.choices, ui.NewButton(operandPrefix+made.param.Name+"-"+choice.ID, choice.Name))
			made.operands = append(made.operands, value.Encode())
		}
	case worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeGroup, worldstate.TypeTemplate,
		worldstate.TypeZone, worldstate.TypeWorkflow, worldstate.TypeSprite:
	}
}

// setSavedOperand shows an operand's saved value: a field's text, the
// button or buttons on, and whether an integer's is a number or another
// value. A choice's Among with nothing in it shows the one choice an older
// is compared with.
func setSavedOperand(made *control, node flow.Node) {
	saved, err := node.Operand(made.param.Name)
	if err != nil {
		return
	}
	if made.param.Name == flow.ParamAmong && flow.IsNoOperand(saved) {
		saved = savedOperand(node, flow.ParamAgainst)
	}
	if len(made.sources) == ui.Half {
		made.sources[0].On, made.sources[1].On = !made.picksValue(), made.picksValue()
	}
	if made.field != nil {
		var integer int
		var text string
		switch {
		case made.operandType == worldstate.TypeText && json.Unmarshal(saved, &text) == nil:
			made.field.Field.SetSaved(text)
		case made.operandType == worldstate.TypeInteger && json.Unmarshal(saved, &integer) == nil:
			made.field.Field.SetSaved(strconv.Itoa(integer))
		}
	}
	items := flow.Operands(saved)
	for index, button := range made.choices {
		button.On = index < len(made.operands) && slices.ContainsFunc(items, func(item json.RawMessage) bool {
			return string(item) == string(made.operands[index])
		})
	}
}

// layoutOperand places an operand's field or buttons, with A number and
// Another value over an integer's and the chooser when it refers to a
// value, or leaves room for a note while its value is not chosen. It
// returns where the next control goes.
func (s *State) layoutOperand(made *control, left, width, top int) int {
	made.clearRects()
	if len(made.sources) > 0 {
		top = s.layoutButtonRows(made.sources, left, width, top)
	}
	if made.picksValue() {
		return layoutValue(made, left, width, top)
	}
	if made.operandType.IsRef() {
		return layoutRef(made, left, width, top)
	}
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

// updateOperand saves an operand's false, true or choice when its button is
// clicked, ticks a choice in a list, and handles A number, Another value
// and the chooser of an integer's operand.
func (s *State) updateOperand(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	if s.updateOperandSource(input, ctx, made, node) {
		return
	}
	if made.picksValue() {
		s.updateOperandChooser(input, ctx, made, node)

		return
	}
	if made.operandType.IsRef() {
		s.updateRef(input, ctx, made, node)

		return
	}
	for index, button := range made.choices {
		if !button.Update(input) || index >= len(made.operands) {
			continue
		}
		switch {
		case made.param.Name == flow.ParamAmong:
			s.saveAmong(ctx, node, made, index)

			return
		case button.On:
		case node.Kind == flow.KindCheckValue && made.param.Name == flow.ParamAgainst:
			s.saveAgainst(ctx, node, made, made.operands[index])
		default:
			s.saveParam(ctx, node, made.param.Name, made.operands[index])
		}
	}
}

// parseOperand reads an integer or text typed into an operand's field.
func parseOperand(made *control, field *ui.TextField, value string) (any, bool, string) {
	if field != made.field || made.field == nil {
		return nil, false, ""
	}
	if made.operandType == worldstate.TypeText {
		return value, true, ""
	}

	return parseWhole(value, -worldstate.MaxInteger, worldstate.MaxInteger)
}

// drawValue draws a value parameter: its path, or what is wrong with it,
// and the chooser when open.
func (s *State) drawValue(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node) {
	ref, err := node.Ref(made.param.Name)
	label, ink, detail := "?", ui.ColorWarn(), ""
	if err == nil {
		declared, path, problem := s.resolve(docs, ref)
		switch {
		case ref.IsZero() && roleOf(node.Kind, made.param.Name) == roleWhere:
			label, ink = "Any NPC", ui.ColorInk()
		case ref.IsZero():
			label, ink = chooseValueLabel, ui.ColorMuted()
		case problem != "" && path == "":
			label = "Broken: " + strings.ToLower(strings.TrimSuffix(problem, "."))
		default:
			label, ink, detail = path, ui.ColorInk(), valueDetail(declared)
		}
	}
	open := s.values != nil && !s.values.operand && s.values.param == made.param.Name
	s.drawPickBox(screen, made.cueBox, label, ink, open)
	if detail != "" {
		s.theme.DrawText(screen, s.clip(detail, made.cueBox.Dx()), made.cueBox.Min.X, made.cueBox.Max.Y+ui.Gap,
			ui.ColorMuted())
	}
}

// drawPickBox draws a box that opens the picker: its label, a caret, and a
// blue outline while the picker it opens is open.
func (s *State) drawPickBox(screen *ebiten.Image, box image.Rectangle, label string, ink color.Color, open bool) {
	ui.Box(screen, box, ui.ColorField())
	if open {
		ui.StrokeRound(screen, box, ui.Radius, ui.Border, ui.ColorSelect())
	}
	textTop := box.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	caret := image.Rect(box.Max.X-textPad-caretWidth, box.Min.Y, box.Max.X-textPad, box.Max.Y)
	ui.DrawTriangle(screen, caret, true, ui.ColorMuted())
	s.theme.DrawMono(screen, s.clip(label, caret.Min.X-box.Min.X-ui.Half*textPad), box.Min.X+textPad, textTop, ink)
}

// valueDetail is the one line shown under a chosen value: its type, an
// integer's limits, and its description.
func valueDetail(declared worldstate.Declaration) string {
	if declared.Description == "" {
		return typeDetail(declared)
	}

	return typeDetail(declared) + " · " + declared.Description
}

// typeDetail is a value's type and an integer's limits.
func typeDetail(declared worldstate.Declaration) string { return valuepicker.TypeDetail(declared) }

// drawOperand draws an operand's field or buttons, or says why it has none.
func (s *State) drawOperand(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node, left int) {
	for _, button := range made.sources {
		button.Draw(screen, s.theme)
	}
	if made.picksValue() {
		s.drawOperandChooser(screen, docs, made, node)

		return
	}
	if made.field != nil {
		made.field.Draw(screen)

		return
	}
	if made.operandType.IsRef() {
		s.drawRef(screen, docs, made, node)

		return
	}
	for _, button := range made.choices {
		button.Draw(screen, s.theme)
	}
	switch {
	case made.unneeded:
		s.theme.DrawText(screen, "Nothing to compare with.", left, made.label+ui.LineHeight, ui.ColorMuted())
	case len(made.choices) == 0:
		s.theme.DrawText(screen, "Choose its value first.", left, made.label+ui.LineHeight, ui.ColorMuted())
	}
}

// valueProblem says what is wrong with a Set value, Check value or Find NPC
// nearby.
func (s *State) valueProblem(node flow.Node, docs frame.Docs) string {
	switch node.Kind {
	case flow.KindFindNPC:
		return s.findProblem(node, docs)
	case flow.KindLog, flow.KindSay, flow.KindShowMessage:
		return s.logProblem(node, docs)
	}
	ref, err := node.Ref(flow.ParamValue)
	if err != nil {
		return problemNoValue
	}
	declared, path, problem := s.resolve(docs, ref)
	if problem != "" {
		return problem
	}
	if problem := kindProblem(node, ref, declared); problem != "" {
		return problem
	}
	if missing := s.missing(docs, ref, path); missing != "" {
		return missing
	}
	if node.Kind == flow.KindCheckValue {
		return s.checkProblem(docs, node, declared)
	}
	if !s.operandsFit(docs, node, declared) {
		return problemOperand
	}

	return ""
}

// kindProblem says what a Set value or Check value cannot do with its
// value's kind: set another's, set or check an NPC, add to or compare
// anything but an integer, or ask about a limit it lacks.
func kindProblem(node flow.Node, ref flow.ValueRef, declared worldstate.Declaration) string {
	compare := choiceOf(node, "compare")
	if problem := setProblem(node, ref, declared); problem != "" {
		return problem
	}
	switch {
	case declared.Type == worldstate.TypeNPC:
		return problemNPCValue
	case node.Kind == flow.KindSetValue && choiceOf(node, "how") == flow.HowAdd && declared.Type != worldstate.TypeInteger:
		return problemAddInteger
	case node.Kind != flow.KindCheckValue:
		return ""
	case !flow.CanCompare(compare, declared.Type):
		return "A " + declared.Type.Label() + " cannot be compared that way. Choose a comparison it takes."
	case compare == flow.CompareMax && declared.Max == nil:
		return "It has no maximum to be at. Give it one in World state's Values window."
	case compare == flow.CompareMin && declared.Min == nil:
		return "It has no minimum to be at. Give it one in World state's Values window."
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

// compareWords says a Check value's comparison in words.
func compareWords(node flow.Node) string {
	switch choiceOf(node, flow.ParamCompare) {
	case flow.CompareNot:
		return "is not"
	case flow.CompareLeast:
		return "at least"
	case flow.CompareMost:
		return "at most"
	case flow.CompareAbove:
		return "above"
	case flow.CompareBelow:
		return "below"
	case flow.CompareBetween:
		return "between"
	case flow.CompareMax:
		return "at its maximum"
	case flow.CompareMin:
		return "at its minimum"
	case flow.CompareAnyOf:
		return "is any of"
	case flow.CompareNoneOf:
		return "is none of"
	}

	return "is"
}

// valueSummary describes a value node in a few words.
func (s *State) valueSummary(node flow.Node, docs frame.Docs) string {
	switch {
	case hasMessage(node.Kind):
		return s.logSummary(docs, node)
	case node.Kind == flow.KindShake:
		return shakeSummary(node)
	case node.Kind == flow.KindSendSignal || node.Kind == flow.KindResume:
		return s.signalSummary(node, docs)
	}
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
	operand := func(param string) string { return s.operandLabel(docs, node, param) }
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
		phrase, is := s.checkPhrase(docs, node)
		if is {
			phrase = "is " + phrase
		}

		return path(flow.ParamValue) + " " + phrase
	case flow.KindFindNPC:
		return s.findSummary(node, path, operand)
	}

	return ""
}

// operandLabel shows an operand as its value's type shows it: the value,
// text in quotes, the path of a value it refers to, or a list's values
// joined with commas; "?" for one that does not fit.
func (s *State) operandLabel(docs frame.Docs, node flow.Node, param string) string {
	declared, found := s.operandDeclaration(docs, node, param)
	saved, err := node.Operand(param)
	if !found || err != nil {
		return "?"
	}
	if ref, refers := flow.OperandRef(saved); refers {
		if _, path, problem := s.resolve(docs, ref); problem == "" {
			return path
		}

		return "?"
	}
	var labels []string
	for _, item := range flow.Operands(saved) {
		value, parseErr := declared.Parse(item)
		if parseErr != nil {
			return "?"
		}
		label := declared.Label(value)
		switch {
		case declared.Type == worldstate.TypeText:
			label = textLabel(value.Text)
		case declared.Type.IsRef():
			label = worldstate.Text(declared, value, s.namer(docs))
		}
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		return "?"
	}

	return strings.Join(labels, ", ")
}

// textLabel shows text on a node as it was typed, in curly quotes, cut
// with an ellipsis when long; empty text reads as empty.
func textLabel(text string) string {
	if text == "" {
		return "empty"
	}
	if runes := []rune(text); len(runes) > textShown {
		text = string(runes[:textShown-1]) + "…"
	}

	return "“" + text + "”"
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

// hasValueState answers "value:<param>=<path>" for the selected node's
// value parameter, "operand:<param>=<saved>" for its operand and
// "title:<kind>:<n>=<title>" for the nth node of a kind's title.
func (s *State) hasValueState(state string, docs frame.Docs) (bool, bool) {
	if found, handled := s.hasPickerState(state, docs); handled {
		return found, true
	}
	if found, handled := s.hasOperandState(state, docs); handled {
		return found, true
	}
	node, selected := s.selectedNode()
	switch {
	case strings.HasPrefix(state, "title:"):
		which, want, _ := strings.Cut(strings.TrimPrefix(state, "title:"), "=")
		drawn, found := s.nthNode(which)

		title := s.valueTitle(drawn.node, docs)
		if kind := drawn.node.Kind; kind != flow.KindSetValue && kind != flow.KindCheckValue && kind != flow.KindFindNPC {
			title = s.nodeLabel(drawn.node, docs)
		}

		return found && title == want, true
	case strings.HasPrefix(state, "value:"):
		param, want, _ := strings.Cut(strings.TrimPrefix(state, "value:"), "=")
		ref, err := node.Ref(param)
		if !selected || err != nil {
			return false, true
		}
		_, path, problem := s.resolve(docs, ref)

		return problem == "" && samePath(path, want), true
	case strings.HasPrefix(state, "operand:"):
		param, want, _ := strings.Cut(strings.TrimPrefix(state, "operand:"), "=")
		saved, err := node.Operand(param)

		return selected && err == nil && string(saved) == want, true
	}

	return false, false
}

// valueTitle names a value node on the canvas as a short sentence, so what
// it sets or asks fits its width: "Set npc.mood to cross", "Add 1 to
// local.tally", "Is npc.count at least 3?", "Is npc.health at its maximum?"
// or "Find NPC within 2 tiles".
func (s *State) valueTitle(node flow.Node, docs frame.Docs) string {
	kind, _ := flow.KindOf(node.Kind)
	path := ""
	if ref, err := node.Ref(flow.ParamValue); err == nil && !ref.IsZero() {
		_, path, _ = s.resolve(docs, ref)
		if path == "" {
			path = "?"
		}
	}
	operand := s.operandLabel(docs, node, operandOfValue(node.Kind, flow.ParamValue))
	switch {
	case node.Kind == flow.KindFindNPC:
		tiles, err := node.Number("within")
		if err != nil {
			return kind.Label
		}

		return "Find NPC within " + strconv.Itoa(tiles) + " " + plural(tiles, "tile", "tiles")
	case path == "":
		return kind.Label + " · no value"
	case node.Kind == flow.KindCheckValue:
		phrase, _ := s.checkPhrase(docs, node)

		return "Is " + path + " " + phrase + "?"
	case choiceOf(node, "how") == flow.HowAdd:
		return "Add " + operand + " to " + path
	}

	return "Set " + path + " to " + operand
}
