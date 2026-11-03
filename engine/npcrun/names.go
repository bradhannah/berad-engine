package npcrun

import (
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Names names zones, workflows and sprites by their names, and places by
// their anchor, floor and zone, as Counter, Ground floor, Market; each
// returns "" for one it does not know.
type Names interface {
	Document(id ids.ULID) string
	Place(place worldstate.Place) string
}

// namer names what values refer to for their text forms: documents and
// places by Names, and an NPC by its name value as it is now.
type namer struct {
	names Names
	state *worldstate.Store
}

func (n namer) Document(documentID ids.ULID) string {
	if n.names == nil {
		return ""
	}

	return n.names.Document(documentID)
}

func (n namer) Place(place worldstate.Place) string {
	if n.names == nil {
		return ""
	}

	return n.names.Place(place)
}

func (n namer) NPC(npcID ids.ULID) string {
	if n.state == nil {
		return n.Document(npcID)
	}
	name, ok := n.state.Catalog().Lookup(worldstate.ScopeNPC, npcID, worldstate.NameID())
	if !ok {
		return n.Document(npcID)
	}

	return n.state.Get(name, npcID).Text
}
