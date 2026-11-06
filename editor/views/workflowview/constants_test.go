package workflowview_test

import (
	"encoding/json"
	"strings"
	"testing"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

var (
	greetingID = frametest.ID(60)
	restockID  = frametest.ID(61)
	mottoID    = frametest.ID(62)
)

// withConstants gives the shop template a greeting, which Ada has her own
// of, and the world a restock time and a motto as long as a page.
func (f *fixture) withConstants() {
	f.t.Helper()
	f.withShop()
	greeting := worldstate.NewConstant(greetingID, "greeting", worldstate.ScopeNPC, worldstate.TypeText)
	greeting.Parent, greeting.Default = shopID, json.RawMessage(`"Welcome in!"`)
	restock := worldstate.NewConstant(restockID, "restock_minutes", worldstate.ScopeWorld, worldstate.TypeInteger)
	restock.Default = json.RawMessage(`30`)
	motto := worldstate.NewConstant(mottoID, "motto_page", worldstate.ScopeWorld, worldstate.TypeText)
	motto.Default = worldstate.Value{Type: worldstate.TypeText, Text: strings.Repeat("Fair winds. ", 30)}.Encode()
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{greeting, restock, motto}, Set: nil, Remove: nil,
	})
	f.carryOutOn(adaID, npcplugin.SetConstantEdit, stateplugin.ConstantArgs{
		Constant: greetingID, Value: json.RawMessage(`"Fresh off the [b]boat[/b]!"`),
	})
}

// TestASayReadsAConstant puts the shop's greeting in a Say's line from the
// picker, as const.npc.shop.greeting, which reads as Ada's own greeting.
func TestASayReadsAConstant(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSay)
	f.withConstants()
	f.typeInto("param-message", "Hello! ")
	f.click("ui:insert-value")
	f.typeText("greeting")
	f.click("valuerow:const.npc.shop.greeting")
	if !f.has("message=Hello! {const.npc.shop.greeting}") || !f.has("reads=Hello! Fresh off the boat!") {
		t.Fatal("a constant stands in a line by its path, and reads as Ada's own, without its markup")
	}
	f.typeInto("param-message", "Back in {const.world.restock_minutes} minutes.")
	if !f.has("reads=Back in 30 minutes.") || f.has("problem:"+flow.KindSay+":1") {
		t.Fatal("a world constant typed by its path reads as its number")
	}
}

// TestCheckingAConstant checks the shop's greeting itself, compares a value
// with the restock time, and finds Set value never offers a constant to
// set.
func TestCheckingAConstant(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindCheckValue)
	f.withConstants()
	f.click("ui:pick-value")
	f.typeText("greeting")
	f.click("valuerow:const.npc.shop.greeting")
	if !f.has("value:value=const.npc.shop.greeting") || f.has("problem:"+flow.KindCheckValue+":1") {
		t.Fatalf("a Check value checks a constant: %q", f.status())
	}
	set := withNode(t, flow.KindSetValue)
	set.withConstants()
	set.click("ui:pick-value")
	set.typeText("restock")
	if _, offered := set.state.Locate("valuerow:const.world.restock_minutes", set.docs); offered {
		t.Fatal("Set value sets only values")
	}
}

// TestSetValueFromALongConstant sets a text value from a page-long
// constant, which is cut to a value's 200 characters, and warns so.
func TestSetValueFromALongConstant(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSetValue)
	f.withConstants()
	note := worldstate.New(frametest.ID(63), "note", worldstate.ScopeWorld, worldstate.TypeText)
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{note}, Set: nil, Remove: nil,
	})
	f.click("ui:pick-value")
	f.click("valuerow:world.note")
	set := f.nth(flow.KindSetValue, 1)
	set, err := set.WithParam("to", flow.RefOperand(flow.ValueRef{
		Scope: worldstate.ScopeWorld, Value: mottoID, Owner: [16]byte{}, Found: [16]byte{},
	}))
	if err != nil {
		t.Fatal(err)
	}
	f.replaceNode(set)
	if !f.has("warning:" + flow.KindSetValue + ":1") {
		t.Fatal("a constant longer than a value holds is warned of")
	}
}
