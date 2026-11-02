package stateview

import (
	"encoding/json"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuepicker"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Find a value opens the value picker over the window, by its button or
// Cmd+F, offering every value the window holds: the world's, each
// template's and each NPC's and zone's own, with how many places use each.
// Picking one shows it in the tree; a name nothing has is declared in the
// world, a template, an NPC or a zone, and shown.
const (
	findID    = "find-value"
	findWidth = 140
	findState = "find"
	// The rail's scopes beyond the world's.
	railZones         = "zones"
	railZoneTemplates = "zone-templates"
	railNPCs          = "npcs"
	railNPCTemplates  = "npc-templates"
	setInInspector    = "the inspector"
)

// place is where a declaration sits in the tree, for Show to open what
// holds it: its scope, its NPC or zone or its template, the groups that
// hold it, outermost first, and its path.
type place struct {
	scope    worldstate.Scope
	owner    ids.ULID
	template ids.ULID
	groups   []ids.ULID
	path     string
}

// findSource is the window's side of Find: the places of the values last
// listed, by their rows' IDs, and of the targets last offered, by theirs.
type findSource struct {
	state   *State
	places  map[string]place
	targets map[string]findTarget
}

// findTarget is where a name typed is declared: the list, scope and parent
// it goes in, its dotted name from there, and its place once declared.
type findTarget struct {
	list   valuetree.List
	scope  worldstate.Scope
	parent ids.ULID
	dotted string
	at     place
}

// findRail is the scopes Find narrows to: every value the window holds,
// and no workflow's locals.
func findRail() []valuepicker.RailEntry {
	return []valuepicker.RailEntry{
		{ID: valuepicker.Everywhere, Label: "Everywhere"}, {ID: string(worldstate.ScopeWorld), Label: "World"},
		{ID: railZones, Label: "Zones"}, {ID: railZoneTemplates, Label: "Zone templates"},
		{ID: railNPCs, Label: "NPCs"}, {ID: railNPCTemplates, Label: "NPC templates"},
	}
}

// openFind opens Find with a search typed, asking how many places use each
// value.
func (s *State) openFind(ctx *frame.Context, query string) {
	s.findUses = nil
	ctx.Requests.WhereUsed(worldstate.ID(), "", func(uses []protocol.Use, err error) {
		if err != nil {
			return
		}
		counts := map[string]int{}
		for _, use := range uses {
			counts[use.Item]++
		}
		s.findUses = counts
	})
	source := &findSource{state: s, places: nil, targets: nil}
	s.find = valuepicker.New(valuepicker.Config{
		Title: "Find a value", For: "in World state", Placeholder: "Find a value, or type a new name",
		Empty: "No value matches. Type a name to declare one.", Only: "", Rail: findRail(), Selected: "",
		Query:     query,
		LastGroup: &s.lastGroup, PickLabel: "Show", PickHint: "shows it", DeclareAs: "Declare and show",
		DeclareLead: "It is shown in the tree once declared.",
	}, source, s.theme)
}

// walk hands on the values under a parent, depth first, with the groups
// that hold each.
func walk(list *worldstate.Declarations, scope worldstate.Scope, parent ids.ULID, groups []ids.ULID,
	each func(worldstate.Declaration, []ids.ULID),
) {
	for _, declared := range list.Children(scope, parent) {
		switch {
		case declared.Type.IsValue():
			each(declared, groups)
		case declared.Type == worldstate.TypeGroup:
			walk(list, scope, declared.ID, append(slices.Clone(groups), declared.ID), each)
		}
	}
}

// Options lists every value the window holds, in the tree's order: the
// world's, then for zones and NPCs each one's own and each template's.
func (f *findSource) Options(docs frame.Docs) []valuepicker.Option {
	f.places = map[string]place{}
	catalog, ready := valuetree.Catalog(docs)
	world, loaded := valuetree.World(docs)
	if !ready || !loaded {
		return nil
	}
	var listed []valuepicker.Option
	add := func(declared worldstate.Declaration, at place, heading, rail, root string) {
		rowID := declaredPrefix + declared.ID.String()
		f.places[rowID] = at
		listed = append(listed, valuepicker.Option{
			ID: rowID, Path: at.path, Declared: declared, Heading: heading, Rail: rail, Root: root,
			Detail: f.state.usedBy(declared.ID),
		})
	}
	walk(world.Values, worldstate.ScopeWorld, ids.ULID{}, nil, func(declared worldstate.Declaration, groups []ids.ULID) {
		at := place{scope: worldstate.ScopeWorld, owner: ids.ULID{}, template: ids.ULID{}, groups: groups,
			path: world.Values.Path(declared.ID)}
		add(declared, at, "WORLD", string(worldstate.ScopeWorld), string(worldstate.ScopeWorld))
	})
	for _, scope := range []worldstate.Scope{worldstate.ScopeZone, worldstate.ScopeNPC} {
		owners, templates := railZones, railZoneTemplates
		if scope == worldstate.ScopeNPC {
			owners, templates = railNPCs, railNPCTemplates
		}
		for _, ownerID := range catalog.Owners(scope) {
			owner, _ := catalog.Owner(ownerID)
			own, held := valuetree.Owned(docs, ownerID)
			if !held {
				continue
			}
			walk(own.Values, scope, ids.ULID{}, nil, func(declared worldstate.Declaration, groups []ids.ULID) {
				at := place{scope: scope, owner: ownerID, template: ids.ULID{}, groups: groups,
					path: worldstate.NamedPath(own.Values.Path(declared.ID), ownerName(owner, ownerID))}
				add(declared, at, strings.ToUpper(owner.Name), owners, ownerPath(owner, ownerID))
			})
		}
		for _, template := range world.Values.Children(scope, ids.ULID{}) {
			if template.Type != worldstate.TypeTemplate {
				continue
			}
			root := world.Values.Path(template.ID)
			walk(world.Values, scope, template.ID, nil, func(declared worldstate.Declaration, groups []ids.ULID) {
				at := place{scope: scope, owner: ids.ULID{}, template: template.ID, groups: groups,
					path: world.Values.Path(declared.ID)}
				add(declared, at, strings.ToUpper(scope.Label())+" TEMPLATE "+template.Name, templates, root)
			})
		}
	}

	return listed
}

// usedBy says how many places use a value, once Find has been told.
func (s *State) usedBy(value ids.ULID) string {
	if s.findUses == nil {
		return ""
	}
	count := s.findUses[value.String()]
	switch count {
	case 0:
		return "unused"
	case 1:
		return "used once"
	}

	return "used " + strconv.Itoa(count) + " times"
}

// Targets lists where a name typed can be declared: in the world, in each
// template, and as each NPC's or zone's own. A name starting with a scope,
// as npc., only in that scope; one whose first name names a template or an
// NPC or zone there, as npc.guards.on_duty, only there.
func (f *findSource) Targets(docs frame.Docs, query string) []valuepicker.Target {
	f.targets = map[string]findTarget{}
	catalog, ready := valuetree.Catalog(docs)
	world, loaded := valuetree.World(docs)
	if !ready || !loaded {
		return nil
	}
	scopes, dotted := worldstate.Scopes(), query
	for _, scope := range worldstate.Scopes() {
		if rest, named := strings.CutPrefix(query, string(scope)+"."); named {
			scopes, dotted = []worldstate.Scope{scope}, rest
		}
	}
	var targets []valuepicker.Target
	offer := func(target findTarget, label string) {
		if _, err := worldstate.SplitName(target.dotted); err != nil || f.targets[target.at.path].dotted != "" {
			return
		}
		f.targets[target.at.path] = target
		targets = append(targets, valuepicker.Target{
			ID: target.at.path, Path: target.at.path, Label: label, Heading: target.at.path, SetIn: setInInspector,
		})
	}
	for _, scope := range scopes {
		if scope == worldstate.ScopeWorld {
			offer(findTarget{list: world, scope: scope, parent: ids.ULID{}, dotted: dotted, at: place{
				scope: scope, owner: ids.ULID{}, template: ids.ULID{}, groups: nil, path: string(scope) + "." + dotted,
			}}, scope.Label())

			continue
		}
		for _, target := range f.ownedTargets(docs, catalog, world, scope, dotted) {
			offer(target.target, target.label)
		}
	}

	return targets
}

// labelled is a declare target with its button's label.
type labelled struct {
	target findTarget
	label  string
}

// ownedTargets lists where a zone or NPC value can be declared: the
// template or the zone or NPC its first name names, or else every one.
func (f *findSource) ownedTargets(
	docs frame.Docs, catalog *worldstate.Catalog, world valuetree.List, scope worldstate.Scope, dotted string,
) []labelled {
	first, rest, _ := strings.Cut(dotted, ".")
	var all, named []labelled
	for _, template := range world.Values.Children(scope, ids.ULID{}) {
		if template.Type != worldstate.TypeTemplate {
			continue
		}
		path := world.Values.Path(template.ID)
		made := func(name string) labelled {
			return labelled{target: findTarget{list: world, scope: scope, parent: template.ID, dotted: name, at: place{
				scope: scope, owner: ids.ULID{}, template: template.ID, groups: nil, path: path + "." + name,
			}}, label: scope.Label() + " template " + template.Name}
		}
		all = append(all, made(dotted))
		if rest != "" && template.Name == first {
			named = append(named, made(rest))
		}
	}
	for _, ownerID := range catalog.Owners(scope) {
		owner, _ := catalog.Owner(ownerID)
		own, held := valuetree.Owned(docs, ownerID)
		if !held {
			continue
		}
		made := func(name string) labelled {
			return labelled{target: findTarget{list: own, scope: scope, parent: ids.ULID{}, dotted: name, at: place{
				scope: scope, owner: ownerID, template: ids.ULID{}, groups: nil, path: ownerPath(owner, ownerID) + "." + name,
			}}, label: owner.Name}
		}
		all = append(all, made(dotted))
		if rest != "" && owner.Path == first {
			named = append(named, made(rest))
		}
	}
	if len(named) > 0 {
		return named
	}

	return all
}

// Pick shows the value picked in the tree.
func (f *findSource) Pick(ctx *frame.Context, option valuepicker.Option) {
	if at, found := f.places[option.ID]; found {
		f.state.showPlace(ctx, at)
	}
}

// Declare declares the value the panel describes, starting where it says,
// with the groups its name needs, then shows it.
func (f *findSource) Declare(ctx *frame.Context, target valuepicker.Target, kind worldstate.Type,
	start json.RawMessage,
) {
	going, found := f.targets[target.ID]
	if !found {
		return
	}
	change, err := valuetree.Declare(going.list.Values, going.scope, going.parent, going.dotted, kind)
	if first, _, _ := strings.Cut(going.dotted, "."); err == nil && !going.at.owner.IsZero() {
		if clash := valuetree.Clash(f.state.shown.catalog, going.at.owner, first); clash != "" {
			ctx.Frame.Status(clash + ".")

			return
		}
	}
	if err != nil {
		ctx.Frame.Status(strings.TrimPrefix(err.Error(), valuetree.ErrName.Error()+": ") + ".")

		return
	}
	if start != nil {
		change.Add[len(change.Add)-1].Default = start
	}
	f.state.change(ctx, going.list, change, "Declared "+going.at.path+". Its limits and description are here.")
	f.state.showPlace(ctx, going.at)
}

// showPlace opens what holds a place in the tree, then selects its row
// once the tree shows it, asking first about typed changes to the row
// being left.
func (s *State) showPlace(ctx *frame.Context, shown place) {
	reveal := func() {
		s.toggled[sectionPrefix+string(shown.scope)] = false
		if !shown.template.IsZero() {
			s.toggled[templatesPrefix+string(shown.scope)] = false
			s.toggled[declaredPrefix+shown.template.String()] = false
		}
		if !shown.owner.IsZero() {
			s.toggled[ownerPrefix+shown.owner.String()] = true
		}
		for _, group := range shown.groups {
			s.toggled[declaredPrefix+group.String()] = false
		}
		s.revealPath, s.revealRename = shown.path, false
	}
	if unsaved := s.Unsaved(ctx.Docs); unsaved > 0 {
		s.focus.Set(nil)
		s.guard.Ask(unsaved, s.UnsavedName(ctx.Docs), reveal)

		return
	}
	reveal()
}

// revealRow selects the row a move or Find is waiting to show, once the
// tree has it, scrolled into view, and for a group a move just made, with
// its name ready to type.
func (s *State) revealRow() {
	if s.revealPath == "" {
		return
	}
	for _, row := range s.list.Rows {
		meaning, found := s.shown.items[row.ID]
		if !found || meaning.path != s.revealPath || meaning.kind != itemDeclared {
			continue
		}
		s.list.Selected, s.revealPath = row.ID, ""
		s.list.Reveal(row.ID)
		s.inspect(meaning)
		if s.revealRename {
			s.nameToType()
		}
		s.revealRename = false

		return
	}
}

// nameNewRow puts the name of a row just added to type, once the tree shows
// it selected.
func (s *State) nameNewRow() {
	if !s.nameAdded || s.named == "" || s.named != s.list.Selected {
		return
	}
	s.nameAdded = false
	s.nameToType()
}

// nameToType focuses the selected declaration's name with its own name
// selected, after the groups its dotted name starts with, so typing
// renames it where it is.
func (s *State) nameToType() {
	name := s.form.Fields()[0]
	s.focus.Set(name)
	name.Field.SelectFrom(strings.LastIndex(name.Field.Value(), ".") + 1)
}

// layoutFind places Find a value at the right of the add buttons.
func (s *State) layoutFind(right int) {
	s.findButton.Rect = image.Rect(right-findWidth, headerTop, right, headerTop+ui.ButtonH)
}
