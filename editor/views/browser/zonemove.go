package browser

import (
	"github.com/bradhannah/berad-engine/editor/protocol"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Moving an NPC to another zone: the move dialog's button, and how wide the
// dialog's lines are.
const (
	zoneMoveID = "zone-move"
	moveLineW  = 380
)

// askMove moves an NPC to another zone, asking first when it is placed, as
// it comes off its map, or when its behaviour runs in another zone than it
// is going to, as it is then flagged; a move that changes neither happens
// at once.
func (s *State) askMove(ctx *frame.Context, npcID, zoneID ids.ULID) {
	docs := ctx.Docs
	where, isPlaced := s.placementOf(docs, npcID)
	found, _ := s.npcOf(docs, npcID)
	runs := s.runsInOf(docs, found.Behaviour)
	misfit := !found.Behaviour.IsZero() && !runs.IsZero() && runs != zoneID
	if !isPlaced && !misfit {
		s.applyMove(ctx, npcID, zoneID)

		return
	}
	index := s.index(docs)
	name, from, going := index.Name(npcID, "The NPC"), index.Name(s.zoneOfNPC(docs, npcID), "its zone"),
		index.Name(zoneID, "the zone")
	lines := []string{from + " → " + going}
	if isPlaced {
		lines = append(lines, name+" comes off "+where.name+" in "+from+". It is placed nowhere until you put it on a "+
			going+" map.")
	}
	behaviour := index.Name(found.Behaviour, "Its behaviour")
	switch {
	case misfit:
		lines = append(lines, "Its behaviour, "+behaviour+", runs in "+index.Name(runs, "another zone")+". "+name+
			" is flagged in its details until it follows one that runs in "+going+" or in any zone.")
	case !found.Behaviour.IsZero():
		lines = append(lines, "Its behaviour, "+behaviour+", runs there too.")
	}
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, s.theme.Wrap(line, moveLineW)...)
	}
	s.dialog = ui.NewDialog("Move "+name+" to "+going+"?", wrapped,
		ui.DialogButton(zoneMoveID, "Move to "+going), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialogKind, s.movingNPC, s.movingTo = dialogMoveZone, npcID, zoneID
}

// applyMove moves an NPC to a zone in one undo step: off its map, then into
// the zone.
func (s *State) applyMove(ctx *frame.Context, npcID, zoneID ids.ULID) {
	gesture := s.gesture()
	if where, isPlaced := s.placementOf(ctx.Docs, npcID); isPlaced {
		s.command(ctx, where.floor, tilefloor.RemovePlacementEdit, gesture,
			tilefloor.RemovePlacementArgs{ID: where.placement}, nil, nil)
	}
	name, zoneName := s.index(ctx.Docs).Name(npcID, "The NPC"), s.index(ctx.Docs).Name(zoneID, "the zone")
	s.command(ctx, npcID, npcplugin.SetZoneEdit, gesture, npcplugin.SetZoneArgs{Zone: zoneID},
		func(protocol.Result, error) {
			ctx.Frame.Status(name + " lives in " + zoneName + ". Place it on a " + zoneName + " map. Undo puts it back.")
		}, nil)
}
