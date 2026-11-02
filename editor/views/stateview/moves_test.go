package stateview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// rightClick right-clicks a named part of the window.
func (f *fixture) rightClick(name string) {
	f.t.Helper()
	input := ui.NewInput(f.at(name))
	input.Secondary = true
	f.frame(input)
}

// moveBy moves a row by its right-click menu's Move into, to "into:<group>"
// or "into-new".
func (f *fixture) moveBy(row, into string) {
	f.t.Helper()
	f.rightClick("row:" + row)
	f.click("menu:move-into")
	f.click("menu:" + into)
}

// TestMoveInto moves a value by its right-click menu into a group, to the
// top level and into a new group, as one change each; refuses a name the
// group has; moves an NPC's own value within its own; and offers a
// template no move.
func TestMoveInto(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sent := len(f.requests.Commands)
	f.moveBy("world.mood", "into:quests")
	if path := f.world().Path(moodID); path != "world.quests.mood" || len(f.requests.Commands) != sent+1 ||
		!strings.Contains(f.status(), "Moved to world.quests.mood") {
		t.Fatalf("moved into quests as one change: %s, %q", path, f.status())
	}
	if !f.has("selected=world.quests.mood") {
		t.Fatal("the row moved stays selected")
	}
	f.moveBy("world.quests.mood", "into:")
	if path := f.world().Path(moodID); path != "world.mood" {
		t.Fatalf("moved to the top level: %s", path)
	}
	f.click("row:world.quests.dragon_slain")
	f.typeInto("value-name", "quests.mood")
	f.moveBy("world.mood", "into:quests")
	if path := f.world().Path(moodID); path != "world.mood" || !strings.Contains(f.status(), "already declared") {
		t.Fatalf("quests has a mood already: %s, %q", path, f.status())
	}
	f.moveBy("world.mood", "into-new")
	if path := f.world().Path(moodID); path != "world.group.mood" || !f.has("selected=world.group") ||
		f.state.Focused() == nil {
		t.Fatalf("moved into a new group, selected with its name to type: %s", path)
	}
	f.keys("Escape")
	f.click("toggle:npc.george")
	f.moveBy("npc.george.secret", "into-new")
	if _, found := f.state.Locate("row:npc.george.group.secret", f.ctx.Docs); !found {
		t.Fatal("George's secret moved into a group of his own")
	}
	f.rightClick("row:npc.guards")
	f.click("menu:move-into")
	if _, found := f.state.Locate("menu:into:", f.ctx.Docs); found {
		t.Fatal("a template does not move")
	}
	f.rightClick("row:world.quests.mood")
	f.click("menu:rename")
	if f.state.Focused() == nil || !f.has("selected=world.quests.mood") {
		t.Fatal("Rename puts the name to type")
	}
}
