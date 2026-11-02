package stateview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// typed types text into whatever has focus.
func (f *fixture) typed(text string) {
	f.t.Helper()
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
}

// TestFind finds a value in a closed group and shows it, narrows to NPCs'
// own values and to templates', counts each value's uses, and opens an NPC
// to show one of its own by Enter.
func TestFind(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("toggle:world.quests")
	f.click("ui:find-value")
	f.answerUses(map[string][]protocol.Use{"": {
		{Document: guardID, Name: "Workflow 1", Format: "workflow", Label: "a Check value node", Item: moodID.String()},
		{Document: guardID, Name: "Workflow 2", Format: "workflow", Label: "a Set value node", Item: moodID.String()},
	}})
	if !f.has("find") || !f.has("picker-detail:world.mood=choice · used 2 times") ||
		!f.has("picker-detail:npc.guards.arrests=integer · unused") {
		t.Fatal("Find opens, counting each value's uses")
	}
	f.typed("slain")
	f.click("valuerow:world.quests.dragon_slain")
	if f.has("find") || !f.has("selected=world.quests.dragon_slain") {
		t.Fatal("a pick shows the value in the tree, its group opened")
	}
	f.keys("Cmd+F")
	f.click("picker:npcs")
	if !f.has("picker-count:npcs=1") || !f.has("picker-count:npc-templates=1") ||
		f.has("picker-count:everywhere=1") {
		t.Fatal("the rail counts NPCs' own values and templates'")
	}
	f.typed("secret")
	f.keys("Enter")
	f.keys()
	if f.has("find") || !f.has("selected=npc.george.secret") {
		t.Fatalf("Enter shows George's secret, opening him: %q", f.status())
	}
	if _, shown := f.state.Locate("row:npc.george.secret", f.ctx.Docs); !shown {
		t.Fatal("his row shows")
	}
}

// TestDeclareAndShow declares from Find in the world, as an NPC's own and
// in a template, each starting where the panel says, and shows each in the
// tree; a name with no scope can go anywhere.
func TestDeclareAndShow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:find-value")
	f.typed("shift")
	for _, where := range []string{"world.shift", "npc.guards.shift", "npc.george.shift", "zone.market.shift"} {
		if _, offered := f.state.Locate("declare:"+where, f.ctx.Docs); !offered {
			t.Errorf("shift can be declared as %s", where)
		}
	}
	f.keys("Escape")
	f.click("ui:find-value")
	f.typed("world.shop.open_hour")
	f.click("ui:declare-start")
	f.keys("Cmd+A")
	f.typed("9")
	f.click("ui:picker-declare")
	f.keys()
	if hour, found := f.world().Find(worldstate.ScopeWorld, f.group("shop"), "open_hour"); !found ||
		string(hour.Default) != "9" || !f.has("selected=world.shop.open_hour") {
		t.Fatalf("declared world.shop.open_hour starting at 9, shown: %+v", hour)
	}
	f.keys("Cmd+F")
	f.typed("npc.george.mood_today")
	f.click("ui:declare-type-bool")
	f.click("ui:declare-start-true")
	f.click("ui:picker-declare")
	f.keys()
	if mood, found := f.npcDoc(georgeID).Owned.Values.Find(worldstate.ScopeNPC, ids.ULID{}, "mood_today"); !found ||
		string(mood.Default) != "true" || !f.has("selected=npc.george.mood_today") {
		t.Fatalf("declared George's own mood_today, true, shown: %+v", mood)
	}
	f.keys("Cmd+F")
	f.typed("npc.guards.on_duty")
	f.keys("Enter")
	f.keys()
	if duty, found := f.world().Find(worldstate.ScopeNPC, guardsID, "on_duty"); !found ||
		!f.has("selected=npc.guards.on_duty") {
		t.Fatalf("Enter declared on_duty in the guards template, shown: %+v", duty)
	}
}

// group finds a world group by its name at the top.
func (f *fixture) group(name string) ids.ULID {
	found, _ := f.world().Find(worldstate.ScopeWorld, ids.ULID{}, name)

	return found.ID
}
