package mapview

import (
	"slices"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// maxModules bounds how many workflows the preview loads for its NPCs'
// Calls.
const maxModules = 64

// moduleCopy is a copy of a workflow an NPC's behaviour calls, at the
// revision last seen.
type moduleCopy struct {
	workflow *flow.Workflow
	revision uint64
}

// refreshModules keeps copies of the workflows the NPCs' behaviours call,
// and those they call in turn, current; an NPC inside a call to one that
// changed starts its behaviour again, as an edit to the behaviour does.
func (s *State) refreshModules(ctx *frame.Context, runners []*npcRun) {
	var waiting []ids.ULID
	for _, run := range runners {
		if run.workflow != nil {
			waiting = append(waiting, run.workflow.Callees()...)
		}
	}
	seen := map[ids.ULID]bool{}
	var changed []ids.ULID
	for len(waiting) > 0 && len(seen) < maxModules {
		target := waiting[0]
		waiting = waiting[1:]
		if seen[target] {
			continue
		}
		seen[target] = true
		s.subscribe(target, ctx)
		ctx.Docs.View(target, func(document client.Document) {
			workflowDoc, ok := document.Model.(*workflowplugin.Document)
			if !ok {
				return
			}
			if held, kept := s.preview.modules[target]; !kept || held.revision != document.Revision {
				s.preview.modules[target] = moduleCopy{workflow: workflowDoc.Workflow.Clone(), revision: document.Revision}
				changed = append(changed, target)
			}
		})
		if held, kept := s.preview.modules[target]; kept {
			waiting = append(waiting, held.workflow.Callees()...)
		}
	}
	for _, run := range runners {
		if run.npc.Module() != (ids.ULID{}) && slices.ContainsFunc(changed, func(target ids.ULID) bool {
			return inside(run, target)
		}) {
			run.npc.Restart()
		}
	}
}

// inside reports whether an NPC is inside a call to a workflow.
func inside(run *npcRun, target ids.ULID) bool {
	if run.npc.Module() == target {
		return true
	}

	return slices.ContainsFunc(run.npc.Callers(), func(call npcrun.Caller) bool { return call.Workflow == target })
}

// module finds a workflow the NPCs' Calls call, as last copied.
func (s *State) module(target ids.ULID) (*flow.Workflow, bool) {
	held, ok := s.preview.modules[target]

	return held.workflow, ok
}

// running is the workflow an NPC is following now: its behaviour, or the
// module it is in a call to.
func (s *State) running(run *npcRun) *flow.Workflow {
	if run.npc.Module() == (ids.ULID{}) {
		return run.workflow
	}
	held, ok := s.preview.modules[run.npc.Module()]
	if !ok {
		return nil
	}

	return held.workflow
}
