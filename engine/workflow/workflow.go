// Package workflow is the workflow document: behaviour authored as typed
// nodes and the connections between them. A workflow starts at its start
// node and follows each node's outputs; when a branch ends with nothing
// connected, it goes back to the start. A time-bound container holds nodes
// that repeat while its time lasts, then it leaves by its exit. Any number of
// links can lead into one node.
//
// The workflow's logic and its layout, where each node sits on the canvas,
// are saved apart (D17), so moving a node never shows up as a change to the
// logic.
package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// FormatName and FormatVersion identify workflow logic inside saved
// envelopes; the Layout names identify its layout. Version 2 gave random
// choices any number of options and waits a unit and a clock.
const (
	FormatName          = "workflow"
	FormatVersion       = 2
	LayoutFormatName    = "workflow-layout"
	LayoutFormatVersion = 1
)

// MaxNodes bounds the nodes in one workflow.
const MaxNodes = 1000

// ErrNode reports an invalid node or change to a workflow.
var ErrNode = errors.New("invalid workflow node")

// ErrData reports saved workflow data that cannot be read.
var ErrData = errors.New("invalid workflow data")

// Node is one step of a workflow.
type Node struct {
	ID   ids.ULID `json:"id"`
	Kind string   `json:"kind"`
	// Parent is the container holding the node; zero at the top level.
	Parent ids.ULID `json:"parent,omitzero"`
	// Params holds the kind's parameters by name, as JSON values.
	Params map[string]json.RawMessage `json:"params,omitempty"`
	// Next connects each output, by name, to the node it leads to. An
	// output with no entry leads nowhere: the branch ends there.
	Next map[string]ids.ULID `json:"next,omitempty"`
}

// Clone returns an independent copy.
func (n Node) Clone() Node {
	n.Params = maps.Clone(n.Params)
	n.Next = maps.Clone(n.Next)

	return n
}

// Equal reports whether two nodes are the same.
func (n Node) Equal(other Node) bool {
	return n.ID == other.ID && n.Kind == other.Kind && n.Parent == other.Parent &&
		maps.Equal(n.Next, other.Next) && maps.EqualFunc(n.Params, other.Params, jsonEqual)
}

// Box is where a node sits on the canvas, in canvas pixels. Containers and
// notes also have a size; other nodes size themselves.
type Box struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w,omitempty"`
	H int `json:"h,omitempty"`
}

// Link is one connection: an output of a node leading to another node.
type Link struct {
	From   ids.ULID `json:"from"`
	Output string   `json:"output"`
	To     ids.ULID `json:"to"`
}

// Workflow is a graph of nodes with a start node, and their layout.
type Workflow struct {
	start ids.ULID
	nodes []Node
	boxes map[ids.ULID]Box
}

type wireWorkflow struct {
	Start ids.ULID `json:"start"`
	Nodes []Node   `json:"nodes"`
}

type wireLayout struct {
	Boxes map[ids.ULID]Box `json:"boxes"`
}

// New returns a workflow holding only its start node.
func New(start ids.ULID) (*Workflow, error) {
	if start.IsZero() {
		return nil, fmt.Errorf("%w: the start node needs an ID", ErrNode)
	}
	node := Node{ID: start, Kind: KindStart, Parent: ids.ULID{}, Params: map[string]json.RawMessage{}, Next: nil}

	return &Workflow{start: start, nodes: []Node{node}, boxes: map[ids.ULID]Box{}}, nil
}

// Start returns the start node's ID.
func (w *Workflow) Start() ids.ULID { return w.start }

// Nodes returns copies of the nodes, in the order they were created.
func (w *Workflow) Nodes() []Node {
	copies := make([]Node, 0, len(w.nodes))
	for _, node := range w.nodes {
		copies = append(copies, node.Clone())
	}

	return copies
}

// Node returns a copy of a node.
func (w *Workflow) Node(id ids.ULID) (Node, bool) {
	index := w.index(id)
	if index < 0 {
		var none Node

		return none, false
	}

	return w.nodes[index].Clone(), true
}

// Children returns the IDs of the nodes a container holds, in order.
func (w *Workflow) Children(container ids.ULID) []ids.ULID {
	var children []ids.ULID
	for _, node := range w.nodes {
		if node.Parent == container && !container.IsZero() {
			children = append(children, node.ID)
		}
	}

	return children
}

// Box returns where a node sits; ok is false for a node not yet placed.
func (w *Workflow) Box(id ids.ULID) (Box, bool) {
	box, ok := w.boxes[id]

	return box, ok
}

// Boxes returns a copy of the layout.
func (w *Workflow) Boxes() map[ids.ULID]Box { return maps.Clone(w.boxes) }

// Insert adds nodes, their boxes and links from existing nodes to them, all
// or nothing.
func (w *Workflow) Insert(nodes []Node, boxes map[ids.ULID]Box, links []Link) error {
	working := w.Clone()
	for _, node := range nodes {
		if node.ID.IsZero() || working.index(node.ID) >= 0 || node.Kind == KindStart {
			return fmt.Errorf("%w: %s is missing, already present or a second start", ErrNode, node.ID)
		}
		working.nodes = append(working.nodes, node.Clone())
	}
	if len(working.nodes) > MaxNodes {
		return fmt.Errorf("%w: a workflow holds at most %d nodes", ErrNode, MaxNodes)
	}
	working.sort()
	maps.Copy(working.boxes, boxes)
	for _, link := range links {
		index := working.index(link.From)
		if index < 0 {
			return fmt.Errorf("%w: no node %s to link from", ErrNode, link.From)
		}
		working.nodes[index] = withLink(working.nodes[index], link.Output, link.To)
	}
	if err := working.check(); err != nil {
		return err
	}
	*w = *working

	return nil
}

// Remove takes nodes out, with the contents of any container among them, and
// clears every link to them. It returns what it removed and the links it
// cleared, so they can be put back with Insert.
func (w *Workflow) Remove(targets []ids.ULID) ([]Node, map[ids.ULID]Box, []Link, error) {
	doomed := map[ids.ULID]bool{}
	for _, target := range targets {
		if target == w.start || w.index(target) < 0 {
			return nil, nil, nil, fmt.Errorf("%w: %s is the start or not in the workflow", ErrNode, target)
		}
		doomed[target] = true
		for _, child := range w.Children(target) {
			doomed[child] = true
		}
	}
	var removed []Node
	boxes := map[ids.ULID]Box{}
	var links []Link
	kept := make([]Node, 0, len(w.nodes))
	for _, node := range w.nodes {
		if doomed[node.ID] {
			removed = append(removed, node)
			if box, ok := w.boxes[node.ID]; ok {
				boxes[node.ID] = box
			}

			continue
		}
		for _, output := range slices.Sorted(maps.Keys(node.Next)) {
			if to := node.Next[output]; doomed[to] {
				links = append(links, Link{From: node.ID, Output: output, To: to})
				node = withLink(node, output, ids.ULID{})
			}
		}
		kept = append(kept, node)
	}
	w.nodes = kept
	for id := range doomed {
		delete(w.boxes, id)
	}

	return removed, boxes, links, nil
}

// Replace changes a node's parameters, links and container, keeping its ID
// and kind, and returns the node as it was.
func (w *Workflow) Replace(node Node) (Node, error) {
	index := w.index(node.ID)
	if index < 0 {
		return Node{}, fmt.Errorf("%w: no node %s", ErrNode, node.ID)
	}
	previous := w.nodes[index]
	if previous.Kind != node.Kind {
		return Node{}, fmt.Errorf("%w: a node cannot change kind", ErrNode)
	}
	working := w.Clone()
	working.nodes[index] = node.Clone()
	if err := working.check(); err != nil {
		return Node{}, err
	}
	*w = *working

	return previous.Clone(), nil
}

// Move sets the boxes of nodes and returns their previous boxes; a node that
// had no box comes back with a zero box marked by a negative width.
func (w *Workflow) Move(boxes map[ids.ULID]Box) (map[ids.ULID]Box, error) {
	previous := map[ids.ULID]Box{}
	for nodeID := range boxes {
		if w.index(nodeID) < 0 {
			return nil, fmt.Errorf("%w: no node %s to move", ErrNode, nodeID)
		}
		old, ok := w.boxes[nodeID]
		if !ok {
			old = Box{X: 0, Y: 0, W: Unplaced, H: 0}
		}
		previous[nodeID] = old
	}
	for nodeID, box := range boxes {
		if box.W == Unplaced {
			delete(w.boxes, nodeID)

			continue
		}
		w.boxes[nodeID] = box
	}

	return previous, nil
}

// Unplaced is the width that marks a box as absent in Move.
const Unplaced = -1

// Clone returns an independent copy.
func (w *Workflow) Clone() *Workflow {
	return &Workflow{start: w.start, nodes: w.Nodes(), boxes: maps.Clone(w.boxes)}
}

// Equal reports whether two workflows hold the same logic and layout.
func (w *Workflow) Equal(other *Workflow) bool {
	return w.start == other.start && slices.EqualFunc(w.nodes, other.nodes, Node.Equal) &&
		maps.Equal(w.boxes, other.boxes)
}

// Encode returns the workflow's logic.
func (w *Workflow) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(wireWorkflow{Start: w.start, Nodes: w.nodes})
	if err != nil {
		return nil, fmt.Errorf("encode workflow: %w", err)
	}

	return data, nil
}

// EncodeLayout returns the workflow's layout.
func (w *Workflow) EncodeLayout() (json.RawMessage, error) {
	data, err := json.Marshal(wireLayout{Boxes: w.boxes})
	if err != nil {
		return nil, fmt.Errorf("encode workflow layout: %w", err)
	}

	return data, nil
}

// Decode reads saved workflow logic. The layout is read separately with
// DecodeLayout.
func Decode(data json.RawMessage) (*Workflow, error) {
	var wire wireWorkflow
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	decoded := &Workflow{start: wire.Start, nodes: wire.Nodes, boxes: map[ids.ULID]Box{}}
	for index := range decoded.nodes {
		decoded.nodes[index] = decoded.nodes[index].Clone()
	}
	decoded.sort()
	if err := decoded.check(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}

	return decoded, nil
}

// DecodeLayout reads a saved layout into the workflow, keeping the boxes of
// nodes it holds and dropping any others.
func (w *Workflow) DecodeLayout(data json.RawMessage) error {
	var wire wireLayout
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("%w: layout: %w", ErrData, err)
	}
	w.boxes = map[ids.ULID]Box{}
	for id, box := range wire.Boxes {
		if w.index(id) >= 0 {
			w.boxes[id] = box
		}
	}

	return nil
}

// Format returns the workflow logic's content format.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: map[int]content.Migration{1: upgradeFrom1}}
}

// upgradeFrom1 turns a version 1 random choice's chance into two options
// that keep its outputs, "chance" and "otherwise", and a wait's minutes into
// minutes on the game clock.
func upgradeFrom1(data json.RawMessage) (json.RawMessage, error) {
	var wire wireWorkflow
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	for index, node := range wire.Nodes {
		var err error
		switch node.Kind {
		case KindRandom:
			var chance int
			if err = node.decode("chance", &chance); err == nil {
				delete(node.Params, "chance")
				node, err = node.WithParam("options", []Share{
					{Port: "yes", Label: "chance", Share: chance},
					{Port: OutputOtherwise, Label: "otherwise", Share: percent - chance},
				})
			}
		case KindWait:
			var minutes int
			if err = node.decode("minutes", &minutes); err == nil {
				delete(node.Params, "minutes")
				node, err = node.WithParam("duration", Duration{Amount: minutes, Unit: UnitMinutes, Clock: ClockGame})
			}
		}
		if err != nil {
			return nil, fmt.Errorf("%w: upgrade %s: %w", ErrData, node.ID, err)
		}
		wire.Nodes[index] = node
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("upgrade workflow: %w", err)
	}

	return encoded, nil
}

// LayoutFormat returns the workflow layout's content format.
func LayoutFormat() content.Format {
	return content.Format{Name: LayoutFormatName, Current: LayoutFormatVersion, Migrations: nil}
}

// check enforces the structure every workflow keeps: one start node; unique
// IDs; containers only at the top level; links that stay within one level,
// except a container's inside output, which leads into it; and no links to
// notes.
func (w *Workflow) check() error {
	seen := map[ids.ULID]Node{}
	starts := 0
	for _, node := range w.nodes {
		if node.ID.IsZero() || seen[node.ID].Kind != "" {
			return fmt.Errorf("%w: missing or repeated ID %s", ErrNode, node.ID)
		}
		seen[node.ID] = node
		if node.Kind == KindStart {
			starts++
		}
	}
	if starts != 1 || seen[w.start].Kind != KindStart {
		return fmt.Errorf("%w: a workflow has exactly one start node", ErrNode)
	}
	for _, node := range w.nodes {
		if err := checkPlace(node, seen); err != nil {
			return err
		}
		for output, to := range node.Next {
			if err := checkLink(node, output, seen[to]); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkPlace(node Node, nodes map[ids.ULID]Node) error {
	if node.Parent.IsZero() {
		return nil
	}
	if node.Kind == KindStart || node.Kind == KindContainer || nodes[node.Parent].Kind != KindContainer {
		return fmt.Errorf("%w: %s cannot sit in %s", ErrNode, node.ID, node.Parent)
	}

	return nil
}

func checkLink(from Node, output string, target Node) error {
	switch {
	case target.Kind == "":
		return fmt.Errorf("%w: %s links to a node that is not there", ErrNode, from.ID)
	case !Runs(target.Kind) || target.Kind == KindStart || !Runs(from.Kind):
		return fmt.Errorf("%w: notes and the start node cannot be linked to", ErrNode)
	case from.Kind == KindContainer && output == OutputInside:
		if target.Parent != from.ID {
			return fmt.Errorf("%w: a container's first step must be inside it", ErrNode)
		}
	case target.Parent != from.Parent:
		return fmt.Errorf("%w: links stay inside one container; leave by its exit", ErrNode)
	}

	return nil
}

// sort keeps nodes in ID order, which is the order they were created, so
// removing nodes and putting them back restores exactly what was saved.
func (w *Workflow) sort() {
	slices.SortFunc(w.nodes, func(a, b Node) int { return a.ID.Compare(b.ID) })
}

func (w *Workflow) index(id ids.ULID) int {
	return slices.IndexFunc(w.nodes, func(node Node) bool { return node.ID == id })
}

// withLink returns a node with one output connected, or cleared when to is
// zero.
func withLink(node Node, output string, target ids.ULID) Node {
	node = node.Clone()
	if target.IsZero() {
		delete(node.Next, output)
		if len(node.Next) == 0 {
			node.Next = nil
		}

		return node
	}
	if node.Next == nil {
		node.Next = map[string]ids.ULID{}
	}
	node.Next[output] = target

	return node
}

func jsonEqual(a, b json.RawMessage) bool {
	var first, second any
	if json.Unmarshal(a, &first) != nil || json.Unmarshal(b, &second) != nil {
		return string(a) == string(b)
	}
	encodedA, errA := json.Marshal(first)
	encodedB, errB := json.Marshal(second)

	return errA == nil && errB == nil && string(encodedA) == string(encodedB)
}
