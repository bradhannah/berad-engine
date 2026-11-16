// Package workflow is the editor half of workflows: the workflow document,
// with its layout kept in a file of its own (D17), and the edits that add,
// remove, change and move its nodes and set the zone it runs in.
package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// PluginName identifies the plug-in.
const PluginName = "workflow"

// Edit kind names.
const (
	AddEdit     = "workflow.nodes.add"
	RemoveEdit  = "workflow.nodes.remove"
	ReplaceEdit = "workflow.node.replace"
	MoveEdit    = "workflow.layout.move"
	// TintEdit colours nodes, containers, outcomes and lines: layout, like
	// a move.
	TintEdit = "workflow.layout.tint"
	// ChangeLocalsEdit declares, changes and removes a workflow's local
	// values, as the world state's change edit does the project's.
	ChangeLocalsEdit = "workflow.locals.change"
	// SetRunsInEdit sets the zone a workflow runs in, or any zone.
	SetRunsInEdit = "workflow.runs-in.set"
)

// RunsHere labels a workflow's reference to the zone it runs in, so the
// zone's uses list the workflows that run there.
const RunsHere = "runs here"

// runsInItem is the undo-clash item for the zone a workflow runs in.
const runsInItem = "runs-in"

// ErrNotWorkflow reports a document that is not a workflow.
var ErrNotWorkflow = errors.New("document is not a workflow")

// Document is a workflow held by the editor.
type Document struct {
	Workflow *flow.Workflow
}

// NewData returns the data of a new NPC behaviour, as NewDataFor does.
func NewData() json.RawMessage { return NewDataFor(flow.PurposeBehaviour) }

// NewDataFor returns the data of a new workflow of a kind, running in any
// zone and holding only its start node, whose ID the orchestrator assigns
// when the workflow is created.
func NewDataFor(purpose flow.Purpose) json.RawMessage {
	start := flow.Node{ID: ids.ULID{}, Kind: flow.KindStart, Parent: ids.ULID{}, Params: nil, Next: nil}
	data, err := json.Marshal(wireWorkflow{
		Start: ids.ULID{}, Nodes: []flow.Node{start}, Purpose: purpose, Zone: ids.ULID{},
	})
	if err != nil {
		panic(err)
	}

	return data
}

type wireWorkflow struct {
	Start   ids.ULID     `json:"start"`
	Nodes   []flow.Node  `json:"nodes"`
	Purpose flow.Purpose `json:"purpose,omitempty"`
	Zone    ids.ULID     `json:"zone,omitzero"`
}

// Format names the document format.
func (d *Document) Format() string { return flow.FormatName }

// Kind is the workflow's kind, as the project index lists it.
func (d *Document) Kind() string { return string(d.Workflow.Purpose()) }

// Encode returns the workflow's logic.
func (d *Document) Encode() (json.RawMessage, error) {
	data, err := d.Workflow.Encode()
	if err != nil {
		return nil, fmt.Errorf("workflow document: %w", err)
	}

	return data, nil
}

// EncodeLayout returns where the nodes sit.
func (d *Document) EncodeLayout() (json.RawMessage, error) {
	data, err := d.Workflow.EncodeLayout()
	if err != nil {
		return nil, fmt.Errorf("workflow document: %w", err)
	}

	return data, nil
}

// DecodeLayout reads where the nodes sit.
func (d *Document) DecodeLayout(data json.RawMessage) error {
	if err := d.Workflow.DecodeLayout(data); err != nil {
		return fmt.Errorf("workflow document: %w", err)
	}

	return nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *Document) Clone() model.Document {
	return &Document{Workflow: d.Workflow.Clone()}
}

// References lists the anchors the workflow's Go to nodes lead to, the
// cues its Play sound nodes play, the world-state values and NPCs its
// value nodes use and its Logs write, the workflows its Calls call, as
// CallItem, with the values they give and keep, and the zone it runs in.
func (d *Document) References() []model.Reference {
	var references []model.Reference
	if zone := d.Workflow.RunsIn(); !zone.IsZero() {
		references = append(references, model.Reference{Document: zone, Item: "", Label: RunsHere})
	}
	for _, node := range d.Workflow.Nodes() {
		references = append(references, numberReferences(node)...)
		switch node.Kind {
		case flow.KindGoTo:
			destination, err := node.Anchor("destination")
			if err != nil || destination.IsZero() {
				continue
			}
			references = append(references, model.Reference{
				Document: destination.Floor, Item: destination.Anchor.String(), Label: "a Go to node",
			})
		case flow.KindPlaySound:
			cue, err := node.Cue("cue")
			if err != nil || cue == "" {
				continue
			}
			references = append(references, model.Reference{
				Document: model.AudioLibraryID(), Item: cue, Label: "a Play sound node",
			})
		case flow.KindSetValue, flow.KindCheckValue, flow.KindFindNPC:
			references = append(references, valueReferences(node)...)
		case flow.KindCall:
			references = append(references, callReferences(node)...)
		case flow.KindLog, flow.KindSay, flow.KindShowMessage:
			references = append(references, messageReferences(node)...)
		}
	}

	return references
}

// Decode reads saved workflow logic.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func Decode(data json.RawMessage) (model.Document, error) {
	decoded, err := flow.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("workflow document: %w", err)
	}

	return &Document{Workflow: decoded}, nil
}

// Prepare gives a new workflow's start node its permanent ID (D4).
func Prepare(data json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	var wire wireWorkflow
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	if !wire.Start.IsZero() {
		return data, nil
	}
	start, err := newID()
	if err != nil {
		return nil, err
	}
	wire.Start = start
	for index := range wire.Nodes {
		if wire.Nodes[index].Kind == flow.KindStart && wire.Nodes[index].ID.IsZero() {
			wire.Nodes[index].ID = start
		}
	}
	prepared, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode workflow: %w", err)
	}

	return prepared, nil
}

// AddArgs adds nodes, where they sit, their tints, and links to them from
// nodes already there. A node with a zero ID is new: preparing the edit
// gives it a permanent ID, and its box may be given under the zero ID.
// Fresh adds a copy: preparing gives every node a new ID, and the parents,
// lines, boxes and tints among them follow it; a line to a node not among
// them is dropped.
type AddArgs struct {
	Nodes []flow.Node           `json:"nodes"`
	Boxes map[ids.ULID]flow.Box `json:"boxes,omitempty"`
	Links []flow.Link           `json:"links,omitempty"`
	Tints map[ids.ULID]string   `json:"tints,omitempty"`
	Fresh bool                  `json:"fresh,omitempty"`
}

// RemoveArgs removes nodes; a container takes its contents with it.
type RemoveArgs struct {
	Nodes []ids.ULID `json:"nodes"`
}

// ReplaceArgs changes one node's parameters, links and container.
type ReplaceArgs struct {
	Node flow.Node `json:"node"`
}

// MoveArgs places nodes. A box whose width is flow.Unplaced removes a node's
// place, letting the window place it.
type MoveArgs struct {
	Boxes map[ids.ULID]flow.Box `json:"boxes"`
	// Lines sets the bends of lines moved by hand, by line key; a bend with
	// no To removes it.
	Lines map[string]flow.Bend `json:"lines,omitempty"`
}

// Add is the edit kind that adds nodes.
type Add struct{}

// Name identifies the edit kind.
func (Add) Name() string { return AddEdit }

// Format names the document format the edit applies to.
func (Add) Format() string { return flow.FormatName }

// Prepare gives new nodes their permanent IDs.
func (Add) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	var add AddArgs
	if err := json.Unmarshal(args, &add); err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	if add.Fresh {
		return prepareCopy(add, newID)
	}
	changed := false
	for index := range add.Nodes {
		if !add.Nodes[index].ID.IsZero() {
			continue
		}
		assigned, err := newID()
		if err != nil {
			return nil, err
		}
		if box, ok := add.Boxes[ids.ULID{}]; ok {
			delete(add.Boxes, ids.ULID{})
			add.Boxes[assigned] = box
		}
		add.Nodes[index].ID = assigned
		changed = true
	}
	if !changed {
		return args, nil
	}
	prepared, err := json.Marshal(add)
	if err != nil {
		return nil, fmt.Errorf("encode add: %w", err)
	}

	return prepared, nil
}

// prepareCopy gives every node of a copy a new ID, which its parent, lines,
// box and tint among the copy follow.
func prepareCopy(add AddArgs, newID model.IDSource) (json.RawMessage, error) {
	fresh := make(map[ids.ULID]ids.ULID, len(add.Nodes))
	for _, node := range add.Nodes {
		assigned, err := newID()
		if err != nil {
			return nil, err
		}
		fresh[node.ID] = assigned
	}
	copied := AddArgs{
		Nodes: make([]flow.Node, 0, len(add.Nodes)), Boxes: map[ids.ULID]flow.Box{}, Links: nil,
		Tints: map[ids.ULID]string{}, Fresh: false,
	}
	for _, node := range add.Nodes {
		node = node.Clone()
		node.ID = fresh[node.ID]
		if parent, among := fresh[node.Parent]; among {
			node.Parent = parent
		}
		for output, target := range node.Next {
			if to, among := fresh[target]; among {
				node.Next[output] = to
			} else {
				delete(node.Next, output)
			}
		}
		copied.Nodes = append(copied.Nodes, node)
	}
	for old, box := range add.Boxes {
		if id, among := fresh[old]; among {
			copied.Boxes[id] = box
		}
	}
	for old, tint := range add.Tints {
		if id, among := fresh[old]; among {
			copied.Tints[id] = tint
		}
	}
	prepared, err := json.Marshal(copied)
	if err != nil {
		return nil, fmt.Errorf("encode copy: %w", err)
	}

	return prepared, nil
}

// Apply adds the nodes, with their tints.
func (Add) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var add AddArgs

	return change(document, args, &add, func(workflow *flow.Workflow) (string, any, []string, error) {
		for _, node := range add.Nodes {
			if err := checkNode(node); err != nil {
				return "", nil, nil, err
			}
		}
		if err := workflow.Insert(add.Nodes, add.Boxes, add.Links); err != nil {
			return "", nil, nil, fmt.Errorf("add nodes: %w", err)
		}
		if len(add.Tints) > 0 {
			if _, err := workflow.SetTints(flow.Tints{Nodes: add.Tints, Outcomes: nil, Lines: nil}); err != nil {
				return "", nil, nil, fmt.Errorf("tint nodes: %w", err)
			}
		}
		added := make([]ids.ULID, 0, len(add.Nodes))
		for _, node := range add.Nodes {
			added = append(added, node.ID)
		}

		return RemoveEdit, RemoveArgs{Nodes: added}, touched(added, add.Links), nil
	})
}

// Remove is the edit kind that removes nodes.
type Remove struct{}

// Name identifies the edit kind.
func (Remove) Name() string { return RemoveEdit }

// Format names the document format the edit applies to.
func (Remove) Format() string { return flow.FormatName }

// Apply removes the nodes; its inverse puts them back with their links.
func (Remove) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var remove RemoveArgs

	return change(document, args, &remove, func(workflow *flow.Workflow) (string, any, []string, error) {
		removed, boxes, links, err := workflow.Remove(remove.Nodes)
		if err != nil {
			return "", nil, nil, fmt.Errorf("remove nodes: %w", err)
		}
		gone := make([]ids.ULID, 0, len(removed))
		for _, node := range removed {
			gone = append(gone, node.ID)
		}

		restored := AddArgs{Nodes: removed, Boxes: boxes, Links: links, Tints: nil, Fresh: false}

		return AddEdit, restored, touched(gone, links), nil
	})
}

// Replace is the edit kind that changes one node.
type Replace struct{}

// Name identifies the edit kind.
func (Replace) Name() string { return ReplaceEdit }

// Format names the document format the edit applies to.
func (Replace) Format() string { return flow.FormatName }

// Apply replaces the node.
func (Replace) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var replace ReplaceArgs

	return change(document, args, &replace, func(workflow *flow.Workflow) (string, any, []string, error) {
		if err := checkNode(replace.Node); err != nil {
			return "", nil, nil, err
		}
		previous, err := workflow.Replace(replace.Node)
		if err != nil {
			return "", nil, nil, fmt.Errorf("replace node: %w", err)
		}

		return "", ReplaceArgs{Node: previous}, []string{NodeItem(previous.ID)}, nil
	})
}

// Move is the edit kind that places nodes on the canvas.
type Move struct{}

// Name identifies the edit kind.
func (Move) Name() string { return MoveEdit }

// Format names the document format the edit applies to.
func (Move) Format() string { return flow.FormatName }

// Apply places the nodes and bends the lines, all or nothing; its inverse
// puts both back.
func (Move) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var move MoveArgs

	return change(document, args, &move, func(workflow *flow.Workflow) (string, any, []string, error) {
		working := workflow.Clone()
		previous, err := working.Move(move.Boxes)
		if err != nil {
			return "", nil, nil, fmt.Errorf("move nodes: %w", err)
		}
		previousLines, err := working.SetBends(move.Lines)
		if err != nil {
			return "", nil, nil, fmt.Errorf("move lines: %w", err)
		}
		*workflow = *working
		items := make([]string, 0, len(previous)+len(previousLines))
		for _, id := range slices.SortedFunc(maps.Keys(previous), ids.ULID.Compare) {
			items = append(items, BoxItem(id))
		}
		for _, key := range slices.Sorted(maps.Keys(previousLines)) {
			items = append(items, LineItem(key))
		}
		inverse := MoveArgs{Boxes: previous, Lines: nil}
		if len(previousLines) > 0 {
			inverse.Lines = previousLines
		}

		return "", inverse, items, nil
	})
}

// TintArgs colours nodes, containers, outcomes and lines by the name of a
// tint; an empty name takes a colour away.
type TintArgs struct {
	Tints flow.Tints `json:"tints"`
}

// Tint is the edit kind that colours parts of a workflow.
type Tint struct{}

// Name identifies the edit kind.
func (Tint) Name() string { return TintEdit }

// Format names the document format the edit applies to.
func (Tint) Format() string { return flow.FormatName }

// Apply sets the tints, all or nothing; its inverse sets what they were.
func (Tint) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var tint TintArgs

	return change(document, args, &tint, func(workflow *flow.Workflow) (string, any, []string, error) {
		previous, err := workflow.SetTints(tint.Tints)
		if err != nil {
			return "", nil, nil, fmt.Errorf("tint: %w", err)
		}
		var items []string
		for _, id := range slices.SortedFunc(maps.Keys(previous.Nodes), ids.ULID.Compare) {
			items = append(items, TintItem("node:"+id.String()))
		}
		for _, key := range slices.Sorted(maps.Keys(previous.Outcomes)) {
			items = append(items, TintItem("outcome:"+key))
		}
		for _, key := range slices.Sorted(maps.Keys(previous.Lines)) {
			items = append(items, TintItem("line:"+key))
		}

		return "", TintArgs{Tints: previous}, items, nil
	})
}

// SetRunsInArgs sets the zone a workflow runs in; zero is any zone.
type SetRunsInArgs struct {
	Zone ids.ULID `json:"zone"`
}

// SetRunsIn is the edit kind that sets the zone a workflow runs in.
type SetRunsIn struct{}

// Name identifies the edit kind.
func (SetRunsIn) Name() string { return SetRunsInEdit }

// Format names the document format the edit applies to.
func (SetRunsIn) Format() string { return flow.FormatName }

// Apply sets the zone; its inverse sets the zone it ran in.
func (SetRunsIn) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var set SetRunsInArgs

	return change(document, args, &set, func(workflow *flow.Workflow) (string, any, []string, error) {
		return "", SetRunsInArgs{Zone: workflow.SetRunsIn(set.Zone)}, []string{runsInItem}, nil
	})
}

// TintItem names a part's tint for undo-clash detection.
func TintItem(key string) string { return "tint:" + key }

// LineItem names a line's bend for undo-clash detection.
func LineItem(key string) string { return "line:" + key }

// NodeItem names a node's logic for undo-clash detection.
func NodeItem(id ids.ULID) string { return "node:" + id.String() }

// BoxItem names a node's place on the canvas for undo-clash detection.
func BoxItem(id ids.ULID) string { return "box:" + id.String() }

// checkNode refuses a node whose parameters or outputs do not fit its kind.
func checkNode(node flow.Node) error {
	if err := flow.CheckParams(node); err != nil {
		return fmt.Errorf("check node: %w", err)
	}
	outputs := flow.Outputs(node)
	for output := range node.Next {
		if !slices.ContainsFunc(outputs, func(declared flow.Output) bool { return declared.Name == output }) {
			return fmt.Errorf("%w: %s has no output %q", flow.ErrNode, node.Kind, output)
		}
	}

	return nil
}

// touched lists the nodes an edit adds or removes, with their boxes, and the
// nodes whose links it changes.
func touched(nodes []ids.ULID, links []flow.Link) []string {
	items := make([]string, 0, len(nodes)*2+len(links))
	for _, id := range nodes {
		items = append(items, NodeItem(id), BoxItem(id))
	}
	for _, link := range links {
		items = append(items, NodeItem(link.From))
	}

	return items
}

func change(
	document model.Document, args json.RawMessage, target any,
	apply func(*flow.Workflow) (string, any, []string, error),
) (model.Applied, error) {
	workflowDoc, ok := document.(*Document)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotWorkflow)
	}
	if err := json.Unmarshal(args, target); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverseEdit, inverse, items, err := apply(workflowDoc.Workflow)
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverseArgs, err := json.Marshal(inverse)
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}

	return model.Applied{Inverse: inverseArgs, InverseEdit: inverseEdit, Touched: items, Covers: nil}, nil
}

// valueReferences lists the values and constants a value node uses, by ID
// under the world state, wherever they are declared, and any NPC or zone it
// names: the value it sets, checks or looks for, and any its operands refer
// to. Local values are the workflow's own.
func valueReferences(node flow.Node) []model.Reference {
	kind, _ := flow.KindOf(node.Kind)

	return refReferences(valueRefs(node), "a "+kind.Label+" node")
}

// valueRefs lists every value a value node uses, its own and its operands'.
func valueRefs(node flow.Node) []flow.ValueRef {
	kind, _ := flow.KindOf(node.Kind)
	var refs []flow.ValueRef
	for _, param := range kind.Params {
		if param.Type == flow.TypeValue {
			if ref, err := node.Ref(param.Name); err == nil {
				refs = append(refs, ref)
			}
		}
		if param.Type != flow.TypeOperand {
			continue
		}
		raw, err := node.Operand(param.Name)
		if err != nil {
			continue
		}
		for _, item := range flow.Operands(raw) {
			if ref, refers := flow.OperandRef(item); refers {
				refs = append(refs, ref)
			}
		}
	}

	return refs
}

// CallItem is the item a Call's reference to the workflow it calls names,
// so asking where a workflow is used for it finds only its callers.
const CallItem = "call"

// numberReferences lists the constants standing in for a node's typed-in
// numbers.
func numberReferences(node flow.Node) []model.Reference {
	standing := node.NumberConstants()
	if len(standing) == 0 {
		return nil
	}
	kind, _ := flow.KindOf(node.Kind)
	refs := make([]flow.ValueRef, 0, len(standing))
	for _, each := range standing {
		refs = append(refs, each.Constant)
	}

	return refReferences(refs, "a "+kind.Label+" node")
}

// callReferences lists the workflow a Call calls and the values outside the
// workflow it gives its inputs or keeps its outputs in.
func callReferences(node flow.Node) []model.Reference {
	const label = "a Call workflow node"
	var references []model.Reference
	if called, ok := node.Called(); ok {
		references = append(references, model.Reference{Document: called, Item: CallItem, Label: label})
	}

	return append(references, refReferences(callRefs(node), label)...)
}

// callRefs lists the values a Call gives its inputs from or keeps its
// outputs in.
func callRefs(node flow.Node) []flow.ValueRef {
	refs := slices.Collect(maps.Values(node.Keeps()))
	for _, given := range node.Inputs() {
		if ref, refers := flow.OperandRef(given); refers {
			refs = append(refs, ref)
		}
	}

	return refs
}

// messageReferences lists the values and constants outside the workflow
// that a Log, a Say or a Show message puts in its line, each named by its
// node's kind.
func messageReferences(node flow.Node) []model.Reference {
	kind, _ := flow.KindOf(node.Kind)

	return refReferences(messageRefs(node), "a "+kind.Label+" node")
}

// messageRefs lists the values a line puts in it.
func messageRefs(node flow.Node) []flow.ValueRef {
	segments, err := node.Message()
	if err != nil {
		return nil
	}
	var refs []flow.ValueRef
	for _, segment := range segments {
		if segment.Ref != nil {
			refs = append(refs, *segment.Ref)
		}
	}

	return refs
}

// LocalUses lists the nodes of a workflow that use one of its local values:
// setting, checking or looking for it, in an operand, in a line, or as what
// a Call gives or keeps. A local in use is not deleted (increment 58).
func LocalUses(workflow *flow.Workflow, local ids.ULID) []flow.Node {
	var uses []flow.Node
	for _, node := range workflow.Nodes() {
		var refs []flow.ValueRef
		switch node.Kind {
		case flow.KindSetValue, flow.KindCheckValue, flow.KindFindNPC:
			refs = valueRefs(node)
		case flow.KindCall:
			refs = callRefs(node)
		case flow.KindLog, flow.KindSay, flow.KindShowMessage:
			refs = messageRefs(node)
		}
		if slices.ContainsFunc(refs, func(ref flow.ValueRef) bool {
			return ref.Scope == worldstate.ScopeLocal && ref.Value == local
		}) {
			uses = append(uses, node)
		}
	}

	return uses
}

// refReferences lists values outside the workflow by ID under the world
// state, and any NPC or zone they name.
func refReferences(refs []flow.ValueRef, label string) []model.Reference {
	var references []model.Reference
	for _, ref := range refs {
		if ref.IsZero() || ref.Scope == worldstate.ScopeLocal {
			continue
		}
		references = append(references, model.Reference{Document: worldstate.ID(), Item: ref.Value.String(), Label: label})
		if !ref.Owner.IsZero() {
			references = append(references, model.Reference{Document: ref.Owner, Item: "", Label: label})
		}
	}

	return references
}

// localValues is a workflow's list of local values, for the edits that
// change it.
func localValues(document model.Document) (*worldstate.Declarations, error) {
	workflowDoc, ok := document.(*Document)
	if !ok {
		return nil, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotWorkflow)
	}

	return workflowDoc.Workflow.Locals(), nil
}

// Plugin returns the workflow plug-in.
func Plugin() plugin.Plugin {
	layout := flow.LayoutFormat()

	return plugin.Plugin{
		Name: PluginName,
		Documents: []model.DocumentType{{
			Format: flow.Format(), Decode: Decode, Layout: &layout, Prepare: Prepare,
		}},
		Edits: append([]model.EditKind{Add{}, Remove{}, Replace{}, Move{}, Tint{}, SetRunsIn{}},
			stateplugin.Change{Kind: ChangeLocalsEdit, On: flow.FormatName, List: localValues}),
		Required: nil,
	}
}
