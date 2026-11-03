package worldstate

import (
	"cmp"
	"strconv"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Namer names what values refer to, for their text forms: a zone, a
// workflow or a sprite by its name, a place by its anchor, floor and zone,
// and an NPC by its name value as it is now. Each returns "" for one it
// does not know.
type Namer interface {
	Document(id ids.ULID) string
	Place(place Place) string
	NPC(id ids.ULID) string
}

// Text is a value's text form, as a Log line prints it or a text value is
// set from it: an integer in digits, true or false, a choice by its name,
// text as it is, an NPC by its name value, a place as Counter, Ground floor,
// Market, and a zone, a workflow or a sprite by its name. None is no zone,
// no behaviour, no workflow, no sprite, no place or none.
func Text(declared Declaration, value Value, namer Namer) string {
	switch declared.Type {
	case TypeInteger:
		return strconv.Itoa(value.Integer)
	case TypeBool:
		return strconv.FormatBool(value.Bool)
	case TypeText:
		return value.Text
	case TypeNPC:
		if value.NPC.IsZero() {
			return labelNone
		}

		return cmp.Or(namer.NPC(value.NPC), "an NPC")
	case TypePlace:
		if value.Place.IsZero() {
			return "no place"
		}

		return cmp.Or(namer.Place(value.Place), "a place")
	case TypeZone, TypeWorkflow, TypeSprite:
		if value.Ref.IsZero() {
			return noRef(declared)
		}

		return cmp.Or(namer.Document(value.Ref), "a deleted "+string(declared.Type))
	case TypeChoice, TypeGroup, TypeTemplate:
	}

	return declared.Label(value)
}

// noRef is how a zone, a workflow or a sprite value that refers to none
// reads.
func noRef(declared Declaration) string {
	if declared.ID == BehaviourID() {
		return "no behaviour"
	}

	return "no " + string(declared.Type)
}
