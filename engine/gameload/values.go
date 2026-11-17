package gameload

import (
	"cmp"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Owners lists the documents that declare values of their own: the
// project's NPCs and zones, NPCs first.
func Owners(source Source) []ids.ULID {
	return append(source.List(npc.FormatName), source.List(zone.FormatName)...)
}

// World returns a copy of the values the world state declares, once it has
// arrived.
func World(source Source) (*worldstate.Declarations, bool) {
	var values *worldstate.Declarations
	source.View(worldstate.ID(), func(document Document) {
		if state, ok := document.Value.(*worldstate.Document); ok {
			values = state.Values.Clone()
		}
	})

	return values, values != nil
}

// Catalog builds the catalog of a project's values: the world state's, and
// each NPC's and zone's that has arrived. It is not ready until the world
// state has.
func Catalog(source Source) (*worldstate.Catalog, bool) {
	world, ok := World(source)
	if !ok {
		return nil, false
	}
	held := map[ids.ULID]worldstate.Owner{}
	for _, ownerID := range Owners(source) {
		if owner, found := ownerOf(source, ownerID); found {
			held[ownerID] = owner
		}
	}

	return worldstate.NewCatalog(world, held), true
}

// ownerOf reads an NPC or zone as values see it.
func ownerOf(source Source, ownerID ids.ULID) (worldstate.Owner, bool) {
	var owner worldstate.Owner
	found := false
	source.View(ownerID, func(document Document) {
		switch held := document.Value.(type) {
		case npc.NPC:
			owner = worldstate.Owner{
				Scope: worldstate.ScopeNPC, Path: held.Path, Name: document.Name, Owned: held.Owned, Zone: held.Zone,
				Behaviour: held.Behaviour, Sprite: held.Sprite,
			}
			found = true
		case *zone.Zone:
			owner = worldstate.Owner{
				Scope: worldstate.ScopeZone, Path: held.Path, Name: document.Name, Owned: held.Owned,
				Zone: ids.ULID{}, Behaviour: ids.ULID{}, Sprite: ids.ULID{},
			}
			found = true
		}
	})

	return owner, found
}

// Signals returns the signals and groups the world state declares, by path,
// and whether it has arrived.
func Signals(source Source) ([]worldstate.Signal, bool) {
	var declared []worldstate.Signal
	arrived := false
	source.View(worldstate.ID(), func(document Document) {
		if state, ok := document.Value.(*worldstate.Document); ok {
			declared, arrived = slices.Clone(state.Signals), true
		}
	})
	paths := map[ids.ULID]string{}
	for _, signal := range declared {
		paths[signal.ID] = worldstate.SignalPath(declared, signal.ID)
	}
	slices.SortFunc(declared, func(a, b worldstate.Signal) int { return cmp.Compare(paths[a.ID], paths[b.ID]) })

	return declared, arrived
}

// Names names documents and places from a source, for values' text forms: a
// place as Counter, Ground floor, Market.
type Names struct {
	source Source
}

// NewNames names from a source's documents, as they are when it is asked.
func NewNames(source Source) Names { return Names{source: source} }

// Document names a document, or returns "" for one the project lacks.
func (n Names) Document(documentID ids.ULID) string { return n.source.Name(documentID) }

// Place names a place by its anchor, floor and zone, or returns "" for one
// whose anchor has not arrived.
func (n Names) Place(place worldstate.Place) string {
	anchor := ""
	n.source.View(place.Floor, func(document Document) {
		if held, ok := document.Value.(*tilefloor.Floor); ok {
			if found, _, has := held.Anchor(place.Anchor); has {
				anchor = found.Name
			}
		}
	})
	if anchor == "" {
		return ""
	}
	parts := []string{anchor}
	if floorName := n.source.Name(place.Floor); floorName != "" {
		parts = append(parts, floorName)
	}
	if zoneID, known := ZoneOf(n.source, place.Floor); known {
		parts = append(parts, n.source.Name(zoneID))
	}

	return strings.Join(parts, ", ")
}

// ZoneOf finds the zone a floor belongs to, among the zones that have
// arrived.
func ZoneOf(source Source, floorID ids.ULID) (ids.ULID, bool) {
	for _, zoneID := range source.List(zone.FormatName) {
		found := false
		source.View(zoneID, func(document Document) {
			if held, ok := document.Value.(*zone.Zone); ok {
				found = slices.Contains(held.Floors(), floorID)
			}
		})
		if found {
			return zoneID, true
		}
	}

	return ids.ULID{}, false
}
