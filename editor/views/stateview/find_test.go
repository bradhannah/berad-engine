package stateview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
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
