package npcrun

import (
	"github.com/bradhannah/berad-engine/engine/show"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Saying returns the line the NPC is saying now, over its head, if any.
func (n *NPC) Saying() (show.Line, bool) { return n.speech.Now() }

// waitsToFinish reports whether a Say or a Show message carries on only
// once its line is said or its box is gone.
func waitsToFinish(node flow.Node) bool {
	carry, err := node.Choice(flow.ParamCarry)

	return err == nil && carry == flow.CarryWhenDone
}

// showTicks is how long a line or a box shows: as long as it takes to read,
// or the seconds set by hand.
func showTicks(node flow.Node, text string) int {
	if node.UsesSeconds() {
		if seconds, err := node.Seconds(); err == nil {
			return show.SecondsTicks(seconds)
		}
	}

	return show.ReadingTicks(text)
}

// actShow does the work of a node of the Show group.
func (n *NPC) actShow(world World, node flow.Node) (string, bool) {
	switch node.Kind {
	case flow.KindSay:
		return n.actSay(world, node)
	case flow.KindShowMessage:
		return n.actShowMessage(world, node)
	default:
		return n.actShake(world, node)
	}
}

// actSay queues a line over the NPC's head, read now, and writes it to the
// log; it carries on at once, or once the line has been said.
func (n *NPC) actSay(world World, node flow.Node) (string, bool) {
	if n.work.started {
		return flow.OutputNext, n.speech.Said(n.work.awaited)
	}
	text, ok := n.messageText(world, node)
	if !ok {
		return flow.OutputNext, true
	}
	said := n.speech.Say(text, showTicks(node, text))
	n.writeLine(world, node, n.names(world).NPC(n.NPC)+" says: "+show.Plain(text))
	if !waitsToFinish(node) {
		return flow.OutputNext, true
	}
	n.work = idle()
	n.work.started, n.work.awaited = true, said

	return flow.OutputNext, false
}

// actShowMessage asks for a message box, read now, and writes it to the
// log; it carries on at once, or once the box has gone. With no stage to
// show it on, it carries on.
func (n *NPC) actShowMessage(world World, node flow.Node) (string, bool) {
	if n.work.started {
		return flow.OutputNext, world.Stage == nil || world.Stage.Gone(n.work.awaited)
	}
	text, ok := n.messageText(world, node)
	if !ok || world.Stage == nil {
		return flow.OutputNext, true
	}
	name := ""
	if named, err := node.Choice(flow.ParamBoxName); err == nil && named == flow.BoxNameNPC && !n.IsLoop() {
		name = n.names(world).NPC(n.NPC)
	}
	away, _ := node.Choice(flow.ParamAway) //nolint:errcheck // A box with no way set goes when read.
	box := world.Stage.Ask(text, name, showTicks(node, text), away == flow.AwayClosed)
	n.writeLine(world, node, "Box: "+show.Plain(text))
	if !waitsToFinish(node) {
		return flow.OutputNext, true
	}
	n.work = idle()
	n.work.started, n.work.awaited = true, box

	return flow.OutputNext, false
}

// shakePixels is how far each strength shakes the screen.
func shakePixels(strength string) float64 {
	switch strength {
	case flow.ShakeGentle:
		return show.GentlePixels
	case flow.ShakeStrong:
		return show.StrongPixels
	}

	return show.ModeratePixels
}

// actShake shakes the screen for its seconds, in real time, and carries on
// when it stops.
func (n *NPC) actShake(world World, node flow.Node) (string, bool) {
	if !n.work.started {
		seconds, err := node.Seconds()
		if err != nil {
			return flow.OutputNext, true
		}
		strength, _ := node.Choice(flow.ParamStrength) //nolint:errcheck // No strength shakes moderately.
		if world.Stage != nil {
			world.Stage.Shake(shakePixels(strength), show.SecondsTicks(seconds))
		}
		n.work = waitWork(float64(seconds), true)
	}

	return flow.OutputNext, n.work.waitLeft <= 0
}
