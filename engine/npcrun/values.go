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
// for a local or a world value, or an NPC's or zone's. An NPC or zone value
// is this NPC's or its zone's, the one named, or the NPC a local holds; it
// is found only if that NPC or zone has it.
func (n *NPC) lookup(world World, ref flow.ValueRef) (worldstate.Declaration, ids.ULID, bool) {
	none := worldstate.New(ids.ULID{}, "", "", "")
	if ref.IsZero() {
		return none, ids.ULID{}, false
	}
	if ref.Scope == worldstate.ScopeLocal {
		declared, ok := world.Workflow.Locals().Value(ref.Value)

		return declared, ids.ULID{}, ok
	}
	owner, known := n.ownerOf(world, ref)
	if world.State == nil || !known {
		return none, ids.ULID{}, false
	}
	declared, found := world.State.Catalog().Lookup(ref.Scope, owner, ref.Value)

	return declared, owner, found
}

// ownerOf returns whose value a reference is.
func (n *NPC) ownerOf(world World, ref flow.ValueRef) (ids.ULID, bool) {
	switch {
	case ref.Scope == worldstate.ScopeWorld:
		return ids.ULID{}, true
	case !ref.Owner.IsZero():
		return ref.Owner, true
	case ref.Scope == worldstate.ScopeZone:
		return world.Here, true
	case !ref.Found.IsZero():
		holder, held := world.Workflow.Locals().Value(ref.Found)
		if !held || holder.Type != worldstate.TypeNPC {
			return ids.ULID{}, false
		}
		found := n.local(holder).NPC

		return found, !found.IsZero()
	}

	return n.NPC, true
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
		return declared, worldstate.NoValue(), false
	}
	if declared.Scope == worldstate.ScopeLocal {
		return declared, n.local(declared), true
	}

	return declared, world.State.Get(declared, owner), true
}

// write sets one of the workflow's own values: a local one, its NPC's, its
// zone's or the world's. It never sets another NPC's or zone's, nor its
// NPC's zone or behaviour, which are set in its details.
func (n *NPC) write(world World, ref flow.ValueRef, value worldstate.Value) bool {
	if !ref.Writable() || worldstate.ReadOnly(ref.Value) {
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
		return worldstate.NoValue(), false
	}
	value, err := declared.Parse(raw)

	return value, err == nil
}

// setTo reads what a Set value sets its value to: the value saved in it,
// or another value it refers to, read now, of the same type or, for a text
// value, of any type, as its text form.
func (n *NPC) setTo(world World, node flow.Node, declared worldstate.Declaration) (worldstate.Value, bool) {
	raw, err := node.Operand("to")
	if err != nil {
		return worldstate.NoValue(), false
	}
	ref, refers := flow.OperandRef(raw)
	if !refers {
		return operand(node, "to", declared)
	}
	other, value, readable := n.read(world, ref)
	switch {
	case !readable:
		return worldstate.NoValue(), false
	case other.Type == declared.Type:
		return value, true
	case declared.Type == worldstate.TypeText:
		text := declared.Empty()
		text.Text = worldstate.Text(other, value, namer{names: world.Names, state: world.State})
		if runes := []rune(text.Text); len(runes) > worldstate.MaxText {
			text.Text = string(runes[:worldstate.MaxText])
		}

		return text, true
	}

	return worldstate.NoValue(), false
}

// operandOf reads a Check value's operand as a value of the declaration's
// type: another value it refers to, read now and of the same type, or the
// value saved in it.
func (n *NPC) operandOf(
	world World, node flow.Node, name string, declared worldstate.Declaration,
) (worldstate.Value, bool) {
	raw, err := node.Operand(name)
	if err != nil {
		return worldstate.NoValue(), false
	}
	if ref, refers := flow.OperandRef(raw); refers {
		other, value, readable := n.read(world, ref)

		return value, readable && other.Type == declared.Type
	}

	return operand(node, name, declared)
}

// actSet sets a value, or adds to an integer, holding an integer at its
// limits, and carries on.
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
	given, fits := n.setTo(world, node, declared)
	if !fits {
		n.work.skipped = StatusNoOperand

		return
	}
	if how, err := node.Choice("how"); err == nil && how == flow.HowAdd {
		if declared.Type != worldstate.TypeInteger {
			n.work.skipped = StatusNoValue

			return
		}
		given.Integer += current.Integer
	}
	if declared.Type == worldstate.TypeInteger {
		given.Integer = declared.Hold(given.Integer)
	}
	if !n.write(world, ref, given) {
		n.work.skipped = StatusNoValue
	}
}

// actCheck compares a value as its type allows, or asks whether an integer
// is at a limit, and takes "yes" or "otherwise". A comparison the type
// cannot make, or an operand that is missing or of the wrong type, takes
// otherwise and says why.
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
	compare, err := node.Choice(flow.ParamCompare)
	if err != nil {
		compare = flow.CompareIs
	}
	if !flow.CanCompare(compare, declared.Type) {
		n.work.skipped = StatusNoOperand

		return flow.OutputOtherwise
	}
	switch compare {
	case flow.CompareMax:
		return outcome(declared.AtMost(current))
	case flow.CompareMin:
		return outcome(declared.AtLeast(current))
	case flow.CompareBetween:
		return n.checkBetween(world, node, declared, current)
	case flow.CompareAnyOf, flow.CompareNoneOf:
		return n.checkAmong(node, compare, declared, current)
	}
	against, fits := n.operandOf(world, node, flow.ParamAgainst, declared)
	if !fits {
		n.work.skipped = StatusNoOperand

		return flow.OutputOtherwise
	}

	return outcome(compares(compare, current, against))
}

// checkBetween asks whether an integer is within a range, both ends
// included, each end a number or another value.
func (n *NPC) checkBetween(
	world World, node flow.Node, declared worldstate.Declaration, current worldstate.Value,
) string {
	low, lowFits := n.operandOf(world, node, flow.ParamAgainst, declared)
	high, highFits := n.operandOf(world, node, flow.ParamTo, declared)
	if !lowFits || !highFits {
		n.work.skipped = StatusNoOperand

		return flow.OutputOtherwise
	}

	return outcome(current.Integer >= low.Integer && current.Integer <= high.Integer)
}

// checkAmong asks whether a choice is any of a list, or none of it; an
// empty list is no operand.
func (n *NPC) checkAmong(
	node flow.Node, compare string, declared worldstate.Declaration, current worldstate.Value,
) string {
	among, err := node.Operand(flow.ParamAmong)
	items := flow.Operands(among)
	if err != nil || len(items) == 0 {
		n.work.skipped = StatusNoOperand

		return flow.OutputOtherwise
	}
	matched := false
	for _, item := range items {
		if parsed, parseErr := declared.Parse(item); parseErr == nil && current.Equal(parsed) {
			matched = true
		}
	}

	return outcome(matched == (compare == flow.CompareAnyOf))
}

// outcome is "yes" when a comparison holds, "otherwise" when not.
func outcome(holds bool) string {
	if holds {
		return flow.OutputYes
	}

	return flow.OutputOtherwise
}

// compares reports whether two values compare as a Check value asks: is or
// is not for any type, below, at most, above and at least for integers.
func compares(compare string, current, against worldstate.Value) bool {
	integer := current.Type == worldstate.TypeInteger
	switch compare {
	case flow.CompareNot:
		return !current.Equal(against)
	case flow.CompareLeast:
		return integer && current.Integer >= against.Integer
	case flow.CompareMost:
		return integer && current.Integer <= against.Integer
	case flow.CompareAbove:
		return integer && current.Integer > against.Integer
	case flow.CompareBelow:
		return integer && current.Integer < against.Integer
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
	found := worldstate.NoValue()
	found.Type, found.NPC = worldstate.TypeNPC, nearest
	n.write(world, keep, found)
	if nearest.IsZero() {
		return flow.OutputNone
	}

	return flow.OutputFound
}

// matcher returns what a Find NPC nearby looks for: any NPC, or one that has
// the value asked about, set to what is asked for.
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
	catalog := world.State.Catalog()
	declared, _, known := catalog.Declared(where.Value)
	if !known || declared.Scope != worldstate.ScopeNPC || !declared.Type.IsValue() {
		n.work.skipped = StatusNoValue

		return nobody
	}
	wanted, fits := operand(node, "is", declared)
	if !fits {
		n.work.skipped = StatusNoOperand

		return nobody
	}

	return func(npc ids.ULID) bool {
		held, has := catalog.Lookup(worldstate.ScopeNPC, npc, where.Value)

		return has && world.State.Get(held, npc).Equal(wanted)
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}
