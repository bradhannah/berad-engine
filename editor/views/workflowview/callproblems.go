package workflowview

import (
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Problems of a Call. The game skips a Call with any of them, carrying on
// by done.
const (
	problemNoWorkflow   = "No workflow chosen yet."
	problemWorkflowGone = "The workflow it calls was deleted. Choose another workflow."
	// maxWatched bounds how many workflows the window loads to find a
	// workflow that calls itself through others.
	maxWatched = 64
)

// callProblem says what is wrong with a Call: no workflow, one deleted,
// one that comes to call this workflow again, a way out the module lost,
// or an input or output whose value no longer fits it.
func (s *State) callProblem(node flow.Node, docs frame.Docs) string {
	target, ok := node.Called()
	if !ok {
		return problemNoWorkflow
	}
	if s.workflowName(docs, target) == "" {
		return problemWorkflowGone
	}
	if back := flow.CallsBack(s.document, target, s.lookup(docs)); back != nil {
		return "Calls itself: " + s.chain(docs, back) + ". Choose another workflow."
	}
	called, loaded := s.modules[target]
	if !loaded {
		return ""
	}
	if problem := s.callFitProblem(docs, called); problem != "" {
		return problem
	}
	if missing := flow.MissingWays(node, called.workflow); len(missing) > 0 {
		return called.name + " has no way out called " + missing[0].Label +
			" any more. Remove the line, or add the Return back."
	}
	if unfit := flow.UnfitInputs(node, called.workflow); len(unfit) > 0 {
		input := unfit[0]

		return "The input " + input.Name + " is now " + input.Type.Label() + "; " +
			strings.Trim(string(node.Inputs()[input.ID]), `"`) + " no longer fits. " + giveAnew(input.Type) +
			", or choose another value."
	}

	if problem := s.refProblem(node, docs, called.workflow); problem != "" {
		return problem
	}

	return s.callReachProblem(docs, node, called.workflow)
}

// callFitProblem says why a workflow called does not fit this one: it is
// of another kind, or runs in another zone than this one does; a workflow
// running in any zone calls only those running in any zone.
func (s *State) callFitProblem(docs frame.Docs, called module) string {
	name := s.name(docs)
	if purpose := called.workflow.Purpose(); purpose != s.workflow.Purpose() {
		return called.name + " is " + article(purpose.Label()) + ", and " + name + " " +
			article(s.workflow.Purpose().Label()) + ". Call one of its own kind."
	}
	runs, here := called.workflow.RunsIn(), s.runsIn(docs)
	if _, exists := s.names(docs).Entry(runs); !exists {
		runs = ids.ULID{}
	}
	switch {
	case runs.IsZero() || runs == here:
		return ""
	case here.IsZero():
		return called.name + " runs in " + s.names(docs).Name(runs, "a zone") + ", and " + name +
			" runs in any zone. Call one that runs in any zone."
	default:
		return called.name + " runs in " + s.names(docs).Name(runs, "another zone") + ", and " + name + " runs in " +
			s.runsInName(docs) + ". Call one that runs there or in any zone."
	}
}

// article puts "a" or "an" before a kind of workflow, lowercased but for
// NPC.
func article(label string) string {
	lowered := strings.ToLower(label[:1]) + label[1:]
	if strings.HasPrefix(label, "NPC") {
		return "an " + label
	}

	return "a " + lowered
}

// refProblem says which input is given, or output kept in, another value
// of a type that no longer fits it.
func (s *State) refProblem(node flow.Node, docs frame.Docs, module *flow.Workflow) string {
	given := node.Inputs()
	for _, input := range module.Inputs() {
		ref, refers := flow.OperandRef(given[input.ID])
		if declared, path, problem := s.resolve(docs, ref); refers && (problem != "" || declared.Type != input.Type) {
			return "The input " + input.Name + " is now " + input.Type.Label() + "; " + path + problem +
				" no longer fits. Choose another value."
		}
	}
	keeps := node.Keeps()
	for _, output := range module.Outputs() {
		kept, keeping := keeps[output.ID]
		if !keeping || kept.IsZero() {
			continue
		}
		if declared, path, problem := s.resolve(docs, kept); problem != "" || declared.Type != output.Type {
			return "The output " + output.Name + " is now " + output.Type.Label() + "; " + path + problem +
				" no longer fits. Choose another value to keep it in."
		}
	}

	return ""
}

// giveAnew says how to give an input of a type a value again.
func giveAnew(kind worldstate.Type) string {
	switch kind {
	case worldstate.TypeInteger:
		return "Type a whole number"
	case worldstate.TypeText:
		return "Type text"
	case worldstate.TypeBool:
		return "Choose true or false"
	case worldstate.TypeChoice:
		return "Choose one of its choices"
	case worldstate.TypePlace:
		return "Pick a place on a map"
	case worldstate.TypeNPC, worldstate.TypeGroup, worldstate.TypeTemplate,
		worldstate.TypeZone, worldstate.TypeWorkflow, worldstate.TypeSprite:
	}

	return "Clear it"
}

// chain names how a workflow comes to call itself, as "Shopkeeper day
// calls Go to work, which calls Shopkeeper day", or "Errands calls
// itself".
func (s *State) chain(docs frame.Docs, path []ids.ULID) string {
	names := make([]string, 0, len(path))
	for _, step := range path {
		names = append(names, s.workflowName(docs, step))
	}
	if len(names) == 2 && path[0] == path[1] {
		return names[0] + " calls itself"
	}

	return names[0] + " calls " + strings.Join(names[1:], ", which calls ")
}

// lookup finds a workflow as loaded: this one as the window shows it, or
// another as last read.
func (s *State) lookup(docs frame.Docs) flow.Lookup {
	return func(target ids.ULID) (*flow.Workflow, bool) {
		if target == s.document {
			return s.workflow, s.workflow != nil
		}
		var found *flow.Workflow
		docs.View(target, func(document client.Document) {
			if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
				found = workflowDoc.Workflow
			}
		})

		return found, found != nil
	}
}

// watchCalls loads the workflows the Calls name, and those they call in
// turn, so a workflow that comes to call itself is found.
func (s *State) watchCalls(ctx *frame.Context) {
	if s.workflow == nil {
		return
	}
	lookup := s.lookup(ctx.Docs)
	waiting, seen := callees(s.workflow), map[ids.ULID]bool{s.document: true}
	for len(waiting) > 0 && len(seen) < maxWatched {
		target := waiting[0]
		waiting = waiting[1:]
		if seen[target] {
			continue
		}
		seen[target] = true
		s.subscribe(target, ctx)
		if called, ok := lookup(target); ok {
			waiting = append(waiting, callees(called)...)
		}
	}
}
