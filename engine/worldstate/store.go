package worldstate

import (
	"fmt"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Store holds values as they are while a game runs: the world's, each
// zone's and each NPC's, by declaration. A value never set, or one its
// declaration no longer fits, such as a choice since removed, is its
// declaration's default. An NPC's values are kept by the NPC's ID, so they
// go with it to whichever zone it is placed in.
type Store struct {
	declared *Declarations
	world    map[ids.ULID]Value
	zones    map[ids.ULID]map[ids.ULID]Value
	npcs     map[ids.ULID]map[ids.ULID]Value
}

// NewStore returns a store in which every value is its default.
func NewStore(declared *Declarations) *Store {
	return &Store{
		declared: declared.Clone(), world: map[ids.ULID]Value{}, zones: map[ids.ULID]map[ids.ULID]Value{},
		npcs: map[ids.ULID]map[ids.ULID]Value{},
	}
}

// Declare takes a project's declarations as they are now, keeping the
// values set so far.
func (s *Store) Declare(declared *Declarations) { s.declared = declared.Clone() }

// Declared returns a declaration by ID.
func (s *Store) Declared(id ids.ULID) (Declaration, bool) { return s.declared.Value(id) }

// Fits reports whether a value is one of a declaration's: its type, and for
// a list one of its choices.
func (d Declaration) Fits(value Value) bool {
	if value.Type != d.Type {
		return false
	}
	if d.Type == TypeChoice {
		_, found := d.Choice(value.Choice)

		return found
	}

	return true
}

// owned returns the values one owner holds in a scope: the world's, a
// zone's or an NPC's.
func (s *Store) owned(scope Scope, owner ids.ULID, create bool) map[ids.ULID]Value {
	var owners map[ids.ULID]map[ids.ULID]Value
	switch scope {
	case ScopeWorld:
		return s.world
	case ScopeZone:
		owners = s.zones
	case ScopeNPC:
		owners = s.npcs
	case ScopeLocal:
		return nil
	}
	values, ok := owners[owner]
	if !ok && create {
		values = map[ids.ULID]Value{}
		owners[owner] = values
	}

	return values
}

// Get returns a value: the world's, the zone's or the NPC's whose ID is
// owner.
func (s *Store) Get(declared Declaration, owner ids.ULID) Value {
	if value, set := s.owned(declared.Scope, owner, false)[declared.ID]; set && declared.Fits(value) {
		return value
	}

	return declared.Zero()
}

// Set sets a value.
func (s *Store) Set(declared Declaration, owner ids.ULID, value Value) error {
	if !declared.Fits(value) {
		return fmt.Errorf("%w: %s cannot be %+v", ErrValue, declared.Path(), value)
	}
	values := s.owned(declared.Scope, owner, true)
	if values == nil {
		return fmt.Errorf("%w: %s is not kept here", ErrValue, declared.Path())
	}
	values[declared.ID] = value

	return nil
}

// Shown is a value as a window shows it: its path and what it is now.
type Shown struct {
	Path  string `json:"path"`
	Value string `json:"value"`
}

// Show lists one owner's values in a scope, in the order they were
// declared.
func (s *Store) Show(scope Scope, owner ids.ULID) []Shown {
	var shown []Shown
	for _, declared := range s.declared.Values() {
		if declared.Scope == scope {
			shown = append(shown, Shown{Path: declared.Path(), Value: declared.Label(s.Get(declared, owner))})
		}
	}

	return shown
}
