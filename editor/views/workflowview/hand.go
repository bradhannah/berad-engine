package workflowview

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The status bar names what is in hand in a Workflow (increment 44), as it
// does in a Map: the workflow, what is selected, and what the pointer is
// over, or that Tab adds a node there.

// handLine is what the status bar says is in hand.
func (s *State) handLine(docs frame.Docs) string {
	if s.workflow == nil {
		return ""
	}
	parts := []string{s.name(docs)}
	switch {
	case len(s.group) > 0:
		parts = append(parts, strconv.Itoa(len(s.group))+" nodes selected")
	case s.lineSelected():
		parts = append(parts, "a line selected")
	case !s.selected.IsZero():
		if node, ok := s.workflow.Node(s.selected); ok {
			parts = append(parts, labelOf(node.Kind)+" selected")
		}
	}
	if over := s.pointerOver(); over != "" {
		parts = append(parts, over)
	}

	return strings.Join(parts, " · ")
}

// pointerOver says what the pointer is over on the canvas: a node, a line,
// or empty canvas, where Tab adds a node; "" off the canvas.
func (s *State) pointerOver() string {
	if !s.cursor.In(s.Canvas()) {
		return ""
	}
	point := s.toCanvas(s.cursor)
	if nodeID, onNode := s.nodeAt(point); onNode {
		if node, ok := s.workflow.Node(nodeID); ok {
			return "over " + labelOf(node.Kind)
		}
	}
	if key, onLine := s.linkAt(point); onLine {
		if from, ok := s.workflow.Node(key.from); ok {
			return "over the line from " + labelOf(from.Kind) + " · " + key.output
		}
	}

	return "Tab adds a node here"
}
