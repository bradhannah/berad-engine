// Package npcrun runs NPCs through their behaviour workflows, a tick of the
// game loop at a time, so the editor's preview and a game behave alike and
// a run repeats exactly.
//
// An NPC follows its workflow from the start node. Nodes that take no time,
// the start, a time switch, a random choice, a container being entered or a
// Play sound, are followed in the same tick; a Wait, a Move and a Go to take
// time. A Play sound only asks for its cue: the host plays what an NPC's
// Sounds lists after each Step, so a run stays the same with or without
// sound; a Log likewise leaves its line in Lines for the host to show. An
// NPC looping without waiting plays and writes nothing. A
// branch with nothing connected goes back to the start, or, inside a
// time-bound container, back to the container's first step; when the
// container's time is up the NPC leaves by its exit at once, even partway
// through a node. When the clock jumps the NPC starts its workflow again from
// where it stands. NPCs walk one cell in play.StepTicks, the player's pace,
// whatever the clock's rate, and pass through each other and the player.
//
// A Together starts each of its lanes at once and runs them in lane order
// every tick, each following its own line of steps until it runs out; the
// NPC goes on by all done once every lane is done. A step that moves the
// NPC waits while it is between cells, so moves in two lanes never overlap.
package npcrun

import (
	"image"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/show"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
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
	StatusOtherFloor  = "Can't reach it: no stairs lead to its floor"
	StatusOtherZone   = "Can't reach it: the destination is in another zone"
	StatusNoCue       = "No sound chosen, so it carried on"
)

// Change is a move from one node to another: the output taken, the tick,
// when the node it left could not do its work and carried on, why, and the
// output of the Together's lane it was in, empty outside one. Why says what
// the node it left read or did that sent it this way, as "Read 9:15: 9:00
// to 12:00 holds", or why it started again.
type Change struct {
	From   ids.ULID
	Output string
	To     ids.ULID
	Tick   uint64
	Status string
	Lane   string
	Why    string
}

// World is what an NPC needs on a tick: the zone's floors as they are now,
// its workflow (nil for none) and that workflow's ID, the workflows its
// Calls can call, the clock, and the game seconds that passed; the values
// of the world, the zones and the NPCs, the zone it is in, and the zone's
// other NPCs as the tick began, in placement order.
type World struct {
	// Names names zones, workflows, sprites and places for values' text
	// forms; nil leaves them as IDs.
	Names     Names
	Zone      play.Zone
	Workflow  *flow.Workflow
	Behaviour ids.ULID
	Workflows flow.Lookup
	Clock     *clock.Clock
	Passed    float64
	State     *worldstate.Store
	Here      ids.ULID
	Others    []Other
	// Stage is the boxes and the shake every workflow of the game shares;
	// nil shows none.
	Stage *show.Stage
	// Signals are the signals the project declares, to name them in the log.
	Signals []worldstate.Signal
	// CueTicks says how many ticks a cue plays for, its longest recording,
	// for a Play sound that waits until it has played; nil, or none for a
	// cue, carries on at once. It comes from the audio library, never from
	// what is heard, so a run repeats whether or not sound is on.
	CueTicks func(cue string) int
}

// NPC is one NPC running in a zone, or a zone's loop, which runs as an NPC
// with no body would: it never moves, and its workflow has no node that
// moves it.
type NPC struct {
	// Placement, NPC and Floor say which placement this is, of which NPC,
	// and the floor it walks on; all are zero for a zone loop.
	Placement ids.ULID
	NPC       ids.ULID
	Floor     ids.ULID
	// Loop is the zone a zone loop runs for; zero for an NPC.
	Loop ids.ULID
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
	// Sounds are what the latest Step's Play sounds asked for, in order,
	// until the next Step.
	Sounds []Sound
	// Lines are what the latest Step's Logs wrote, oldest first, until the
	// next Step.
	Lines []Line
	// Sent are the signals the latest Step's Send signals sent, in order,
	// until the next Step; the host delivers them before the next tick.
	Sent []Sent
	// Status says what keeps the NPC from following its workflow now: it has
	// none, or it is looping without waiting.
	Status string

	work  work
	rng   *rand.Rand
	jumps int
	// locals are the local values the workflow it is following has set;
	// calls are the calls it is inside, outermost first, and module the
	// workflow it is in a call to, zero in its behaviour.
	locals map[ids.ULID]worldstate.Value
	calls  []caller
	module ids.ULID
	// speech is the lines it says, one after another, over its head.
	speech show.Speech
	// lanes are the lanes of the Together it is on, nil until they start.
	lanes []lane
	// heard are the signals waiting to land, and handling the one being
	// handled, nil for none.
	heard    []Heard
	handling *interrupt
}

// work is what the current node is doing: started once its node has set it
// up, and waiting during a Wait.
type work struct {
	started bool
	waiting bool
	// skipped says why the node is carrying on without doing its work, and
	// why what it read or did that sends it on.
	skipped string
	why     string
	// waitLeft and waitTotal are the seconds of a Wait, on its clock.
	waitLeft, waitTotal float64
	real                bool
	// movesLeft and movesTotal are the tiles of a Move, and way its
	// direction's name.
	movesLeft, movesTotal int
	direction             image.Point
	way                   string
	// route is the cells a Go to still walks, across floors, climbs the
	// stairs it still takes, goal the anchor's name and target its cell.
	route  []image.Point
	climbs []Climb
	goal   string
	target image.Point
	// awaited is the line or box a Say or a Show message waits for.
	awaited uint64
}

// idle is no work: a node not yet set up.
func idle() work {
	return work{
		started: false, waiting: false, skipped: "", why: "", waitLeft: 0, waitTotal: 0, real: false, movesLeft: 0,
		movesTotal: 0, direction: image.Pt(0, 0), way: "", route: nil, climbs: nil, goal: "", target: image.Pt(0, 0),
		awaited: 0,
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
		Placement: placement.ID, NPC: placement.NPC, Floor: floor, Loop: ids.ULID{}, Cell: cell, To: cell, Progress: 0,
		Facing: image.Pt(0, 1), Node: ids.ULID{}, Container: ids.ULID{},
		Last:    Change{From: ids.ULID{}, Output: "", To: ids.ULID{}, Tick: 0, Status: "", Lane: "", Why: ""},
		Changes: 0, Taken: nil, Sounds: nil, Lines: nil, Status: "", work: idle(), jumps: 0, locals: nil, calls: nil,
		module: ids.ULID{}, speech: show.Speech{}, lanes: nil, Sent: nil, heard: nil, handling: nil,
		//nolint:gosec // Random choices need a seed that repeats a run, not secrecy.
		rng: rand.New(rand.NewPCG(seed, seed^seedMix)),
	}
}

// StartLoop begins a zone's loop: a runner with no placement, no NPC and
// no floor, which stays where it is. The seed fixes its random choices.
func StartLoop(zone ids.ULID, seed uint64) *NPC {
	loop := Start(tilefloor.Placement{ID: ids.ULID{}, NPC: ids.ULID{}, Col: 0, Row: 0}, ids.ULID{}, seed)
	loop.Loop = zone

	return loop
}

// IsLoop reports whether the runner is a zone's loop rather than an NPC.
func (n *NPC) IsLoop() bool { return !n.Loop.IsZero() }

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
// as after a clock jump or an edit to its workflow, out of any calls. Its
// local values clear. Why says what started it again; the change to the
// start says that and what was cleared.
func (n *NPC) Restart(why string) {
	cleared := n.clearing()
	n.Node, n.Container, n.work, n.Status, n.locals = ids.ULID{}, ids.ULID{}, idle(), "", nil
	n.calls, n.module, n.lanes, n.heard, n.handling = nil, ids.ULID{}, nil, nil, nil
	n.work.why = why
	if cleared != "" {
		n.work.why += ", clearing " + cleared
	}
}

// clearing says what a restart clears: what the NPC was part way through,
// the calls it was in and its local values; "" for none of them.
func (n *NPC) clearing() string {
	var parts []string
	if n.work.started {
		parts = append(parts, "what it was doing")
	}
	if len(n.calls) > 0 {
		parts = append(parts, "the calls it was in")
	}
	if len(n.locals) > 0 {
		parts = append(parts, "its local values")
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}

	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// Step runs the NPC for one tick: it finishes or continues a step between
// cells, leaves a container whose time is up, and follows its workflow.
func (n *NPC) Step(world World, tick uint64) {
	n.Taken, n.Sounds, n.Lines, n.Sent = n.Taken[:0], n.Sounds[:0], n.Lines[:0], n.Sent[:0]
	n.speech.Tick()
	if world.Workflow == nil {
		n.Node, n.Container, n.Status = ids.ULID{}, ids.ULID{}, StatusNoBehaviour
		n.walk(world.Zone)

		return
	}
	if world.Clock.Jumps() != n.jumps {
		n.jumps = world.Clock.Jumps()
		n.Restart("Started again: the clock moved to " + ClockText(world.Clock.MinuteOfDay()))
	}
	if n.Running(world) == nil {
		n.Restart("Started again: the workflow it was in is gone")
	}
	if running := n.Running(world); !hasNode(running, n.Node) {
		n.enter(running, running.Start(), "", tick)
	}
	n.timeUp(world, tick)
	n.wait(world)
	n.walk(world.Zone)
	if !n.Moving() {
		n.land(world, tick)
	}
	if n.Moving() && n.lanes == nil {
		return
	}
	n.follow(world, tick)
}

// walk advances a step between cells. A step onto stairs takes them, to
// the same cell of the floor above or below, as the player's does.
func (n *NPC) walk(zone play.Zone) {
	if !n.Moving() {
		return
	}
	n.Progress++
	if n.Progress >= play.StepTicks {
		n.Cell, n.Progress = n.To, 0
		n.climb(zone)
	}
}

// climb takes the stairs on the cell the NPC has just stepped onto, if any
// lead somewhere it can stand.
func (n *NPC) climb(zone play.Zone) {
	floors := zone.Floors()
	target := climbAt(zone, floors, n.Floor, n.Cell)
	if target < 0 {
		return
	}
	n.Floor = floors[target]
	if len(n.work.climbs) > 0 && n.work.climbs[0].At == n.Cell {
		n.work.climbs = n.work.climbs[1:]
	}
}

// hasNode reports whether a workflow has a node.
func hasNode(workflow *flow.Workflow, node ids.ULID) bool {
	_, ok := workflow.Node(node)

	return ok
}

// timeUp leaves the container the NPC is in by its exit, at once, when its
// time is up, and every call inside it with it: the outermost container
// whose time is up is left.
func (n *NPC) timeUp(world World, tick uint64) {
	for _, inside := range n.levels(world) {
		if inside.container.IsZero() || inside.workflow == nil {
			continue
		}
		container, ok := inside.workflow.Node(inside.container)
		bound, err := container.TimeRange("bound")
		if !ok || err != nil || bound.Contains(world.Clock.MinuteOfDay()) {
			continue
		}
		n.unwind(inside.depth)
		n.Node, n.work = inside.container, idle()
		n.work.why = "Its time, " + RangeText(bound) + ", was up at " + ClockText(world.Clock.MinuteOfDay())
		n.take(world, flow.OutputExit, tick)

		return
	}
}

// wait counts down a Wait in progress, the NPC's and each lane's.
func (n *NPC) wait(world World) {
	n.work.count(world)
	for index := range n.lanes {
		n.lanes[index].work.count(world)
	}
}

// count counts down a Wait in progress: game seconds on the game clock, or a
// tick's worth of real time, which ignores the clock's rate and stop.
func (w *work) count(world World) {
	if !w.waiting {
		return
	}
	if w.real {
		w.waitLeft -= 1.0 / clock.TicksPerSecond
	} else {
		w.waitLeft -= world.Passed
	}
}

// follow does the current node's work, following nodes that take no time
// until one takes time or MaxInstant is reached. A loop that never waits
// would ask for its sounds and write its lines every tick, so it plays and
// writes none.
func (n *NPC) follow(world World, tick uint64) {
	n.Status = ""
	for range MaxInstant {
		here := n.here(world)
		if here.Workflow == nil {
			n.Restart("Started again: the workflow it was in is gone")
			here = world
		}
		node, ok := here.Workflow.Node(n.Node)
		switch {
		case !ok:
			n.enter(here.Workflow, here.Workflow.Start(), "", tick)
		case node.Kind == flow.KindCall:
			if !n.call(world, node, tick) {
				n.work.skipped = StatusCallSkipped
				n.take(world, flow.OutputDone, tick)
			}
		case node.Kind == flow.KindReturn:
			n.actReturn(world, node, tick)
		case node.Kind == flow.KindResume:
			n.actResume(world, node, tick)
		case node.Kind == flow.KindTogether:
			if !n.together(here, node, tick) || n.Moving() {
				return
			}
			n.take(world, flow.OutputDone, tick)
		default:
			output, done := n.act(here, node)
			if !done {
				return
			}
			n.take(world, output, tick)
		}
	}
	n.Status, n.Sounds, n.Lines, n.Sent = StatusLooping, n.Sounds[:0], n.Lines[:0], n.Sent[:0]
}

// act does a node's work for this tick. It returns the output to leave by
// when the node is done, or false while it still has work.
func (n *NPC) act(world World, node flow.Node) (string, bool) {
	switch node.Kind {
	case flow.KindTimeSwitch:
		output, why := timeSwitch(node, world.Clock.MinuteOfDay())
		n.work.why = why

		return output, true
	case flow.KindRandom:
		return n.choose(world, node), true
	case flow.KindContainer:
		return n.enterContainer(node, world.Clock.MinuteOfDay()), true
	case flow.KindWait:
		return n.actWait(world, node)
	case flow.KindMove:
		return n.actMove(world, node)
	case flow.KindGoTo:
		return n.actGoTo(world, node)
	case flow.KindPlaySound:
		return n.actPlaySound(world, node)
	case flow.KindSetValue:
		n.actSet(world, node)

		return flow.OutputNext, true
	case flow.KindLog:
		n.actLog(world, node)

		return flow.OutputNext, true
	case flow.KindSendSignal:
		n.actSend(world, node)

		return flow.OutputNext, true
	case flow.KindSay, flow.KindShowMessage, flow.KindShake:
		return n.actShow(world, node)
	case flow.KindCheckValue:
		return n.actCheck(world, node), true
	case flow.KindFindNPC:
		return n.actFind(world, node), true
	default:
		return flow.OutputNext, true
	}
}

// Sound is what one Play sound asks for: its cue, and the step of package
// loudness to play it at this time, empty to play it at the cue's own.
type Sound struct {
	Cue      string
	Loudness string
}

// actPlaySound asks for a Play sound's cue and carries on at once, or once
// the cue has played for its ticks; with no cue chosen it carries on and
// says why.
func (n *NPC) actPlaySound(world World, node flow.Node) (string, bool) {
	if n.work.started {
		return flow.OutputNext, n.work.waitLeft <= 0
	}
	cue, err := node.Cue("cue")
	if err != nil || cue == "" {
		n.work.skipped = StatusNoCue

		return flow.OutputNext, true
	}
	asked := Sound{Cue: cue, Loudness: ""}
	if step, err := node.Choice("loudness"); err == nil && step != flow.LoudnessAsCue {
		asked.Loudness = step
	}
	n.Sounds = append(n.Sounds, asked)
	carry, _ := node.Choice(flow.ParamCarry) //nolint:errcheck // No setting carries on at once.
	if carry != flow.CarryWhenDone || world.CueTicks == nil || world.CueTicks(cue) <= 0 {
		return flow.OutputNext, true
	}
	n.work = waitWork(float64(world.CueTicks(cue))/clock.TicksPerSecond, true)

	return flow.OutputNext, false
}

// timeSwitch takes the output of the first time that holds the minute, or
// "otherwise", and says what it read.
func timeSwitch(node flow.Node, minute int) (string, string) {
	read := "Read " + ClockText(minute) + ": "
	ranges, err := node.TimeRanges("ranges")
	if err != nil {
		return flow.OutputOtherwise, read + "it has no times"
	}
	for _, span := range ranges {
		if span.Contains(minute) {
			return span.Port, read + RangeText(span) + " holds"
		}
	}

	return flow.OutputOtherwise, read + "no time holds"
}

// choose picks one of a random choice's options, each its share of the
// time: its share typed, or the constant standing in for it.
func (n *NPC) choose(world World, node flow.Node) string {
	options, err := node.Shares("options")
	if err != nil || len(options) == 0 {
		return ""
	}
	total := 0
	low, high := flow.Limits(flow.TypeShares)
	for index, option := range options {
		options[index].Share = n.numberOf(world, flow.Amount{Number: option.Share, Constant: option.Constant}, low, high)
		total += options[index].Share
	}
	if total <= 0 {
		return options[0].Port
	}
	roll := n.rng.IntN(total)
	n.work.why = "Rolled " + strconv.Itoa(roll+1) + " of " + strconv.Itoa(total)
	for _, option := range options {
		if roll < option.Share {
			return option.Port
		}
		roll -= option.Share
	}

	return options[len(options)-1].Port
}

// enterContainer goes inside a container while its time holds, or straight
// on by its exit, and says what it read.
func (n *NPC) enterContainer(node flow.Node, minute int) string {
	bound, err := node.TimeRange("bound")
	if err != nil {
		return flow.OutputExit
	}
	read := "Read " + ClockText(minute) + ": "
	if !bound.Contains(minute) {
		n.work.why = read + "outside " + RangeText(bound)

		return flow.OutputExit
	}
	if _, has := node.Next[flow.OutputInside]; !has {
		n.work.why = read + "within " + RangeText(bound) + ", but nothing is inside"

		return flow.OutputExit
	}
	n.work.why = read + "within " + RangeText(bound)

	return flow.OutputInside
}

// actWait starts or finishes a Wait, for its amount typed or the constant
// standing in for it.
func (n *NPC) actWait(world World, node flow.Node) (string, bool) {
	if !n.work.started {
		duration, err := node.Duration("duration")
		if err != nil {
			return flow.OutputNext, true
		}
		low, high := flow.WaitLimits(duration)
		duration.Amount = n.numberOf(world, flow.Amount{Number: duration.Amount, Constant: duration.Constant}, low, high)
		seconds := float64(duration.Seconds())
		n.work = waitWork(seconds, duration.Clock == flow.ClockReal)
	}
	if n.work.waitLeft > 0 {
		return "", false
	}
	n.work.why = "Waited " + secondsText(n.work.waitTotal)
	if n.work.real {
		n.work.why += " of real time"
	}

	return flow.OutputNext, true
}

// actMove takes the next tile of a Move, or carries on when it is done or
// something is in the way.
func (n *NPC) actMove(world World, node flow.Node) (string, bool) {
	if !n.work.started {
		direction, dirErr := node.Choice("direction")
		tiles, tilesErr := node.Amount("tiles")
		if dirErr != nil || tilesErr != nil {
			return flow.OutputNext, true
		}
		low, high := flow.Limits(flow.TypeTiles)
		n.work = moveWork(n.numberOf(world, tiles, low, high), direction)
	}
	if n.work.movesLeft <= 0 {
		n.work.why = "Moved " + tilesText(n.work.movesTotal) + " " + n.work.way

		return flow.OutputNext, true
	}
	if !n.step(world.Zone, n.work.direction) {
		in := n.Cell.Add(n.work.direction)
		n.work.skipped = StatusBlocked
		n.work.why = "Something was in its way at " + strconv.Itoa(in.X) + ", " + strconv.Itoa(in.Y)

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
// its first step. In a module, a branch of the top level that ends goes back
// to the Call by done.
func (n *NPC) take(world World, output string, tick uint64) {
	workflow := n.Running(world)
	from, _ := workflow.Node(n.Node)
	next, linked := from.Next[output]
	if _, exists := workflow.Node(next); linked && exists {
		n.enter(workflow, next, output, tick)

		return
	}
	next, top := n.restartOf(workflow, from, output)
	if top && len(n.calls) > 0 {
		n.giveBack(world, flow.OutputDone, tick)

		return
	}
	n.enter(workflow, next, output, tick)
}

// restartOf is where a branch with nothing connected goes: inside a
// container, its first step, or out by its exit when it has none; at the top
// level, the start, which top reports.
func (n *NPC) restartOf(workflow *flow.Workflow, from flow.Node, output string) (ids.ULID, bool) {
	if from.Kind == flow.KindContainer && output == flow.OutputExit || from.Parent.IsZero() {
		return workflow.Start(), true
	}
	container, ok := workflow.Node(from.Parent)
	if first, has := container.Next[flow.OutputInside]; ok && has {
		return first, false
	}

	return workflow.Start(), true
}

// enter moves the NPC onto a node, recording the change.
func (n *NPC) enter(workflow *flow.Workflow, target ids.ULID, output string, tick uint64) {
	n.Last = Change{
		From: n.Node, Output: output, To: target, Tick: tick, Status: n.work.skipped, Lane: "", Why: n.work.why,
	}
	n.Taken = append(n.Taken, n.Last)
	n.Changes++
	n.Node, n.work, n.lanes = target, idle(), nil
	node, _ := workflow.Node(target)
	n.Container = node.Parent
	n.takenOver(target)
}
