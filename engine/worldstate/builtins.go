package worldstate

import (
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// The IDs of the built-in values every NPC has besides its name: the zone
// it lives in, its behaviour and the sprite it shows. Their time parts are
// zero, so no generated ID ever equals them.
const (
	zoneID      = "00000000000000000000000ZN1"
	behaviourID = "00000000000000000000000BHV"
	spriteID    = "00000000000000000000000SPR"
)

// ZoneID returns the ID of every NPC's zone value.
func ZoneID() ids.ULID { return ids.MustParse(zoneID) }

// BehaviourID returns the ID of every NPC's behaviour value.
func BehaviourID() ids.ULID { return ids.MustParse(behaviourID) }

// SpriteID returns the ID of every NPC's sprite value.
func SpriteID() ids.ULID { return ids.MustParse(spriteID) }

// IsBuiltIn reports whether an ID is one of every NPC's built-in values:
// its name, zone, behaviour or sprite.
func IsBuiltIn(id ids.ULID) bool {
	return slices.Contains([]ids.ULID{NameID(), ZoneID(), BehaviourID(), SpriteID()}, id)
}

// ReadOnly reports a built-in value no workflow sets: an NPC's zone and
// behaviour, which are set in its details.
func ReadOnly(id ids.ULID) bool { return id == ZoneID() || id == BehaviourID() }

// BuiltIns returns an NPC's built-in values, each starting as the NPC has
// it: its name, the zone it lives in, its behaviour and its sprite.
func BuiltIns(owner Owner) []Declaration {
	return []Declaration{
		NameDeclaration(owner.Name),
		builtIn(ZoneID(), "zone", TypeZone, owner.Zone, "The zone it lives in, set in its details."),
		builtIn(BehaviourID(), "behaviour", TypeWorkflow, owner.Behaviour, "Its behaviour, set in its details."),
		builtIn(SpriteID(), "sprite", TypeSprite, owner.Sprite,
			"The sprite it shows. It starts as the sprite in its details, and its workflows can change it."),
	}
}

// builtInOf returns one of an NPC's built-in values by ID.
func builtInOf(owner Owner, id ids.ULID) Declaration {
	for _, declared := range BuiltIns(owner) {
		if declared.ID == id {
			return declared
		}
	}

	return New(ids.ULID{}, "", "", "")
}

// builtIn declares a built-in value referring to a document, starting as
// it.
func builtIn(id ids.ULID, name string, kind Type, ref ids.ULID, description string) Declaration {
	declared := New(id, name, ScopeNPC, kind)
	start := declared.Empty()
	start.Ref = ref
	declared.Default, declared.Description = start.Encode(), description

	return declared
}
