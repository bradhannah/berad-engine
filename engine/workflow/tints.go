package workflow

import (
	"fmt"
	"maps"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Tints are the colours an author gave parts of a workflow to keep it
// organised: nodes and containers by ID, outcomes (one output of a node)
// by their line key, and lines by the same key. Each is the name of a tint
// the editor offers, never a colour value, and the engine never reads them.
// A line takes its outcome's tint unless it has one of its own; TintNone
// as a line's own tint gives it no colour. Tints are layout, saved apart
// from logic.
type Tints struct {
	Nodes    map[ids.ULID]string `json:"nodes,omitempty"`
	Outcomes map[string]string   `json:"outcomes,omitempty"`
	Lines    map[string]string   `json:"lines,omitempty"`
}

// TintNone is a line's own tint that overrides its outcome's with no colour.
const TintNone = "none"

// newTints returns empty tints, every map ready to write to.
func newTints() Tints {
	return Tints{Nodes: map[ids.ULID]string{}, Outcomes: map[string]string{}, Lines: map[string]string{}}
}

// noTints returns tints with no maps, for an error's return.
func noTints() Tints { return Tints{Nodes: nil, Outcomes: nil, Lines: nil} }

// Empty reports whether the tints hold nothing.
func (t Tints) Empty() bool { return len(t.Nodes) == 0 && len(t.Outcomes) == 0 && len(t.Lines) == 0 }

func (t Tints) clone() Tints {
	return Tints{Nodes: maps.Clone(t.Nodes), Outcomes: maps.Clone(t.Outcomes), Lines: maps.Clone(t.Lines)}
}

// Tints returns the tints of the nodes, outcomes and lines the workflow
// holds; a removed node's go with it.
func (w *Workflow) Tints() Tints {
	kept := newTints()
	for id, name := range w.tints.Nodes {
		if w.index(id) >= 0 {
			kept.Nodes[id] = name
		}
	}
	for key, name := range w.tints.Outcomes {
		if w.hasLineKey(key) {
			kept.Outcomes[key] = name
		}
	}
	for key, name := range w.tints.Lines {
		if w.hasLineKey(key) {
			kept.Lines[key] = name
		}
	}

	return kept
}

// hasLineKey reports whether a line key names an output of a node the
// workflow holds.
func (w *Workflow) hasLineKey(key string) bool {
	from, _, ok := SplitLineKey(key)

	return ok && w.index(from) >= 0
}

// SetTints sets tints by key, an empty name taking one away, and returns
// what each was before, so setting those again undoes it. A key that names
// no node is refused.
func (w *Workflow) SetTints(tints Tints) (Tints, error) {
	previous := newTints()
	for _, id := range slices.SortedFunc(maps.Keys(tints.Nodes), ids.ULID.Compare) {
		if w.index(id) < 0 {
			return noTints(), fmt.Errorf("%w: no node %s to tint", ErrNode, id)
		}
		previous.Nodes[id] = w.tints.Nodes[id]
	}
	for _, key := range slices.Sorted(maps.Keys(tints.Outcomes)) {
		if !w.hasLineKey(key) {
			return noTints(), fmt.Errorf("%w: no outcome %q to tint", ErrNode, key)
		}
		previous.Outcomes[key] = w.tints.Outcomes[key]
	}
	for _, key := range slices.Sorted(maps.Keys(tints.Lines)) {
		if !w.hasLineKey(key) {
			return noTints(), fmt.Errorf("%w: no line %q to tint", ErrNode, key)
		}
		previous.Lines[key] = w.tints.Lines[key]
	}
	setTints(w.tints.Nodes, tints.Nodes)
	setTints(w.tints.Outcomes, tints.Outcomes)
	setTints(w.tints.Lines, tints.Lines)

	return previous, nil
}

// setTints writes tints into a map, an empty name removing its key.
func setTints[K comparable](into, from map[K]string) {
	for key, name := range from {
		if name == "" {
			delete(into, key)

			continue
		}
		into[key] = name
	}
}

// tintsEqual reports whether two sets of tints are the same.
func tintsEqual(a, b Tints) bool {
	return maps.Equal(a.Nodes, b.Nodes) && maps.Equal(a.Outcomes, b.Outcomes) && maps.Equal(a.Lines, b.Lines)
}
