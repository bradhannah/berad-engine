package tilefloor

import (
	"errors"
	"fmt"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// How the screen changes when a portal takes the player (increment 64). Each
// is a named kind, so another is an addition.
const (
	// ChangeCut shows where the portal leads at once.
	ChangeCut = "cut"
	// ChangeFade fades to black, and from black where the portal leads.
	ChangeFade = "fade"
)

// Changes lists the ways the screen can change, the one a new portal has
// first.
func Changes() []string { return []string{ChangeCut, ChangeFade} }

// ErrPortal reports an invalid change to a floor's portals.
var ErrPortal = errors.New("invalid portal")

// ErrNoPortal reports a portal ID the floor does not hold.
var ErrNoPortal = errors.New("no such portal")

// Portal is a cell that leads somewhere (increment 64): stepping onto it
// takes the player to an anchor, in another zone or in this one, and Change
// says how the screen changes as they go. It leads one way; the way back is
// another portal. Each has its own permanent ID; a cell holds at most one.
type Portal struct {
	ID     ids.ULID  `json:"id"`
	Col    int       `json:"col"`
	Row    int       `json:"row"`
	To     AnchorRef `json:"to"`
	Change string    `json:"change"`
}

// Portals returns the floor's portals in their stored order.
func (f *Floor) Portals() []Portal {
	return slices.Clone(f.portals)
}

// Portal returns a portal by ID and its position in the stored order.
func (f *Floor) Portal(id ids.ULID) (Portal, int, bool) {
	index := slices.IndexFunc(f.portals, func(portal Portal) bool { return portal.ID == id })
	if index < 0 {
		return noPortal(), -1, false
	}

	return f.portals[index], index, true
}

// PortalAt returns the portal on a cell, if any.
func (f *Floor) PortalAt(col, row int) (Portal, bool) {
	for _, portal := range f.portals {
		if portal.Col == col && portal.Row == row {
			return portal, true
		}
	}

	return noPortal(), false
}

func noPortal() Portal {
	var none Portal

	return none
}

// InsertPortal adds a portal at a position in the stored order; an index
// past the end appends. A portal needs its own ID, an anchor to lead to and
// a way the screen changes, must lie on the floor, and may not share a cell
// with another.
func (f *Floor) InsertPortal(index int, portal Portal) error {
	if portal.ID.IsZero() {
		return fmt.Errorf("%w: missing ID", ErrPortal)
	}
	if _, _, exists := f.Portal(portal.ID); exists {
		return fmt.Errorf("%w: ID %s already used", ErrPortal, portal.ID)
	}
	if err := f.checkPortal(portal, ids.ULID{}); err != nil {
		return err
	}
	index = min(max(index, 0), len(f.portals))
	f.portals = slices.Insert(f.portals, index, portal)

	return nil
}

// RemovePortal deletes a portal and returns it with its former position.
func (f *Floor) RemovePortal(id ids.ULID) (Portal, int, error) {
	portal, index, ok := f.Portal(id)
	if !ok {
		return noPortal(), -1, fmt.Errorf("%w: %s", ErrNoPortal, id)
	}
	f.portals = slices.Delete(f.portals, index, index+1)

	return portal, index, nil
}

// UpdatePortal moves a portal, or changes where it leads or how the screen
// changes, returning its previous state.
func (f *Floor) UpdatePortal(changed Portal) (Portal, error) {
	previous, index, ok := f.Portal(changed.ID)
	if !ok {
		return noPortal(), fmt.Errorf("%w: %s", ErrNoPortal, changed.ID)
	}
	if err := f.checkPortal(changed, changed.ID); err != nil {
		return noPortal(), err
	}
	f.portals[index] = changed

	return previous, nil
}

// checkPortal validates a portal, ignoring the portal with the given ID when
// checking cells.
func (f *Floor) checkPortal(portal Portal, self ids.ULID) error {
	if portal.To.Floor.IsZero() || portal.To.Anchor.IsZero() {
		return fmt.Errorf("%w: a portal leads to an anchor", ErrPortal)
	}
	if !slices.Contains(Changes(), portal.Change) {
		return fmt.Errorf("%w: %q is not a way the screen changes", ErrPortal, portal.Change)
	}
	if !f.In(portal.Col, portal.Row) {
		return fmt.Errorf("%w: (%d, %d) is outside the floor", ErrPortal, portal.Col, portal.Row)
	}
	for _, other := range f.portals {
		if other.ID != self && other.Col == portal.Col && other.Row == portal.Row {
			return fmt.Errorf("%w: another portal is already at (%d, %d)", ErrPortal, portal.Col, portal.Row)
		}
	}

	return nil
}
