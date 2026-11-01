package workflowview_test

import (
	"encoding/json"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// worldValues reads the fixture's world state.
func (f *fixture) worldValues() []worldstate.Declaration {
	var values []worldstate.Declaration
	f.docs.View(worldstate.ID(), func(document client.Document) {
		values = document.Model.(*stateplugin.Document).State.Values.Values()
	})

	return values
}

// withNode adds a node of a kind after the start and selects it.
func withNode(t *testing.T, kind string) *fixture {
	t.Helper()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + kind)
	if !f.has("selected:" + kind) {
		t.Fatalf("a new %s is selected", kind)
	}

	return f
}

// TestSetValueDeclaresAndSets adds a Set value with no value, a problem;
// declares npc.mood from its chooser, which chooses it; makes mood a list
// in the world state; and sets it to a choice.
func TestSetValueDeclaresAndSets(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSetValue)
	if !f.has("problem:" + flow.KindSetValue + ":1") {
		t.Fatal("a Set value with no value is a problem")
	}
	f.click("ui:pick-value")
	f.typeText("mood")
	f.click("declare:npc")
	values := f.worldValues()
	if len(values) != 1 || values[0].Path() != "npc.mood" || values[0].Type != worldstate.TypeNumber {
		t.Fatalf("declared %+v", values)
	}
	if !f.has("value:value=npc.mood") || !f.has("operand:to=0") {
		t.Fatal("the declared value is chosen, and set to its default")
	}
	if f.has("problem:" + flow.KindSetValue + ":1") {
		t.Fatal("a Set value with a value and an operand is no problem")
	}
	f.click("ui:operand-to")
	f.keys("Cmd+A", "Backspace")
	f.typeText("x")
	f.keys("Enter")
	if f.has("operand:to=0") == false {
		t.Fatal("a number that is not a number is refused")
	}
	f.keys("Cmd+A", "Backspace")
	f.typeText("-3")
	f.keys("Enter")
	if !f.has("operand:to=-3") {
		t.Fatal("a typed number is saved")
	}
	mood := values[0]
	mood.Type, mood.Choices = worldstate.TypeChoice, []worldstate.Choice{{ID: "c1", Name: "calm"}, {ID: "c2", Name: "cross"}}
	mood.Default = []byte(`"c1"`)
	f.carryOutOn(worldstate.ID(), stateplugin.SetValueEdit, stateplugin.ValueArgs{Value: mood})
	if !f.has("problem:" + flow.KindSetValue + ":1") {
		t.Fatal("a number set into what is now a list no longer fits")
	}
	f.click("ui:operand-to-c2")
	if !f.has(`operand:to="c2"`) || f.has("problem:"+flow.KindSetValue+":1") {
		t.Fatal("setting it to a choice")
	}
}

// TestCheckValueReadsOthers lists another NPC's values in a Check value's
// chooser, by its path name, and the value of the NPC a local holds; a Set
// value cannot choose them.
func TestCheckValueReadsOthers(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindCheckValue)
	waiting := worldstate.New(worldstate.ID(), "waiting", worldstate.ScopeNPC, worldstate.TypeYesNo)
	waiting.ID[15] = 50
	f.carryOutOn(worldstate.ID(), stateplugin.AddValueEdit, stateplugin.ValueArgs{Value: waiting})
	customer := worldstate.New(waiting.ID, "customer", worldstate.ScopeLocal, worldstate.TypeNPC)
	customer.ID[15] = 51
	f.carryOutOn(workflowID, "workflow.local.add", stateplugin.ValueArgs{Value: customer})
	f.click("ui:pick-value")
	for _, path := range []string{"npc.waiting", "npc.ada.waiting", "local.customer.waiting"} {
		if _, ok := f.state.Locate("valuerow:"+path, f.docs); !ok {
			t.Errorf("a Check value can read %s", path)
		}
	}
	f.click("valuerow:npc.ada.waiting")
	if !f.has("value:value=npc.ada.waiting") || !f.has("operand:against=false") {
		t.Fatal("chose Ada's value")
	}
	set := withNode(t, flow.KindSetValue)
	set.carryOutOn(worldstate.ID(), stateplugin.AddValueEdit, stateplugin.ValueArgs{Value: waiting})
	set.click("ui:pick-value")
	if _, ok := set.state.Locate("valuerow:npc.ada.waiting", set.docs); ok {
		t.Fatal("a Set value never sets another NPC's value")
	}
	if _, ok := set.state.Locate("valuerow:npc.waiting", set.docs); !ok {
		t.Fatal("a Set value sets its own NPC's value")
	}
}

// TestFindNPCKeepsWhatItFinds needs a local NPC value to keep what it
// finds, and declares one from its chooser.
func TestFindNPCKeepsWhatItFinds(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindFindNPC)
	if !f.has("problem:" + flow.KindFindNPC + ":1") {
		t.Fatal("a Find NPC nearby with nowhere to keep what it finds is a problem")
	}
	f.click("ui:pick-into")
	f.typeText("customer")
	f.click("declare:local")
	locals := f.workflow().Locals().Values()
	if len(locals) != 1 || locals[0].Path() != "local.customer" || locals[0].Type != worldstate.TypeNPC {
		t.Fatalf("declared %+v", locals)
	}
	if !f.has("value:into=local.customer") || f.has("problem:"+flow.KindFindNPC+":1") {
		t.Fatal("it keeps what it finds in local.customer")
	}
}

// carryOutOn applies an edit to a document as an edit made elsewhere would.
func (f *fixture) carryOutOn(document ids.ULID, edit string, args any) {
	f.t.Helper()
	data, err := json.Marshal(args)
	if err != nil {
		f.t.Fatal(err)
	}
	f.carryOut(frametest.Command{
		Command: protocol.Command{Document: document, Edit: edit, Gesture: "", Args: data}, Done: nil,
	})
}
