package npcrun

import (
	"image"

	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Statuses of the value nodes, when one cannot do its work and carries on.
const (
	StatusNoValue   = "Its value is missing or does not fit, so it carried on"
	StatusNoKeep    = "Nowhere to keep what it finds, so it carried on"
	StatusNoOperand = "Nothing to set or compare it with, so it carried on"
)

// Other is another NPC in the zone, as a tick begins: which placement, of
// which NPC, on which floor and in which cell.
type Other struct {
	Placement ids.ULID
	NPC       ids.ULID
	Floor     ids.ULID
	Cell      image.Point
}

// Locals returns a copy of the NPC's local values as set so far; a local
// not set is its default. They last through its workflow's repeats and
// clear when it restarts, and a new runner, as in another zone, starts
// with none.
func (n *NPC) Locals() map[ids.ULID]worldstate.Value {
	copied := make(map[ids.ULID]worldstate.Value, len(n.locals))
	for id, value := range n.locals {
		copied[id] = value
	}

	return copied
}

// lookup finds a reference's declaration and whose value it is: no one's
// for a local or a world value, the zone's, or an NPC's. An NPC value is
// this NPC's, the NPC a local holds, or the NPC named.
func (n *NPC) lookup(world World, ref flow.ValueRef) (worldstate.Declaration, ids.ULID, bool) {
	var none worldstate.Declaration
	if ref.IsZero() {
		return none, ids.ULID{}, false
	}
	if ref.Scope == worldstate.ScopeLocal {
		declared, ok := world.Workflow.Locals().Value(ref.Value)

		return declared, ids.ULID{}, ok
	}
	if world.State == nil {
		return none, ids.ULID{}, false
	}
	declared, ok := world.State.Declared(ref.Value)
	if !ok || declared.Scope != ref.Scope {
		return none, ids.ULID{}, false
	}
	switch {
	case ref.Scope == worldstate.ScopeZone:
		return declared, world.Here, true
	case ref.Scope != worldstate.ScopeNPC:
		return declared, ids.ULID{}, true
	case !ref.NPC.IsZero():
		return declared, ref.NPC, true
	case !ref.Found.IsZero():
		holder, held := world.Workflow.Locals().Value(ref.Found)
		if !held || holder.Type != worldstate.TypeNPC {
			return none, ids.ULID{}, false
		}
		found := n.local(holder).NPC

		return declared, found, !found.IsZero()
	}

	return declared, n.NPC, true
}

// local returns a local value as it is now.
func (n *NPC) local(declared worldstate.Declaration) worldstate.Value {
	if value, set := n.locals[declared.ID]; set && declared.Fits(value) {
		return value
	}

	return declared.Zero()
}

// read returns a value as it is now.
func (n *NPC) read(world World, ref flow.ValueRef) (worldstate.Declaration, worldstate.Value, bool) {
	declared, owner, ok := n.lookup(world, ref)
	if !ok {
		return declared, worldstate.Value{Type: "", Number: 0, Yes: false, Choice: "", NPC: ids.ULID{}}, false
	}
	if declared.Scope == worldstate.ScopeLocal {
		return declared, n.local(declared), true
	}

	return declared, world.State.Get(declared, owner), true
}

// write sets one of the workflow's own values: a local one, its NPC's, its
// zone's or the world's. It never sets another NPC's.
func (n *NPC) write(world World, ref flow.ValueRef, value worldstate.Value) bool {
	if !ref.Writable() {
		return false
	}
	declared, owner, ok := n.lookup(world, ref)
	if !ok || !declared.Fits(value) {
		return false
	}
	if declared.Scope == worldstate.ScopeLocal {
		if n.locals == nil {
			n.locals = map[ids.ULID]worldstate.Value{}
		}
		n.locals[declared.ID] = value

		return true
	}

	return world.State.Set(declared, owner, value) == nil
}

// operand reads a node's operand as a value of a declaration's type.
func operand(node flow.Node, name string, declared worldstate.Declaration) (worldstate.Value, bool) {
	raw, err := node.Operand(name)
	if err != nil || flow.IsNoOperand(raw) {
		return worldstate.Value{Type: "", Number: 0, Yes: false, Choice: "", NPC: ids.ULID{}}, false
	}
	value, err := declared.Parse(raw)

	return value, err == nil
}

// actSet sets a value, or adds to a number, and carries on.
func (n *NPC) actSet(world World, node flow.Node) {
	ref, err := node.Ref(flow.ParamValue)
	if err != nil {
		n.work.skipped = StatusNoValue

		return
	}
	declared, current, readable := n.read(world, ref)
	if !readable || declared.Type == worldstate.TypeNPC {
		n.work.skipped = StatusNoValue

		return
	}
	given, fits := operand(node, "to", declared)
	if !fits {
		n.work.skipped = StatusNoOperand

		return
	}
	if how, err := node.Choice("how"); err == nil && how == flow.HowAdd {
		if declared.Type != worldstate.TypeNumber {
			n.work.skipped = StatusNoValue

			return
		}
		given.Number = min(max(current.Number+given.Number, -worldstate.MaxNumber), worldstate.MaxNumber)
	}
	if !n.write(world, ref, given) {
		n.work.skipped = StatusNoValue
	}
}

// actCheck compares a value and takes "yes" or "otherwise".
func (n *NPC) actCheck(world World, node flow.Node) string {
	ref, err := node.Ref(flow.ParamValue)
	if err != nil {
		n.work.skipped = StatusNoValue

		return flow.OutputOtherwise
	}
	declared, current, readable := n.read(world, ref)
	if !readable || declared.Type == worldstate.TypeNPC {
		n.work.skipped = StatusNoValue

		return flow.OutputOtherwise
	}
	against, fits := operand(node, "against", declared)
	if !fits {
		n.work.skipped = StatusNoOperand

		return flow.OutputOtherwise
	}
	compare, err := node.Choice("compare")
	if err == nil && compares(compare, current, against) {
		return flow.OutputYes
	}

	return flow.OutputOtherwise
}

// compares reports whether two values compare as a Check value asks: is, at least or
// below, the last two for numbers only.
func compares(compare string, current, against worldstate.Value) bool {
	switch compare {
	case flow.CompareLeast:
		return current.Type == worldstate.TypeNumber && current.Number >= against.Number
	case flow.CompareBelow:
		return current.Type == worldstate.TypeNumber && current.Number < against.Number
	}

	return current.Equal(against)
}

// actFind looks for the nearest other NPC on this NPC's floor within a
// number of tiles, in a square around it, optionally only one whose value
// matches, and keeps it in a local value: "found", or "none" with the local
// cleared. Of two as near, the first in placement order is found.
func (n *NPC) actFind(world World, node flow.Node) string {
	keep, err := node.Ref("into")
	declared, _, ok := n.lookup(world, keep)
	if err != nil || !ok || declared.Scope != worldstate.ScopeLocal || declared.Type != worldstate.TypeNPC {
		n.work.skipped = StatusNoKeep

		return flow.OutputNone
	}
	within, err := node.Number("within")
	if err != nil {
		within = 0
	}
	matches := n.matcher(world, node)
	nearest, best := ids.ULID{}, within+1
	for _, other := range world.Others {
		gap := other.Cell.Sub(n.Cell)
		distance := max(abs(gap.X), abs(gap.Y))
		if other.Placement == n.Placement || other.Floor != n.Floor || distance >= best || !matches(other.NPC) {
			continue
		}
		nearest, best = other.NPC, distance
	}
	found := worldstate.Value{Type: worldstate.TypeNPC, Number: 0, Yes: false, Choice: "", NPC: nearest}
	n.write(world, keep, found)
	if nearest.IsZero() {
		return flow.OutputNone
	}

	return flow.OutputFound
}

// matcher returns what a Find NPC nearby looks for: any NPC, or one whose
// own value is the one asked for.
func (n *NPC) matcher(world World, node flow.Node) func(npc ids.ULID) bool {
	where, err := node.Ref("where")
	if err != nil || where.IsZero() {
		return func(ids.ULID) bool { return true }
	}
	nobody := func(ids.ULID) bool { return false }
	if world.State == nil || where.Scope != worldstate.ScopeNPC || where.Others() {
		n.work.skipped = StatusNoValue

		return nobody
	}
	declared, known := world.State.Declared(where.Value)
	if !known || declared.Scope != worldstate.ScopeNPC {
		n.work.skipped = StatusNoValue

		return nobody
	}
	wanted, fits := operand(node, "is", declared)
	if !fits {
		n.work.skipped = StatusNoOperand

		return nobody
	}

	return func(npc ids.ULID) bool { return world.State.Get(declared, npc).Equal(wanted) }
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}
