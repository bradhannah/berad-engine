package orchestrator

import (
	"cmp"
	"errors"
	"slices"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// settleHomes gives each NPC that lives in no zone yet, as every NPC saved
// before NPCs had zones does, the zone of the floor it is placed on. Like an
// upgrade, the change is saved at the next save. An NPC placed nowhere
// keeps no zone until its author gives it one. It returns the NPCs it
// changed.
func (o *Orchestrator) settleHomes() []*documentEntry {
	zoneOf := map[ids.ULID]ids.ULID{}
	placedOn := map[ids.ULID]ids.ULID{}
	for documentID, entry := range o.docs {
		switch held := entry.document.(type) {
		case *zoneplugin.Document:
			for _, floorID := range held.Zone.Floors() {
				zoneOf[floorID] = documentID
			}
		case *tilefloor.Document:
			for _, placement := range held.Floor.Placements() {
				placedOn[placement.NPC] = documentID
			}
		}
	}
	var settled []*documentEntry
	for documentID, entry := range o.docs {
		npcDoc, isNPC := entry.document.(*npcplugin.Document)
		if !isNPC || !npcDoc.NPC.Zone.IsZero() {
			continue
		}
		if zoneID, placed := zoneOf[placedOn[documentID]]; placed {
			npcDoc.NPC.Zone = zoneID
			entry.dirty = true
			settled = append(settled, entry)
		}
	}

	return settled
}

// SettleHomes gives each NPC placed on a zone's floor that lives in no zone
// yet that zone, as opening a project does, and saves them, for content
// made before Run, such as a sample project's. Call before Run.
func (o *Orchestrator) SettleHomes() error {
	if o.running.Load() {
		return ErrRunning
	}
	var errs []error
	for _, entry := range o.settleHomes() {
		errs = append(errs, o.save(entry))
	}

	return errors.Join(errs...)
}

// runners lists the NPCs that run a workflow, by name: those whose
// behaviour it is, and those whose behaviour calls it through any number of
// Calls.
func (o *Orchestrator) runners(workflowID ids.ULID) []protocol.Use {
	running := map[ids.ULID]bool{workflowID: true}
	waiting := []ids.ULID{workflowID}
	for len(waiting) > 0 {
		called := waiting[0]
		waiting = waiting[1:]
		question := protocol.Query{Question: protocol.QuestionWhereUsed, Document: called, Item: workflowplugin.CallItem}
		for _, use := range o.whereUsed(question) {
			if !running[use.Document] {
				running[use.Document] = true
				waiting = append(waiting, use.Document)
			}
		}
	}
	runs := []protocol.Use{}
	for documentID, entry := range o.docs {
		npcDoc, isNPC := entry.document.(*npcplugin.Document)
		if !isNPC || !running[npcDoc.NPC.Behaviour] {
			continue
		}
		label := protocol.RunsByCall
		if npcDoc.NPC.Behaviour == workflowID {
			label = protocol.RunsAsBehaviour
		}
		runs = append(runs, protocol.Use{
			Document: documentID, Name: entry.info.Name, Format: entry.info.Format, Label: label, Item: "",
		})
	}
	slices.SortFunc(runs, func(a, b protocol.Use) int { return cmp.Compare(a.Name, b.Name) })

	return runs
}
