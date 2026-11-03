package workflowview

import (
	"strconv"
	"unicode/utf8"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Problems of a Return, and what its name cannot be.
const (
	problemReturnInside = "A Return goes at the top level. Inside a container it is not a way out " +
		"until containers get named exits."
	problemWayName = "A way out needs a name of up to 40 characters, and not done, which every Call has."
	// firstWayName is a new Return's name, numbered when taken.
	firstWayName = "finished"
)

// freshReturn gives a Return being added a way out of its own: a port no
// Return has and a name no way out has.
func (s *State) freshReturn(node flow.Node) flow.Node {
	if s.workflow == nil {
		return node
	}
	taken := map[string]bool{}
	for _, way := range s.workflow.WaysOut() {
		taken[way.Label] = true
	}
	name := firstWayName
	for number := 2; taken[name]; number++ {
		name = firstWayName + " " + strconv.Itoa(number)
	}
	named, err := node.WithParam(flow.ParamWayName, name)
	if err != nil {
		return node
	}
	ported, err := named.WithParam(flow.ParamWayPort, s.workflow.NewPort())
	if err != nil {
		return node
	}

	return ported
}

// nameWayOut renames a Return's way out, or says why it cannot be named
// so. A name another Return has joins that way out; otherwise the Return
// keeps its port, so callers keep the lines drawn from it, unless other
// Returns share it, when it leaves them for a port of its own.
func (s *State) nameWayOut(node flow.Node, name string) (flow.Node, string) {
	if name == "" || name == flow.OutputDone || utf8.RuneCountInString(name) > flow.MaxWayName {
		return node, problemWayName
	}
	own, err := node.WayOut()
	if err != nil || s.workflow == nil {
		return node, problemWayName
	}
	port, shared := own.Port, false
	for _, other := range s.workflow.Nodes() {
		way, wayErr := other.WayOut()
		if other.ID == node.ID || other.Kind != flow.KindReturn || wayErr != nil {
			continue
		}
		if way.Label == name && other.Parent.IsZero() {
			return withPort(node, name, way.Port)
		}
		shared = shared || way.Port == own.Port
	}
	if shared {
		port = s.workflow.NewPort()
	}

	return withPort(node, name, port)
}

// withPort sets a Return's name and port.
func withPort(node flow.Node, name, port string) (flow.Node, string) {
	named, err := node.WithParam(flow.ParamWayName, name)
	if err == nil {
		named, err = named.WithParam(flow.ParamWayPort, port)
	}
	if err != nil {
		return node, err.Error()
	}

	return named, ""
}

// returnProblem says what is wrong with a Return: one inside a container.
func returnProblem(node flow.Node) string {
	if !node.Parent.IsZero() {
		return problemReturnInside
	}

	return ""
}

// wayLabel is a Return's way out's name, for its node and summary.
func wayLabel(node flow.Node) string {
	way, err := node.WayOut()
	if err != nil {
		return "?"
	}

	return way.Label
}
