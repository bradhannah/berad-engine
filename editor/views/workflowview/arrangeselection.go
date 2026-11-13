package workflowview

import (
	"image"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// arrangeSelection lays out the selected nodes alone, as Auto-arrange does
// a workflow, with their top left where it was; the rest stay put, as do
// notes placed by hand. Lines from the selected nodes lose their bends.
func (s *State) arrangeSelection(ctx *frame.Context) {
	sub, firsts, ok := s.selectionWorkflow()
	if !ok {
		return
	}
	boxes := arrangeFrom(sub, s.heights(ctx.Docs), firsts)
	if !s.isSelected(s.workflow.Start()) {
		delete(boxes, s.workflow.Start())
	}
	for nodeID := range boxes {
		if node, _ := s.workflow.Node(nodeID); node.Kind == flow.KindNote {
			delete(boxes, nodeID)
		}
	}
	before, after := s.selectionCorner(), cornerOf(boxes, sub)
	delta := before.Sub(after)
	for nodeID, box := range boxes {
		box.X, box.Y = box.X+delta.X, box.Y+delta.Y
		boxes[nodeID] = box
	}
	args := workflowplugin.MoveArgs{Boxes: boxes, Lines: nil}
	for key := range s.workflow.Bends() {
		from, _, _ := flow.SplitLineKey(key)
		if _, moved := boxes[from]; moved {
			if args.Lines == nil {
				args.Lines = map[string]flow.Bend{}
			}
			args.Lines[key] = flow.Bend{To: ids.ULID{}, Lane: nil, Depth: nil, End: nil}
		}
	}
	count := len(s.selection())
	s.command(ctx, workflowplugin.MoveEdit, "", args, func(protocol.Result) {
		ctx.Frame.Status("Arranged " + countNodes(count) + ". Undo puts them back where they were.")
	})
}

// selectionWorkflow is the selected nodes, with what their containers
// hold, as a workflow of their own, keeping the lines among them; and the
// nodes its layout starts from, those no line among them leads to.
func (s *State) selectionWorkflow() (*flow.Workflow, []ids.ULID, bool) {
	sub, err := flow.NewFor(s.workflow.Start(), s.workflow.Purpose())
	if err != nil {
		return nil, nil, false
	}
	among := s.selectedWithContents()
	var nodes []flow.Node
	var links []flow.Link
	led := map[ids.ULID]bool{}
	for _, node := range s.workflow.Nodes() {
		if !among[node.ID] {
			continue
		}
		for output, target := range node.Next {
			if among[target] {
				led[target] = true
			} else {
				delete(node.Next, output)
			}
		}
		if node.ID == s.workflow.Start() {
			for output, target := range node.Next {
				links = append(links, flow.Link{From: node.ID, Output: output, To: target})
			}

			continue
		}
		nodes = append(nodes, node)
	}
	if sub.Insert(nodes, nil, links) != nil {
		return nil, nil, false
	}
	var firsts []ids.ULID
	for _, node := range sub.Nodes() {
		if node.Parent.IsZero() && !led[node.ID] && among[node.ID] {
			firsts = append(firsts, node.ID)
		}
	}

	return sub, firsts, len(nodes) > 0 || among[s.workflow.Start()]
}

// selectedWithContents is the selected nodes and what their containers
// hold.
func (s *State) selectedWithContents() map[ids.ULID]bool {
	among := map[ids.ULID]bool{}
	for _, top := range s.movers() {
		among[top] = true
		for _, inside := range s.held(top) {
			among[inside] = true
		}
	}

	return among
}

// selectionCorner is the top left of the selected nodes as they sit.
func (s *State) selectionCorner() image.Point {
	corner, first := image.Pt(0, 0), true
	for _, nodeID := range s.movers() {
		at := s.shapes[nodeID].rect.Min
		if first || at.X < corner.X {
			corner.X = at.X
		}
		if first || at.Y < corner.Y {
			corner.Y = at.Y
		}
		first = false
	}

	return corner
}

// cornerOf is the top left of the top-level boxes of a layout.
func cornerOf(boxes map[ids.ULID]flow.Box, sub *flow.Workflow) image.Point {
	corner, first := image.Pt(0, 0), true
	for nodeID, box := range boxes {
		if node, _ := sub.Node(nodeID); !node.Parent.IsZero() {
			continue
		}
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
