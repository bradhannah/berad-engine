// Package valuetree reads and changes a project's declared values for the
// windows that show them: the World state window, and the Workflow window's
// value choosers and local values. It builds the catalog of every value from
// the documents a window holds, and turns a dotted name, such as
// quests.dragon_slain, into the change that declares it, or moves a
// declaration there, with whatever groups the name needs.
package valuetree

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// ErrName reports a dotted name that cannot be declared or moved to.
var ErrName = errors.New("cannot use that name")

// List is one list of declarations a window can change: the document that
// holds it and the edit that changes it.
type List struct {
	Document ids.ULID
	Edit     string
	Values   *worldstate.Declarations
}

// Documents lists what a catalog is built from, for a window to keep
// loaded: the world state and every NPC and zone.
func Documents(docs frame.Docs) []ids.ULID {
	entries := owners(docs)
	documents := make([]ids.ULID, 0, len(entries)+1)
	documents = append(documents, worldstate.ID())
	for _, entry := range entries {
		documents = append(documents, entry.ID)
	}

	return documents
}

// owners lists the project's NPCs and zones, NPCs first.
func owners(docs frame.Docs) []project.Entry {
	var entries []project.Entry
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = append(index.OfFormat(npc.FormatName), index.OfFormat(zone.FormatName)...)
		}
	})

	return entries
}

// World returns the world state's list, once it has arrived.
func World(docs frame.Docs) (List, bool) {
	list := List{Document: worldstate.ID(), Edit: stateplugin.ChangeEdit, Values: nil}
	docs.View(worldstate.ID(), func(document client.Document) {
		if stateDoc, ok := document.Model.(*stateplugin.Document); ok {
			list.Values = stateDoc.State.Values.Clone()
		}
	})

	return list, list.Values != nil
}

// Catalog builds the catalog of a project's values from the documents a
// window holds: the world state, and each NPC and zone loaded so far. It is
// not ready until the world state has arrived.
func Catalog(docs frame.Docs) (*worldstate.Catalog, bool) {
	world, ok := World(docs)
	if !ok {
		return nil, false
	}
	held := map[ids.ULID]worldstate.Owner{}
	for _, entry := range owners(docs) {
		if owner, found := ownerOf(docs, entry); found {
			held[entry.ID] = owner
		}
	}

	return worldstate.NewCatalog(world.Values, held), true
}

// ownerOf reads an NPC or zone as values see it.
func ownerOf(docs frame.Docs, entry project.Entry) (worldstate.Owner, bool) {
	var owner worldstate.Owner
	found := false
	docs.View(entry.ID, func(document client.Document) {
		switch held := document.Model.(type) {
		case *npcplugin.Document:
			owner = worldstate.Owner{Scope: worldstate.ScopeNPC, Path: held.NPC.Path, Name: entry.Name, Owned: held.NPC.Owned}
			found = true
		case *zoneplugin.Document:
			owner = worldstate.Owner{
				Scope: worldstate.ScopeZone, Path: held.Zone.Path, Name: entry.Name, Owned: held.Zone.Owned,
			}
			found = true
		}
	})

	return owner, found
}

// Owned returns an NPC's or zone's own list, once it has arrived.
func Owned(docs frame.Docs, owner ids.ULID) (List, bool) {
	list := List{Document: owner, Edit: "", Values: nil}
	docs.View(owner, func(document client.Document) {
		switch held := document.Model.(type) {
		case *npcplugin.Document:
			list.Edit, list.Values = npcplugin.ChangeValuesEdit, held.NPC.Owned.Values.Clone()
		case *zoneplugin.Document:
			list.Edit, list.Values = zoneplugin.ChangeValuesEdit, held.Zone.Owned.Values.Clone()
		}
	})

	return list, list.Values != nil
}

// Declare returns the change that declares a value, group or template by a
// dotted name under a parent, zero for the top of a scope: the groups the
// name needs that the list lacks, then the declaration, last, so a
// prepared change's Created is its ID.
func Declare(
	list *worldstate.Declarations, scope worldstate.Scope, parent ids.ULID, dotted string, kind worldstate.Type,
) (stateplugin.ChangeArgs, error) {
	change := stateplugin.ChangeArgs{Add: nil, Set: nil, Remove: nil}
	groups, name, err := split(dotted)
	if err != nil {
		return change, err
	}
	parent, change.Add = within(list, scope, parent, groups)
	if _, taken := list.Find(scope, parent, name); taken && !isTemp(change.Add, parent) {
		return change, fmt.Errorf("%w: %s is already declared there", ErrName, name)
	}
	declared := worldstate.New(stateplugin.TempID(len(change.Add)+1), name, scope, kind)
	declared.Parent = parent
	change.Add = append(change.Add, declared)

	return change, nil
}

// Move returns the change that renames a declaration, and moves it into
// the groups a dotted name gives, under root: zero for the top of its
// scope, or its template. Groups the name needs that the list lacks are
// declared with it.
func Move(list *worldstate.Declarations, declared worldstate.Declaration, root ids.ULID, dotted string) (
	stateplugin.ChangeArgs, error,
) {
	change := stateplugin.ChangeArgs{Add: nil, Set: nil, Remove: nil}
	groups, name, err := split(dotted)
	if err != nil {
		return change, err
	}
	parent, added := within(list, declared.Scope, root, groups)
	if parent == declared.ID || slices.Contains(list.Within(declared.ID), parent) {
		return change, fmt.Errorf("%w: a group cannot go inside itself", ErrName)
	}
	if found, taken := list.Find(declared.Scope, parent, name); taken && found.ID != declared.ID {
		return change, fmt.Errorf("%w: %s is already declared there", ErrName, name)
	}
	moved := declared.Clone()
	moved.Name, moved.Parent = name, parent
	change.Add, change.Set = added, []worldstate.Declaration{moved}

	return change, nil
}

// split splits a dotted name into its groups and its own name.
func split(dotted string) ([]string, string, error) {
	names, err := worldstate.SplitName(dotted)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrName, err)
	}

	return names[:len(names)-1], names[len(names)-1], nil
}

// within finds the group a path of group names leads to under a parent,
// and the groups to declare where none has that name, with temporary IDs.
// A name another value already has cannot be a group, so a new group of
// that name is not made; the change then fails as the name is taken.
func within(
	list *worldstate.Declarations, scope worldstate.Scope, parent ids.ULID, groups []string,
) (ids.ULID, []worldstate.Declaration) {
	var added []worldstate.Declaration
	for _, name := range groups {
		if len(added) == 0 {
			if found, ok := list.Find(scope, parent, name); ok && !found.Type.IsValue() {
				parent = found.ID

				continue
			}
		}
		group := worldstate.New(stateplugin.TempID(len(added)+1), name, scope, worldstate.TypeGroup)
		group.Parent = parent
		added = append(added, group)
		parent = group.ID
	}

	return parent, added
}

func isTemp(added []worldstate.Declaration, id ids.ULID) bool {
	return slices.ContainsFunc(added, func(declared worldstate.Declaration) bool { return declared.ID == id })
}

// Clash says why an NPC or zone cannot have a top-level name of its own and
// a template of that name at once, or "": both would give one path two
// values. It checks a name for its own values against its templates.
func Clash(catalog *worldstate.Catalog, owner ids.ULID, name string) string {
	held, known := catalog.Owner(owner)
	if !known {
		return ""
	}
	for _, template := range held.Owned.Templates {
		if declared, found := catalog.World().Value(template); found && declared.Name == name {
			return held.Name + " has the " + name + " template, so its own values cannot be called " + name
		}
	}

	return ""
}

// Owns says why an NPC or zone cannot have a template applied, or "": it
// has its own top-level value or group of that name.
func Owns(catalog *worldstate.Catalog, owner ids.ULID, template worldstate.Declaration) string {
	held, known := catalog.Owner(owner)
	if !known {
		return ""
	}
	if _, taken := held.Owned.Values.Find(held.Scope, ids.ULID{}, template.Name); taken {
		return held.Name + " has its own " + template.Name + "; rename it before applying the " + template.Name +
			" template"
	}

	return ""
}

// Root returns where a declaration's dotted name starts: its template, or
// the top of its scope.
func Root(list *worldstate.Declarations, declared ids.ULID) ids.ULID {
	template, ok := list.Template(declared)
	if !ok || template == declared {
		return ids.ULID{}
	}

	return template
}

// Dotted returns a declaration's name from its root, such as
// quests.dragon_slain, as a window shows it for editing.
func Dotted(list *worldstate.Declarations, declared ids.ULID) string {
	names := list.Names(declared)
	if root := Root(list, declared); !root.IsZero() && len(names) > 1 {
		names = names[1:]
	}

	return strings.Join(names, ".")
}

// Removal returns what deleting a declaration takes with it: itself and
// everything inside it, and how many of those are values.
func Removal(list *worldstate.Declarations, declared ids.ULID) ([]ids.ULID, int) {
	removing := append(list.Within(declared), declared)
	values := 0
	for _, id := range removing {
		if held, ok := list.Value(id); ok && held.Type.IsValue() {
			values++
		}
	}

	return removing, values
}
