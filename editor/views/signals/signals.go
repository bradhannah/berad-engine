// Package signals reads a project's signals for the windows that show
// them: the signals the world state declares, and where workflows send and
// answer each, from the workflows a window holds.
package signals

import (
	"cmp"
	"slices"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Declared returns the signals the world state declares, by name, and
// whether it has loaded.
func Declared(docs frame.Docs) ([]worldstate.Signal, bool) {
	var declared []worldstate.Signal
	loaded := false
	docs.View(worldstate.ID(), func(document client.Document) {
		if stateDoc, ok := document.Model.(*stateplugin.Document); ok {
			declared, loaded = slices.Clone(stateDoc.State.Signals), true
		}
	})
	slices.SortFunc(declared, func(a, b worldstate.Signal) int { return cmp.Compare(a.Name, b.Name) })

	return declared, loaded
}

// Name names a signal, or says it is missing.
func Name(docs frame.Docs, signal ids.ULID) string {
	declared, _ := Declared(docs)
	for _, each := range declared {
		if each.ID == signal {
			return each.Name
		}
	}

	return "a missing signal"
}

// Documents lists what signals' uses are read from, for a window to keep
// loaded: every workflow and NPC.
func Documents(docs frame.Docs) []ids.ULID {
	var documents []ids.ULID
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			for _, entry := range slices.Concat(index.OfFormat(flow.FormatName), index.OfFormat(npc.FormatName)) {
				documents = append(documents, entry.ID)
			}
		}
	})

	return documents
}

// Use is a node that sends or answers a signal: its workflow, the
// workflow's name, the node and its kind.
type Use struct {
	Workflow ids.ULID
	Name     string
	Node     ids.ULID
	Kind     string
}

// Uses lists, for each signal, the nodes that send or answer it, in the
// workflows loaded so far, by workflow name.
func Uses(docs frame.Docs) map[ids.ULID][]Use {
	uses := map[ids.ULID][]Use{}
	var entries []project.Entry
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = index.OfFormat(flow.FormatName)
		}
	})
	for _, entry := range entries {
		docs.View(entry.ID, func(document client.Document) {
			workflowDoc, ok := document.Model.(*workflowplugin.Document)
			if !ok {
				return
			}
			for _, node := range workflowDoc.Workflow.Nodes() {
				signal, chosen := node.Signal()
				if chosen && (node.Kind == flow.KindOnSignal || node.Kind == flow.KindSendSignal) {
					uses[signal] = append(uses[signal], Use{Workflow: entry.ID, Name: entry.Name, Node: node.ID, Kind: node.Kind})
				}
			}
		})
	}

	return uses
}

// Answered reports whether any use of a signal answers it, and Sent whether
// any sends it.
func Answered(uses []Use) bool {
	return slices.ContainsFunc(uses, func(use Use) bool { return use.Kind == flow.KindOnSignal })
}

// Sent reports whether any use of a signal sends it.
func Sent(uses []Use) bool {
	return slices.ContainsFunc(uses, func(use Use) bool { return use.Kind == flow.KindSendSignal })
}

// Answers reports whether a workflow answers a signal, and whether the
// workflow has loaded.
func Answers(docs frame.Docs, workflow, signal ids.ULID) (bool, bool) {
	answers, loaded := false, false
	docs.View(workflow, func(document client.Document) {
		if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
			_, answers = workflowDoc.Workflow.Receivers()[signal]
			loaded = true
		}
	})

	return answers, loaded
}

// Behaviour returns an NPC's behaviour, and whether the NPC has loaded.
func Behaviour(docs frame.Docs, npcID ids.ULID) (ids.ULID, bool) {
	behaviour, loaded := ids.ULID{}, false
	docs.View(npcID, func(document client.Document) {
		if npcDoc, ok := document.Model.(*npcplugin.Document); ok {
			behaviour, loaded = npcDoc.NPC.Behaviour, true
		}
	})

	return behaviour, loaded
}

// Loaded reports whether every workflow and NPC the project holds has
// loaded, so a signal's uses are all known.
func Loaded(docs frame.Docs) bool {
	for _, document := range Documents(docs) {
		loaded := false
		docs.View(document, func(client.Document) { loaded = true })
		if !loaded {
			return false
		}
	}

	return true
}
