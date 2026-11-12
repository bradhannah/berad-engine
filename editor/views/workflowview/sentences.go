package workflowview

import (
	"bytes"
	"strconv"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// A Workflow says its changes in its own words (increment 51): which node,
// joined or unlinked by which output, and which of its settings changed, as
// "Added a Wait to Clerk's day" or "Changed Wait's How long". The
// orchestrator says them in place of its own sentence, and Undo answers
// with them.

// sentenceOf says an edit to the workflow in words; "" leaves it to the
// orchestrator.
func (s *State) sentenceOf(docs frame.Docs, args any) string {
	if s.workflow == nil {
		return ""
	}
	name := s.name(docs)
	switch change := args.(type) {
	case workflowplugin.AddArgs:
		return "Added " + s.nodesSaid(kindsOf(change.Nodes)) + " to " + name
	case workflowplugin.RemoveArgs:
		kinds := make([]string, 0, len(change.Nodes))
		for _, nodeID := range change.Nodes {
			if node, ok := s.workflow.Node(nodeID); ok {
				kinds = append(kinds, node.Kind)
			}
		}

		return "Removed " + s.nodesSaid(kinds) + " from " + name
	case workflowplugin.ReplaceArgs:
		return s.replaceSaid(change.Node)
	case workflowplugin.MoveArgs:
		if len(change.Boxes) == 0 {
			return "Moved a line in " + name
		}

		return "Moved " + s.nodesSaid(make([]string, len(change.Boxes))) + " in " + name
	}

	return ""
}

// kindsOf lists nodes' kinds.
func kindsOf(nodes []flow.Node) []string {
	kinds := make([]string, 0, len(nodes))
	for _, node := range nodes {
		kinds = append(kinds, node.Kind)
	}

	return kinds
}

// nodesSaid names nodes: "a Wait" for one of a known kind, "a node", or
// "3 nodes".
func (s *State) nodesSaid(kinds []string) string {
	switch {
	case len(kinds) == 1 && kinds[0] != "":
		return "a " + labelOf(kinds[0])
	case len(kinds) == 1:
		return "a node"
	}

	return strconv.Itoa(len(kinds)) + " nodes"
}

// replaceSaid says how a node changed: a line joined or unlinked, or which
// of its settings.
func (s *State) replaceSaid(changed flow.Node) string {
	before, ok := s.workflow.Node(changed.ID)
	if !ok {
		return ""
	}
	label := labelOf(changed.Kind)
	for _, output := range flow.Outputs(changed) {
		was, now := before.Next[output.Name], changed.Next[output.Name]
		if was == now {
			continue
		}
		if now.IsZero() {
			return "Unlinked " + label + " · " + output.Label + " from " + s.kindOf(was)
		}

		return "Joined " + label + " · " + output.Label + " to " + s.kindOf(now)
	}
	if kind, known := flow.KindOf(changed.Kind); known {
		for _, param := range kind.Params {
			if !bytes.Equal(before.Params[param.Name], changed.Params[param.Name]) {
				return "Changed " + label + "'s " + param.Label
			}
		}
	}

	return "Changed " + label
}

// kindOf names a node of the workflow by its kind, or "a node" once gone.
func (s *State) kindOf(nodeID ids.ULID) string {
	if node, ok := s.workflow.Node(nodeID); ok {
		return labelOf(node.Kind)
	}

	return "a node"
}
