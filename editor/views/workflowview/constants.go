package workflowview

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// problemSetConstant is a Set value on a constant, which never changes.
const problemSetConstant = "A constant never changes. Set value sets a value; change a constant in World state's " +
	"Constants window."

// readsConstants reports whether a role's chooser offers constants: what a
// Check value checks or compares with, what a message or a Call's input
// reads, and what Find NPC nearby looks for. Set value sets only values.
func readsConstants(role valueRole) bool {
	return role == roleCheck || role == roleGive || role == roleWhere || makesConstant(role)
}

// constantOptions adds the constants a role reads, listed where values
// are: its NPC's, its zone's and the world's, then for a Check value or a
// message other NPCs' and zones'.
func (s *State) constantOptions(found *options, docs frame.Docs, catalog *worldstate.Catalog, role valueRole,
	locals []worldstate.Declaration,
) {
	if !readsConstants(role) {
		return
	}
	heading := "THIS NPC"
	if role == roleWhere {
		heading = "THE NPC IT FINDS"
	}
	if role == roleWhere || s.workflow.Purpose() != flow.PurposeZoneLoop {
		for _, held := range s.npcConstants(docs, catalog, role) {
			found.add(scopePlace(heading, worldstate.ScopeNPC), refTo(held.Declaration, ids.ULID{}, ids.ULID{}),
				held.Path, held.Declaration)
		}
	}
	if role == roleWhere {
		return
	}
	zoneConstants := catalog.TemplateConstants(worldstate.ScopeZone)
	for _, zone := range catalog.Owners(worldstate.ScopeZone) {
		zoneConstants = append(zoneConstants, catalog.HeldConstants(worldstate.ScopeZone, zone)...)
	}
	for _, held := range zoneConstants {
		found.add(scopePlace("THIS ZONE", worldstate.ScopeZone), refTo(held.Declaration, ids.ULID{}, ids.ULID{}),
			held.Path, held.Declaration)
	}
	for _, held := range catalog.HeldConstants(worldstate.ScopeWorld, ids.ULID{}) {
		found.add(scopePlace("WORLD", worldstate.ScopeWorld), refTo(held.Declaration, ids.ULID{}, ids.ULID{}),
			held.Path, held.Declaration)
	}
	if makesConstant(role) {
		kind := worldstate.TypeInteger
		if role == roleMakeText {
			kind = worldstate.TypeText
		}
		found.keepType(kind)

		return
	}
	s.othersConstants(found, catalog, locals)
}

// keepType keeps only constants of one type: whole numbers for a typed-in
// number's, text for words in a message.
func (o *options) keepType(kind worldstate.Type) {
	kept := o.listed[:0]
	for _, option := range o.listed {
		if option.declared.Type == kind {
			kept = append(kept, option)
		}
	}
	o.listed = kept
}

// npcConstants lists the constants of the NPCs following the workflow, or
// with none following it, or for the NPC Find NPC nearby finds, every NPC
// template's.
func (s *State) npcConstants(docs frame.Docs, catalog *worldstate.Catalog, role valueRole) []worldstate.Held {
	followers := s.followers(docs, catalog)
	if len(followers) == 0 || role == roleWhere {
		return catalog.TemplateConstants(worldstate.ScopeNPC)
	}
	var constants []worldstate.Held
	for _, follower := range followers {
		constants = append(constants, catalog.HeldConstants(worldstate.ScopeNPC, follower)...)
	}

	return constants
}

// othersConstants adds other NPCs' and zones' constants: those of each NPC
// a local value holds, and of each NPC and zone with a path name.
func (s *State) othersConstants(found *options, catalog *worldstate.Catalog, locals []worldstate.Declaration) {
	shared := catalog.TemplateConstants(worldstate.ScopeNPC)
	for _, holder := range locals {
		if holder.Type != worldstate.TypeNPC {
			continue
		}
		where := place{heading: "THE NPC IN " + localPath(holder), rail: railOthers, root: localPath(holder)}
		for _, held := range shared {
			found.add(where, refTo(held.Declaration, ids.ULID{}, holder.ID),
				localPath(holder)+strings.TrimPrefix(held.Path, string(worldstate.ScopeNPC)), held.Declaration)
		}
	}
	for _, scope := range []worldstate.Scope{worldstate.ScopeNPC, worldstate.ScopeZone} {
		rail := railOthers
		if scope == worldstate.ScopeZone {
			rail = railZones
		}
		for _, ownerID := range catalog.Owners(scope) {
			owner, _ := catalog.Owner(ownerID)
			if owner.Path == "" {
				continue
			}
			where := place{heading: strings.ToUpper(owner.Name), rail: rail, root: string(scope) + "." + owner.Path}
			for _, held := range catalog.HeldConstants(scope, ownerID) {
				found.add(where, refTo(held.Declaration, ownerID, ids.ULID{}),
					worldstate.NamedPath(held.Path, owner.Path), held.Declaration)
			}
		}
	}
}

// holds reports whether an NPC or zone has a value or a constant.
func holds(catalog *worldstate.Catalog, scope worldstate.Scope, owner, id ids.ULID) bool {
	if _, has := catalog.Lookup(scope, owner, id); has {
		return true
	}
	_, _, has := catalog.Constant(scope, owner, id)

	return has
}

// constantValue reads a constant as an NPC or zone has it, or a value's
// declaration as it starts, for the editor's preview of a line.
func constantValue(catalog *worldstate.Catalog, scope worldstate.Scope, owner ids.ULID,
	declared worldstate.Declaration,
) (worldstate.Declaration, worldstate.Value) {
	if declared.IsConstant() {
		if held, value, has := catalog.Constant(scope, owner, declared.ID); has {
			return held, value
		}

		return declared, declared.Zero()
	}
	if own, ok := catalog.Lookup(scope, owner, declared.ID); ok {
		declared = own
	}

	return declared, declared.Zero()
}

// constantWarning warns of a Set value that copies a text constant into a
// text value, when the constant, or an NPC's or zone's own value of it, is
// longer than the value holds: it is cut.
func (s *State) constantWarning(node flow.Node, docs frame.Docs) string {
	if node.Kind != flow.KindSetValue {
		return ""
	}
	target, err := node.Ref(flow.ParamValue)
	raw, rawErr := node.Operand("to")
	ref, refers := flow.OperandRef(raw)
	if err != nil || rawErr != nil || !refers {
		return ""
	}
	declared, _, problem := s.resolve(docs, target)
	source, path, sourceProblem := s.resolve(docs, ref)
	if problem != "" || sourceProblem != "" || !source.IsConstant() || declared.Type != worldstate.TypeText {
		return ""
	}
	longest := utf8.RuneCountInString(source.Zero().Text)
	catalog := s.catalog(docs)
	if template, inTemplate := catalog.World().Template(source.ID); inTemplate {
		for _, holder := range catalog.Holders(template) {
			owner, _ := catalog.Owner(holder)
			if value, own := owner.Owned.ConstantValue(source); own {
				longest = max(longest, utf8.RuneCountInString(value.Text))
			}
		}
	}
	if longest <= declared.TextLimit() {
		return ""
	}

	return path + " runs to " + strconv.Itoa(longest) + " characters, longer than a value's " +
		strconv.Itoa(declared.TextLimit()) + ". Set value keeps the first " + strconv.Itoa(declared.TextLimit()) + "."
}

// warningOf says what deserves a look on a node but is not wrong.
func (s *State) warningOf(node flow.Node, docs frame.Docs) string {
	if warning := s.signalWarning(node, docs); warning != "" {
		return warning
	}

	return s.constantWarning(node, docs)
}

// setProblem says what a Set value cannot set: a constant, another's value,
// or an NPC's zone or behaviour.
func setProblem(node flow.Node, ref flow.ValueRef, declared worldstate.Declaration) string {
	switch {
	case node.Kind != flow.KindSetValue:
		return ""
	case declared.IsConstant():
		return problemSetConstant
	case !ref.Writable():
		return problemReadOnly
	case worldstate.ReadOnly(ref.Value):
		return problemBuiltInFixed
	}

	return ""
}
