package stateview

import (
	"strings"

	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/signals"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// What the SIGNALS heading's and a signal group's right-click menus add,
// located as "menu:<id>": a signal, or a group of them. Move into, Rename
// and Delete are the values' own ids.
const (
	menuAddSignal      = "add-signal"
	menuAddSignalGroup = "add-signal-group"
)

// signalMenu is the SIGNALS heading's menu, a group's or a signal's: the
// heading and a group add inside them; a group and a signal move into
// another group, rename and delete.
func signalMenu(row string, meaning item, found bool) []ui.MenuItem {
	adds := func(where string) []ui.MenuItem {
		return []ui.MenuItem{
			menuHeading(where), choice(menuAddSignal, "Add signal", false),
			choice(menuAddSignalGroup, "Add group", false),
		}
	}
	if row == signalsSection {
		return adds("At the top of " + signalsTopLabel)
	}
	if !found || !isSignal(meaning.kind) {
		return nil
	}
	var items []ui.MenuItem
	what := "signal"
	if meaning.kind == itemSignalGroup {
		items, what = adds("In "+meaning.path), newGroupName
	}
	items = append(items, choice(menuMoveInto, "Move into", len(items) > 0), choice(menuRename, "Rename", true))
	remove := choice(menuDelete, "Delete "+what, true)
	remove.Danger = true

	return append(items, remove)
}

// signalIntoItems lists where a signal or group can move: the top of
// Signals and each group but itself and those inside it, nested groups
// indented. Where it is already is shown, not offered.
func signalIntoItems(declared []worldstate.Signal, moving ids.ULID) []ui.MenuItem {
	var parent ids.ULID
	for _, signal := range declared {
		if signal.ID == moving {
			parent = signal.Parent
		}
	}
	items := []ui.MenuItem{
		menuHeading("Inside " + signalsTopLabel),
		{ID: menuInto, Label: "Top level", Icon: ui.IconNone, Disabled: parent.IsZero(), Danger: false, Rule: false},
	}
	inside := worldstate.SignalsWithin(declared, moving)
	var walk func(group ids.ULID, depth int)
	walk = func(group ids.ULID, depth int) {
		for _, held := range worldstate.SignalChildren(declared, group) {
			if !held.Group || held.ID == moving || containsID(inside, held.ID) {
				continue
			}
			items = append(items, ui.MenuItem{
				ID: menuInto + worldstate.SignalPath(declared, held.ID), Label: strings.Repeat(indentSpaces, depth) +
					held.Name,
				Icon: ui.IconFolder, Disabled: parent == held.ID, Danger: false, Rule: false,
			})
			walk(held.ID, depth+1)
		}
	}
	walk(ids.ULID{}, 0)

	return items
}

// chooseSignal carries out a choice from a signal's, a group's or the
// SIGNALS heading's menu, and reports whether it was one.
func (s *State) chooseSignal(ctx *frame.Context, chosen string) bool {
	switch chosen {
	case menuAddSignal, menuAddSignalGroup:
		parent := ids.ULID{}
		if meaning, found := s.shown.items[s.menuOf]; found && meaning.kind == itemSignalGroup {
			parent = meaning.ownerID
		}
		s.addSignal(ctx, parent, chosen == menuAddSignalGroup)

		return true
	}
	selected, found := s.selected()
	if !found || !isSignal(selected.kind) {
		return false
	}
	switch {
	case chosen == menuRename:
		s.renameSignal()
	case chosen == menuDelete:
		s.startSignalDelete(ctx, selected)
	case strings.HasPrefix(chosen, menuInto):
		s.moveSignal(ctx, selected, strings.TrimPrefix(chosen, menuInto))
	}

	return true
}

// moveSignal moves a signal or group into the group at a path, or to the
// top for "", keeping its name; workflows name it by its identity, so they
// follow it. The status says where it went, or why it cannot.
func (s *State) moveSignal(ctx *frame.Context, selected item, group string) {
	declared, _ := signals.Declared(ctx.Docs)
	var moved worldstate.Signal
	parent := ids.ULID{}
	for _, signal := range declared {
		if signal.ID == selected.ownerID {
			moved = signal
		}
		if signal.Group && group != "" && worldstate.SignalPath(declared, signal.ID) == group {
			parent = signal.ID
		}
	}
	where := signalWhere(declared, parent)
	for _, beside := range worldstate.SignalChildren(declared, parent) {
		if beside.Name == moved.Name && beside.ID != moved.ID {
			ctx.Frame.Refuse("There is a " + moved.Name + " " + where + " already.")

			return
		}
	}
	moved.Parent = parent
	path := moved.Name
	if group != "" {
		path = group + "." + moved.Name
	}
	if !parent.IsZero() {
		s.toggled[signalPrefix+parent.String()] = false
	}
	s.send(ctx, worldstate.ID(), stateplugin.SignalsEdit,
		stateplugin.SignalsArgs{Add: nil, Set: []worldstate.Signal{moved}, Remove: nil},
		ctx.Frame.Report("Moved to "+path+". Cmd+Z undoes it."))
}

// signalDropInto says where dropping a dragged signal or group on another
// row moves it: into a group, by its path, or to the top for "", dropped
// on the SIGNALS heading. It refuses any other row, where it already is,
// and itself or what it holds.
func (s *State) signalDropInto(declared []worldstate.Signal, moving item, target string) (string, bool) {
	var parent ids.ULID
	for _, signal := range declared {
		if signal.ID == moving.ownerID {
			parent = signal.Parent
		}
	}
	if target == signalsSection {
		return "", !parent.IsZero()
	}
	onto, found := s.shown.items[target]
	if !found || onto.kind != itemSignalGroup || onto.ownerID == parent || onto.ownerID == moving.ownerID ||
		containsID(worldstate.SignalsWithin(declared, moving.ownerID), onto.ownerID) {
		return "", false
	}

	return onto.path, true
}

// signalDragLabel is what a dragged signal or group carries beside the
// pointer: its name, and its new path over a row it can drop on.
func signalDragLabel(name, group string, ok bool) string {
	if !ok {
		return name
	}
	if group == "" {
		return name + "  →  " + name
	}

	return name + "  →  " + group + "." + name
}
