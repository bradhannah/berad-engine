package npcrun

import (
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Calls. A Call workflow runs the module it names: the NPC goes to the
// module's start with fresh locals, its inputs read once from what the Call
// gives them, and when it reaches a Return, or a branch of the module's top
// level ends with nothing connected, it goes back to the Call, hands the
// module's outputs to the values the Call keeps them in and carries on from
// the Call's output named for the way out, or done. A Call that does not
// fit its module is skipped, by done.

// StatusCallSkipped says a Call was skipped because it does not fit its
// module.
const StatusCallSkipped = "Its call does not fit the workflow it calls, so it carried on by done"

// Caller is a workflow the NPC is in a call from: the workflow, zero for its
// behaviour, and the Call it called from.
type Caller struct {
	Workflow ids.ULID
	Call     ids.ULID
}

// caller is a call the NPC will go back to: where it was and its locals.
type caller struct {
	Caller

	container ids.ULID
	locals    map[ids.ULID]worldstate.Value
}

// Callers returns the calls the NPC is inside, outermost first: its
// behaviour's Call first. It is in none while its behaviour runs.
func (n *NPC) Callers() []Caller {
	callers := make([]Caller, 0, len(n.calls))
	for _, call := range n.calls {
		callers = append(callers, call.Caller)
	}

	return callers
}

// Module returns the workflow the NPC is in a call to, zero while its
// behaviour runs.
func (n *NPC) Module() ids.ULID { return n.module }

// Running returns the workflow the NPC is following now: its behaviour, or
// the module it is in a call to, if the world has it.
func (n *NPC) Running(world World) *flow.Workflow {
	if len(n.calls) == 0 {
		return world.Workflow
	}
	if world.Workflows == nil {
		return nil
	}
	module, ok := world.Workflows(n.module)
	if !ok {
		return nil
	}

	return module
}

// here is the world as the workflow the NPC is following sees it: its own
// locals and nodes.
func (n *NPC) here(world World) World {
	world.Workflow = n.Running(world)

	return world
}

// call goes into the module a Call names, or reports that it cannot: the
// Call names none, the world lacks it, it would come to call this workflow
// again or go too deep, or the Call does not fit it.
func (n *NPC) call(world World, node flow.Node, tick uint64) bool {
	target, ok := node.Called()
	if !ok || world.Workflows == nil || len(n.calls) >= flow.MaxCallDepth {
		return false
	}
	module, found := world.Workflows(target)
	self := world.Behaviour
	if len(n.calls) > 0 {
		self = n.module
	}
	if !found || flow.CallsBack(self, target, world.Workflows) != nil || len(flow.MissingWays(node, module)) > 0 ||
		len(flow.UnfitInputs(node, module)) > 0 {
		return false
	}
	locals, fits := n.inputs(world, node, module)
	if !fits {
		return false
	}
	n.calls = append(n.calls, caller{
		Caller:    Caller{Workflow: n.module, Call: node.ID},
		container: n.Container, locals: n.locals,
	})
	n.module, n.locals = target, locals
	n.enter(module, module.Start(), "", tick)

	return true
}

// inputs reads what a Call gives its module's inputs, once, as the call
// starts: another value is read now, and must be of the input's type.
func (n *NPC) inputs(world World, node flow.Node, module *flow.Workflow) (map[ids.ULID]worldstate.Value, bool) {
	given := node.Inputs()
	locals := map[ids.ULID]worldstate.Value{}
	for _, input := range module.Inputs() {
		raw, filled := given[input.ID]
		if !filled {
			continue
		}
		if ref, refers := flow.OperandRef(raw); refers {
			declared, value, readable := n.read(world, ref)
			if !readable || declared.Type != input.Type || !input.Fits(value) {
				return nil, false
			}
			locals[input.ID] = value

			continue
		}
		value, err := input.Parse(raw)
		if err != nil || !input.Fits(value) {
			return nil, false
		}
		locals[input.ID] = value
	}

	return locals, true
}

// giveBack goes back from a module to the Call that called it, by a way
// out, handing back its outputs, and carries on from the Call's output of
// that way out's port.
func (n *NPC) giveBack(world World, port string, tick uint64) {
	module := n.Running(world)
	outputs := map[ids.ULID]worldstate.Value{}
	if module != nil {
		for _, output := range module.Outputs() {
			outputs[output.ID] = n.local(output)
		}
	}
	back := n.calls[len(n.calls)-1]
	n.calls = n.calls[:len(n.calls)-1]
	n.module, n.locals, n.Node, n.Container = back.Workflow, back.locals, back.Call, back.container
	calling := n.here(world)
	if calling.Workflow == nil {
		n.Restart()

		return
	}
	if call, ok := calling.Workflow.Node(back.Call); ok {
		for output, keep := range call.Keeps() {
			if value, handed := outputs[output]; handed && !keep.IsZero() {
				n.write(calling, keep, value)
			}
		}
	}
	n.take(world, port, tick)
}

// actReturn goes back to the caller by a Return's way out; in a behaviour,
// which nothing called, it starts again.
func (n *NPC) actReturn(world World, node flow.Node, tick uint64) {
	way, err := node.WayOut()
	if len(n.calls) == 0 || err != nil {
		n.enter(world.Workflow, world.Workflow.Start(), "", tick)

		return
	}
	n.giveBack(world, way.Port, tick)
}

// level is one workflow the NPC is in, outermost first: the container it is
// in there, zero at its top level, and how deep in calls it is.
type level struct {
	workflow  *flow.Workflow
	container ids.ULID
	depth     int
}

// levels lists the workflows the NPC is in, its behaviour first.
func (n *NPC) levels(world World) []level {
	levels := make([]level, 0, len(n.calls)+1)
	for depth, call := range n.calls {
		running := world.Workflow
		if !call.Workflow.IsZero() && world.Workflows != nil {
			running, _ = world.Workflows(call.Workflow)
		}
		levels = append(levels, level{workflow: running, container: call.container, depth: depth})
	}

	return append(levels, level{workflow: n.Running(world), container: n.Container, depth: len(n.calls)})
}

// unwind leaves every call deeper than a depth at once, as when the time of
// a container they are inside is up, and puts the NPC on the Call it was
// in at that depth.
func (n *NPC) unwind(depth int) {
	if depth >= len(n.calls) {
		return
	}
	back := n.calls[depth]
	n.calls = n.calls[:depth]
	n.module, n.locals, n.Node, n.Container = back.Workflow, back.locals, back.Call, back.container
}
