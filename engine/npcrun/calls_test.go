package npcrun_test

import (
	"encoding/json"
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

const (
	behaviourID = 150
	moduleID    = 151
)

// caller runs an NPC whose behaviour may call a module.
type caller struct {
	*runner

	flows map[ids.ULID]*flow.Workflow
}

func newCaller(t *testing.T, behaviour, module *flow.Workflow, start float64) *caller {
	t.Helper()
	made := &caller{
		runner: newRunner(t, behaviour, image.Pt(2, 1), start, clock.Real),
		flows:  map[ids.ULID]*flow.Workflow{id(behaviourID): behaviour, id(moduleID): module},
	}

	return made
}

func (c *caller) run(ticks int) {
	lookup := func(target ids.ULID) (*flow.Workflow, bool) {
		found, ok := c.flows[target]

		return found, ok
	}
	for range ticks {
		c.tick++
		passed := c.clock.Advance()
		c.npc.Step(npcrun.World{
			Zone: c.zone, Workflow: c.workflow, Behaviour: id(behaviourID), Workflows: lookup, Clock: c.clock,
			Passed: passed,
		}, c.tick)
	}
}

// local declares a local with a role and a start.
func local(t *testing.T, workflow *flow.Workflow, seed byte, kind worldstate.Type, role worldstate.Role,
	start *worldstate.Value,
) {
	t.Helper()
	declared := worldstate.New(id(seed), "v"+string(rune('a'+seed%26)), worldstate.ScopeLocal, kind)
	declared.Role = role
	if start != nil {
		declared.Default = start.Encode()
	}
	if err := workflow.Locals().Insert(declared); err != nil {
		t.Fatal(err)
	}
}

// returnTo is a Return of a way out.
func returnTo(t *testing.T, seed byte, label, port string) flow.Node {
	t.Helper()

	return node(t, seed, flow.KindReturn, map[string]any{flow.ParamWayName: label, flow.ParamWayPort: port})
}

// callOf is a Call of the module, with ways out, inputs and keeps.
func callOf(t *testing.T, seed byte, ways []flow.WayOut, inputs map[ids.ULID]json.RawMessage,
	keeps map[ids.ULID]flow.ValueRef,
) flow.Node {
	t.Helper()

	return node(t, seed, flow.KindCall, map[string]any{
		flow.ParamWorkflow: id(moduleID).String(), flow.ParamWays: ways, flow.ParamInputs: inputs, flow.ParamKeep: keeps,
	})
}

// goThere is a module that goes to its place input, counts a try into its
// output and returns by Arrived, or, with no place, by Lost.
func goThere(t *testing.T) *flow.Workflow {
	t.Helper()
	module := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindSetValue, map[string]any{
			flow.ParamValue: ref(worldstate.ScopeLocal, 21), "how": flow.HowAdd, "to": json.RawMessage("1"),
		}),
		node(t, 3, flow.KindGoTo, map[string]any{flow.ParamGoes: flow.GoesToValue, flow.ParamPlace: ref(worldstate.ScopeLocal, 20)}),
		returnTo(t, 4, "Arrived", "w1"),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputNext, 4))
	local(t, module, 20, worldstate.TypePlace, worldstate.RoleInput, nil)
	local(t, module, 21, worldstate.TypeInteger, worldstate.RoleOutput, nil)

	return module
}

// deskPlace is the desk, as a place given to an input.
func deskPlace() json.RawMessage {
	value := worldstate.NoValue()
	value.Type, value.Place = worldstate.TypePlace, worldstate.Place{Floor: id(groundID), Anchor: id(deskID)}

	return value.Encode()
}

// TestCallingAModule calls a module with a place, which it goes to, and
// carries on from the way out it returns by, keeping its output; it calls
// again with its locals fresh, so the output counts one try each time.
func TestCallingAModule(t *testing.T) {
	t.Parallel()
	ways := []flow.WayOut{{Port: "w1", Label: "Arrived"}}
	behaviour := workflowOf(t, []flow.Node{
		callOf(t, 2, ways, map[ids.ULID]json.RawMessage{id(20): deskPlace()},
			map[ids.ULID]flow.ValueRef{id(21): ref(worldstate.ScopeLocal, 30)}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(10, flow.UnitSeconds, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, "w1", 3))
	local(t, behaviour, 30, worldstate.TypeInteger, worldstate.RoleLocal, nil)
	run := newCaller(t, behaviour, goThere(t), eight)
	run.run(1)
	if callers := run.npc.Callers(); len(callers) != 1 || callers[0].Call != id(2) || run.npc.Module() != id(moduleID) {
		t.Fatalf("inside the call: %+v %v", callers, run.npc.Module())
	}
	run.run(40 * play.StepTicks)
	if run.npc.Cell != image.Pt(8, 1) || run.npc.Node != id(3) || len(run.npc.Callers()) != 0 {
		t.Fatalf("went to the desk and came back by Arrived: %v on %v, %+v", run.npc.Cell, run.npc.Node, run.npc.Callers())
	}
	if tries := run.npc.Locals()[id(30)]; tries.Integer != 1 {
		t.Fatalf("its output, one try, is kept: %+v", tries)
	}
	run.run(12 * clock.TicksPerSecond)
	if tries := run.npc.Locals()[id(30)]; tries.Integer != 1 || run.npc.Node != id(3) {
		t.Fatalf("called again, its locals start fresh: %+v on %v", tries, run.npc.Node)
	}
}

// TestCallsThatDoNotRun skips a Call with no workflow, one that calls its
// own behaviour and one given a value that does not fit, each by done; a
// module whose branch ends goes back by done; a Return in a behaviour
// starts it again.
func TestCallsThatDoNotRun(t *testing.T) {
	t.Parallel()
	wait := node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)})
	for name, call := range map[string]flow.Node{
		"no workflow": node(t, 2, flow.KindCall, nil),
		"unfit": callOf(t, 2, nil, map[ids.ULID]json.RawMessage{id(20): json.RawMessage("3")},
			map[ids.ULID]flow.ValueRef{}),
	} {
		behaviour := workflowOf(t, []flow.Node{call, wait}, link(1, flow.OutputNext, 2), link(2, flow.OutputDone, 3))
		run := newCaller(t, behaviour, goThere(t), eight)
		run.run(1)
		if run.npc.Node != id(3) || run.npc.Last.Status != npcrun.StatusCallSkipped {
			t.Fatalf("%s: skipped by done: %v %q", name, run.npc.Node, run.npc.Last.Status)
		}
	}
	back := workflowOf(t, []flow.Node{node(t, 2, flow.KindCall, map[string]any{flow.ParamWorkflow: id(behaviourID).String()}),
		wait}, link(1, flow.OutputNext, 2), link(2, flow.OutputDone, 3))
	run := newCaller(t, back, goThere(t), eight)
	run.run(1)
	if run.npc.Node != id(3) || run.npc.Last.Status != npcrun.StatusCallSkipped {
		t.Fatalf("a call of its own behaviour is skipped: %v %q", run.npc.Node, run.npc.Last.Status)
	}
	ends := workflowOf(t, []flow.Node{node(t, 2, flow.KindMove, map[string]any{"direction": flow.DirectionDown, "tiles": 1})},
		link(1, flow.OutputNext, 2))
	behaviour := workflowOf(t, []flow.Node{callOf(t, 2, nil, nil, nil), wait},
		link(1, flow.OutputNext, 2), link(2, flow.OutputDone, 3))
	run = newCaller(t, behaviour, ends, eight)
	run.run(2 * play.StepTicks)
	if run.npc.Node != id(3) || len(run.npc.Callers()) != 0 {
		t.Fatalf("a module whose branch ends goes back by done: %v %+v", run.npc.Node, run.npc.Callers())
	}
	alone := workflowOf(t, []flow.Node{returnTo(t, 2, "Arrived", "w1")}, link(1, flow.OutputNext, 2))
	run = newCaller(t, alone, goThere(t), eight)
	run.run(1)
	if run.npc.Status != npcrun.StatusLooping {
		t.Fatalf("a Return in a behaviour starts it again: %q", run.npc.Status)
	}
}

// TestTimeUpLeavesCalls leaves the module it called, and the container
// the Call is in, at once when the container's time is up.
func TestTimeUpLeavesCalls(t *testing.T) {
	t.Parallel()
	shop := node(t, 2, flow.KindContainer, map[string]any{"bound": flow.TimeRange{Port: "", From: 8 * 60, To: 9 * 60}})
	call := callOf(t, 3, nil, nil, nil)
	call.Parent = shop.ID
	after := node(t, 4, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)})
	behaviour := workflowOf(t, []flow.Node{shop, call, after},
		link(1, flow.OutputNext, 2), link(2, flow.OutputInside, 3), link(2, flow.OutputExit, 4))
	long := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindWait, map[string]any{"duration": waitFor(2, flow.UnitHours, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2))
	run := newCaller(t, behaviour, long, 8*3600+59*60+50)
	run.run(1)
	if len(run.npc.Callers()) != 1 || run.npc.Node != id(2) {
		t.Fatalf("waiting inside the call: %+v on %v", run.npc.Callers(), run.npc.Node)
	}
	run.run(15 * clock.TicksPerSecond)
	if len(run.npc.Callers()) != 0 || run.npc.Node != id(4) || run.npc.Container != (ids.ULID{}) {
		t.Fatalf("time up left the call and the container: %+v on %v in %v", run.npc.Callers(), run.npc.Node,
			run.npc.Container)
	}
}
