package workflow

import (
	"fmt"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Purpose is a workflow's kind: what it is for, chosen when it is made,
// which decides the nodes it offers and how the engine runs it.
type Purpose string

// The kinds of workflow.
const (
	// PurposeBehaviour is an NPC behaviour: it runs for an NPC, over time,
	// and moves its body.
	PurposeBehaviour Purpose = "npc-behaviour"
	// PurposeZoneLoop is a zone loop: it runs for a zone while the zone is
	// alive, and has no body to move or to look around from.
	PurposeZoneLoop Purpose = "zone-loop"
)

// Purposes lists the kinds of workflow, in the order the editor offers
// them.
func Purposes() []Purpose { return []Purpose{PurposeBehaviour, PurposeZoneLoop} }

// Valid reports whether the purpose is a kind of workflow.
func (p Purpose) Valid() bool { return slices.Contains(Purposes(), p) }

// Label names the kind for people.
func (p Purpose) Label() string {
	if p == PurposeZoneLoop {
		return "Zone loop"
	}

	return "NPC behaviour"
}

// Allows reports whether a workflow of the kind runs nodes of a node kind:
// a zone loop has no Go to, Move or Find NPC nearby, as a zone has no body,
// and no Say, as it has no head to speak over.
func (p Purpose) Allows(kind string) bool {
	if p == PurposeZoneLoop {
		return kind != KindGoTo && kind != KindMove && kind != KindFindNPC && kind != KindSay
	}

	return true
}

// KindsFor returns the node kinds a kind of workflow offers, in palette
// order.
func KindsFor(purpose Purpose) []Kind {
	var offered []Kind
	for _, kind := range Kinds() {
		if purpose.Allows(kind.Name) {
			offered = append(offered, kind)
		}
	}

	return offered
}

// Purpose returns the workflow's kind.
func (w *Workflow) Purpose() Purpose { return w.purpose }

// RunsIn returns the zone the workflow runs in; zero is any zone.
func (w *Workflow) RunsIn() ids.ULID { return w.zone }

// SetRunsIn sets the zone the workflow runs in, zero for any zone, and
// returns the zone it ran in.
func (w *Workflow) SetRunsIn(zone ids.ULID) ids.ULID {
	previous := w.zone
	w.zone = zone

	return previous
}

// NewFor returns a workflow of a kind, running in any zone, holding only
// its start node.
func NewFor(start ids.ULID, purpose Purpose) (*Workflow, error) {
	if !purpose.Valid() {
		return nil, fmt.Errorf("%w: no kind of workflow %q", ErrNode, purpose)
	}
	made, err := New(start)
	if err != nil {
		return nil, err
	}
	made.purpose = purpose

	return made, nil
}

// checkPurpose refuses a workflow of an unknown kind, or holding a node its
// kind does not run.
func (w *Workflow) checkPurpose() error {
	if !w.purpose.Valid() {
		return fmt.Errorf("%w: no kind of workflow %q", ErrNode, w.purpose)
	}
	for _, node := range w.nodes {
		if !w.purpose.Allows(node.Kind) {
			kind, _ := KindOf(node.Kind)

			return fmt.Errorf("%w: a %s has no %s", ErrNode, w.purpose.Label(), kind.Label)
		}
	}

	return nil
}
