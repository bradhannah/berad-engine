package orchestrator

import (
	"maps"
	"slices"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// workflowZonesSince is the workflow format version that gave each
// workflow the zone it runs in; a workflow saved before it is given one
// when the project opens.
const workflowZonesSince = 8

// settleRunsIn gives each of the workflows named that runs in any zone, as
// one saved before workflows had zones does, the zone it belongs in: the
// zone its followers live in, when they all live in one; else the zone its
// fixed places are in, when they are all in one; else, when it names fixed
// places in several zones, the zone most of them are in, the rest then
// flagged. One that names no fixed place keeps running in any zone. Like an
// upgrade, the change is saved at the next save. It returns the workflows
// it changed.
func (o *Orchestrator) settleRunsIn(settle map[ids.ULID]bool) []*documentEntry {
	zoneOf := map[ids.ULID]ids.ULID{}
	homes := map[ids.ULID][]ids.ULID{}
	for documentID, entry := range o.docs {
		switch held := entry.document.(type) {
		case *zoneplugin.Document:
			for _, floorID := range held.Zone.Floors() {
				zoneOf[floorID] = documentID
			}
		case *npcplugin.Document:
			if !held.NPC.Behaviour.IsZero() && !held.NPC.Zone.IsZero() {
				homes[held.NPC.Behaviour] = append(homes[held.NPC.Behaviour], held.NPC.Zone)
			}
		}
	}
	modules := func(workflowID ids.ULID) (*flow.Workflow, bool) {
		if entry, held := o.docs[workflowID]; held {
			if workflowDoc, isWorkflow := entry.document.(*workflowplugin.Document); isWorkflow {
				return workflowDoc.Workflow, true
			}
		}

		return nil, false
	}
	var settled []*documentEntry
	for _, workflowID := range slices.SortedFunc(maps.Keys(settle), ids.ULID.Compare) {
		workflow, isWorkflow := modules(workflowID)
		if !isWorkflow || !workflow.RunsIn().IsZero() {
			continue
		}
		var placed []ids.ULID
		for _, use := range flow.PlacesIn(workflow, modules) {
			if zoneID, known := zoneOf[use.Place.Floor]; known {
				placed = append(placed, zoneID)
			}
		}
		if zoneID := belongsIn(homes[workflowID], placed); !zoneID.IsZero() {
			workflow.SetRunsIn(zoneID)
			o.docs[workflowID].dirty = true
			settled = append(settled, o.docs[workflowID])
		}
	}

	return settled
}

// belongsIn picks the zone a workflow belongs in from the zones its
// followers live in and the zones of its fixed places, as settleRunsIn
// says; zero is any zone.
func belongsIn(homes, placed []ids.ULID) ids.ULID {
	if distinct := slices.Compact(slices.SortedFunc(slices.Values(homes), ids.ULID.Compare)); len(distinct) == 1 {
		return distinct[0]
	}
	counts := map[ids.ULID]int{}
	for _, zoneID := range placed {
		counts[zoneID]++
	}
	best := ids.ULID{}
	for _, zoneID := range slices.SortedFunc(maps.Keys(counts), ids.ULID.Compare) {
		if best.IsZero() || counts[zoneID] > counts[best] {
			best = zoneID
		}
	}

	return best
}

// workflows names every workflow the project holds.
func (o *Orchestrator) workflows() map[ids.ULID]bool {
	found := map[ids.ULID]bool{}
	for documentID, entry := range o.docs {
		if _, isWorkflow := entry.document.(*workflowplugin.Document); isWorkflow {
			found[documentID] = true
		}
	}

	return found
}
