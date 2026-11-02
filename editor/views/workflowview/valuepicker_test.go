package workflowview_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

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
