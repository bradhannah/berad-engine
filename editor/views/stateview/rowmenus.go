package stateview

import (
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// What a row's right-click menu adds inside it, located as "menu:<id>":
// Add value, whose submenu lists the types as "add:<type>"; Add group; Add
// template on a zone's or NPC's heading or Templates row; and Apply
// template, whose submenu lists the templates not yet applied as
// "apply:<name>".
const (
	menuAddValue    = "add-value"
	menuAdd         = "add:"
	menuAddGroup    = "add-group"
	menuAddTemplate = "add-template"
	menuApply       = "apply-template"
	menuApplyOne    = "apply:"
)

// menuHeading is a menu's first line, saying where what it adds goes.
func menuHeading(label string) ui.MenuItem {
	return ui.MenuItem{ID: "", Label: label, Icon: ui.IconNone, Disabled: true, Danger: false, Rule: false}
}

// choice is a menu item; rule draws a line above it, to start a group.
func choice(id, label string, rule bool) ui.MenuItem {
	return ui.MenuItem{ID: id, Label: label, Icon: ui.IconNone, Disabled: false, Danger: false, Rule: rule}
}

// rowMenu is a row's menu and the submenus its items open: a group's, a
// template's, an NPC's or zone's or the World heading's adds inside it; a
// value's and a group's moves; a zone's or NPC's heading or Templates row
// adds a template. Rows the window does not change have none.
func (s *State) rowMenu(row string, meaning item, found bool) ([]ui.MenuItem, map[string][]ui.MenuItem) {
	subs := map[string][]ui.MenuItem{}
	adds := func(where string, scope worldstate.Scope) []ui.MenuItem {
		types := make([]ui.MenuItem, 0, len(worldstate.Types(scope)))
		for _, kind := range worldstate.Types(scope) {
			types = append(types, choice(menuAdd+string(kind), kind.Label(), false))
		}
		subs[menuAddValue] = types

		return []ui.MenuItem{
			menuHeading(where), choice(menuAddValue, "Add value", false), choice(menuAddGroup, "Add group", false),
		}
	}
	if scope, section := strings.CutPrefix(row, sectionPrefix); section {
		if worldstate.Scope(scope) == worldstate.ScopeWorld {
			return adds("At the top of World", worldstate.ScopeWorld), subs
		}

		return []ui.MenuItem{choice(menuAddTemplate, "Add template", false)}, subs
	}
	if !found {
		return nil, subs
	}
	switch meaning.kind {
	case itemTemplates:
		return []ui.MenuItem{choice(menuAddTemplate, "Add template", false)}, subs
	case itemOwner:
		items := append(adds(meaning.owner.Name+"'s own", meaning.scope), choice(menuApply, "Apply template", true))
		var templates []ui.MenuItem
		for _, template := range s.templatesOf(meaning.scope) {
			if !slices.Contains(meaning.owner.Owned.Templates, template.ID) {
				templates = append(templates, choice(menuApplyOne+template.Name, template.Name, false))
			}
		}
		if len(templates) == 0 {
			items[len(items)-1].Disabled = true
		} else {
			subs[menuApply] = templates
		}

		return items, subs
	case itemDeclared:
		return s.declaredMenu(meaning, adds), subs
	case itemApplied, itemHeld, itemName, itemSignal:
	}

	return nil, subs
}

// declaredMenu is a value's, group's or template's menu: adds inside a group
// or template, Move into for a value or group, Rename and Delete.
func (s *State) declaredMenu(meaning item, adds func(string, worldstate.Scope) []ui.MenuItem) []ui.MenuItem {
	kind := meaning.declared.Type
	var items []ui.MenuItem
	switch kind {
	case worldstate.TypeGroup:
		items = adds("In "+meaning.declared.Name, meaning.scope)
	case worldstate.TypeTemplate:
		items = adds("In the "+meaning.declared.Name+" template", meaning.scope)
	case worldstate.TypeInteger, worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeText, worldstate.TypeNPC,
		worldstate.TypePlace,
		worldstate.TypeZone, worldstate.TypeWorkflow, worldstate.TypeSprite:
	}
	if kind != worldstate.TypeTemplate {
		items = append(items, choice(menuMoveInto, "Move into", len(items) > 0))
	}
	items = append(items, choice(menuRename, "Rename", kind == worldstate.TypeTemplate))
	remove := choice(menuDelete, "Delete "+kindName(kind), true)
	remove.Danger = true

	return append(items, remove)
}

// openRowMenu opens a row's menu at the pointer, with its submenus.
func (s *State) openRowMenu(input *ui.Input, row string, meaning item, found bool) {
	items, subs := s.rowMenu(row, meaning, found)
	if len(items) == 0 {
		return
	}
	s.menuOf = row
	s.menu.ClearSubs()
	s.menu.Items, s.menu.Width = items, ui.MenuWidth(items, s.theme)
	if slices.ContainsFunc(items, func(each ui.MenuItem) bool { return each.ID == menuMoveInto }) {
		into := s.intoItems(meaning)
		s.menu.SetSub(menuMoveInto, into, ui.MenuWidth(into, s.theme))
	}
	for id, sub := range subs {
		s.menu.SetSub(id, sub, ui.MenuWidth(sub, s.theme))
	}
	s.menu.OpenAt(input.Cursor, s.screen())
}

// menuTarget is where the menu's row adds: inside its group or template,
// among its NPC's or zone's own values, or at the top of the world.
func (s *State) menuTarget(docs frame.Docs) (target, bool) {
	if scope, section := strings.CutPrefix(s.menuOf, sectionPrefix); section {
		world, loaded := valuetree.World(docs)

		return target{list: world, scope: worldstate.Scope(scope), parent: ids.ULID{}}, loaded
	}
	meaning, found := s.shown.items[s.menuOf]
	switch {
	case !found:
		return target{list: valuetree.List{Document: ids.ULID{}, Edit: "", Values: nil}, scope: "", parent: ids.ULID{}},
			false
	case meaning.kind == itemOwner:
		own, loaded := valuetree.Owned(docs, meaning.ownerID)

		return target{list: own, scope: meaning.scope, parent: ids.ULID{}}, loaded
	case meaning.kind == itemTemplates:
		world, loaded := valuetree.World(docs)

		return target{list: world, scope: meaning.scope, parent: ids.ULID{}}, loaded
	}

	return target{list: meaning.list, scope: meaning.scope, parent: meaning.declared.ID}, true
}

// chooseAdd carries out an add or an apply from a row's menu, and reports
// whether the choice was one.
func (s *State) chooseAdd(ctx *frame.Context, chosen string) bool {
	into, ok := s.menuTarget(ctx.Docs)
	switch {
	case !ok:
		return strings.HasPrefix(chosen, menuAdd) || chosen == menuAddGroup || chosen == menuAddTemplate
	case strings.HasPrefix(chosen, menuAdd):
		s.declare(ctx, into, "value", worldstate.Type(strings.TrimPrefix(chosen, menuAdd)))
	case chosen == menuAddGroup:
		s.declare(ctx, into, "group", worldstate.TypeGroup)
	case chosen == menuAddTemplate:
		into.parent = ids.ULID{}
		s.declare(ctx, into, "template", worldstate.TypeTemplate)
	case strings.HasPrefix(chosen, menuApplyOne):
		owner := s.shown.items[s.menuOf]
		for _, template := range s.templatesOf(owner.scope) {
			if template.Name == strings.TrimPrefix(chosen, menuApplyOne) {
				s.toggleTemplate(ctx, owner, template)
			}
		}
	default:
		return false
	}

	return true
}

// addsWhere says where a target adds: inside a group or template, among an
// NPC's or zone's own, or at the top of the world.
func (s *State) addsWhere(into target) string {
	if !into.parent.IsZero() {
		parent, _ := into.list.Values.Value(into.parent)
		if parent.Type == worldstate.TypeTemplate {
			return "in the " + parent.Name + " template"
		}

		return "in " + parent.Name
	}
	if into.list.Document != worldstate.ID() {
		owner, _ := s.shown.catalog.Owner(into.list.Document)

		return "to " + owner.Name + "'s own"
	}

	return "at the top of " + into.scope.Label()
}

// addsNote says where Add value and Add group add, beside them, or "" when
// they cannot.
func (s *State) addsNote(docs frame.Docs) string {
	into, ok := s.addTarget(docs)
	if !ok || s.shown.catalog == nil {
		return ""
	}

	return "adds " + s.addsWhere(into)
}

// kindPhrase says what was added, as a bool or a group.
func kindPhrase(kind worldstate.Type) string {
	switch kind {
	case worldstate.TypeInteger:
		return "an integer"
	case worldstate.TypeGroup, worldstate.TypeTemplate:
		return "a " + kindName(kind)
	case worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeText, worldstate.TypeNPC, worldstate.TypePlace,
		worldstate.TypeZone, worldstate.TypeWorkflow, worldstate.TypeSprite:
	}

	return "a " + kind.Label()
}
