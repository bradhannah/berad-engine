package stateview_test

import (
	"encoding/json"
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/stateview"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

var (
	greetingID = frametest.ID(46)
	letterID   = frametest.ID(47)
)

// constantsFixture is newFixture's project in the Constants window, with a
// greeting constant in the guards template, which Guard 1 has his own of,
// and a world constant holding a letter.
func constantsFixture(t *testing.T) *fixture {
	t.Helper()
	f := fixtureOf(t, stateview.NewConstants)
	greeting := in(worldstate.NewConstant(greetingID, "greeting", worldstate.ScopeNPC, worldstate.TypeText), guardsID)
	greeting.Default = worldstate.Value{Type: worldstate.TypeText, Text: "Halt! Who goes there?"}.Encode()
	letter := worldstate.NewConstant(letterID, "letter", worldstate.ScopeWorld, worldstate.TypeText)
	letter.Default = worldstate.Value{Type: worldstate.TypeText, Text: "Dear Marta,\nThe roof held."}.Encode()
	f.docs.View(worldstate.ID(), func(document client.Document) {
		if err := document.Model.(*stateplugin.Document).State.Values.InsertAll([]worldstate.Declaration{
			greeting, letter,
		}); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.View(guardID, func(document client.Document) {
		document.Model.(*npcplugin.Document).NPC.Owned.SetConstant(greetingID, json.RawMessage(`"Stand fast, brave one."`))
	})
	f.docs.Touch(worldstate.ID())
	f.docs.Touch(guardID)
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

// TestConstantsAreApartFromValues lists constants in the Constants window
// and values in the Values window, with groups and templates in both, and
// a template's constant saying how many hold their own.
func TestConstantsAreApartFromValues(t *testing.T) {
	t.Parallel()
	f := constantsFixture(t)
	for _, row := range []string{"row:world.letter", "row:npc.guards.greeting", "row:world.quests"} {
		if _, shown := f.state.Locate(row, f.ctx.Docs); !shown {
			t.Errorf("the Constants window shows %s", row)
		}
	}
	for _, row := range []string{"row:world.mood", "row:npc.guards.arrests", "row:world.quests.dragon_slain"} {
		if _, shown := f.state.Locate(row, f.ctx.Docs); shown {
			t.Errorf("the Constants window hides the value %s", row)
		}
	}
	if !f.has(`value=npc.guards.greeting:text:"Halt! Who goes there?"`) {
		t.Fatal("a constant is listed with its type and its text")
	}
	values := fixtureOf(t, stateview.New)
	if _, shown := values.state.Locate("row:world.mood", values.ctx.Docs); !shown {
		t.Fatal("the Values window still shows values")
	}
}

// TestConstantsAreMadeAndChanged adds a text constant, retypes it, renames
// it into a group, and deletes it, as nothing uses it.
func TestConstantsAreMadeAndChanged(t *testing.T) {
	t.Parallel()
	f := constantsFixture(t)
	f.click("ui:add-constant")
	made, found := f.world().Find(worldstate.ScopeWorld, [16]byte{}, "constant")
	if !found || !made.IsConstant() || made.Type != worldstate.TypeText {
		t.Fatalf("Add constant declares a text constant at the top of the world: %+v %q", made, f.status())
	}
	f.answerUses(nil)
	f.click("ui:value-type-integer")
	if retyped, _ := f.world().Value(made.ID); retyped.Type != worldstate.TypeInteger || !retyped.IsConstant() {
		t.Fatalf("a constant is retyped while nothing uses it: %+v", retyped)
	}
	f.answerUses(nil)
	f.typeInto("value-name", "shop.bread_price")
	if moved, _ := f.world().Value(made.ID); moved.Name != "bread_price" || moved.Parent.IsZero() {
		t.Fatalf("a dotted name moves it into a group: %+v %q", moved, f.status())
	}
	if !f.has("selected=world.shop.bread_price") {
		t.Fatal("it stays selected")
	}
	f.answerUses(nil)
	f.click("ui:value-delete")
	f.answerUses(nil)
	if _, still := f.world().Value(made.ID); still {
		t.Fatalf("an unused constant is deleted at once: %q", f.status())
	}
}

// TestUsedConstantIsKept fixes the type of a constant a workflow uses, and
// refuses to delete it.
func TestUsedConstantIsKept(t *testing.T) {
	t.Parallel()
	f := constantsFixture(t)
	f.click("row:world.letter")
	used := map[string][]protocol.Use{letterID.String(): {
		{Document: guardID, Name: "Read letter", Format: "workflow", Label: "a Show message node", Item: letterID.String()},
	}}
	f.answerUses(used)
	f.click("ui:value-type-integer")
	if kept, _ := f.world().Value(letterID); kept.Type != worldstate.TypeText {
		t.Fatal("a used constant keeps its type")
	}
	f.click("ui:value-delete")
	f.answerUses(used)
	if _, still := f.world().Value(letterID); !still {
		t.Fatal("a used constant is not deleted")
	}
}

// TestAnNPCsOwnValue opens Guard 1, sets his own greeting in the inspector,
// and gives him the default back with Use the default.
func TestAnNPCsOwnValue(t *testing.T) {
	t.Parallel()
	f := constantsFixture(t)
	f.click("toggle:npc.guard1")
	f.click("row:npc.guard1.guards.greeting")
	if !f.has(`value=npc.guard1.guards.greeting:text:"Stand fast, brave one."`) {
		t.Fatal("Guard 1's own greeting shows in the tree")
	}
	f.click("ui:held-value")
	f.keys("Cmd+A", "Backspace")
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = "Halt!\nThe gate is shut."
	f.frame(input)
	f.click("ui:save")
	greeting, _ := f.world().Value(greetingID)
	if own, has := f.npcDoc(guardID).Owned.ConstantValue(greeting); !has || own.Text != "Halt!\nThe gate is shut." {
		t.Fatalf("his own greeting is saved, line break and all: %+v %q", own, f.status())
	}
	f.click("ui:use-default")
	if _, has := f.npcDoc(guardID).Owned.ConstantValue(greeting); has {
		t.Fatal("Use the default clears his own")
	}
}

// TestFindAConstantByItsWords finds a constant by words only Guard 1's own
// value has, and shows it.
func TestFindAConstantByItsWords(t *testing.T) {
	t.Parallel()
	f := constantsFixture(t)
	f.click("ui:find-value")
	f.answerUses(nil)
	f.typed("brave")
	f.click("valuerow:npc.guards.greeting")
	if f.has("find") || !f.has("selected=npc.guards.greeting") {
		t.Fatalf("a constant is found by what it says, and shown: %q", f.status())
	}
}
