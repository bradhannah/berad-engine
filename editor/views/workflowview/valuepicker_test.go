package workflowview_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// withCoins declares world.gold and world.silver, integers, and world.open,
// a bool.
func (f *fixture) withCoins() {
	f.t.Helper()
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{Add: []worldstate.Declaration{
		worldstate.New(frametest.ID(60), "gold", worldstate.ScopeWorld, worldstate.TypeInteger),
		worldstate.New(frametest.ID(61), "silver", worldstate.ScopeWorld, worldstate.TypeInteger),
		worldstate.New(frametest.ID(62), "open", worldstate.ScopeWorld, worldstate.TypeBool),
	}, Set: nil, Remove: nil})
}

// shown reports whether the open picker lists a value.
func (f *fixture) shown(path string) bool {
	_, ok := f.state.Locate("valuerow:"+path, f.docs)

	return ok
}

// TestPickerTypes narrows a Value picker by a type chip, counting what it
// hides, and offers every type again; an operand's picker is locked to its
// value's type, counts the values it hides and lists them on request, never
// to be picked.
func TestPickerTypes(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindCheckValue)
	f.withCoins()
	f.click("ui:pick-value")
	if !f.shown("world.gold") || !f.shown("world.open") || !f.has("picker-hidden=0") {
		t.Fatal("every type is offered at first")
	}
	f.click("picker:type:bool")
	if f.shown("world.gold") || !f.shown("world.open") || f.has("picker-hidden=0") {
		t.Fatal("the bool chip hides the integers, counted")
	}
	f.click("ui:picker-show")
	if !f.shown("world.gold") || !f.has("picker-hidden=0") {
		t.Fatal("Show them offers every type again")
	}
	f.click("valuerow:world.gold")
	f.click("ui:source-value")
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.click("ui:pick-" + flow.ParamAgainst)
	if _, all := f.state.Locate("picker:type:all", f.docs); all || !f.shown("world.silver") || f.shown("world.open") {
		t.Fatal("an operand's picker offers only integers, locked")
	}
	if _, locked := f.state.Locate("picker:type:integer", f.docs); !locked || f.has("picker-hidden=0") {
		t.Fatal("the locked chip shows, and the bool it hides is counted")
	}
	f.click("ui:picker-show")
	f.click("valuerow:world.open")
	if !f.has("picker") || f.has("refers:against=world.open") {
		t.Fatal("a value of another type is listed on request but never picked")
	}
	f.click("valuerow:world.silver")
	if f.has("picker") || !f.has("refers:against=world.silver") {
		t.Fatal("an integer is picked")
	}
}

// TestFindNPCPickers locks Keep it in's picker to NPC values, and offers Only
// one whose the values of the NPC it finds, not the world's.
func TestFindNPCPickers(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindFindNPC)
	f.withCoins()
	f.click("ui:pick-into")
	if _, locked := f.state.Locate("picker:type:npc", f.docs); !locked {
		t.Fatal("Keep it in offers NPC values only")
	}
	f.keys("Escape")
	f.click("ui:pick-where")
	if _, all := f.state.Locate("picker:type:all", f.docs); !all || !f.shown("npc.name") || f.shown("world.gold") {
		t.Fatal("Only one whose offers the found NPC's values of any type, and no world value")
	}
}

// withQuests declares world.gold, an integer, and the group world.quests
// holding dragon_slain, a bool, and scales, an integer; and local.tally.
func (f *fixture) withQuests() {
	f.t.Helper()
	slain := worldstate.New(frametest.ID(64), "dragon_slain", worldstate.ScopeWorld, worldstate.TypeBool)
	scales := worldstate.New(frametest.ID(65), "scales", worldstate.ScopeWorld, worldstate.TypeInteger)
	slain.Parent, scales.Parent = frametest.ID(63), frametest.ID(63)
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{Add: []worldstate.Declaration{
		worldstate.New(frametest.ID(60), "gold", worldstate.ScopeWorld, worldstate.TypeInteger),
		worldstate.New(frametest.ID(63), "quests", worldstate.ScopeWorld, worldstate.TypeGroup), slain, scales,
	}, Set: nil, Remove: nil})
	f.carryOutOn(workflowID, workflowplugin.ChangeLocalsEdit, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{
			worldstate.New(frametest.ID(66), "tally", worldstate.ScopeLocal, worldstate.TypeInteger),
		}, Set: nil, Remove: nil,
	})
}

// TestPickerRailAndTree counts values by scope for the search typed and
// narrows to one scope; with nothing typed a group folds, opens by a click
// and by the arrows, and is open again in the next picker.
func TestPickerRailAndTree(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindCheckValue)
	f.withQuests()
	f.click("ui:pick-value")
	if !f.shown("world.gold") || f.shown("world.quests.dragon_slain") || !f.has("picker-count:world=3") ||
		!f.has("picker-count:local=1") {
		t.Fatal("with nothing typed, world.quests is folded and the rail counts every value")
	}
	f.typeText("scales")
	if !f.shown("world.quests.scales") || !f.has("picker-count:everywhere=1") || !f.has("picker-count:local=0") {
		t.Fatal("typing flattens the list to matches, and the counts follow")
	}
	f.keys("Cmd+A", "Backspace")
	f.click("picker:group:world.quests")
	if !f.shown("world.quests.dragon_slain") {
		t.Fatal("a click opens the group")
	}
	f.click("picker:local")
	if !f.shown("local.tally") || f.shown("world.gold") {
		t.Fatal("the rail narrows to local values")
	}
	f.click("picker:everywhere")
	f.keys("Escape")
	f.click("ui:pick-value")
	if !f.shown("world.quests.dragon_slain") {
		t.Fatal("the last group opened is open in the next picker")
	}
	f.click("picker:group:world.quests")
	if f.shown("world.quests.dragon_slain") {
		t.Fatal("a click closes it")
	}
	f.keys("Right")
	if !f.shown("world.quests.dragon_slain") {
		t.Fatal("the right arrow opens the selected group")
	}
	f.keys("Down")
	f.keys("Left")
	f.keys("Left")
	if f.shown("world.quests.dragon_slain") {
		t.Fatal("left goes from a value to its group, and closes it")
	}
	f.keys("Enter")
	f.keys("Down")
	f.keys("Enter")
	if f.has("picker") || !f.has("value:value=world.quests.dragon_slain") {
		t.Fatal("Enter opens the group, and picks the value below it")
	}
}
