package tilefloor

import (
	"errors"
	"fmt"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// ErrPlacement reports an invalid change to a floor's placements.
var ErrPlacement = errors.New("invalid placement")

// ErrNoPlacement reports a placement ID the floor does not hold.
var ErrNoPlacement = errors.New("no such placement")

// Placement is an NPC put on a floor: where it starts when its zone is
// played. Each placement has its own permanent ID; NPC names the NPC's
// definition, which can have many placements.
type Placement struct {
	ID  ids.ULID `json:"id"`
	NPC ids.ULID `json:"npc"`
	Col int      `json:"col"`
	Row int      `json:"row"`
}

// Placements returns the floor's placements in their stored order.
func (f *Floor) Placements() []Placement {
	return slices.Clone(f.placements)
}

// Placement returns a placement by ID and its position in the stored order.
func (f *Floor) Placement(id ids.ULID) (Placement, int, bool) {
	index := slices.IndexFunc(f.placements, func(placement Placement) bool { return placement.ID == id })
	if index < 0 {
		return noPlacement(), -1, false
	}

	return f.placements[index], index, true
}

// PlacementAt returns the placement on a cell, if any.
func (f *Floor) PlacementAt(col, row int) (Placement, bool) {
	for _, placement := range f.placements {
		if placement.Col == col && placement.Row == row {
			return placement, true
		}
	}

	return noPlacement(), false
}

func noPlacement() Placement {
	var none Placement

	return none
}

// InsertPlacement adds a placement at a position in the stored order; an
// index past the end appends. A placement needs its own ID and an NPC, must
// lie on the floor, and may not share a cell with another.
func (f *Floor) InsertPlacement(index int, placement Placement) error {
	if placement.ID.IsZero() {
		return fmt.Errorf("%w: missing ID", ErrPlacement)
	}
	if _, _, exists := f.Placement(placement.ID); exists {
		return fmt.Errorf("%w: ID %s already used", ErrPlacement, placement.ID)
	}
	if err := f.checkPlacement(placement, ids.ULID{}); err != nil {
		return err
	}
	index = min(max(index, 0), len(f.placements))
	f.placements = slices.Insert(f.placements, index, placement)

	return nil
}

// RemovePlacement deletes a placement and returns it with its former
// position.
func (f *Floor) RemovePlacement(id ids.ULID) (Placement, int, error) {
	placement, index, ok := f.Placement(id)
	if !ok {
		return noPlacement(), -1, fmt.Errorf("%w: %s", ErrNoPlacement, id)
	}
	f.placements = slices.Delete(f.placements, index, index+1)

	return placement, index, nil
}

// UpdatePlacement moves a placement, returning its previous state.
func (f *Floor) UpdatePlacement(changed Placement) (Placement, error) {
	previous, index, ok := f.Placement(changed.ID)
	if !ok {
		return noPlacement(), fmt.Errorf("%w: %s", ErrNoPlacement, changed.ID)
	}
	if err := f.checkPlacement(changed, changed.ID); err != nil {
		return noPlacement(), err
	}
	f.placements[index] = changed

	return previous, nil
}

// checkPlacement validates a placement, ignoring the placement with the given
// ID when checking cells.
func (f *Floor) checkPlacement(placement Placement, self ids.ULID) error {
	if placement.NPC.IsZero() {
		return fmt.Errorf("%w: no NPC", ErrPlacement)
	}
	if !f.In(placement.Col, placement.Row) {
		return fmt.Errorf("%w: (%d, %d) is outside the floor", ErrPlacement, placement.Col, placement.Row)
	}
	for _, other := range f.placements {
		if other.ID != self && other.Col == placement.Col && other.Row == placement.Row {
			return fmt.Errorf("%w: another NPC is already at (%d, %d)", ErrPlacement, placement.Col, placement.Row)
		}
	}

	return nil
}
