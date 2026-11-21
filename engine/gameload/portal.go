package gameload

import (
	"slices"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Why a portal leads nowhere.
const (
	// NoFloor is a portal whose anchor's floor is not in the project.
	NoFloor = "the floor its anchor was on is gone"
	// NoAnchor is a portal whose anchor is not on its floor.
	NoAnchor = "its anchor is gone"
	// NoZone is a portal whose anchor's floor is in no zone.
	NoZone = "its anchor's floor is in no zone"
)

// Destination finds where a portal leads (increment 64): the zone, floor and
// cell of its anchor, for the game to put the player on, as Load's from. It
// reports Held with the place; Waiting while a document it needs has not
// arrived, as an editor window's may not have; and Missing with why the
// portal leads nowhere. Whether the player can stand there is for the game
// to say, when it is asked to put them there.
func Destination(source Source, leads tilefloor.AnchorRef) (game.Start, State, string) {
	nowhere := game.Start{Zone: ids.ULID{}, Floor: ids.ULID{}, Col: 0, Row: 0}
	var anchor tilefloor.Anchor
	found := false
	switch source.View(leads.Floor, func(document Document) {
		if held, ok := document.Value.(*tilefloor.Floor); ok {
			anchor, _, found = held.Anchor(leads.Anchor)
		}
	}) {
	case Waiting:
		return nowhere, Waiting, ""
	case Missing:
		return nowhere, Missing, NoFloor
	case Held:
	}
	if !found {
		return nowhere, Missing, NoAnchor
	}
	waiting := false
	for _, zoneID := range source.List(zone.FormatName) {
		lists := false
		if source.View(zoneID, func(document Document) {
			if held, ok := document.Value.(*zone.Zone); ok {
				lists = slices.Contains(held.Floors(), leads.Floor)
			}
		}) == Waiting {
			waiting = true
		}
		if lists {
			return game.Start{Zone: zoneID, Floor: leads.Floor, Col: anchor.Col, Row: anchor.Row}, Held, ""
		}
	}
	if waiting {
		return nowhere, Waiting, ""
	}

	return nowhere, Missing, NoZone
}
