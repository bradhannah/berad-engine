package browser

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Moving an NPC to another zone: the move dialog's button, how long the
// window waits for the NPC's workflows to load before asking with what it
// has, and how wide the dialog's lines are.
const (
	zoneMoveID   = "zone-move"
	moveWaitMost = 120
	moveLineW    = 380
)

// zoneMove is a move of an NPC to another zone, gathering what it would
// break before it asks: the workflows the NPC runs, and, as the
// orchestrator answers, the other NPCs that run each.
type zoneMove struct {
	npc, to ids.ULID
	runs    []ids.ULID
	sharers map[ids.ULID][]protocol.Use
	asked   map[ids.ULID]bool
	waited  int
}

// moveZone moves an NPC that lives in a zone to another, asking first when
// it is placed or its workflows name places, and at once when not.
func (s *State) moveZone(npcID, zoneID ids.ULID) {
	s.moving = &zoneMove{
		npc: npcID, to: zoneID, runs: nil, sharers: map[ids.ULID][]protocol.Use{}, asked: map[ids.ULID]bool{},
		waited: 0,
	}
}

// workflowOf returns a workflow the window has loaded.
func workflowOf(docs frame.Docs, workflowID ids.ULID) (*flow.Workflow, bool) {
	var found *flow.Workflow
	docs.View(workflowID, func(document client.Document) {
		if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
			found = workflowDoc.Workflow
		}
	})

	return found, found != nil
}

// updateMove gathers what a move would break, a frame at a time, and asks
// once the NPC's workflows have loaded and the orchestrator has said who
// else runs each.
func (s *State) updateMove(ctx *frame.Context) {
	move := s.moving
	found, _ := s.npcOf(ctx.Docs, move.npc)
	move.waited++
	waiting := false
	move.runs = nil
	if !found.Behaviour.IsZero() {
		move.runs = places.Callees(found.Behaviour, func(workflowID ids.ULID) (*flow.Workflow, bool) {
			return workflowOf(ctx.Docs, workflowID)
		}, func(workflowID ids.ULID) {
			if _, exists := s.index(ctx.Docs).Entry(workflowID); exists {
				s.subscribe(workflowID, ctx)
				waiting = true
			}
		})
	}
	for _, workflowID := range move.runs {
		if !move.asked[workflowID] {
			move.asked[workflowID] = true
			ctx.Requests.Runners(workflowID, func(uses []protocol.Use, err error) {
				if err == nil {
					move.sharers[workflowID] = uses
				} else {
					move.sharers[workflowID] = nil
				}
			})
		}
		if _, answered := move.sharers[workflowID]; !answered {
			waiting = true
		}
	}
	if waiting && move.waited < moveWaitMost {
		return
	}
	s.moving = nil
	s.askMove(ctx, move)
}

// breaking is a place an NPC's workflows name that a move would put out of
// its reach: the workflow, the use and the zone the place is in.
type breaking struct {
	workflow ids.ULID
	use      places.Use
	zone     ids.ULID
}

// breaks lists the places a move would put out of the NPC's reach: those
// its workflows name on floors of another zone.
func (s *State) breaks(docs frame.Docs, move *zoneMove) []breaking {
	modules := func(workflowID ids.ULID) (*flow.Workflow, bool) { return workflowOf(docs, workflowID) }
	names := func(documentID ids.ULID) string { return s.index(docs).Name(documentID, "a workflow") }
	var found []breaking
	for _, workflowID := range move.runs {
		workflow, loaded := workflowOf(docs, workflowID)
		if !loaded {
			continue
		}
		for _, use := range places.UsesIn(workflow, modules, names) {
			if zoneID, known := places.ZoneOf(docs, use.Ref.Floor); known && zoneID != move.to {
				found = append(found, breaking{workflow: workflowID, use: use, zone: zoneID})
			}
		}
	}

	return found
}

// askMove asks before moving an NPC that is placed or would lose places,
// listing what changes, and moves one that is neither at once.
func (s *State) askMove(ctx *frame.Context, move *zoneMove) {
	docs := ctx.Docs
	where, isPlaced := s.placementOf(docs, move.npc)
	broken := s.breaks(docs, move)
	if !isPlaced && len(broken) == 0 {
		s.applyMove(ctx, move.npc, move.to)

		return
	}
	index := s.index(docs)
	name, from, going := index.Name(move.npc, "The NPC"), index.Name(s.zoneOfNPC(docs, move.npc), "its zone"),
		index.Name(move.to, "the zone")
	lines := []string{from + " → " + going}
	if isPlaced {
		lines = append(lines, name+" comes off "+where.name+" in "+from+". It is placed nowhere until you put it on a "+
			going+" map.")
	}
	lines = append(lines, s.sharedLines(docs, move, broken)...)
	if len(broken) > 0 {
		lines = append(lines, "Each is flagged on its node until it has a place in "+going+".",
			"Places "+name+" could no longer go:")
	} else {
		lines = append(lines, "Nothing its workflows go to is outside "+going+".")
	}
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, s.theme.Wrap(line, moveLineW)...)
	}
	s.dialog = ui.NewDialog("Move "+name+" to "+going+"?", wrapped,
		ui.DialogButton(zoneMoveID, "Move to "+going), ui.DialogButton(ui.DialogCancel, "Cancel"))
	if len(broken) > 0 {
		s.dialog.List = s.brokenList(docs, broken)
	}
	s.dialogKind, s.movingNPC, s.movingTo = dialogMoveZone, move.npc, move.to
}

// sharedLines says which workflows with places the move breaks other NPCs
// also run, those that do not live where the NPC is going.
func (s *State) sharedLines(docs frame.Docs, move *zoneMove, broken []breaking) []string {
	var lines []string
	index := s.index(docs)
	said := map[ids.ULID]bool{}
	for _, each := range broken {
		if said[each.workflow] {
			continue
		}
		said[each.workflow] = true
		var others []string
		for _, sharer := range move.sharers[each.workflow] {
			if sharer.Document != move.npc && s.zoneOfNPC(docs, sharer.Document) != move.to {
				others = append(others, sharer.Name)
			}
		}
		if len(others) > 0 {
			lines = append(lines, index.Name(each.workflow, "A workflow")+" also runs for "+strings.Join(others, ", ")+
				": a place in "+index.Name(move.to, "the zone")+" chosen there would break it for them.")
		}
	}

	return lines
}

// brokenList lists the places a move would put out of reach, each with the
// workflow and what names it, and the zone it is in.
func (s *State) brokenList(docs frame.Docs, broken []breaking) *ui.List {
	index := s.index(docs)
	list := ui.NewList("breaks", "")
	for number, each := range broken {
		name := "a place"
		docs.View(each.use.Ref.Floor, func(document client.Document) {
			if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
				if anchor, _, found := floorDoc.Floor.Anchor(each.use.Ref.Anchor); found {
					name = anchor.Name
				}
			}
		})
		row := ui.NewRow("break-"+strconv.Itoa(number), name)
		row.Icon = ui.IconPlace
		row.Detail = index.Name(each.workflow, "a workflow") + " · " + each.use.What + " · " + index.Name(each.zone, "")
		list.Rows = append(list.Rows, row)
	}

	return list
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
