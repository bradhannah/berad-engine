package stateview

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Row ID prefixes: a scope's section, its templates, a declaration, an NPC
// or zone, a template applied to one, a template's value as one holds it,
// and an NPC's name.
const (
	sectionPrefix   = "section:"
	templatesPrefix = "templates:"
	declaredPrefix  = "declared:"
	ownerPrefix     = "owner:"
	appliedPrefix   = "applied:"
	heldPrefix      = "held:"
	namePrefix      = "name:"
	shortID         = 6
	// heldDepth is how deep a template's values sit under an NPC or zone.
	heldDepth = 2
)

// itemKind is what a row of the tree is.
type itemKind int

const (
	// itemDeclared is a value, group or template the window changes.
	itemDeclared itemKind = iota
	// itemTemplates is a scope's templates.
	itemTemplates
	// itemOwner is an NPC or zone.
	itemOwner
	// itemApplied is a template applied to an NPC or zone.
	itemApplied
	// itemHeld is a template's value as an NPC or zone holds it, shown but
	// changed in its template.
	itemHeld
	// itemName is an NPC's name.
	itemName
	// itemSignal is a signal workflows send and answer.
	itemSignal
	// itemSignalGroup is a group of signals.
	itemSignalGroup
)

// item is one row's meaning: what it is, the list holding it, and the path
// input scripts locate it by, such as world.quests.dragon_slain,
// npc.guards.how_many_arrested or npc.george.secret.
type item struct {
	kind     itemKind
	scope    worldstate.Scope
	list     valuetree.List
	declared worldstate.Declaration
	owner    worldstate.Owner
	ownerID  ids.ULID
	path     string
}

// tree is the window's rows and what each means, and the signals and
// groups it shows.
type tree struct {
	rows    []ui.Row
	items   map[string]item
	catalog *worldstate.Catalog
	signals []worldstate.Signal
}

// build makes the tree from the documents the window holds: the world's
// values, then each scope's templates and NPCs or zones with their own
// values, their templates and, for an NPC, its name.
func (s *State) build(docs frame.Docs) (tree, bool) {
	catalog, ready := valuetree.Catalog(docs)
	world, loaded := valuetree.World(docs)
	if !ready || !loaded {
		return tree{rows: nil, items: nil, catalog: nil, signals: nil}, false
	}
	built := tree{rows: nil, items: map[string]item{}, catalog: catalog, signals: nil}
	for _, scope := range worldstate.Scopes() {
		section := sectionPrefix + string(scope)
		row := ui.NewHeading(section, scope.Label())
		row.Section, row.Toggle = true, s.disclosure(section)
		built.rows = append(built.rows, row)
		if !s.open(section) {
			continue
		}
		if scope == worldstate.ScopeWorld {
			s.declaredRows(&built, world, scope, ids.ULID{}, 0, nil)

			continue
		}
		s.templateRows(&built, world, scope)
		for _, owner := range catalog.Owners(scope) {
			s.ownerRows(&built, docs, world, owner)
		}
	}
	s.signalRows(&built, docs)

	return built, true
}

// open reports whether a row shows what it holds: an NPC or zone once
// opened, as a project has many, and anything else until closed.
func (s *State) open(row string) bool {
	if strings.HasPrefix(row, ownerPrefix) {
		return s.toggled[row]
	}

	return !s.toggled[row]
}

// disclosure is a row's triangle.
func (s *State) disclosure(row string) ui.Disclosure {
	if s.open(row) {
		return ui.DisclosureOpen
	}

	return ui.DisclosureClosed
}

// add adds a row and what it means, drawn as part of the tree so every
// row lines up under the one holding it.
func (t *tree) add(row ui.Row, meaning item) {
	row.Tree = true
	t.rows = append(t.rows, row)
	t.items[row.ID] = meaning
}

// templateRows lists a scope's templates and what each holds.
func (s *State) templateRows(built *tree, world valuetree.List, scope worldstate.Scope) {
	rowID := templatesPrefix + string(scope)
	row := ui.NewRow(rowID, "Templates")
	row.Icon = ui.IconTemplate
	row.Strong, row.Indent, row.Toggle = true, 0, s.disclosure(rowID)
	built.add(row, item{
		kind: itemTemplates, scope: scope, list: world, declared: worldstate.New(ids.ULID{}, "", "", ""),
		owner: noOwner(), ownerID: ids.ULID{}, path: rowID,
	})
	if s.open(rowID) {
		s.declaredRows(built, world, scope, ids.ULID{}, 1, nil)
	}
}

// ownerRows lists an NPC or zone: its own values, an NPC's name, and the
// templates applied to it with what each gives it.
func (s *State) ownerRows(built *tree, docs frame.Docs, world valuetree.List, ownerID ids.ULID) {
	owner, _ := built.catalog.Owner(ownerID)
	own, loaded := valuetree.Owned(docs, ownerID)
	rowID := ownerPrefix + ownerID.String()
	row := ui.NewRow(rowID, owner.Name)
	row.Icon, row.Strong, row.Toggle = ui.IconNPC, true, s.disclosure(rowID)
	if owner.Scope == worldstate.ScopeZone {
		row.Icon = ui.IconZone
	}
	row.Detail = ownerPath(owner, ownerID)
	built.add(row, item{
		kind: itemOwner, scope: owner.Scope, list: own, declared: worldstate.New(ids.ULID{}, "", "", ""), owner: owner,
		ownerID: ownerID, path: ownerPath(owner, ownerID),
	})
	if !s.open(rowID) || !loaded {
		return
	}
	if owner.Scope == worldstate.ScopeNPC {
		name := worldstate.NameDeclaration(owner.Name)
		nameRow := ui.NewRow(namePrefix+ownerID.String(), "name")
		nameRow.Indent, nameRow.Detail = 1, detail(name, built.catalog)
		built.add(nameRow, item{
			kind: itemName, scope: owner.Scope, list: own, declared: name, owner: owner, ownerID: ownerID,
			path: ownerPath(owner, ownerID) + ".name",
		})
	}
	s.declaredRows(built, own, owner.Scope, ids.ULID{}, 1, &ownerRef{id: ownerID, owner: owner})
	for _, template := range owner.Owned.Templates {
		s.appliedRows(built, world, ownerID, owner, template)
	}
}

// ownerRef is the NPC or zone a list of rows belongs to.
type ownerRef struct {
	id    ids.ULID
	owner worldstate.Owner
}

// appliedRows lists a template applied to an NPC or zone, and what it gives
// it, which is changed in the template.
func (s *State) appliedRows(
	built *tree, world valuetree.List, ownerID ids.ULID, owner worldstate.Owner, template ids.ULID,
) {
	declared, found := world.Values.Value(template)
	if !found {
		return
	}
	rowID := appliedPrefix + ownerID.String() + ":" + template.String()
	row := ui.NewRow(rowID, declared.Name)
	row.Indent, row.Aside, row.Toggle, row.Icon = 1, "template", s.disclosure(rowID), ui.IconTemplate
	built.add(row, item{
		kind: itemApplied, scope: owner.Scope, list: world, declared: declared, owner: owner, ownerID: ownerID,
		path: ownerPath(owner, ownerID) + "." + declared.Name,
	})
	if !s.open(rowID) {
		return
	}
	s.heldRows(built, world, template, heldDepth, ownerID, owner)
}

// heldRows lists what a template gives an NPC or zone, read only.
func (s *State) heldRows(
	built *tree, world valuetree.List, parent ids.ULID, depth int, ownerID ids.ULID, owner worldstate.Owner,
) {
	for _, declared := range world.Values.Children(owner.Scope, parent) {
		row := ui.NewRow(heldPrefix+ownerID.String()+":"+declared.ID.String(), declared.Name)
		row.Indent, row.Detail = depth, detail(declared, built.catalog)
		row.Strong = !declared.Type.IsValue()
		row.Icon = declaredIcon(declared.Type)
		built.add(row, item{
			kind: itemHeld, scope: owner.Scope, list: world, declared: declared, owner: owner, ownerID: ownerID,
			path: worldstate.NamedPath(world.Values.Path(declared.ID), ownerName(owner, ownerID)),
		})
		if !declared.Type.IsValue() {
			s.heldRows(built, world, declared.ID, depth+1, ownerID, owner)
		}
	}
}

// declaredRows lists what a list declares under a parent, depth first:
// the world's values, a scope's templates, or an NPC's or zone's own.
func (s *State) declaredRows(
	built *tree, list valuetree.List, scope worldstate.Scope, parent ids.ULID, depth int, owner *ownerRef,
) {
	for _, declared := range list.Values.Children(scope, parent) {
		rowID := declaredPrefix + declared.ID.String()
		row := ui.NewRow(rowID, declared.Name)
		row.Indent, row.Detail = depth, detail(declared, built.catalog)
		path := list.Values.Path(declared.ID)
		meaning := item{
			kind: itemDeclared, scope: scope, list: list, declared: declared, owner: noOwner(), ownerID: ids.ULID{},
			path: path,
		}
		if owner != nil {
			meaning.owner, meaning.ownerID = owner.owner, owner.id
			meaning.path = worldstate.NamedPath(path, ownerName(owner.owner, owner.id))
		}
		row.Icon = declaredIcon(declared.Type)
		if !declared.Type.IsValue() {
			row.Strong = true
			if len(list.Values.Children(scope, declared.ID)) > 0 {
				row.Toggle = s.disclosure(rowID)
			}
		}
		built.add(row, meaning)
		if !declared.Type.IsValue() && s.open(rowID) {
			s.declaredRows(built, list, scope, declared.ID, depth+1, owner)
		}
	}
}

// declaredIcon is a group's icon or a template's; a value has none.
func declaredIcon(kind worldstate.Type) ui.Icon {
	switch kind {
	case worldstate.TypeGroup:
		return ui.IconFolder
	case worldstate.TypeTemplate:
		return ui.IconTemplate
	case worldstate.TypeInteger, worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeText,
		worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeZone, worldstate.TypeWorkflow, worldstate.TypeSprite:
	}

	return ui.IconNone
}

// ownerPath is how an NPC or zone is named in a path, such as npc.george.
func ownerPath(owner worldstate.Owner, id ids.ULID) string {
	return string(owner.Scope) + "." + ownerName(owner, id)
}

// ownerName is an NPC's or zone's path name, or its ID before it has one.
func ownerName(owner worldstate.Owner, id ids.ULID) string {
	if owner.Path == "" {
		return "#" + id.Short(shortID)
	}

	return owner.Path
}

func noOwner() worldstate.Owner {
	return worldstate.Owner{
		Scope: "", Path: "", Name: "", Owned: worldstate.NewOwned(worldstate.ScopeNPC), Zone: ids.ULID{},
		Behaviour: ids.ULID{}, Sprite: ids.ULID{},
	}
}

// detail says what a declaration is: a value's type, limits and default, a
// group, or a template and how many it is applied to.
func detail(declared worldstate.Declaration, catalog *worldstate.Catalog) string {
	switch declared.Type {
	case worldstate.TypeGroup:
		return newGroupName
	case worldstate.TypeTemplate:
		applied := 0
		for _, owner := range catalog.Owners(declared.Scope) {
			held, _ := catalog.Owner(owner)
			for _, template := range held.Owned.Templates {
				if template == declared.ID {
					applied++
				}
			}
		}

		return "template · on " + strconv.Itoa(applied)
	case worldstate.TypeInteger, worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeText, worldstate.TypeNPC,
		worldstate.TypePlace,
		worldstate.TypeZone, worldstate.TypeWorkflow, worldstate.TypeSprite:
	}
	written := declared.Type.Label()
	if declared.Min != nil || declared.Max != nil {
		written += " " + limits(declared)
	}

	return written + " · " + declared.Label(declared.Zero())
}

// limits shows an integer's limits, such as 0 to 100.
func limits(declared worldstate.Declaration) string {
	low, high := "…", "…"
	if declared.Min != nil {
		low = strconv.Itoa(*declared.Min)
	}
	if declared.Max != nil {
		high = strconv.Itoa(*declared.Max)
	}

	return low + " to " + high
}
