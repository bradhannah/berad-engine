package stateview

import (
	"image"
	"strconv"
	"strings"

	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The right-click menu on a value, group or template, located as
// "menu:<id>": Move into, whose submenu lists the groups of the row's own
// section as "into:<dotted group>", "into:" for its top level and
// "into-new" for a new group; Rename; and Delete.
const (
	menuLocate   = "menu"
	menuMoveInto = "move-into"
	menuInto     = "into:"
	menuNewGroup = "into-new"
	menuRename   = "rename"
	menuDelete   = "delete"
	newGroupName = "group"
	indentSpaces = "    "
)

// rightClick selects the row a right-click lands on, as a click would, and
// opens its menu when it is a value, group or template the window
// changes. Typed changes to the row being left are asked about first, and
// the menu waits for another right-click.
func (s *State) rightClick(input *ui.Input, ctx *frame.Context, row string) {
	meaning, found := s.shown.items[row]
	if !found || meaning.kind != itemDeclared {
		return
	}
	if row != s.list.Selected {
		if unsaved := s.Unsaved(ctx.Docs); unsaved > 0 {
			s.focus.Set(nil)
			s.guard.Ask(unsaved, s.UnsavedName(ctx.Docs), func() { s.list.Selected = row })

			return
		}
		s.list.Selected = row
	}
	items := s.menuItems(meaning)
	s.menu.ClearSubs()
	s.menu.Items, s.menu.Width = items, ui.MenuWidth(items, s.theme)
	if !items[0].Disabled {
		into := s.intoItems(meaning)
		s.menu.SetSub(menuMoveInto, into, ui.MenuWidth(into, s.theme))
	}
	s.menu.OpenAt(input.Cursor, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
}

// menuItems are a row's choices: Move into, which a template cannot do,
// Rename and Delete.
func (s *State) menuItems(meaning item) []ui.MenuItem {
	kind := meaning.declared.Type

	return []ui.MenuItem{
		{
			ID: menuMoveInto, Label: "Move into", Icon: ui.IconNone, Disabled: kind == worldstate.TypeTemplate,
			Danger: false, Rule: false,
		},
		{ID: menuRename, Label: "Rename", Icon: ui.IconNone, Disabled: false, Danger: false, Rule: false},
		{
			ID: menuDelete, Label: "Delete " + kind.Label(), Icon: ui.IconNone, Disabled: false, Danger: true,
			Rule: true,
		},
	}
}

// intoItems lists where a value or group can move inside its own section:
// its top level, each group there but itself and those inside it, nested
// groups indented, and a new group. Where it is already is shown, not
// offered.
func (s *State) intoItems(meaning item) []ui.MenuItem {
	list, declared := meaning.list.Values, meaning.declared
	root := valuetree.Root(list, declared.ID)
	items := []ui.MenuItem{
		{
			ID: "", Label: "Inside " + sectionName(meaning, list, root), Icon: ui.IconNone, Disabled: true, Danger: false,
			Rule: false,
		},
		{
			ID: menuInto, Label: "Top level", Icon: ui.IconNone, Disabled: declared.Parent == root, Danger: false,
			Rule: false,
		},
	}
	inside := list.Within(declared.ID)
	var walk func(parent ids.ULID, depth int)
	walk = func(parent ids.ULID, depth int) {
		for _, group := range list.Children(declared.Scope, parent) {
			if group.Type != worldstate.TypeGroup || group.ID == declared.ID || containsID(inside, group.ID) {
				continue
			}
			items = append(items, ui.MenuItem{
				ID: menuInto + valuetree.Dotted(list, group.ID), Label: strings.Repeat(indentSpaces, depth) + group.Name,
				Icon: ui.IconFolder, Disabled: declared.Parent == group.ID, Danger: false, Rule: false,
			})
			walk(group.ID, depth+1)
		}
	}
	walk(root, 0)

	return append(items, ui.MenuItem{
		ID: menuNewGroup, Label: "New group…", Icon: ui.IconNone, Disabled: false, Danger: false, Rule: true,
	})
}

// sectionName names where a row's moves stay: the world, a template, or an
// NPC's or zone's own values.
func sectionName(meaning item, list *worldstate.Declarations, root ids.ULID) string {
	switch {
	case !root.IsZero():
		template, _ := list.Value(root)

		return "the " + template.Name + " template"
	case meaning.ownerID.IsZero():
		return meaning.scope.Label()
	}

	return meaning.owner.Name + "'s own"
}

func containsID(within []ids.ULID, id ids.ULID) bool {
	for _, each := range within {
		if each == id {
			return true
		}
	}

	return false
}

// chooseFromMenu carries out a choice from the right-click menu on the
// selected row.
func (s *State) chooseFromMenu(ctx *frame.Context, chosen string) {
	selected, found := s.selected()
	if !found || chosen == "" {
		return
	}
	switch {
	case chosen == menuRename:
		s.focus.Set(s.form.Fields()[0])
	case chosen == menuDelete:
		s.startDelete(ctx, selected)
	case chosen == menuNewGroup:
		s.moveIntoNew(ctx, selected)
	case strings.HasPrefix(chosen, menuInto):
		s.moveInto(ctx, selected, strings.TrimPrefix(chosen, menuInto))
	}
}

// moveIntoNew moves a value or group into a new group at the top of its
// section, then selects the group for its name to be typed.
func (s *State) moveIntoNew(ctx *frame.Context, selected item) {
	list := selected.list.Values
	root := valuetree.Root(list, selected.declared.ID)
	taken := map[string]bool{}
	for _, held := range list.Children(selected.declared.Scope, root) {
		taken[held.Name] = true
	}
	group := worldstate.UniqueName(newGroupName, taken)
	s.renaming = strings.TrimSuffix(selected.path, valuetree.Dotted(list, selected.declared.ID)) + group
	s.moveInto(ctx, selected, group)
}

// moveInto moves a value or group into the group with a dotted name from
// its root, or to its top level for "", as one change, keeping its name;
// the status says where it went, or why it cannot.
func (s *State) moveInto(ctx *frame.Context, selected item, group string) {
	list, declared := selected.list, selected.declared
	dotted := declared.Name
	if group != "" {
		dotted = group + "." + declared.Name
	}
	change, err := valuetree.Move(list.Values, declared, valuetree.Root(list.Values, declared.ID), dotted)
	if err == nil && group == "" && list.Document != worldstate.ID() {
		if clash := valuetree.Clash(s.shown.catalog, list.Document, declared.Name); clash != "" {
			ctx.Frame.Status(clash + ".")
			s.renaming = ""

			return
		}
	}
	if err != nil {
		ctx.Frame.Status(strings.TrimPrefix(err.Error(), valuetree.ErrName.Error()+": ") + ".")
		s.renaming = ""

		return
	}
	moved := strings.TrimSuffix(selected.path, valuetree.Dotted(list.Values, declared.ID)) + dotted
	going := pendingMove{list: list, change: change, success: "Moved to " + moved + ". Cmd+Z undoes it."}
	whereUsed(ctx, selected, func(uses []protocol.Use) {
		if len(uses) == 0 {
			s.change(ctx, going.list, going.change, going.success)

			return
		}
		s.askMove(selected, moved, group, uses, going)
	})
}

// usesLine says how many uses a move would carry along.
func usesLine(count int) string {
	if count == 1 {
		return "1 use follows it."
	}

	return strconv.Itoa(count) + " uses follow it."
}

// pendingMove is a move waiting for its dialog: the list, the change and
// what the status says once it is made.
type pendingMove struct {
	list    valuetree.List
	change  stateplugin.ChangeArgs
	success string
}

// askMove asks before moving what something uses, showing the path before
// and after and listing each use, which follows the value as it names the
// value itself, not its path.
func (s *State) askMove(selected item, moved, group string, uses []protocol.Use, going pendingMove) {
	where := "the top level"
	if group != "" {
		where = group
	}
	lines := []string{
		selected.path + "  →  " + moved, "",
		usesLine(len(uses)) + " A use names the value itself,",
		"not its path, so nothing needs rewriting.",
	}
	for index, use := range uses {
		if index == usesListed {
			lines = append(lines, "  and "+strconv.Itoa(len(uses)-usesListed)+" more")

			break
		}
		lines = append(lines, "  "+use.Name+" · "+use.Label)
	}
	lines = append(lines, "", "Cmd+Z puts it back.")
	s.dialog = ui.NewDialog("Move "+selected.declared.Name+" into "+where+"?", lines,
		ui.DialogButton(moveID, "Move"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.moving = &going
}

// renameNewGroup selects a group just made by a move, once the tree shows
// it, with its name ready to type.
func (s *State) renameNewGroup() {
	if s.renaming == "" {
		return
	}
	for _, row := range s.list.Rows {
		if meaning, found := s.shown.items[row.ID]; found && meaning.path == s.renaming {
			s.list.Selected, s.renaming = row.ID, ""
			if selected, ok := s.selected(); ok {
				s.inspect(selected)
			}
			s.focus.Set(s.form.Fields()[0])

			return
		}
	}
}

// dropInto says where dropping a dragged row on another moves it: into a
// group of its own section, by the group's dotted name, or to its top level
// for "", dropped on its section's row. It refuses another section, where
// it already is, and itself or what it holds.
func (s *State) dropInto(dragged, target string) (string, bool) {
	moving, found := s.shown.items[dragged]
	if !found || moving.kind != itemDeclared || moving.declared.Type == worldstate.TypeTemplate {
		return "", false
	}
	list := moving.list.Values
	root := valuetree.Root(list, moving.declared.ID)
	if target == topRow(moving, root) {
		return "", moving.declared.Parent != root
	}
	onto, found := s.shown.items[target]
	if !found || !sameSection(moving, onto, root) || onto.declared.Type != worldstate.TypeGroup ||
		onto.declared.ID == moving.declared.Parent || onto.declared.ID == moving.declared.ID ||
		containsID(list.Within(moving.declared.ID), onto.declared.ID) {
		return "", false
	}

	return valuetree.Dotted(list, onto.declared.ID), true
}

// topRow is the row whose drop takes a row to its section's top level: its
// template's, its NPC's or zone's, or its scope's heading.
func topRow(moving item, root ids.ULID) string {
	switch {
	case !root.IsZero():
		return declaredPrefix + root.String()
	case !moving.ownerID.IsZero():
		return ownerPrefix + moving.ownerID.String()
	}

	return sectionPrefix + string(moving.scope)
}

// sameSection reports whether a row is declared in the same section as one
// being moved: the same list, scope and root.
func sameSection(moving, other item, root ids.ULID) bool {
	return other.kind == itemDeclared && other.list.Document == moving.list.Document &&
		other.declared.Scope == moving.declared.Scope && valuetree.Root(moving.list.Values, other.declared.ID) == root
}

// faded reports whether a row is faded while another is dragged: anything
// outside the dragged row's section but its section's own row.
func (s *State) faded(dragged, row string) bool {
	moving, found := s.shown.items[dragged]
	if !found {
		return false
	}
	root := valuetree.Root(moving.list.Values, moving.declared.ID)
	if row == topRow(moving, root) {
		return false
	}
	other, found := s.shown.items[row]

	return !found || !sameSection(moving, other, root)
}

// dragLabel is what a dragged row carries beside the pointer: its name,
// and its new path over a row it can drop on.
func (s *State) dragLabel(dragged, over string) string {
	moving := s.shown.items[dragged]
	group, ok := s.dropInto(dragged, over)
	if !ok {
		return moving.declared.Name
	}
	dotted := moving.declared.Name
	if group != "" {
		dotted = group + "." + dotted
	}

	return moving.declared.Name + "  →  " +
		strings.TrimSuffix(moving.path, valuetree.Dotted(moving.list.Values, moving.declared.ID)) + dotted
}

// dropped moves a row dropped on a group or its section's row, and opens a
// closed group a drag rests on.
func (s *State) dropped(ctx *frame.Context, result ui.ListResult) {
	if result.Rested != "" && s.toggled[result.Rested] && strings.HasPrefix(result.Rested, declaredPrefix) {
		s.toggled[result.Rested] = false
	}
	if result.Dragged == "" || s.guard.Active() || s.Unsaved(ctx.Docs) > 0 {
		return
	}
	if group, ok := s.dropInto(result.Dragged, result.Dropped); ok {
		s.moveInto(ctx, s.shown.items[result.Dragged], group)
	}
}
