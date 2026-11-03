package workflowview

import (
	"github.com/bradhannah/berad-engine/editor/views/frame"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// problemNoPlaceValue is what a Go to says that goes to a place value with
// none chosen.
const problemNoPlaceValue = "No place value chosen yet. Choose a place local, or pick a place on a map."

// shapeGoTo shows a Go to's anchor while it goes to a place on a map, and
// its place value while it goes to one.
func shapeGoTo(built *form, node flow.Node) {
	byValue := choiceOf(node, flow.ParamGoes) == flow.GoesToValue
	for index := range built.controls {
		made := &built.controls[index]
		switch made.param.Name {
		case "destination":
			made.hidden = byValue
		case flow.ParamPlace:
			made.hidden = !byValue
		}
	}
}

// placeValueLabel is the place value a Go to goes to, by its path.
func (s *State) placeValueLabel(node flow.Node, docs frame.Docs) string {
	ref, err := node.Ref(flow.ParamPlace)
	if err != nil || ref.IsZero() {
		return "no place value"
	}
	_, path, problem := s.resolve(docs, ref)
	if problem != "" {
		return problem
	}

	return path
}

// placeValueProblem says what is wrong with a Go to's place value: none
// chosen, or one missing or not a place.
func (s *State) placeValueProblem(node flow.Node, docs frame.Docs) string {
	ref, err := node.Ref(flow.ParamPlace)
	if err != nil || ref.IsZero() {
		return problemNoPlaceValue
	}
	declared, _, problem := s.resolve(docs, ref)
	if problem != "" {
		return problem
	}
	if declared.Type != worldstate.TypePlace {
		return "Its place value is now " + declared.Type.Label() + ", not a place."
	}

	return ""
}
