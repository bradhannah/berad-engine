package npcrun

import (
	"strings"

	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// MaxLine is how many characters of a Log's line are kept; the rest is cut.
const MaxLine = 200

// Unreadable stands in a line for a value a Log can't read, as one since
// deleted, or another NPC's it does not have.
const Unreadable = "(no such value)"

// Line is what one Log wrote: its text, with each value as its text form,
// and the workflow and node that wrote it.
type Line struct {
	Text     string
	Workflow ids.ULID
	Node     ids.ULID
}

// actLog writes a Log's line, reading each value in its message now, and
// carries on.
func (n *NPC) actLog(world World, node flow.Node) {
	if line, ok := n.messageText(world, node); ok {
		n.writeLine(world, node, line)
	}
}

// names is what turns the documents and places values refer to into text.
func (n *NPC) names(world World) namer { return namer{names: world.Names, state: world.State} }

// messageText is a node's message as it reads now: its text, and each value
// in it as its text form; ok is false for a message that can't be read.
func (n *NPC) messageText(world World, node flow.Node) (string, bool) {
	segments, err := node.Message()
	if err != nil {
		return "", false
	}
	names := n.names(world)
	var text strings.Builder
	for _, segment := range segments {
		if segment.Ref == nil {
			text.WriteString(segment.Text)

			continue
		}
		declared, value, readable := n.read(world, *segment.Ref)
		if !readable {
			text.WriteString(Unreadable)

			continue
		}
		text.WriteString(worldstate.Text(declared, value, names))
	}

	return text.String(), true
}

// writeLine writes a line to the log, cut to MaxLine characters, naming the
// workflow and node that wrote it.
func (n *NPC) writeLine(world World, node flow.Node, line string) {
	if runes := []rune(line); len(runes) > MaxLine {
		line = string(runes[:MaxLine])
	}
	n.Lines = append(n.Lines, Line{Text: line, Workflow: n.writing(world), Node: node.ID})
}

// writing returns the workflow the NPC is following now: the module it is
// in a call to, or its behaviour.
func (n *NPC) writing(world World) ids.ULID {
	if len(n.calls) > 0 {
		return n.module
	}

	return world.Behaviour
}
