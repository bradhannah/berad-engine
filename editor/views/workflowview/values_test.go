package workflowview_test

import (
	"encoding/json"
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

var (
	shopID    = frametest.ID(50)
	waitingID = frametest.ID(51)
	bobID     = frametest.ID(52)
)

func noInput() ui.Input { return ui.NewInput(image.Pt(0, 0)) }

// world reads the fixture's world state.
func (f *fixture) world() *worldstate.Declarations {
	var values *worldstate.Declarations
	f.docs.View(worldstate.ID(), func(document client.Document) {
		values = document.Model.(*stateplugin.Document).State.Values.Clone()
	})

	return values
}

// adaValues reads Ada's own values.
func (f *fixture) adaValues() *worldstate.Declarations {
	var values *worldstate.Declarations
	f.docs.View(adaID, func(document client.Document) {
		values = document.Model.(*npcplugin.Document).NPC.Owned.Values.Clone()
	})

	return values
}

// declare declares a value from the open picker: where, by its path, then
// Declare and choose.
func (f *fixture) declare(path string) {
	f.t.Helper()
	f.click("declare:" + path)
	f.click("ui:picker-declare")
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

// withShop declares a shop template holding waiting, a bool, and applies
// it to Ada.
func (f *fixture) withShop() {
	f.t.Helper()
	waiting := worldstate.New(waitingID, "waiting", worldstate.ScopeNPC, worldstate.TypeBool)
	waiting.Parent = shopID
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{Add: []worldstate.Declaration{
		worldstate.New(shopID, "shop", worldstate.ScopeNPC, worldstate.TypeTemplate), waiting,
	}, Set: nil, Remove: nil})
	f.carryOutOn(adaID, npcplugin.ApplyTemplateEdit, stateplugin.TemplateArgs{Template: shopID, Index: 0})
}

// TestSetValueDeclaresAndSets adds a Set value with no value, a problem;
// declares a world value in a new group from its chooser, which chooses
// it; then npc.mood for Ada, who follows the workflow, makes it a list and
// sets it to a choice.
func TestSetValueDeclaresAndSets(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSetValue)
	if !f.has("problem:" + flow.KindSetValue + ":1") {
		t.Fatal("a Set value with no value is a problem")
	}
	f.click("ui:pick-value")
	f.typeText("world.quests.dragon_slain")
	f.declare("world.quests.dragon_slain")
	world := f.world()
	if world.Len() != 2 || !f.has("value:value=world.quests.dragon_slain") || !f.has("operand:to=0") {
		t.Fatalf("declared a group and an integer in it, and chose it: %+v", world.Values())
	}
	f.click("ui:pick-value")
	f.typeText("mood")
	f.declare("npc.mood for ada")
	own := f.adaValues()
	if own.Len() != 1 || !f.has("value:value=npc.mood") {
		t.Fatalf("declared npc.mood as Ada's own: %+v", own.Values())
	}
	f.click("ui:operand-to")
	f.keys("Cmd+A", "Backspace")
	f.typeText("x")
	f.keys("Enter")
	if !f.has("operand:to=0") {
		t.Fatal("an integer that is not an integer is refused")
	}
	f.keys("Cmd+A", "Backspace")
	f.typeText("-3")
	f.keys("Enter")
	if !f.has("operand:to=-3") {
		t.Fatal("a typed integer is saved")
	}
	mood := own.Values()[0]
	mood.Type, mood.Choices = worldstate.TypeChoice, []worldstate.Choice{{ID: "c1", Name: "calm"}, {ID: "c2", Name: "cross"}}
	mood.Default = []byte(`"c1"`)
	f.carryOutOn(adaID, npcplugin.ChangeValuesEdit, stateplugin.ChangeArgs{
		Add: nil, Set: []worldstate.Declaration{mood}, Remove: nil,
	})
	if !f.has("problem:" + flow.KindSetValue + ":1") {
		t.Fatal("an integer set into what is now a list no longer fits")
	}
	f.click("ui:operand-to-c2")
	if !f.has(`operand:to="c2"`) || f.has("problem:"+flow.KindSetValue+":1") {
		t.Fatal("setting it to a choice")
	}
}

// TestCheckValueReadsOthers lists another NPC's values in a Check value's
// picker, by its path name, and the values of the NPC a local holds; a Set
// value cannot choose them. A second NPC following the workflow without the
// template is a problem.
func TestCheckValueReadsOthers(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindCheckValue)
	f.withShop()
	customer := worldstate.New(frametest.ID(53), "customer", worldstate.ScopeLocal, worldstate.TypeNPC)
	f.carryOutOn(workflowID, workflowplugin.ChangeLocalsEdit, stateplugin.ChangeArgs{
		Add: []worldstate.Declaration{customer}, Set: nil, Remove: nil,
	})
	f.click("ui:pick-value")
	if _, ok := f.state.Locate("valuerow:npc.name", f.docs); !ok {
		t.Error("a Check value can read its NPC's name")
	}
	f.typeText("waiting")
	for _, path := range []string{"npc.shop.waiting", "npc.ada.shop.waiting", "local.customer.shop.waiting"} {
		if _, ok := f.state.Locate("valuerow:"+path, f.docs); !ok {
			t.Errorf("a Check value can read %s", path)
		}
	}
	f.click("valuerow:npc.shop.waiting")
	if !f.has("value:value=npc.shop.waiting") || !f.has("operand:against=false") ||
		f.has("problem:"+flow.KindCheckValue+":1") {
		t.Fatal("chose its own NPC's template value")
	}
	bob := npcplugin.NewDocument()
	bob.NPC.Path, bob.NPC.Behaviour = "bob", workflowID
	f.docs.Put(bobID, "Bob", bob)
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: workflowID, Name: "Daily routine", Format: flow.FormatName},
		{ID: adaID, Name: "Ada Clerk", Format: npc.FormatName}, {ID: bobID, Name: "Bob", Format: npc.FormatName},
	}))
	f.frame(noInput())
	if !f.has("problem:" + flow.KindCheckValue + ":1") {
		t.Fatal("Bob follows the workflow without the shop template")
	}
	set := withNode(t, flow.KindSetValue)
	set.withShop()
	set.click("ui:pick-value")
	set.typeText("waiting")
	if _, ok := set.state.Locate("valuerow:npc.ada.shop.waiting", set.docs); ok {
		t.Fatal("a Set value never sets another NPC's value")
	}
	if _, ok := set.state.Locate("valuerow:npc.shop.waiting", set.docs); !ok {
		t.Fatal("a Set value sets its own NPC's value")
	}
}

// TestAtItsLimit asks whether an integer is at its maximum: a problem while
// it has none, fine once it has one, and titled as a question.
func TestAtItsLimit(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindCheckValue)
	f.click("ui:pick-value")
	f.typeText("world.patience")
	f.declare("world.patience")
	f.click("ui:compare-" + flow.CompareMax)
	if !f.has("problem:" + flow.KindCheckValue + ":1") {
		t.Fatal("an integer with no maximum is never at it")
	}
	patience := f.world().Values()[0]
	patience.Max = worldstate.Limit(3)
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{
		Add: nil, Set: []worldstate.Declaration{patience}, Remove: nil,
	})
	if f.has("problem:" + flow.KindCheckValue + ":1") {
		t.Fatal("with a maximum it can be at it, with nothing to compare with")
	}
	if !f.has("title:check-value:1=Is world.patience at its maximum?") {
		t.Fatal("titled as a question")
	}
}

// TestValuePicker opens the picker from the Value box, declares from it,
// narrows by typing and picks the one match with Enter, and closes it with
// Escape and with a click outside.
func TestValuePicker(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSetValue)
	f.click("ui:pick-value")
	if !f.has("picker") {
		t.Fatal("the Value box opens the picker")
	}
	f.typeText("world.gold")
	f.declare("world.gold")
	if f.has("picker") || !f.has("value:value=world.gold") {
		t.Fatal("declaring from the picker chooses the value and closes it")
	}
	f.click("ui:pick-value")
	f.typeText("world.silver")
	f.declare("world.silver")
	f.click("ui:pick-value")
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.typeText("gol")
	f.keys("Enter")
	if f.has("picker") || !f.has("value:value=world.gold") {
		t.Fatal("Enter picks the one value that matches")
	}
	f.click("ui:pick-value")
	f.keys("Escape")
	if f.has("picker") {
		t.Fatal("Escape closes the picker")
	}
	f.click("ui:pick-value")
	f.press(image.Pt(10, 300))
	f.release(image.Pt(10, 300))
	if f.has("picker") || !f.has("value:value=world.gold") {
		t.Fatal("a click outside closes the picker and keeps the value")
	}
}

// TestComparisonsFollowTheType declares an integer and sees its symbols,
// between's second number and another value on the right; turns the value
// into a bool, whose Compare row goes and whose click mends the comparison;
// into a choice, ticked into any of or none of, an older is carried over;
// and into text, which is or is not.
func TestComparisonsFollowTheType(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindCheckValue)
	f.click("ui:pick-value")
	f.typeText("world.patience")
	f.declare("world.patience")
	has := func(widget string) bool {
		_, ok := f.state.Locate("ui:"+widget, f.docs)

		return ok
	}
	if !has("compare-"+flow.CompareNot) || !has("compare-"+flow.CompareBetween) || has("compare-"+flow.CompareAnyOf) {
		t.Fatal("an integer offers its symbols and between, never any of")
	}
	if has("operand-" + flow.ParamTo) {
		t.Fatal("between's second number waits for between")
	}
	f.click("ui:compare-" + flow.CompareBetween)
	if !has("operand-" + flow.ParamTo) {
		t.Fatal("between shows its second number")
	}
	f.click("ui:operand-" + flow.ParamTo)
	f.keys("Cmd+A", "Backspace")
	f.typeText("5")
	f.keys("Enter")
	if !f.has("operand:to=5") || !f.has("operand:against=0") || f.has("problem:"+flow.KindCheckValue+":1") ||
		!f.has("title:check-value:1=Is world.patience between 0 and 5?") {
		t.Fatalf("between 0 and 5 is whole: %q", f.status())
	}
	f.click("ui:compare-" + flow.CompareLeast)
	if f.has("another:" + flow.ParamAgainst) {
		t.Fatal("A number is on until another value is asked for")
	}
	f.click("ui:source-value")
	if _, open := f.state.Locate("valuerow:world.patience", f.docs); !f.has("another:"+flow.ParamAgainst) || open {
		t.Fatal("Another value lights as soon as it is chosen and puts up the box, closed")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.click("ui:pick-" + flow.ParamAgainst)
	if _, open := f.state.Locate("valuerow:world.patience", f.docs); !open || !f.has("another:"+flow.ParamAgainst) {
		t.Fatal("the box opens the chooser, as the Value box does")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.click("valuerow:world.patience")
	if !f.has("refers:against=world.patience") || f.has("problem:"+flow.KindCheckValue+":1") ||
		!f.has("title:check-value:1=Is world.patience ≥ world.patience?") {
		t.Fatal("at least another integer value, read when it runs")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.click("ui:source-number")
	if !f.has("operand:against=0") {
		t.Fatal("A number drops the reference for a zero to type over")
	}
	value := f.world().Values()[0]
	value.Type, value.Default = worldstate.TypeBool, []byte("false")
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{
		Add: nil, Set: []worldstate.Declaration{value}, Remove: nil,
	})
	f.frame(noInput())
	if has("compare-"+flow.CompareIs) || !has("operand-"+flow.ParamAgainst+"-true") || !f.has("problem:"+flow.KindCheckValue+":1") {
		t.Fatalf("a bool has no Compare row (%v), only true and false (%v), and at least no longer fits (%v)",
			has("compare-"+flow.CompareIs), has("operand-"+flow.ParamAgainst+"-true"), f.has("problem:"+flow.KindCheckValue+":1"))
	}
	f.click("ui:operand-" + flow.ParamAgainst + "-true")
	if !f.has("operand:against=true") || !f.has(`operand:compare="is"`) || f.has("problem:"+flow.KindCheckValue+":1") ||
		!f.has("title:check-value:1=Is world.patience true?") {
		t.Fatal("true mends the comparison to is")
	}
	value.Type, value.Default = worldstate.TypeChoice, []byte(`"c1"`)
	value.Choices = []worldstate.Choice{{ID: "c1", Name: "calm"}, {ID: "c2", Name: "cross"}, {ID: "c3", Name: "glad"}}
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{
		Add: nil, Set: []worldstate.Declaration{value}, Remove: nil,
	})
	f.frame(noInput())
	if !has("compare-"+flow.CompareAnyOf) || !has("compare-"+flow.CompareNoneOf) || has("compare-"+flow.CompareIs) ||
		!has("operand-"+flow.ParamAmong+"-c2") || has("operand-"+flow.ParamAgainst+"-c2") {
		t.Fatal("a choice is any of or none of its choices, ticked")
	}
	f.click("ui:operand-" + flow.ParamAmong + "-c1")
	f.click("ui:operand-" + flow.ParamAmong + "-c3")
	if !f.has(`operand:among=["c1","c3"]`) || !f.has(`operand:compare="`+flow.CompareAnyOf+`"`) ||
		f.has("problem:"+flow.KindCheckValue+":1") || !f.has("title:check-value:1=Is world.patience any of calm, glad?") {
		t.Fatalf("ticking two choices makes any of them: %q", f.status())
	}
	f.click("ui:operand-" + flow.ParamAmong + "-c1")
	f.click("ui:compare-" + flow.CompareNoneOf)
	if !f.has(`operand:among=["c3"]`) || !f.has(`operand:compare="`+flow.CompareNoneOf+`"`) {
		t.Fatal("unticking one and choosing none of")
	}
	f.click("ui:operand-" + flow.ParamAmong + "-c3")
	if !f.has("problem:" + flow.KindCheckValue + ":1") {
		t.Fatal("none ticked is a problem")
	}
	older := f.nth(flow.KindCheckValue, 1)
	older, _ = older.WithParam(flow.ParamCompare, flow.CompareIs)
	older, _ = older.WithParam(flow.ParamAgainst, json.RawMessage(`"c2"`))
	older, _ = older.WithParam(flow.ParamAmong, flow.NoOperand())
	f.carryOutOn(workflowID, workflowplugin.ReplaceEdit, workflowplugin.ReplaceArgs{Node: older})
	f.frame(noInput())
	if f.has("problem:" + flow.KindCheckValue + ":1") {
		t.Fatal("a choice checked with is, from before any of, still fits")
	}
	f.click("ui:operand-" + flow.ParamAmong + "-c1")
	if !f.has(`operand:among=["c1","c2"]`) || !f.has(`operand:compare="`+flow.CompareAnyOf+`"`) {
		t.Fatal("ticking another keeps the older choice and becomes any of")
	}
	value.Type, value.Default, value.Choices = worldstate.TypeText, []byte(`""`), nil
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{
		Add: nil, Set: []worldstate.Declaration{value}, Remove: nil,
	})
	f.frame(noInput())
	if !has("compare-"+flow.CompareNot) || has("compare-"+flow.CompareLeast) || !has("operand-"+flow.ParamAgainst) {
		t.Fatal("text is or is not typed text")
	}
	f.click("ui:compare-" + flow.CompareNot)
	if !f.has(`operand:compare="` + flow.CompareNot + `"`) {
		t.Fatal("is not is saved")
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
	f.declare("local.customer")
	locals := f.workflow().Locals().Values()
	if len(locals) != 1 || locals[0].Name != "customer" || locals[0].Type != worldstate.TypeNPC {
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

// TestTextOnANode shows text on a node once, in curly quotes, as typed,
// cut when long, and empty text as empty.
func TestTextOnANode(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindCheckValue)
	f.carryOutOn(worldstate.ID(), stateplugin.ChangeEdit, stateplugin.ChangeArgs{Add: []worldstate.Declaration{
		worldstate.New(frametest.ID(60), "motto", worldstate.ScopeWorld, worldstate.TypeText),
	}, Set: nil, Remove: nil})
	f.click("ui:pick-value")
	f.click("valuerow:world.motto")
	for typed, title := range map[string]string{
		`""`:                              "Is world.motto empty?",
		`"Ada"`:                           "Is world.motto “Ada”?",
		`"Say \"hi\" to the clerk today"`: "Is world.motto “Say \"hi\" to the clerk t…”?",
	} {
		node, err := f.nth(flow.KindCheckValue, 1).WithParam(flow.ParamAgainst, json.RawMessage(typed))
		if err != nil {
			t.Fatal(err)
		}
		f.carryOutOn(workflowID, workflowplugin.ReplaceEdit, workflowplugin.ReplaceArgs{Node: node})
		f.frame(noInput())
		if !f.has("title:check-value:1=" + title) {
			t.Errorf("%s reads %s", typed, title)
		}
	}
}
