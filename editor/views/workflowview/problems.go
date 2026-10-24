package workflowview

import (
	"image"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/frame"
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
	parts := []string{strconv.FormatUint(s.laidOut, 10)}
	docs.View(project.IndexID(), func(document client.Document) {
		parts = append(parts, "index:"+strconv.FormatUint(document.Revision, 10))
	})
	for _, floorID := range s.destinationFloors() {
		revision := "-"
		docs.View(floorID, func(document client.Document) { revision = strconv.FormatUint(document.Revision, 10) })
		parts = append(parts, floorID.String()+":"+revision)
	}
	key := strings.Join(parts, "|")
	if key == s.problemKey {
		return
	}
	s.problemKey = key
	s.problems = map[ids.ULID]string{}
	for _, node := range s.workflow.Nodes() {
		if problem := s.problemOf(node, docs); problem != "" {
			s.problems[node.ID] = problem
		}
	}
}

// problemOf says what is wrong with a node, or returns "": a Go to with no
// destination or a broken one, a container with no first step, or a node
// nothing links into.
func (s *State) problemOf(node flow.Node, docs frame.Docs) string {
	switch node.Kind {
	case flow.KindGoTo:
		if problem := s.destinationProblem(node, docs); problem != "" {
			return problem
		}
	case flow.KindContainer:
		if _, linked := node.Next[flow.OutputInside]; !linked {
			return problemNoFirstStep
		}
	}
	if takesEntries(node.Kind) && len(s.shapes[node.ID].entries) == 0 {
		return problemUnreached
	}

	return ""
}

// destinationProblem says what is wrong with a Go to's destination. A floor
// not loaded yet is not a problem until it arrives.
func (s *State) destinationProblem(node flow.Node, docs frame.Docs) string {
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
