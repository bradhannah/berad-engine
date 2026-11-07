package workflowview

import (
	"image"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// dupOffset is how far below and right of the nodes copied a duplicate
// lands, in canvas pixels.
const dupOffset = 24

// clip is a copy of nodes: each with what its containers hold, where they
// sit and their tints. Adding it gives every node a new ID; a line to a
// node not among them goes.
type clip struct {
	Nodes []flow.Node           `json:"nodes"`
	Boxes map[ids.ULID]flow.Box `json:"boxes"`
	Tints map[ids.ULID]string   `json:"tints,omitempty"`
}

// clipSelection copies the selected nodes but the start, with what their
// containers hold; ok is false when that leaves nothing.
func (s *State) clipSelection() (clip, bool) {
	tops := s.removable()
	chosen := make([]ids.ULID, 0, len(tops))
	for _, top := range tops {
		chosen = append(chosen, top)
		chosen = append(chosen, s.held(top)...)
	}
	copied := clip{Nodes: make([]flow.Node, 0, len(chosen)), Boxes: map[ids.ULID]flow.Box{}, Tints: map[ids.ULID]string{}}
	for _, nodeID := range chosen {
		node, ok := s.workflow.Node(nodeID)
		if !ok {
			continue
		}
		copied.Nodes = append(copied.Nodes, node)
		copied.Boxes[nodeID] = s.movedBox(s.shapes[nodeID], image.Pt(0, 0))
		if tint := s.tints.Nodes[nodeID]; tint != "" {
			copied.Tints[nodeID] = tint
		}
	}

	return copied, len(copied.Nodes) > 0
}

// shifted is the clip moved by a distance.
func (c clip) shifted(delta image.Point) clip {
	boxes := make(map[ids.ULID]flow.Box, len(c.Boxes))
	for id, box := range c.Boxes {
		box.X, box.Y = box.X+delta.X, box.Y+delta.Y
		boxes[id] = box
	}
	c.Boxes = boxes

	return c
}

// duplicate copies the selection a little below and right of itself, and
// selects the copies.
func (s *State) duplicate(ctx *frame.Context) {
	copied, ok := s.clipSelection()
	if !ok {
		return
	}
	s.addCopy(ctx, copied.shifted(image.Pt(dupOffset, dupOffset)), "", "Duplicated "+countNodes(len(copied.Nodes))+
		". Undo removes the copies.")
}

// addCopy adds a clip to the workflow as new nodes, in a gesture, and
// selects them once they are there.
func (s *State) addCopy(ctx *frame.Context, copied clip, gesture, said string) {
	before := map[ids.ULID]bool{}
	for _, node := range s.workflow.Nodes() {
		before[node.ID] = true
	}
	args := workflowplugin.AddArgs{
		Nodes: copied.Nodes, Boxes: copied.Boxes, Links: nil, Tints: copied.Tints, Fresh: true,
	}
	s.command(ctx, workflowplugin.AddEdit, gesture, args, func(result protocol.Result) {
		if result.OK {
			s.selectAdded = before
			ctx.Frame.Status(said)
		}
	})
}

// selectAddedNodes selects the nodes a copy added, once the workflow shows
// them: those it holds that it did not before, but what their containers
// hold, which comes with them.
func (s *State) selectAddedNodes() {
	if s.selectAdded == nil || s.workflow == nil || len(s.workflow.Nodes()) <= len(s.selectAdded) {
		return
	}
	var added []ids.ULID
	for _, node := range s.workflow.Nodes() {
		if !s.selectAdded[node.ID] && (node.Parent.IsZero() || s.selectAdded[node.Parent]) {
			added = append(added, node.ID)
		}
	}
	s.selectAdded = nil
	s.selectNodes(added)
}

// selectionKeys handles the keys that act on the selection, Cmd+D, Cmd+C,
// Cmd+V and the arrows, and reports whether one did.
func (s *State) selectionKeys(input *ui.Input, ctx *frame.Context) bool {
	return s.copyKeys(input, ctx) || s.nudge(input, ctx)
}

// copyKeys handles Cmd+D, which duplicates the selection, Cmd+C, which
// copies it, and Cmd+V, which pastes; it reports whether one did.
func (s *State) copyKeys(input *ui.Input, ctx *frame.Context) bool {
	selected := len(s.selection()) > 0
	switch {
	case selected && input.Consume(ui.KeyD, ui.Cmd()):
		s.duplicate(ctx)
	case selected && input.Consume(ui.KeyC, ui.Cmd()):
		s.copySelection(ctx)
	case input.Consume(ui.KeyV, ui.Cmd()):
		s.paste(ctx)
	default:
		return false
	}

	return true
}
