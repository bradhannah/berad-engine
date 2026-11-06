package workflowview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/signals"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Problem marks, in canvas pixels: a small round mark at a node's top-right
// corner.
const (
	markRadius = 7
	markInset  = 6
)

// Problems a node can have.
const (
	problemNoDestination = "No destination yet. Pick one on a map."
	problemNoFloor       = "Its destination's floor was deleted."
	problemNoAnchor      = "Its destination anchor was deleted."
	problemNoFirstStep   = "Nothing inside runs. Link its first step."
	problemUnreached     = "Nothing links here, so it never runs."
)

// refreshProblems finds the workflow's problems again when the workflow, the
// project's names or a destination floor has changed. Floors are read one at
// a time, never one inside another.
func (s *State) refreshProblems(docs frame.Docs) {
	parts := []string{
		strconv.FormatUint(s.laidOut, 10), "audio:" + strconv.Itoa(s.audioRevision), "calls:" + s.callsKey,
		"reach:" + s.reachKey(docs),
	}
	docs.View(project.IndexID(), func(document client.Document) {
		parts = append(parts, "index:"+strconv.FormatUint(document.Revision, 10))
	})
	for _, valueDoc := range valuetree.Documents(docs) {
		revision := "-"
		docs.View(valueDoc, func(document client.Document) { revision = strconv.FormatUint(document.Revision, 10) })
		parts = append(parts, valueDoc.String()+":"+revision)
	}
	for _, floorID := range slices.Concat(s.destinationFloors(), signals.Documents(docs)) {
		revision := "-"
		docs.View(floorID, func(document client.Document) { revision = strconv.FormatUint(document.Revision, 10) })
		parts = append(parts, floorID.String()+":"+revision)
	}
	key := strings.Join(parts, "|")
	if key == s.problemKey {
		return
	}
	s.problemKey = key
	s.problems, s.warnings, s.localProblems = map[ids.ULID]string{}, map[ids.ULID]string{}, s.localReach(docs)
	for _, node := range s.workflow.Nodes() {
		if problem := s.problemOf(node, docs); problem != "" {
			s.problems[node.ID] = problem
		}
		if warning := s.warningOf(node, docs); warning != "" {
			s.warnings[node.ID] = warning
		}
	}
}

// problemOf says what is wrong with a node, or returns "": being in a lane
// it can't be in, its own settings, or nothing linking into it.
func (s *State) problemOf(node flow.Node, docs frame.Docs) string {
	if problem := s.laneProblem(node); problem != "" {
		return problem
	}
	if problem := s.kindProblem(node, docs); problem != "" {
		return problem
	}
	if takesEntries(node.Kind) && len(s.shapes[node.ID].entries) == 0 {
		return problemUnreached
	}

	return ""
}

// kindProblem says what is wrong with a node's own settings: a Go to with
// no destination or a broken one, a container with no first step, and so on.
func (s *State) kindProblem(node flow.Node, docs frame.Docs) string {
	if problem := s.numberProblem(node, docs); problem != "" {
		return problem
	}
	switch node.Kind {
	case flow.KindGoTo:
		if problem := s.goToProblem(node, docs); problem != "" {
			return problem
		}
	case flow.KindContainer:
		if _, linked := node.Next[flow.OutputInside]; !linked {
			return problemNoFirstStep
		}
	case flow.KindPlaySound:
		if problem := s.cueProblem(node); problem != "" {
			return problem
		}
	case flow.KindSetValue, flow.KindCheckValue, flow.KindFindNPC, flow.KindLog, flow.KindSay, flow.KindShowMessage:
		if problem := s.valueProblem(node, docs); problem != "" {
			return problem
		}
	case flow.KindReturn:
		if problem := returnProblem(node); problem != "" {
			return problem
		}
	case flow.KindOnSignal, flow.KindResume, flow.KindSendSignal:
		return s.signalProblem(node, docs)
	case flow.KindCall:
		if problem := s.callProblem(node, docs); problem != "" {
			return problem
		}
	}

	return ""
}

// destinationProblem says what is wrong with a Go to's destination. A floor
// not loaded yet is not a problem until it arrives.
func (s *State) destinationProblem(node flow.Node, docs frame.Docs) string {
	if choiceOf(node, flow.ParamGoes) == flow.GoesToValue {
		return s.placeValueProblem(node, docs)
	}
	destination, err := node.Anchor("destination")
	if err != nil || destination.IsZero() {
		return problemNoDestination
	}
	if s.floorName(docs, destination.Floor) == "" {
		return problemNoFloor
	}
	missing := false
	docs.View(destination.Floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			_, _, found := floorDoc.Floor.Anchor(destination.Anchor)
			missing = !found
		}
	})
	if missing {
		return problemNoAnchor
	}

	return ""
}

// goToProblem says what is wrong with a Go to: its destination, or that
// the workflow's NPCs cannot reach it.
func (s *State) goToProblem(node flow.Node, docs frame.Docs) string {
	if problem := s.destinationProblem(node, docs); problem != "" {
		return problem
	}

	return s.goToReach(node, docs)
}

// goToReach says why the workflow's NPCs cannot reach a Go to's place on a
// map, or returns "".
func (s *State) goToReach(node flow.Node, docs frame.Docs) string {
	if choiceOf(node, flow.ParamGoes) == flow.GoesToValue {
		return ""
	}
	destination, err := node.Anchor("destination")
	if err != nil {
		return ""
	}

	return s.reachProblem(docs, destination)
}

// markCentre is where a node's problem mark sits: inside its top-right
// corner.
func markCentre(drawn shape) image.Point {
	return image.Pt(drawn.rect.Max.X-markInset-markRadius, drawn.rect.Min.Y+markInset+markRadius)
}

// markAt returns the node whose problem mark is at a canvas point.
func (s *State) markAt(point image.Point) ids.ULID {
	for nodeID := range s.problems {
		centre := markCentre(s.shapes[nodeID])
		if distance := point.Sub(centre); abs(distance.X) <= markRadius && abs(distance.Y) <= markRadius {
			return nodeID
		}
	}

	return ids.ULID{}
}
