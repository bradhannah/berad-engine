package workflow

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Bend is where an author moved a line by hand, kept with the layout. A
// line that goes forward turns in an upright part Lane pixels right of the
// output it leaves; a line that goes back runs level Depth pixels below its
// output, or above when Depth is negative. Both are measured from the
// output, so the line keeps its shape when its nodes move. To is the node
// the line led to when it was moved: if the line now leads elsewhere, the
// bend no longer applies.
type Bend struct {
	To    ids.ULID `json:"to"`
	Lane  *int     `json:"lane,omitempty"`
	Depth *int     `json:"depth,omitempty"`
}

// LineKey names the line that leaves a node by an output, for its bend.
func LineKey(from ids.ULID, output string) string { return from.String() + "/" + output }

// SplitLineKey reads a line key back into the node and output it names.
func SplitLineKey(key string) (ids.ULID, string, bool) {
	from, output, found := strings.Cut(key, "/")
	if !found || output == "" {
		return ids.ULID{}, "", false
	}
	id, err := ids.Parse(from)

	return id, output, err == nil
}

// Bend returns the bend of the line leaving a node by an output, when it
// still applies: the line still leads where it did when it was moved.
func (w *Workflow) Bend(from ids.ULID, output string) (Bend, bool) {
	bend, ok := w.bends[LineKey(from, output)]
	if !ok {
		return Bend{To: ids.ULID{}, Lane: nil, Depth: nil}, false
	}
	index := w.index(from)
	if index < 0 || w.nodes[index].Next[output] != bend.To || bend.To.IsZero() {
		return Bend{To: ids.ULID{}, Lane: nil, Depth: nil}, false
	}

	return bend, true
}

// Bends returns the bends that still apply, by line key.
func (w *Workflow) Bends() map[string]Bend {
	applied := map[string]Bend{}
	for key := range w.bends {
		from, output, ok := SplitLineKey(key)
		if !ok {
			continue
		}
		if bend, applies := w.Bend(from, output); applies {
			applied[key] = bend
		}
	}

	return applied
}

// SetBends sets the bends of lines by key, and returns what each was before;
// a bend with no To removes the line's bend, as Unplaced removes a box.
func (w *Workflow) SetBends(bends map[string]Bend) (map[string]Bend, error) {
	previous := map[string]Bend{}
	for _, key := range slices.Sorted(maps.Keys(bends)) {
		from, _, ok := SplitLineKey(key)
		if !ok || w.index(from) < 0 {
			return nil, fmt.Errorf("%w: no line %q to bend", ErrNode, key)
		}
		old, had := w.bends[key]
		if !had {
			old = Bend{To: ids.ULID{}, Lane: nil, Depth: nil}
		}
		previous[key] = old
	}
	for key, bend := range bends {
		if bend.To.IsZero() {
			delete(w.bends, key)

			continue
		}
		w.bends[key] = bend
	}

	return previous, nil
}

// bendsEqual reports whether two sets of bends are the same.
func bendsEqual(a, b map[string]Bend) bool {
	return maps.EqualFunc(a, b, func(x, y Bend) bool {
		return x.To == y.To && sameOffset(x.Lane, y.Lane) && sameOffset(x.Depth, y.Depth)
	})
}

func sameOffset(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
