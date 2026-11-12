package workflowview

import (
	"strings"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// A line in History selects what it names (increment 45): a Workflow finds
// the first node a change to it touched that is still in it, and selects it
// and brings it into view when the line is clicked, as Back and Forward do.

// Named is the first node a change to this workflow touched that is still
// in it, as the item that shows it, and its name.
func (s *State) Named(touched protocol.Touch, _ frame.Docs) (string, string, bool) {
	if s.workflow == nil || touched.Document != s.document {
		return "", "", false
	}
	for _, item := range touched.Items {
		text, isNode := strings.CutPrefix(item, NodeItem)
		if !isNode {
			continue
		}
		nodeID, err := ids.Parse(text)
		if err != nil {
			continue
		}
		if node, ok := s.workflow.Node(nodeID); ok {
			return NodeItem + nodeID.String(), labelOf(node.Kind), true
		}
	}

	return "", "", false
}
