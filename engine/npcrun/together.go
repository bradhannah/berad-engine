package npcrun

import (
	"fmt"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// lane is one lane of the Together an NPC is on: its output and name, the
// node it is on and that node's work, and whether it has run out.
type lane struct {
	port, label string
	node        ids.ULID
	work        work
	done        bool
}

// Lane is what one lane of a Together is doing, as Watch shows it: its
// output and name, the node it is on, in a few words what it is doing, and
// whether it is done.
type Lane struct {
	Port, Label string
	Node        ids.ULID
	Doing       string
	Done        bool
}

// LaneDone is what a lane that has run out is doing.
const LaneDone = "Done"

// together starts a Together's lanes when the NPC comes to it and runs each
// for this tick, in lane order, so a run repeats. It reports whether every
// lane is done.
func (n *NPC) together(world World, node flow.Node, tick uint64) bool {
	if n.lanes == nil {
		n.lanes = startLanes(node)
	}
	for index := range n.lanes {
		n.runLane(world, index, tick)
	}

	return !slices.ContainsFunc(n.lanes, func(each lane) bool { return !each.done })
}

// startLanes sets each of a Together's lanes on its first step; a lane with
// none is done at once.
func startLanes(node flow.Node) []lane {
	declared, err := node.Lanes()
	if err != nil {
		return []lane{}
	}
	lanes := make([]lane, 0, len(declared))
	for _, each := range declared {
		first, linked := node.Next[each.Port]
		lanes = append(lanes, lane{port: each.Port, label: each.Label, node: first, work: idle(), done: !linked})
	}

	return lanes
}

// runLane follows one lane for this tick as follow does the NPC's flow:
// nodes that take no time follow one another up to MaxInstant, and a node
// that moves the NPC waits while it is between cells. The lane is done
// where its line runs out, or at a node a lane can't hold.
func (n *NPC) runLane(world World, index int, tick uint64) {
	running := &n.lanes[index]
	if running.done {
		return
	}
	flowWork := n.work
	n.work = running.work
	defer func() { running.work, n.work = n.work, flowWork }()
	for range MaxInstant {
		node, ok := world.Workflow.Node(running.node)
		if !ok || !flow.InLane(node.Kind) {
			running.done = true

			return
		}
		if flow.Moves(node.Kind) && n.Moving() {
			return
		}
		output, done := n.act(world, node)
		if !done {
			return
		}
		n.laneTake(world.Workflow, running, node, output, tick)
		if running.done {
			return
		}
	}
	n.Status, n.Sounds, n.Lines = StatusLooping, n.Sounds[:0], n.Lines[:0]
}

// laneTake moves a lane on by an output, recording the change with the
// lane's output; with nothing connected the lane is done.
func (n *NPC) laneTake(workflow *flow.Workflow, running *lane, from flow.Node, output string, tick uint64) {
	next, linked := from.Next[output]
	if _, exists := workflow.Node(next); !linked || !exists {
		running.done = true

		return
	}
	n.Last = Change{From: from.ID, Output: output, To: next, Tick: tick, Status: n.work.skipped, Lane: running.port}
	n.Taken = append(n.Taken, n.Last)
	n.Changes++
	running.node, n.work = next, idle()
}

// togetherActivity says how many of a Together's lanes are still running.
func (n *NPC) togetherActivity() string {
	if len(n.lanes) == 0 {
		return "Together"
	}
	running := 0
	for _, each := range n.lanes {
		if !each.done {
			running++
		}
	}

	return fmt.Sprintf("Together, %d of %d lanes running", running, len(n.lanes))
}

// Lanes says what each lane of the Together the NPC is on is doing, in lane
// order; none when it is on no Together.
func (n *NPC) Lanes(workflow *flow.Workflow) []Lane {
	shown := make([]Lane, 0, len(n.lanes))
	for _, each := range n.lanes {
		doing := LaneDone
		if !each.done {
			doing = "Starting"
			if node, ok := nodeOf(workflow, each.node); ok {
				doing = describe(node, each.work)
			}
		}
		shown = append(shown, Lane{Port: each.port, Label: each.label, Node: each.node, Doing: doing, Done: each.done})
	}

	return shown
}

// nodeOf finds a node in a workflow that may be nil.
func nodeOf(workflow *flow.Workflow, node ids.ULID) (flow.Node, bool) {
	if workflow == nil {
		var none flow.Node

		return none, false
	}

	return workflow.Node(node)
}
