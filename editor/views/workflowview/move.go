package workflowview

import (
	"fmt"
	"image"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Arrow keys nudge the selection nudgeStep canvas pixels, or nudgeFar with
// Shift. Nudges of the same selection less than nudgeRun frames apart are
// one undo step.
const (
	nudgeStep = 8
	nudgeFar  = 40
	nudgeRun  = 60
)

// nudging is the run of nudges under way: its gesture, the selection it
// moves, and how many frames since the last nudge.
type nudging struct {
	gesture string
	of      string
	idle    int
}

// movers are the selected nodes that move themselves: those no selected
// container holds, which moves them with it.
func (s *State) movers() []ids.ULID {
	var tops []ids.ULID
	for _, id := range s.selection() {
		if !s.heldBySelected(id) {
			tops = append(tops, id)
		}
	}

	return tops
}

// heldBySelected reports whether a selected container holds a node, however
// deep.
func (s *State) heldBySelected(id ids.ULID) bool {
	node, ok := s.workflow.Node(id)
	for ok && !node.Parent.IsZero() {
		if s.isSelected(node.Parent) {
			return true
		}
		node, ok = s.workflow.Node(node.Parent)
	}

	return false
}

// held lists what a container holds, however deep.
func (s *State) held(container ids.ULID) []ids.ULID {
	children := s.workflow.Children(container)
	inside := make([]ids.ULID, 0, len(children))
	for _, child := range children {
		inside = append(inside, child)
		inside = append(inside, s.held(child)...)
	}

	return inside
}

// fitMove shortens a move so each node moving stays inside the container
// it is in, as one node dragged does.
func (s *State) fitMove(movers []ids.ULID, delta image.Point) image.Point {
	for _, id := range movers {
		moving := s.shapes[id]
		container, inContainer := s.shapes[moving.node.Parent]
		if !inContainer {
			continue
		}
		delta = clampInto(moving.rect.Min.Add(delta), moving.rect.Size(), innerOf(container)).Sub(moving.rect.Min)
	}

	return delta
}

// moveSelection moves the selected nodes, and what their containers hold,
// by a distance, short of any container's wall, in one gesture. It reports
// whether anything moved.
func (s *State) moveSelection(ctx *frame.Context, delta image.Point, gesture string) bool {
	movers := s.movers()
	delta = s.fitMove(movers, delta)
	if delta == image.Pt(0, 0) {
		return false
	}
	boxes := map[ids.ULID]flow.Box{}
	for _, id := range movers {
		boxes[id] = s.movedBox(s.shapes[id], delta)
		for _, inside := range s.held(id) {
			boxes[inside] = s.movedBox(s.shapes[inside], delta)
		}
	}
	s.command(ctx, workflowplugin.MoveEdit, gesture, workflowplugin.MoveArgs{Boxes: boxes, Lines: nil}, nil)

	return true
}

// updateGroupDrag moves the selection with the node dragged. A press let go
// without moving selects that node alone.
func (s *State) updateGroupDrag(input *ui.Input, ctx *frame.Context, point image.Point) {
	drag := s.drag
	if !input.Down {
		if !drag.moved {
			s.selectNodes([]ids.ULID{drag.id})
		}
		s.drag = nil

		return
	}
	grabbed, ok := s.shapes[drag.id]
	if !ok || point == drag.last {
		return
	}
	drag.last = point
	if s.moveSelection(ctx, point.Sub(drag.grab).Sub(grabbed.rect.Min), drag.gesture) {
		drag.moved = true
	}
}

// nudge moves the selection with an arrow key, further with Shift, and
// reports whether it did.
func (s *State) nudge(input *ui.Input, ctx *frame.Context) bool {
	if len(s.selection()) == 0 {
		return false
	}
	ways := []struct {
		key ui.Key
		way image.Point
	}{
		{ui.KeyLeft, image.Pt(-1, 0)}, {ui.KeyRight, image.Pt(1, 0)}, {ui.KeyUp, image.Pt(0, -1)},
		{ui.KeyDown, image.Pt(0, 1)},
	}
	far := ui.Plain()
	far.Shift = true
	for _, each := range ways {
		switch {
		case input.Consume(each.key, ui.Plain()):
			s.moveSelection(ctx, each.way.Mul(nudgeStep), s.nudgeGesture())
		case input.Consume(each.key, far):
			s.moveSelection(ctx, each.way.Mul(nudgeFar), s.nudgeGesture())
		default:
			continue
		}

		return true
	}

	return false
}

// nudgeGesture is the gesture a nudge joins: the run's, while it moves the
// same selection soon after the last, or a new one.
func (s *State) nudgeGesture() string {
	of := fmt.Sprint(s.selection())
	if s.nudges.gesture == "" || s.nudges.idle > nudgeRun || s.nudges.of != of {
		s.nudges.gesture, s.nudges.of = s.gesture(), of
	}
	s.nudges.idle = 0

	return s.nudges.gesture
}
