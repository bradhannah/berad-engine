package worldstate

import (
	"fmt"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Store holds values as they are while a game runs: the world's, each
// zone's and each NPC's, by declaration. A value never set, or one its
// declaration no longer fits, such as a choice since removed, is its
// declaration's default. An NPC's values are kept by the NPC's ID, so they
// go with it wherever it is.
type Store struct {
	catalog *Catalog
	world   map[ids.ULID]Value
	owned   map[ids.ULID]map[ids.ULID]Value
}

// NewStore returns a store in which every value is its default.
func NewStore(catalog *Catalog) *Store {
	return &Store{catalog: catalog, world: map[ids.ULID]Value{}, owned: map[ids.ULID]map[ids.ULID]Value{}}
}

// Declare takes a project's declarations as they are now, keeping the
// values set so far.
func (s *Store) Declare(catalog *Catalog) { s.catalog = catalog }

// Catalog returns the declarations the store holds values for.
func (s *Store) Catalog() *Catalog { return s.catalog }

// values returns the values one owner holds: the world's for a world value,
// or an NPC's or zone's.
func (s *Store) values(declared Declaration, owner ids.ULID, create bool) map[ids.ULID]Value {
	if declared.Scope == ScopeWorld {
		return s.world
	}
	if !declared.Scope.Owned() {
		return nil
	}
	values, ok := s.owned[owner]
	if !ok && create {
		values = map[ids.ULID]Value{}
		s.owned[owner] = values
	}

	return values
}

// Get returns a value: the world's, or the NPC's or zone's whose ID is
// owner. An NPC's name starts as that NPC's own display name, whichever
// NPC's name declaration is given.
func (s *Store) Get(declared Declaration, owner ids.ULID) Value {
	if declared.ID == NameID() {
		if own, ok := s.catalog.Lookup(ScopeNPC, owner, NameID()); ok {
			declared = own
		}
	}
	if value, set := s.values(declared, owner, false)[declared.ID]; set && declared.Fits(value) {
		return value
	}

	return declared.Zero()
}

// Set sets a value.
func (s *Store) Set(declared Declaration, owner ids.ULID, value Value) error {
	if !declared.Fits(value) {
		return fmt.Errorf("%w: %s cannot be %+v", ErrValue, declared.Name, value)
	}
	values := s.values(declared, owner, true)
	if values == nil {
		return fmt.Errorf("%w: %s is not kept here", ErrValue, declared.Name)
	}
	values[declared.ID] = value

	return nil
}

// Shown is a value as a window shows it: its path and what it is now.
type Shown struct {
	Path  string `json:"path"`
	Value string `json:"value"`
}

// Show lists the world's values, or one NPC's or zone's, as a tree reads.
func (s *Store) Show(scope Scope, owner ids.ULID) []Shown {
	held := s.catalog.Held(scope, owner)
	shown := make([]Shown, 0, len(held))
	for _, held := range held {
		shown = append(shown, Shown{Path: held.Path, Value: held.Declaration.Label(s.Get(held.Declaration, owner))})
	}

	return shown
}
