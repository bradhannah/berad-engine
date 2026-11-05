package workflowview

import (
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Families of problem, as the Project window's Problems facet lists them:
// what is wrong with a node, by the kind of node, or that nothing links to
// it; a receiver nothing sends to; and a workflow nothing runs or calls.
const (
	FamilyNotLinked = "Not linked"
	FamilyGoTo      = "Go to"
	FamilySignals   = "Signals"
	FamilyValues    = "Values"
	FamilySounds    = "Sounds"
	FamilyCalls     = "Calls"
	FamilyLanes     = "Lanes"
	FamilyUnheard   = "Unheard receiver"
	FamilyUnused    = "Unused"
)

// Check finds a workflow's problems as its canvas does, without a window
// showing it, for the Project window's Problems facet: the same checks on
// the same documents, asked of the orchestrator the same way.
type Check struct {
	state *State
}

// NewCheck returns a check of one workflow.
func NewCheck(document ids.ULID, theme *ui.Theme) *Check {
	return &Check{state: New(ids.ULID{}, document, theme)}
}

// Update keeps loaded what the workflow's problems depend on, and finds them
// again when any of it has changed.
func (c *Check) Update(ctx *frame.Context) {
	state := c.state
	state.subscribe(state.document, ctx)
	state.subscribe(project.IndexID(), ctx)
	state.load(ctx.Docs)
	state.ask(ctx)
	if state.watchFloors && state.workflow != nil {
		state.watchFloors = false
		for _, floorID := range state.destinationFloors() {
			state.subscribe(floorID, ctx)
		}
	}
}

// Families returns the families of the workflow's problems and warnings,
// each once, and Unused once the orchestrator has said that no NPC or zone
// runs it and nothing calls it.
func (c *Check) Families() []string {
	state := c.state
	var families []string
	add := func(family string) {
		if !slices.Contains(families, family) {
			families = append(families, family)
		}
	}
	if state.workflow == nil {
		return nil
	}
	for _, node := range state.workflow.Nodes() {
		if problem, found := state.problems[node.ID]; found {
			add(state.family(node, problem))
		}
		if _, warned := state.warnings[node.ID]; warned {
			add(FamilyUnheard)
		}
	}
	if state.runs.known && state.callers.known && len(state.runs.uses) == 0 && len(state.callers.names) == 0 {
		add(FamilyUnused)
	}

	return families
}

// family is the family of a node's problem.
func (s *State) family(node flow.Node, problem string) string {
	switch {
	case problem == problemUnreached || problem == problemNoFirstStep:
		return FamilyNotLinked
	case problem == s.laneProblem(node):
		return FamilyLanes
	}
	switch node.Kind {
	case flow.KindGoTo:
		return FamilyGoTo
	case flow.KindOnSignal, flow.KindResume, flow.KindSendSignal:
		return FamilySignals
	case flow.KindPlaySound:
		return FamilySounds
	case flow.KindCall, flow.KindReturn:
		return FamilyCalls
	default:
		return FamilyValues
	}
}

// UsedBy says what runs or calls the workflow, as "Clerk", "3 NPCs",
// "Sample zone" or "2 workflows" joined by " · ", or "" before the
// orchestrator has said or when nothing does.
func (c *Check) UsedBy() string {
	state := c.state
	var npcs, zones []string
	for _, use := range state.runs.uses {
		if use.Label == protocol.RunsByCall {
			continue
		}
		if use.Label == protocol.RunsAsLoop {
			zones = append(zones, use.Name)
		} else if !slices.Contains(npcs, use.Name) {
			npcs = append(npcs, use.Name)
		}
	}
	var parts []string
	for _, group := range []struct {
		names []string
		many  string
	}{{npcs, "NPCs"}, {zones, "zones"}, {state.callers.names, "workflows"}} {
		switch {
		case len(group.names) == 1:
			parts = append(parts, group.names[0])
		case len(group.names) > 1:
			parts = append(parts, strconv.Itoa(len(group.names))+" "+group.many)
		}
	}

	return strings.Join(parts, " · ")
}
