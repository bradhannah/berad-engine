package worldstate

import (
	"cmp"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Owner is one NPC or zone as values see it: its scope, its path name, an
// NPC's display name, and what it owns; and an NPC's zone, behaviour and
// sprite, which its built-in values start as.
type Owner struct {
	Scope     Scope
	Path      string
	Name      string
	Owned     Owned
	Zone      ids.ULID
	Behaviour ids.ULID
	Sprite    ids.ULID
}

// Catalog is every declaration a game's values come from: the world
// state's, with its templates, and each NPC's and zone's own.
type Catalog struct {
	world  *Declarations
	owners map[ids.ULID]Owner
}

// NewCatalog returns a catalog of the world state's declarations and the
// NPCs and zones, by document ID.
func NewCatalog(world *Declarations, owners map[ids.ULID]Owner) *Catalog {
	copied := make(map[ids.ULID]Owner, len(owners))
	for id, owner := range owners {
		owner.Owned = owner.Owned.Clone()
		copied[id] = owner
	}

	return &Catalog{world: world.Clone(), owners: copied}
}

// World returns the world state's declarations.
func (c *Catalog) World() *Declarations { return c.world }

// Owner returns an NPC or zone by document ID.
func (c *Catalog) Owner(id ids.ULID) (Owner, bool) {
	owner, ok := c.owners[id]

	return owner, ok
}

// Owners returns a scope's NPCs or zones, in path order.
func (c *Catalog) Owners(scope Scope) []ids.ULID {
	var found []ids.ULID
	for id, owner := range c.owners {
		if owner.Scope == scope {
			found = append(found, id)
		}
	}
	slices.SortFunc(found, func(a, b ids.ULID) int {
		return cmp.Or(cmp.Compare(c.owners[a].Path, c.owners[b].Path), a.Compare(b))
	})

	return found
}

// Named returns the NPC or zone with a path name.
func (c *Catalog) Named(scope Scope, path string) (ids.ULID, bool) {
	for _, id := range c.Owners(scope) {
		if path != "" && c.owners[id].Path == path {
			return id, true
		}
	}

	return ids.ULID{}, false
}

// Lookup returns the value an ID names for whoever it belongs to: for the
// world, a world value; for an NPC or zone, one of the NPC's built-in
// values, one of its own, or one in a template applied to it.
func (c *Catalog) Lookup(scope Scope, owner, value ids.ULID) (Declaration, bool) {
	if scope == ScopeWorld {
		declared, ok := c.world.Value(value)

		return declared, ok && declared.Scope == ScopeWorld && declared.IsValue()
	}
	held, known := c.owners[owner]
	if !known || held.Scope != scope {
		return New(ids.ULID{}, "", "", ""), false
	}
	if scope == ScopeNPC && IsBuiltIn(value) {
		return builtInOf(held, value), true
	}
	if declared, ok := held.Owned.Values.Value(value); ok {
		return declared, declared.IsValue()
	}
	declared, ok := c.world.Value(value)
	template, inTemplate := c.world.Template(value)
	if !ok || !inTemplate || declared.Scope != scope || !declared.IsValue() ||
		!slices.Contains(held.Owned.Templates, template) {
		return New(ids.ULID{}, "", "", ""), false
	}

	return declared, true
}

// Declared finds a declaration by ID wherever it is declared, with the path
// a workflow names it by for its own NPC, zone or world, such as
// npc.guards.how_many_arrested.
func (c *Catalog) Declared(value ids.ULID) (Declaration, string, bool) {
	if IsBuiltIn(value) {
		declared := builtInOf(Owner{
			Scope: ScopeNPC, Path: "", Name: "", Owned: NewOwned(ScopeNPC), Zone: ids.ULID{}, Behaviour: ids.ULID{},
			Sprite: ids.ULID{},
		}, value)

		return declared, string(ScopeNPC) + "." + declared.Name, true
	}
	if declared, ok := c.world.Value(value); ok {
		return declared, c.world.Path(value), true
	}
	for _, id := range slices.Concat(c.Owners(ScopeNPC), c.Owners(ScopeZone)) {
		list := c.owners[id].Owned.Values
		if declared, ok := list.Value(value); ok {
			return declared, list.Path(value), true
		}
	}

	return New(ids.ULID{}, "", "", ""), "", false
}

// Held is one value an NPC, zone or the world has, with the path its own
// workflows name it by.
type Held struct {
	Declaration Declaration
	Path        string
}

// Held lists every value the world, an NPC or a zone has, as a tree reads:
// an NPC's built-in values, its own, then each template's in the order
// applied.
func (c *Catalog) Held(scope Scope, owner ids.ULID) []Held {
	if scope == ScopeWorld {
		return walk(c.world, ScopeWorld, ids.ULID{}, nil, Declaration.IsValue)
	}
	held, known := c.owners[owner]
	if !known || held.Scope != scope {
		return nil
	}
	var values []Held
	if scope == ScopeNPC {
		for _, declared := range BuiltIns(held) {
			values = append(values, Held{Declaration: declared, Path: string(ScopeNPC) + "." + declared.Name})
		}
	}
	values = walk(held.Owned.Values, scope, ids.ULID{}, values, Declaration.IsValue)
	for _, template := range held.Owned.Templates {
		if declared, ok := c.world.Value(template); ok && declared.Scope == scope {
			values = walk(c.world, scope, template, values, Declaration.IsValue)
		}
	}

	return values
}

// HeldConstants lists every constant the world, an NPC or a zone has, as a
// tree reads: an NPC's or zone's own, then each template's in the order
// applied.
func (c *Catalog) HeldConstants(scope Scope, owner ids.ULID) []Held {
	if scope == ScopeWorld {
		return walk(c.world, ScopeWorld, ids.ULID{}, nil, Declaration.IsConstant)
	}
	held, known := c.owners[owner]
	if !known || held.Scope != scope {
		return nil
	}
	constants := walk(held.Owned.Values, scope, ids.ULID{}, nil, Declaration.IsConstant)
	for _, template := range held.Owned.Templates {
		if declared, ok := c.world.Value(template); ok && declared.Scope == scope {
			constants = walk(c.world, scope, template, constants, Declaration.IsConstant)
		}
	}

	return constants
}

// Constant returns a constant an ID names for whoever reads it, with its
// value for them: for the world, a world constant and its value; for an NPC
// or zone, one of its own, or one in a template applied to it with its own
// value of it or else the default.
func (c *Catalog) Constant(scope Scope, owner, constant ids.ULID) (Declaration, Value, bool) {
	none := New(ids.ULID{}, "", "", "")
	if scope == ScopeWorld {
		declared, ok := c.world.Value(constant)
		if !ok || declared.Scope != ScopeWorld || !declared.IsConstant() {
			return none, NoValue(), false
		}

		return declared, declared.Zero(), true
	}
	held, known := c.owners[owner]
	if !known || held.Scope != scope {
		return none, NoValue(), false
	}
	if declared, ok := held.Owned.Values.Value(constant); ok {
		return declared, declared.Zero(), declared.IsConstant()
	}
	declared, ok := c.world.Value(constant)
	template, inTemplate := c.world.Template(constant)
	if !ok || !inTemplate || declared.Scope != scope || !declared.IsConstant() ||
		!slices.Contains(held.Owned.Templates, template) {
		return none, NoValue(), false
	}
	if own, has := held.Owned.ConstantValue(declared); has {
		return declared, own, true
	}

	return declared, declared.Zero(), true
}

// Holders lists the NPCs or zones a template is applied to, in path order.
func (c *Catalog) Holders(template ids.ULID) []ids.ULID {
	declared, ok := c.world.Value(template)
	if !ok || declared.Type != TypeTemplate {
		return nil
	}
	var holders []ids.ULID
	for _, id := range c.Owners(declared.Scope) {
		if slices.Contains(c.owners[id].Owned.Templates, template) {
			holders = append(holders, id)
		}
	}

	return holders
}

// Templates lists every value in a scope's templates, template by template
// in name order: what an NPC or zone can be given.
func (c *Catalog) Templates(scope Scope) []Held {
	var values []Held
	for _, declared := range c.world.Children(scope, ids.ULID{}) {
		if declared.Type == TypeTemplate {
			values = walk(c.world, scope, declared.ID, values, Declaration.IsValue)
		}
	}

	return values
}

// walk adds the values or constants under a parent that keep says to,
// depth first in name order.
func walk(list *Declarations, scope Scope, parent ids.ULID, values []Held, keep func(Declaration) bool) []Held {
	for _, declared := range list.Children(scope, parent) {
		switch {
		case !declared.Type.IsValue():
			values = walk(list, scope, declared.ID, values, keep)
		case keep(declared):
			values = append(values, Held{Declaration: declared, Path: list.Path(declared.ID)})
		}
	}

	return values
}

// Prefixes say, wherever values and constants mix, which one a path names:
// on nodes, in pickers, in a message's text and in logs. Inside their own
// lists, paths go without them.
const (
	ValuePrefix    = "var."
	ConstantPrefix = "const."
)

// Prefixed returns how a declaration's path reads where values and constants
// mix: a constant's after const., a world, zone or NPC value's after var.,
// built-in ones too. A workflow's own local value keeps its local., as no
// constant is local; a value of an NPC a local holds reads as that NPC's,
// after var. or const. like any other.
func Prefixed(path string, declared Declaration) string {
	switch {
	case declared.Scope == ScopeLocal:
		return path
	case declared.IsConstant():
		return ConstantPrefix + path
	}

	return ValuePrefix + path
}

// Unprefixed returns a path without its const. or var., reporting whether
// it named a constant; a path without either is returned as it is.
func Unprefixed(path string) (string, bool) {
	if rest, constant := strings.CutPrefix(path, ConstantPrefix); constant {
		return rest, true
	}

	return strings.TrimPrefix(path, ValuePrefix), false
}

// NamedPath is how a workflow names another NPC's or zone's value: the path
// it is named by for its own, such as npc.guards.how_many_arrested, with
// the owner's path name after the scope, as npc.guard1.guards.how_many_arrested.
func NamedPath(path, owner string) string {
	scope, rest, found := strings.Cut(path, ".")
	if !found {
		return path
	}

	return scope + "." + owner + "." + rest
}
