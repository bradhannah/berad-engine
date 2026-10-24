// Package workflow is the editor half of workflows: the workflow document,
// with its layout kept in a file of its own (D17), and the edits that add,
// remove, change and move its nodes.
package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// PluginName identifies the plug-in.
const PluginName = "workflow"

// Edit kind names.
const (
	AddEdit     = "workflow.nodes.add"
	RemoveEdit  = "workflow.nodes.remove"
	ReplaceEdit = "workflow.node.replace"
	MoveEdit    = "workflow.layout.move"
)

// ErrNotWorkflow reports a document that is not a workflow.
var ErrNotWorkflow = errors.New("document is not a workflow")

// Document is a workflow held by the editor.
type Document struct {
	Workflow *flow.Workflow
}

// NewData returns the data of a new workflow, holding only its start node,
// whose ID the orchestrator assigns when the workflow is created.
func NewData() json.RawMessage {
	start := flow.Node{ID: ids.ULID{}, Kind: flow.KindStart, Parent: ids.ULID{}, Params: nil, Next: nil}
	data, err := json.Marshal(wireWorkflow{Start: ids.ULID{}, Nodes: []flow.Node{start}})
	if err != nil {
		panic(err)
	}

	return data
}

type wireWorkflow struct {
	Start ids.ULID    `json:"start"`
	Nodes []flow.Node `json:"nodes"`
}

// Format names the document format.
func (d *Document) Format() string { return flow.FormatName }

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

// References lists the anchors the workflow's Go to nodes lead to.
func (d *Document) References() []model.Reference {
	var references []model.Reference
	for _, node := range d.Workflow.Nodes() {
		if node.Kind != flow.KindGoTo {
			continue
		}
		destination, err := node.Anchor("destination")
		if err != nil || destination.IsZero() {
			continue
		}
		references = append(references, model.Reference{
			Document: destination.Floor, Item: destination.Anchor.String(), Label: "a Go to node",
		})
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

// AddArgs adds nodes, where they sit, and links to them from nodes already
// there. A node with a zero ID is new: preparing the edit gives it a
// permanent ID, and its box may be given under the zero ID.
type AddArgs struct {
	Nodes []flow.Node           `json:"nodes"`
	Boxes map[ids.ULID]flow.Box `json:"boxes,omitempty"`
	Links []flow.Link           `json:"links,omitempty"`
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

// Apply adds the nodes.
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

		return AddEdit, AddArgs{Nodes: removed, Boxes: boxes, Links: links}, touched(gone, links), nil
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

// Plugin returns the workflow plug-in.
func Plugin() plugin.Plugin {
	layout := flow.LayoutFormat()

	return plugin.Plugin{
		Name: PluginName,
		Documents: []model.DocumentType{{
			Format: flow.Format(), Decode: Decode, Layout: &layout, Prepare: Prepare,
		}},
		Edits: []model.EditKind{Add{}, Remove{}, Replace{}, Move{}},
	}
}
