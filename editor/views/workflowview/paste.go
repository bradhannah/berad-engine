package workflowview

import (
	"bytes"
	"encoding/json"
	"image"
	"slices"
	"strconv"
	"strings"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// clipKind marks a copy of a workflow's nodes on the clipboard, so other
// text pasted is left alone.
const clipKind = "berad.workflow.nodes"

// The fields of a value reference that name a local value: the value read,
// and the local holding the NPC found nearby whose value is read.
const (
	refScope = "scope"
	refValue = "value"
	refFound = "found"
)

// copied is a copy of nodes as the clipboard holds it: the nodes, and the
// workflow's own local values they use.
type copied struct {
	Kind   string                   `json:"kind"`
	Nodes  []flow.Node              `json:"nodes"`
	Boxes  map[ids.ULID]flow.Box    `json:"boxes"`
	Tints  map[ids.ULID]string      `json:"tints,omitempty"`
	Locals []worldstate.Declaration `json:"locals,omitempty"`
}

// copySelection puts the selected nodes but the start on the clipboard,
// with the local values they use.
func (s *State) copySelection(ctx *frame.Context) {
	taken, ok := s.clipSelection()
	if !ok || ctx.Clipboard == nil {
		return
	}
	held := copied{Kind: clipKind, Nodes: taken.Nodes, Boxes: taken.Boxes, Tints: taken.Tints, Locals: nil}
	for _, id := range localsUsed(taken.Nodes) {
		if declared, found := s.workflow.Locals().Value(id); found {
			held.Locals = append(held.Locals, declared)
		}
	}
	data, err := json.Marshal(held)
	if err == nil {
		err = ctx.Clipboard.Write(string(data))
	}
	if err != nil {
		ctx.Frame.Status("Couldn't copy: " + err.Error())

		return
	}
	ctx.Frame.Status("Copied " + countNodes(len(taken.Nodes)) + ". Cmd+V pastes them into any workflow.")
}

// paste adds the nodes on the clipboard to this workflow, declaring the
// local values they use that it lacks, as one undo step. Text that is not
// a copy of nodes is left alone.
func (s *State) paste(ctx *frame.Context) {
	if ctx.Clipboard == nil {
		return
	}
	text, err := ctx.Clipboard.Read()
	var held copied
	if err != nil || json.Unmarshal([]byte(text), &held) != nil || held.Kind != clipKind || len(held.Nodes) == 0 {
		return
	}
	if kind, refused := s.notOffered(held.Nodes); refused {
		ctx.Frame.Refuse("This workflow has no " + kind + ", so nothing was pasted.")

		return
	}
	declare, same := s.localsFor(held.Locals)
	taken := clip{Nodes: nil, Boxes: held.Boxes, Tints: held.Tints}
	among := map[ids.ULID]bool{}
	for _, node := range held.Nodes {
		among[node.ID] = true
	}
	for _, node := range held.Nodes {
		if !among[node.Parent] {
			node.Parent = ids.ULID{}
		}
		node.Params = remapLocals(node.Params, same)
		taken.Nodes = append(taken.Nodes, node)
	}
	pasted, ways := s.settleWays(taken.Nodes)
	taken.Nodes = pasted
	gesture := s.gesture()
	said := "Pasted " + countNodes(len(taken.Nodes)) + "."
	if len(ways) > 0 {
		said += " New here: the way out " + strings.Join(ways, ", ") + "."
	}
	if len(declare) > 0 {
		s.command(ctx, workflowplugin.ChangeLocalsEdit, gesture,
			stateplugin.ChangeArgs{Add: declare, Set: nil, Remove: nil}, nil)
		said += " Declared " + localNames(declare) + " here. Undo removes them with the nodes."
	}
	s.addCopy(ctx, taken.shifted(s.pasteAt().Sub(topLeft(taken))), gesture, said)
}

// settleWays gives pasted Returns the ways out their names say. A way out
// is known to its author by its name, and to Calls by its port, which means
// nothing outside its own workflow: a Return copied from another workflow
// can carry a port that here belongs to a way out of another name, and
// would silently become one more Return of that. So a pasted Return joins
// the way out of its name here, if there is one; otherwise it keeps its
// port when that is free, and takes a new one when it is not. Returns
// pasted together under one name stay one way out. It returns the nodes and
// the names of the ways out that are new here.
func (s *State) settleWays(nodes []flow.Node) ([]flow.Node, []string) {
	var added []string
	ports, taken := map[string]string{}, map[string]bool{}
	for _, node := range s.workflow.Nodes() {
		way, err := node.WayOut()
		if node.Kind != flow.KindReturn || err != nil {
			continue
		}
		taken[way.Port] = true
		if _, named := ports[way.Label]; !named && node.Parent.IsZero() {
			ports[way.Label] = way.Port
		}
	}
	for index, node := range nodes {
		way, err := node.WayOut()
		if node.Kind != flow.KindReturn || err != nil {
			continue
		}
		port, named := ports[way.Label]
		if !named {
			port = way.Port
			for number := 1; taken[port]; number++ {
				port = "w" + strconv.Itoa(number)
			}
			ports[way.Label], taken[port] = port, true
			if node.Parent.IsZero() {
				added = append(added, way.Label)
			}
		}
		if moved, problem := withPort(node, way.Label, port); problem == "" {
			nodes[index] = moved
		}
	}

	return nodes, added
}

// notOffered names the first kind of node among some that this workflow's
// purpose does not offer.
func (s *State) notOffered(nodes []flow.Node) (string, bool) {
	for _, node := range nodes {
		if !s.workflow.Purpose().Allows(node.Kind) {
			label := node.Kind
			if kind, ok := flow.KindOf(node.Kind); ok {
				label = kind.Label
			}

			return label, true
		}
	}

	return "", false
}

// localsFor works out the local values a paste needs: those to declare
// here, keeping their IDs, and those this workflow already has by the same
// name and type, which the nodes are pointed at instead. A name taken by a
// local of another type gets a number.
func (s *State) localsFor(needed []worldstate.Declaration) ([]worldstate.Declaration, map[ids.ULID]ids.ULID) {
	locals := s.workflow.Locals()
	same := map[ids.ULID]ids.ULID{}
	var declare []worldstate.Declaration
	for _, wanted := range needed {
		if _, here := locals.Value(wanted.ID); here {
			continue
		}
		if existing, found := localNamed(locals, wanted.Name); found && existing.Type == wanted.Type {
			same[wanted.ID] = existing.ID

			continue
		}
		wanted.Name = freeName(locals, declare, wanted.Name)
		wanted.Parent, wanted.Role = ids.ULID{}, worldstate.RoleLocal
		declare = append(declare, wanted)
	}

	return declare, same
}

// localNamed finds a workflow's local value by its name.
func localNamed(locals *worldstate.Declarations, name string) (worldstate.Declaration, bool) {
	for _, declared := range locals.Values() {
		if declared.Name == name && declared.Scope == worldstate.ScopeLocal {
			return declared, true
		}
	}

	return worldstate.New(ids.ULID{}, "", "", ""), false
}

// freeName is a name no local value has, here or about to be declared:
// the name itself, or it with the first free number, as count_2.
func freeName(locals *worldstate.Declarations, declaring []worldstate.Declaration, name string) string {
	taken := func(candidate string) bool {
		_, here := localNamed(locals, candidate)

		return here || slices.ContainsFunc(declaring, func(declared worldstate.Declaration) bool {
			return declared.Name == candidate
		})
	}
	if !taken(name) {
		return name
	}
	for number := 2; ; number++ {
		if candidate := name + "_" + strconv.Itoa(number); !taken(candidate) {
			return candidate
		}
	}
}

// localNames lists local values' names for the status bar.
func localNames(declared []worldstate.Declaration) string {
	names := make([]string, 0, len(declared))
	for _, each := range declared {
		names = append(names, each.Name)
	}

	return strings.Join(names, ", ")
}

// pasteAt is where a paste's top left goes: under the pointer when it is
// over the canvas, or else the middle of what the canvas shows.
func (s *State) pasteAt() image.Point {
	canvas := s.Canvas()
	if s.cursor.In(canvas) {
		return s.toCanvas(s.cursor)
	}

	return s.toCanvas(ui.Center(canvas))
}

// topLeft is the top left of where a clip's nodes sit.
func topLeft(taken clip) image.Point {
	first, corner := true, image.Pt(0, 0)
	for _, box := range taken.Boxes {
		if first || box.X < corner.X {
			corner.X = box.X
		}
		if first || box.Y < corner.Y {
			corner.Y = box.Y
		}
		first = false
	}

	return corner
}

// localsUsed lists the local values nodes use, by ID, in the order first
// used.
func localsUsed(nodes []flow.Node) []ids.ULID {
	var used []ids.ULID
	for _, node := range nodes {
		for _, name := range sortedParams(node) {
			walkRefs(decodeParam(node.Params[name]), func(ref map[string]any) {
				for _, local := range localsIn(ref) {
					if !slices.Contains(used, local.id) {
						used = append(used, local.id)
					}
				}
			})
		}
	}

	return used
}

// remapLocals points a node's parameters at other local values, by ID.
func remapLocals(params map[string]json.RawMessage, same map[ids.ULID]ids.ULID) map[string]json.RawMessage {
	if len(same) == 0 {
		return params
	}
	remapped := make(map[string]json.RawMessage, len(params))
	for name, raw := range params {
		value := decodeParam(raw)
		changed := false
		walkRefs(value, func(ref map[string]any) {
			for _, local := range localsIn(ref) {
				if other, found := same[local.id]; found {
					ref[local.field], changed = other.String(), true
				}
			}
		})
		remapped[name] = raw
		if data, err := json.Marshal(value); changed && err == nil {
			remapped[name] = data
		}
	}

	return remapped
}

// localField is a field of a value reference that names a local value, and
// the local's ID.
type localField struct {
	field string
	id    ids.ULID
}

// localsIn are the fields of a value reference that name a local value:
// its value when it is a local, and the local holding the NPC found
// nearby.
func localsIn(ref map[string]any) []localField {
	scope, _ := ref[refScope].(string) //nolint:errcheck // Not text is no scope.
	var fields []localField
	for _, field := range []string{refValue, refFound} {
		text, named := ref[field].(string)
		id, err := ids.Parse(text)
		if !named || err != nil || field == refValue && scope != string(worldstate.ScopeLocal) {
			continue
		}
		fields = append(fields, localField{field: field, id: id})
	}

	return fields
}

// walkRefs visits every value reference within a parameter's value: each
// object with a scope.
func walkRefs(value any, visit func(map[string]any)) {
	switch typed := value.(type) {
	case map[string]any:
		if _, isRef := typed[refScope]; isRef {
			visit(typed)
		}
		for _, inner := range typed {
			walkRefs(inner, visit)
		}
	case []any:
		for _, inner := range typed {
			walkRefs(inner, visit)
		}
	}
}

// decodeParam reads a parameter's value, keeping its numbers as written.
func decodeParam(raw json.RawMessage) any {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil
	}

	return value
}

// sortedParams are a node's parameter names in order.
func sortedParams(node flow.Node) []string {
	names := make([]string, 0, len(node.Params))
	for name := range node.Params {
		names = append(names, name)
	}
	slices.Sort(names)

	return names
}
