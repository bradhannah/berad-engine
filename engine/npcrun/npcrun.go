// Package npcrun runs NPCs through their behaviour workflows, a tick of the
// game loop at a time, so the editor's preview and a game behave alike and
// a run repeats exactly.
//
// An NPC follows its workflow from the start node. Nodes that take no time,
// the start, a time switch, a random choice, a container being entered or a
// Play sound, are followed in the same tick; a Wait, a Move and a Go to take
// time. A Play sound only asks for its cue: the host plays what an NPC's
// Sounds lists after each Step, so a run stays the same with or without
// sound. An NPC looping without waiting plays nothing. A
// branch with nothing connected goes back to the start, or, inside a
// time-bound container, back to the container's first step; when the
// container's time is up the NPC leaves by its exit at once, even partway
// through a node. When the clock jumps the NPC starts its workflow again from
// where it stands. NPCs walk one cell in play.StepTicks, as the player does,
// whatever the clock's rate, and pass through each other and the player.
package npcrun

import (
	"image"
	"math/rand/v2"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// MaxInstant bounds how many nodes that take no time an NPC follows in one
// tick; past it the NPC is looping without waiting, and stops until the next
// tick.
const MaxInstant = 64

// Statuses: what keeps an NPC from following its workflow (no behaviour,
// looping), and why a node could not do its work and carried on (the rest),
// which goes with the change of node it carried on by.
const (
	StatusNoBehaviour = "No behaviour"
	StatusLooping     = "Looping without waiting"
	StatusBlocked     = "Blocked, so it carried on"
	StatusNoRoute     = "No way there, so it carried on"
	StatusNoAnchor    = "No destination, so it carried on"
	StatusOtherFloor  = "Can't reach it: the destination is on another floor"
	StatusNoCue       = "No sound chosen, so it carried on"
)

// Change is a move from one node to another: the output taken, the tick,
// and, when the node it left could not do its work and carried on, why.
type Change struct {
	From   ids.ULID
	Output string
	To     ids.ULID
	Tick   uint64
	Status string
}

// World is what an NPC needs on a tick: the zone's floors as they are now,
// its workflow (nil for none), the clock, and the game seconds that passed.
type World struct {
	Zone     play.Zone
	Workflow *flow.Workflow
	Clock    *clock.Clock
	Passed   float64
}

// NPC is one NPC running in a zone.
type NPC struct {
	// Placement, NPC and Floor say which placement this is, of which NPC,
	// and the floor it walks on.
	Placement ids.ULID
	NPC       ids.ULID
	Floor     ids.ULID
	// Cell is where it stands, or the cell a step is leaving; To is where the
	// step goes, equal to Cell when it stands still.
	Cell, To image.Point
	Progress int
	Facing   image.Point
	// Node is the node it is on and Container the container it is in, zero
	// at the top level.
	Node      ids.ULID
	Container ids.ULID
	// Last is the latest change of node, and Changes counts them. Taken is
	// the changes the latest Step made, oldest first, until the next Step.
	Last    Change
	Changes uint64
	Taken   []Change
	// Sounds are the cues the latest Step's Play sounds asked for, in order,
	// until the next Step.
	Sounds []string
	// Status says what keeps the NPC from following its workflow now: it has
	// none, or it is looping without waiting.
	Status string

	work  work
	rng   *rand.Rand
	jumps int
}

// work is what the current node is doing: started once its node has set it
// up, and waiting during a Wait.
type work struct {
	started bool
	waiting bool
	// skipped says why the node is carrying on without doing its work.
	skipped string
	// waitLeft and waitTotal are the seconds of a Wait, on its clock.
	waitLeft, waitTotal float64
	real                bool
	// movesLeft and movesTotal are the tiles of a Move, and way its
	// direction's name.
	movesLeft, movesTotal int
	direction             image.Point
	way                   string
	// route is the cells a Go to still walks, goal the anchor's name and
	// target its cell.
	route  []image.Point
	goal   string
	target image.Point
}

// idle is no work: a node not yet set up.
func idle() work {
	return work{
		started: false, waiting: false, skipped: "", waitLeft: 0, waitTotal: 0, real: false, movesLeft: 0, movesTotal: 0,
		direction: image.Pt(0, 0), way: "", route: nil, goal: "", target: image.Pt(0, 0),
	}
}

// waitWork is a Wait of a number of seconds, on the game clock or a real one.
func waitWork(seconds float64, onRealClock bool) work {
	made := idle()
	made.started, made.waiting, made.waitLeft, made.waitTotal, made.real = true, true, seconds, seconds, onRealClock

	return made
}

// moveWork is a Move of a number of tiles one way.
func moveWork(tiles int, way string) work {
	made := idle()
	made.started, made.movesLeft, made.movesTotal, made.direction, made.way = true, tiles, tiles, deltaOf(way), way

	return made
}

// routeWork is a Go to walking a route to an anchor.
func routeWork(route []image.Point, goal string, target image.Point) work {
	made := idle()
	made.started, made.route, made.goal, made.target = true, route, goal, target

	return made
}

// Start begins an NPC on its placement. The seed fixes its random choices,
// so a run repeats.
func Start(placement tilefloor.Placement, floor ids.ULID, seed uint64) *NPC {
	cell := image.Pt(placement.Col, placement.Row)

	return &NPC{
		Placement: placement.ID, NPC: placement.NPC, Floor: floor, Cell: cell, To: cell, Progress: 0,
		Facing: image.Pt(0, 1), Node: ids.ULID{}, Container: ids.ULID{},
		Last:    Change{From: ids.ULID{}, Output: "", To: ids.ULID{}, Tick: 0, Status: ""},
		Changes: 0, Taken: nil, Sounds: nil, Status: "", work: idle(), jumps: 0,
		//nolint:gosec // Random choices need a seed that repeats a run, not secrecy.
		rng: rand.New(rand.NewPCG(seed, seed^seedMix)),
	}
}

// seedMix spreads one seed over the generator's two words.
const seedMix = 0x9e3779b97f4a7c15

// Moving reports whether the NPC is between two cells.
func (n *NPC) Moving() bool { return n.To != n.Cell }

// At returns where the NPC is in cells, between Cell and To during a step.
func (n *NPC) At() (float64, float64) {
	share := float64(n.Progress) / play.StepTicks

	return float64(n.Cell.X) + float64(n.To.X-n.Cell.X)*share, float64(n.Cell.Y) + float64(n.To.Y-n.Cell.Y)*share
}

// Restart starts the NPC's workflow again from the start, where it stands,
// as after a clock jump or an edit to its workflow.
func (n *NPC) Restart() {
	n.Node, n.Container, n.work, n.Status = ids.ULID{}, ids.ULID{}, idle(), ""
}

// Step runs the NPC for one tick: it finishes or continues a step between
// cells, leaves a container whose time is up, and follows its workflow.
func (n *NPC) Step(world World, tick uint64) {
	n.Taken, n.Sounds = n.Taken[:0], n.Sounds[:0]
	if world.Workflow == nil {
		n.Node, n.Container, n.Status = ids.ULID{}, ids.ULID{}, StatusNoBehaviour
		n.walk()

		return
	}
	if world.Clock.Jumps() != n.jumps {
		n.jumps = world.Clock.Jumps()
		n.Restart()
	}
	if _, ok := world.Workflow.Node(n.Node); !ok {
		n.enter(world.Workflow, world.Workflow.Start(), "", tick)
	}
	n.timeUp(world, tick)
	n.wait(world)
	n.walk()
	if n.Moving() {
		return
	}
	n.follow(world, tick)
}

// walk advances a step between cells.
func (n *NPC) walk() {
	if !n.Moving() {
		return
	}
	n.Progress++
	if n.Progress >= play.StepTicks {
		n.Cell, n.Progress = n.To, 0
	}
}

// timeUp leaves the container the NPC is in by its exit, at once, when its
// time is up.
func (n *NPC) timeUp(world World, tick uint64) {
	if n.Container.IsZero() {
		return
	}
	container, ok := world.Workflow.Node(n.Container)
	bound, err := container.TimeRange("bound")
	if !ok || err != nil || bound.Contains(world.Clock.MinuteOfDay()) {
		return
	}
	n.Node = n.Container
	n.take(world.Workflow, flow.OutputExit, tick)
}

// wait counts down a Wait in progress: game seconds on the game clock, or a
// tick's worth of real time, which ignores the clock's rate and stop.
func (n *NPC) wait(world World) {
	if !n.work.waiting {
		return
	}
	if n.work.real {
		n.work.waitLeft -= 1.0 / clock.TicksPerSecond
	} else {
		n.work.waitLeft -= world.Passed
	}
}

// follow does the current node's work, following nodes that take no time
// until one takes time or MaxInstant is reached. A loop that never waits
// would ask for its sounds every tick, so it plays none.
func (n *NPC) follow(world World, tick uint64) {
	n.Status = ""
	for range MaxInstant {
		node, ok := world.Workflow.Node(n.Node)
		if !ok {
			n.enter(world.Workflow, world.Workflow.Start(), "", tick)

			continue
		}
		output, done := n.act(world, node)
		if !done {
			return
		}
		n.take(world.Workflow, output, tick)
	}
	n.Status, n.Sounds = StatusLooping, n.Sounds[:0]
}

// act does a node's work for this tick. It returns the output to leave by
// when the node is done, or false while it still has work.
func (n *NPC) act(world World, node flow.Node) (string, bool) {
	switch node.Kind {
	case flow.KindTimeSwitch:
		return timeSwitch(node, world.Clock.MinuteOfDay()), true
	case flow.KindRandom:
		return n.choose(node), true
	case flow.KindContainer:
		return n.enterContainer(node, world.Clock.MinuteOfDay()), true
	case flow.KindWait:
		return n.actWait(node)
	case flow.KindMove:
		return n.actMove(world, node)
	case flow.KindGoTo:
		return n.actGoTo(world, node)
	case flow.KindPlaySound:
		n.actPlaySound(node)

		return flow.OutputNext, true
	default:
		return flow.OutputNext, true
	}
}

// actPlaySound asks for a Play sound's cue and carries on at once; with no
// cue chosen it carries on and says why.
func (n *NPC) actPlaySound(node flow.Node) {
	cue, err := node.Cue("cue")
	if err != nil || cue == "" {
		n.work.skipped = StatusNoCue

		return
	}
	n.Sounds = append(n.Sounds, cue)
}

// timeSwitch takes the output of the first time that holds the minute, or
// "otherwise".
func timeSwitch(node flow.Node, minute int) string {
	ranges, err := node.TimeRanges("ranges")
	if err != nil {
		return flow.OutputOtherwise
	}
	for _, span := range ranges {
		if span.Contains(minute) {
			return span.Port
		}
	}

	return flow.OutputOtherwise
}

// choose picks one of a random choice's options, each its share of the time.
func (n *NPC) choose(node flow.Node) string {
	options, err := node.Shares("options")
	if err != nil || len(options) == 0 {
		return ""
	}
	total := 0
	for _, option := range options {
		total += option.Share
	}
	if total <= 0 {
		return options[0].Port
	}
	roll := n.rng.IntN(total)
	for _, option := range options {
		if roll < option.Share {
			return option.Port
		}
		roll -= option.Share
	}

	return options[len(options)-1].Port
}

// enterContainer goes inside a container while its time holds, or straight
// on by its exit.
func (n *NPC) enterContainer(node flow.Node, minute int) string {
	bound, err := node.TimeRange("bound")
	if err == nil && bound.Contains(minute) {
		if _, has := node.Next[flow.OutputInside]; has {
			return flow.OutputInside
		}
	}

	return flow.OutputExit
}

// actWait starts or finishes a Wait.
func (n *NPC) actWait(node flow.Node) (string, bool) {
	if !n.work.started {
		duration, err := node.Duration("duration")
		if err != nil {
			return flow.OutputNext, true
		}
		seconds := float64(duration.Seconds())
		n.work = waitWork(seconds, duration.Clock == flow.ClockReal)
	}
	if n.work.waitLeft > 0 {
		return "", false
	}

	return flow.OutputNext, true
}

// actMove takes the next tile of a Move, or carries on when it is done or
// something is in the way.
func (n *NPC) actMove(world World, node flow.Node) (string, bool) {
	if !n.work.started {
		direction, dirErr := node.Choice("direction")
		tiles, tilesErr := node.Number("tiles")
		if dirErr != nil || tilesErr != nil {
			return flow.OutputNext, true
		}
		n.work = moveWork(tiles, direction)
	}
	if n.work.movesLeft <= 0 {
		return flow.OutputNext, true
	}
	if !n.step(world.Zone, n.work.direction) {
		n.work.skipped = StatusBlocked

		return flow.OutputNext, true
	}
	n.work.movesLeft--

	return "", false
}

// deltaOf is a Move's direction as one cell.
func deltaOf(direction string) image.Point {
	switch direction {
	case flow.DirectionUp:
		return image.Pt(0, -1)
	case flow.DirectionDown:
		return image.Pt(0, 1)
	case flow.DirectionLeft:
		return image.Pt(-1, 0)
	default:
		return image.Pt(1, 0)
	}
}

// step starts a step of one cell when it can be taken, and reports whether
// it did.
func (n *NPC) step(zone play.Zone, delta image.Point) bool {
	n.Facing = delta
	can := false
	zone.Floor(n.Floor, func(floor *tilefloor.Floor) { can = play.CanStep(floor, n.Cell, delta) })
	if can {
		n.To = n.Cell.Add(delta)
	}

	return can
}

// take leaves the current node by an output: to the node it leads to, or,
// with nothing connected, back to the start, or inside a container back to
// its first step.
func (n *NPC) take(workflow *flow.Workflow, output string, tick uint64) {
	from, _ := workflow.Node(n.Node)
	next, linked := from.Next[output]
	if _, exists := workflow.Node(next); !linked || !exists {
		next = n.restartOf(workflow, from, output)
	}
	n.enter(workflow, next, output, tick)
}

// restartOf is where a branch with nothing connected goes: inside a
// container, its first step, or out by its exit when it has none; at the top
// level, the start.
func (n *NPC) restartOf(workflow *flow.Workflow, from flow.Node, output string) ids.ULID {
	if from.Kind == flow.KindContainer && output == flow.OutputExit || from.Parent.IsZero() {
		return workflow.Start()
	}
	container, ok := workflow.Node(from.Parent)
	if first, has := container.Next[flow.OutputInside]; ok && has {
		return first
	}

	return workflow.Start()
}

// enter moves the NPC onto a node, recording the change.
func (n *NPC) enter(workflow *flow.Workflow, to ids.ULID, output string, tick uint64) {
	n.Last = Change{From: n.Node, Output: output, To: to, Tick: tick, Status: n.work.skipped}
	n.Taken = append(n.Taken, n.Last)
	n.Changes++
	n.Node, n.work = to, idle()
	node, _ := workflow.Node(to)
	n.Container = node.Parent
}
