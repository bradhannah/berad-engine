package stateview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/worldstate"
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
	f.answerUses(nil)
	f.keys()
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

// TestMovingWhatIsUsed asks before moving a value a workflow uses, listing
// the use, and moves it on Move; Cancel leaves it; a group is asked about
// for the values it holds.
func TestMovingWhatIsUsed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	used := map[string][]protocol.Use{moodID.String(): {{
		Document: frametest.ID(70), Name: "Workflow 1", Format: "workflow", Label: "a Check value node",
	}}}
	f.rightClick("row:world.mood")
	f.click("menu:move-into")
	f.click("menu:into:quests")
	f.answerUses(used)
	if f.world().Path(moodID) != "world.mood" || !f.dialogSays("Workflow 1 · a Check value node") ||
		!f.dialogSays("world.mood  →  world.quests.mood") {
		t.Fatal("a used value asks first, listing the use and the path before and after")
	}
	f.click("dialog:cancel")
	if f.world().Path(moodID) != "world.mood" {
		t.Fatal("Cancel leaves it")
	}
	f.rightClick("row:world.mood")
	f.click("menu:move-into")
	f.click("menu:into:quests")
	f.answerUses(used)
	f.click("dialog:move")
	if f.world().Path(moodID) != "world.quests.mood" || !strings.Contains(f.status(), "Moved to world.quests.mood") {
		t.Fatalf("Move moves it: %s", f.world().Path(moodID))
	}
	f.rightClick("row:world.quests")
	f.click("menu:move-into")
	f.click("menu:into-new")
	f.answerUses(used)
	if _, asks := f.state.Locate("dialog:move", f.ctx.Docs); !asks || f.world().Path(moodID) != "world.quests.mood" ||
		!f.dialogSays("Move quests into a new group, group?") {
		t.Fatal("a group holding a used value asks first")
	}
}

// dialogSays reports whether the open dialog has a line.
func (f *fixture) dialogSays(line string) bool { return f.has("says=" + line) }

// drag presses one named part, moves to another, stays there some frames
// and lets go, then answers where the value is used: nowhere.
func (f *fixture) drag(from, onto string, stay int) {
	f.t.Helper()
	start, end := f.at(from), f.at(onto)
	input := ui.NewInput(start)
	input.Pressed, input.Down = true, true
	f.frame(input)
	for range stay + 1 {
		input = ui.NewInput(end)
		input.Down = true
		f.frame(input)
	}
	input = ui.NewInput(end)
	input.Released = true
	f.frame(input)
	f.answerUses(nil)
	f.keys()
}

// TestDragging moves a value by dragging it onto a group and back onto its
// section's heading, opens a closed group it rests on, and refuses another
// section.
func TestDragging(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.drag("row:world.mood", "row:world.quests", 1)
	if path := f.world().Path(moodID); path != "world.quests.mood" {
		t.Fatalf("dropped into quests: %s", path)
	}
	f.drag("row:world.quests.mood", "section:world", 1)
	if path := f.world().Path(moodID); path != "world.mood" {
		t.Fatalf("dropped on World, out of quests: %s", path)
	}
	f.drag("row:world.mood", "row:npc.guards", 1)
	if path := f.world().Path(moodID); path != "world.mood" {
		t.Fatalf("a world value cannot drop into a template: %s", path)
	}
	f.click("toggle:world.quests")
	f.drag("row:world.mood", "row:world.quests", 40)
	if _, open := f.state.Locate("row:world.quests.dragon_slain", f.ctx.Docs); !open ||
		f.world().Path(moodID) != "world.quests.mood" {
		t.Fatal("resting on a closed group opens it, and the drop goes in")
	}
	f.click("toggle:npc.guard1")
	f.drag("row:npc.guards.arrests", "row:world.quests", 1)
	if path := f.world().Path(arrestsID); path != "npc.guards.arrests" {
		t.Fatalf("a template's value stays in its template: %s", path)
	}
}

// TestUsedValueKeepsItsType keeps a value's type while where it is used is
// asked, and while a workflow uses it, offering to declare a new value
// beside it in Find; a value nothing uses changes type.
func TestUsedValueKeepsItsType(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:world.quests.dragon_slain")
	f.click("ui:value-type-integer")
	if slain, _ := f.world().Value(slainID); slain.Type != worldstate.TypeBool {
		t.Fatal("its type waits while its uses are asked")
	}
	f.answerUses(map[string][]protocol.Use{slainID.String(): {{
		Document: frametest.ID(70), Name: "Workflow 1", Format: "workflow", Label: "a Check value node",
	}}})
	f.click("ui:value-type-integer")
	if slain, _ := f.world().Value(slainID); slain.Type != worldstate.TypeBool {
		t.Fatal("a value a workflow uses keeps its type")
	}
	f.click("ui:value-declare-new")
	if field := f.state.Focused(); !f.has("find") || field == nil || field.Field.Value() != "world.quests." {
		t.Fatal("Declare a new value opens Find beside it")
	}
	f.keys("Escape")
	f.click("row:world.mood")
	f.answerUses(nil)
	f.click("ui:value-type-integer")
	if mood, _ := f.world().Value(moodID); mood.Type != worldstate.TypeInteger {
		t.Fatal("a value nothing uses changes type")
	}
	if _, offered := f.state.Locate("ui:value-declare-new", f.ctx.Docs); offered {
		t.Fatal("and has no need to declare a new one")
	}
}
