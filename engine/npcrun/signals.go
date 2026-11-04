package npcrun

import (
	"slices"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Statuses a Send signal carries on with when it sends nothing.
const (
	StatusNoSignal    = "No signal chosen, so it carried on"
	StatusNoAddressee = "No NPC to send it to, so it carried on"
)

// Heard is a signal an NPC has heard: which signal, and who sent it, an
// NPC, or the zone of the zone loop that did.
type Heard struct {
	Signal ids.ULID
	From   ids.ULID
	Loop   ids.ULID
}

// Sent is a signal a runner sent on its latest Step: the signal and who
// sent it; whom to, one NPC or everyone in its zone; the NPC or the zone;
// and the placement or zone loop that sent it, which never hears its own.
type Sent struct {
	Heard

	To     string
	NPC    ids.ULID
	Zone   ids.ULID
	Sender ids.ULID
}

// interrupt is a signal being handled: what was heard, and where the NPC
// was when it landed, to resume there: its node, container, work, lanes,
// calls, the module it was in and that module's locals, and the tick it
// landed. main is every node its behaviour's start reaches, where the
// receiver takes over.
type interrupt struct {
	heard     Heard
	node      ids.ULID
	container ids.ULID
	work      work
	lanes     []lane
	calls     []caller
	module    ids.ULID
	locals    map[ids.ULID]worldstate.Value
	tick      uint64
	main      map[ids.ULID]bool
}

// Deliver gives each runner the signals sent to it on the latest tick, in
// the order they were sent: to one NPC, its runners; to everyone in a zone,
// the runners in it. The runners are those of one zone, the one running; a
// runner never hears a signal it sent.
func Deliver(sent []Sent, runners []*NPC, zone ids.ULID) {
	for _, signal := range sent {
		for _, runner := range runners {
			if runner.sender() == signal.Sender || !signal.reaches(runner, zone) {
				continue
			}
			runner.hear(signal.Heard)
		}
	}
}

// reaches reports whether a signal goes to a runner of a zone.
func (s Sent) reaches(runner *NPC, zone ids.ULID) bool {
	if s.To == flow.ToNPC {
		return !s.NPC.IsZero() && runner.NPC == s.NPC
	}

	return s.To == flow.ToZone && s.Zone == zone
}

// sender identifies a runner as the sender of a signal: its placement, or
// its zone for a zone loop.
func (n *NPC) sender() ids.ULID {
	if n.IsLoop() {
		return n.Loop
	}

	return n.Placement
}

// hear queues a signal to land at the NPC's next safe moment; one of each
// signal waits at a time.
func (n *NPC) hear(heard Heard) {
	if !slices.ContainsFunc(n.heard, func(waiting Heard) bool { return waiting.Signal == heard.Signal }) {
		n.heard = append(n.heard, heard)
	}
}

// land interrupts the NPC with the first signal waiting that its behaviour
// answers, unless it is handling one already; a signal it doesn't answer is
// gone. It is called only at a safe moment, standing on a cell between
// ticks, never inside a run of nodes that take no time.
func (n *NPC) land(world World, tick uint64) {
	if n.handling != nil || world.Workflow == nil {
		return
	}
	receivers := world.Workflow.Receivers()
	for len(n.heard) > 0 {
		next := n.heard[0]
		n.heard = n.heard[1:]
		if receiver, answers := receivers[next.Signal]; answers {
			n.interruptBy(world, next, receiver, tick)

			return
		}
	}
}

// interruptBy keeps where the NPC is and sends it to a receiver in its
// behaviour, out of any calls, keeping who sent the signal if the receiver
// asks; lines it has yet to say are dropped.
func (n *NPC) interruptBy(world World, heard Heard, receiver ids.ULID, tick uint64) {
	kept := &interrupt{
		heard: heard, node: n.Node, container: n.Container, work: n.work, lanes: n.lanes, calls: n.calls,
		module: n.module, locals: nil, tick: tick, main: world.Workflow.Reach(world.Workflow.Start()),
	}
	if len(n.calls) > 0 {
		kept.locals, n.locals = n.locals, n.calls[0].locals
	}
	n.calls, n.module, n.work = nil, ids.ULID{}, idle()
	n.speech.DropWaiting()
	n.enter(world.Workflow, receiver, "", tick)
	n.handling = kept
	if node, ok := world.Workflow.Node(receiver); ok {
		if keep, err := node.Ref(flow.ParamSender); err == nil && !keep.IsZero() {
			sender := worldstate.NoValue()
			sender.Type, sender.NPC = worldstate.TypeNPC, heard.From
			n.write(world, keep, sender)
		}
		n.writeLine(world, node, "Heard "+signalName(world, heard.Signal)+n.fromWhom(world, heard))
	}
}

// fromWhom says who sent a signal, for the log: " from Baker", or nothing
// for a zone loop.
func (n *NPC) fromWhom(world World, heard Heard) string {
	if heard.From.IsZero() {
		return ""
	}

	return " from " + n.names(world).NPC(heard.From)
}

// signalName names a signal by its declaration, or by its identity when it
// is not declared.
func signalName(world World, signal ids.ULID) string {
	for _, declared := range world.Signals {
		if declared.ID == signal {
			return declared.Name
		}
	}

	return signal.String()
}

// actResume goes back to where the NPC was when its signal landed: a Wait
// with the time it had left, a Move with its tiles left, a Go to finding a
// new way from where the NPC stands, a sound or a shake done if it ended
// meanwhile, a Together each lane where it was. With no signal being
// handled it starts again, as a branch with nothing after it does.
func (n *NPC) actResume(world World, node flow.Node, tick uint64) {
	kept := n.handling
	if kept == nil {
		running := n.Running(world)
		n.enter(running, running.Start(), "", tick)

		return
	}
	n.handling = nil
	n.Last = Change{From: node.ID, Output: "", To: kept.node, Tick: tick, Status: "", Lane: ""}
	n.Taken = append(n.Taken, n.Last)
	n.Changes++
	if len(kept.calls) > 0 {
		kept.calls[0].locals, n.locals = n.locals, kept.locals
	}
	elapsed := tick - kept.tick
	n.Node, n.Container, n.calls, n.module = kept.node, kept.container, kept.calls, kept.module
	n.work, n.lanes = resumed(n.kindOf(world, kept.node), kept.work, elapsed), kept.lanes
	for index := range n.lanes {
		n.lanes[index].work = resumed(n.kindOf(world, n.lanes[index].node), n.lanes[index].work, elapsed)
	}
}

// kindOf is the kind of a node of the workflow the NPC is following.
func (n *NPC) kindOf(world World, nodeID ids.ULID) string {
	node, _ := nodeOf(n.Running(world), nodeID)

	return node.Kind
}

// resumed is a node's work as it carries on after an interruption of some
// ticks: a Go to plans its way again; a sound or a shake has kept playing;
// anything else carries on as it was.
func resumed(kind string, kept work, elapsed uint64) work {
	switch kind {
	case flow.KindGoTo:
		return idle()
	case flow.KindPlaySound, flow.KindShake:
		kept.waitLeft -= float64(elapsed) / clock.TicksPerSecond
	}

	return kept
}

// takenOver forgets the signal being handled once its receiver's path
// leads into the flow its behaviour's start reaches.
func (n *NPC) takenOver(to ids.ULID) {
	if n.handling != nil && len(n.calls) == 0 && n.handling.main[to] {
		n.handling = nil
	}
}

// actSend sends a Send signal's signal, at once, to land on the next tick.
func (n *NPC) actSend(world World, node flow.Node) {
	signal, chosen := node.Signal()
	if !chosen {
		n.work.skipped = StatusNoSignal

		return
	}
	sent := Sent{
		Heard: Heard{Signal: signal, From: n.NPC, Loop: n.Loop}, To: choiceOr(node, flow.ParamSendTo, flow.ToNPC),
		NPC: ids.ULID{}, Zone: world.Here, Sender: n.sender(),
	}
	switch sent.To {
	case flow.ToNPC:
		sent.NPC, _ = node.Addressee()
	case flow.ToLocal:
		sent.To = flow.ToNPC
		if holder, err := node.Ref(flow.ParamHolder); err == nil {
			if declared, value, ok := n.read(world, holder); ok && declared.Type == worldstate.TypeNPC {
				sent.NPC = value.NPC
			}
		}
	}
	if sent.To == flow.ToNPC && sent.NPC.IsZero() {
		n.work.skipped = StatusNoAddressee

		return
	}
	n.Sent = append(n.Sent, sent)
}

// choiceOr reads a choice setting, or a fallback when it can't be read.
func choiceOr(node flow.Node, name, fallback string) string {
	chosen, err := node.Choice(name)
	if err != nil {
		return fallback
	}

	return chosen
}

// Handling returns the signal the NPC is handling, and whether it is
// handling one.
func (n *NPC) Handling() (Heard, bool) {
	if n.handling == nil {
		return Heard{Signal: ids.ULID{}, From: ids.ULID{}, Loop: ids.ULID{}}, false
	}

	return n.handling.heard, true
}

// Resumes says what the NPC goes back to when it resumes: the node it was
// on and what it was doing there, in its behaviour or the module it was in;
// "" when it is handling no signal.
func (n *NPC) Resumes(behaviour *flow.Workflow, workflows flow.Lookup) (ids.ULID, string) {
	kept := n.handling
	if kept == nil {
		return ids.ULID{}, ""
	}
	workflow := behaviour
	if !kept.module.IsZero() && workflows != nil {
		workflow, _ = workflows(kept.module)
	}
	node, ok := nodeOf(workflow, kept.node)
	if !ok {
		return kept.node, ""
	}

	return kept.node, describe(node, kept.work)
}

// Held lists the signals waiting to land, in the order they will.
func (n *NPC) Held() []Heard { return slices.Clone(n.heard) }
