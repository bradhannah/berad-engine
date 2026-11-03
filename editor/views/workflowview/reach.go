package workflowview

import (
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// subscribeHomes keeps loaded what says whether the workflow's NPCs can
// reach its places: the NPCs, every zone, and the floors of the zones they
// live in.
func (s *State) subscribeHomes(ctx *frame.Context) {
	index := project.NewIndex(nil)
	ctx.Docs.View(project.IndexID(), func(document client.Document) {
		if held, ok := document.Model.(*project.Index); ok {
			index = project.NewIndex(held.Entries())
		}
	})
	for _, entry := range index.OfFormat(zone.FormatName) {
		s.subscribe(entry.ID, ctx)
	}
	for _, each := range s.runnersOf(ctx.Docs) {
		s.subscribe(each.npc, ctx)
		if _, exists := index.Entry(each.zone); !exists {
			continue
		}
		for _, floorID := range zoneFloors(ctx.Docs, each.zone) {
			s.subscribe(floorID, ctx)
		}
	}
}

// zoneFloors lists a loaded zone's floors from the bottom up.
func zoneFloors(docs frame.Docs, zoneID ids.ULID) []ids.ULID {
	var floors []ids.ULID
	docs.View(zoneID, func(document client.Document) {
		if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
			floors = zoneDoc.Zone.Floors()
		}
	})

	return floors
}

// reachKey changes whenever whether the workflow's NPCs can reach a place
// may have: who runs it, where they live, and their zones and floors.
func (s *State) reachKey(docs frame.Docs) string {
	parts := make([]string, 0, len(s.runs.uses))
	revision := func(documentID ids.ULID) string {
		seen := "-"
		docs.View(documentID, func(document client.Document) { seen = strconv.FormatUint(document.Revision, 10) })

		return documentID.String() + ":" + seen
	}
	for _, each := range s.runnersOf(docs) {
		parts = append(parts, revision(each.npc), revision(each.zone))
		for _, floorID := range zoneFloors(docs, each.zone) {
			parts = append(parts, revision(floorID))
		}
	}

	return strings.Join(parts, ",")
}

// reachProblem says why a fixed place does not fit the workflow: it is in
// another zone than the workflow runs in, the workflow runs in any zone, or
// it is on a floor no stairs lead to from the one an NPC following the
// workflow is placed on, naming the first NPC that cannot reach it. It
// returns "" for a place that fits.
func (s *State) reachProblem(docs frame.Docs, ref floor.AnchorRef) string {
	placeZone, known := places.ZoneOf(docs, ref.Floor)
	if !known || s.workflow == nil {
		return ""
	}
	index := s.names(docs)
	place, name, runs := anchorName(docs, ref), s.name(docs), s.runsIn(docs)
	if runs.IsZero() {
		return place + " is a fixed place, and " + name + " runs in any zone. Give it a place input or another " +
			"value, or run it in " + index.Name(placeZone, "its zone") + "."
	}
	if placeZone != runs {
		return place + " is in " + index.Name(placeZone, "another zone") + ", and " + name + " runs in " +
			s.runsInName(docs) + ". Choose a place in " + s.runsInName(docs) + "."
	}
	for _, each := range s.runnersOf(docs) {
		if each.zone != runs {
			continue
		}
		from, placed := placedOn(docs, each.zone, each.npc)
		if placed && !stairsLead(docs, zoneFloors(docs, each.zone), from, ref.Floor) {
			return "No stairs lead from " + index.Name(from, "its floor") + " to " + index.Name(ref.Floor, "that floor") +
				", so " + each.name + " can't reach " + place + "."
		}
	}

	return ""
}

// names returns the project index as the window has it.
func (s *State) names(docs frame.Docs) *project.Index {
	index := project.NewIndex(nil)
	docs.View(project.IndexID(), func(document client.Document) {
		if held, ok := document.Model.(*project.Index); ok {
			index = project.NewIndex(held.Entries())
		}
	})

	return index
}

// anchorName names a place, or says it is a place when its floor is not
// loaded.
func anchorName(docs frame.Docs, ref floor.AnchorRef) string {
	name := "The place"
	docs.View(ref.Floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			if anchor, _, found := floorDoc.Floor.Anchor(ref.Anchor); found {
				name = anchor.Name
			}
		}
	})

	return name
}

// placedOn finds the floor of its zone an NPC is placed on.
func placedOn(docs frame.Docs, zoneID, npcID ids.ULID) (ids.ULID, bool) {
	for _, floorID := range zoneFloors(docs, zoneID) {
		found := false
		docs.View(floorID, func(document client.Document) {
			if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
				for _, placement := range floorDoc.Floor.Placements() {
					found = found || placement.NPC == npcID
				}
			}
		})
		if found {
			return floorID, true
		}
	}

	return ids.ULID{}, false
}

// stairsLead reports whether stairs lead from one floor of a zone to a
// target floor, through any floors between: a floor's stairs up lead to the
// floor above it and its stairs down to the one below.
func stairsLead(docs frame.Docs, floors []ids.ULID, from, target ids.ULID) bool {
	reached := map[ids.ULID]bool{from: true}
	waiting := []ids.ULID{from}
	for len(waiting) > 0 {
		at := waiting[0]
		waiting = waiting[1:]
		position := slices.Index(floors, at)
		for _, climb := range climbs(docs, at) {
			next := position + climb
			if position < 0 || next < 0 || next >= len(floors) || reached[floors[next]] {
				continue
			}
			reached[floors[next]] = true
			waiting = append(waiting, floors[next])
		}
	}

	return reached[target]
}

// climbs lists where a floor's stairs lead: 1 for up, -1 for down, each
// once.
func climbs(docs frame.Docs, floorID ids.ULID) []int {
	var found []int
	docs.View(floorID, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloor.Document)
		if !ok {
			return
		}
		bounds := floorDoc.Floor.Bounds()
		for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
			for col := bounds.Min.X; col < bounds.Max.X; col++ {
				top, err := floorDoc.Floor.Top(col, row)
				if climb := tiles.Climb(top); err == nil && climb != 0 && !slices.Contains(found, climb) {
					found = append(found, climb)
				}
			}
		}
	})

	return found
}

// callReachProblem says which place a Call gives its module the NPCs that
// run the workflow cannot reach, or returns "".
func (s *State) callReachProblem(docs frame.Docs, node flow.Node, module *flow.Workflow) string {
	for _, input := range module.Inputs() {
		if input.Type != worldstate.TypePlace {
			continue
		}
		value, err := input.Parse(node.Inputs()[input.ID])
		if err != nil || value.Place.IsZero() {
			continue
		}
		ref := floor.AnchorRef{Floor: value.Place.Floor, Anchor: value.Place.Anchor}
		if problem := s.reachProblem(docs, ref); problem != "" {
			return "Its " + input.Name + ": " + problem
		}
	}

	return ""
}

// localReach says, for each place local that starts at a place the
// workflow's NPCs cannot reach, why; an input's start is only where it
// starts when nothing gives it one, so inputs are left out.
func (s *State) localReach(docs frame.Docs) map[ids.ULID]string {
	found := map[ids.ULID]string{}
	if s.workflow == nil {
		return found
	}
	for _, declared := range s.workflow.Locals().Values() {
		start := declared.Zero().Place
		if declared.Type != worldstate.TypePlace || declared.Role == worldstate.RoleInput || start.IsZero() {
			continue
		}
		if problem := s.reachProblem(docs, floor.AnchorRef{Floor: start.Floor, Anchor: start.Anchor}); problem != "" {
			found[declared.ID] = problem
		}
	}

	return found
}
