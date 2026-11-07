package workflowview_test

import (
	"encoding/json"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

var (
	fetchID     = frametest.ID(70)
	shelfWaitID = frametest.ID(71)
	ovenID      = frametest.ID(72)
)

// withAnotherWorkflow adds Fetch stock, with a Wait at the shelf typed as
// 30 minutes, a Say typed as "Fresh bread!", and a Wait for the oven typed
// as 45 minutes.
func (f *fixture) withAnotherWorkflow() {
	f.t.Helper()
	data, err := workflowplugin.Prepare(workflowplugin.NewData(), func() (ids.ULID, error) { return frametest.ID(73), nil })
	if err != nil {
		f.t.Fatal(err)
	}
	document, err := workflowplugin.Decode(data)
	if err != nil {
		f.t.Fatal(err)
	}
	shelf, err := flow.NewNode(flow.KindWait)
	if err != nil {
		f.t.Fatal(err)
	}
	shelf.ID = shelfWaitID
	oven, err := flow.NewNode(flow.KindWait)
	if err != nil {
		f.t.Fatal(err)
	}
	oven.ID = ovenID
	if oven, err = oven.WithParam("duration", flow.Duration{
		Amount: 45, Unit: flow.UnitMinutes, Clock: flow.ClockGame, Constant: flow.NoConstant(),
	}); err != nil {
		f.t.Fatal(err)
	}
	say, err := flow.NewNode(flow.KindSay)
	if err != nil {
		f.t.Fatal(err)
	}
	say.ID = frametest.ID(74)
	if say, err = say.WithParam(flow.ParamMessage, []flow.Segment{{Text: "Fresh bread! Come in.", Ref: nil}}); err != nil {
		f.t.Fatal(err)
	}
	args, err := json.Marshal(workflowplugin.AddArgs{Nodes: []flow.Node{shelf, oven, say}, Boxes: nil, Links: nil})
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := (workflowplugin.Add{}).Apply(document, args); err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(fetchID, "Fetch stock", document)
	f.docs.View(project.IndexID(), func(document client.Document) {
		index := document.Model.(*project.Index)
		f.docs.Put(project.IndexID(), "Project", project.NewIndex(append(index.Entries(), project.Entry{
			ID: fetchID, Name: "Fetch stock", Format: flow.FormatName,
		})))
	})
}

// nodeIn reads a node of another workflow.
func (f *fixture) nodeIn(workflow, node ids.ULID) flow.Node {
	var found flow.Node
	f.docs.View(workflow, func(document client.Document) {
		found, _ = document.Model.(*workflowplugin.Document).Workflow.Node(node)
	})

	return found
}

// TestMakeAConstantFromAWait makes the restock time a constant from a
// Wait's 30 minutes, which then stands for them, and switches the other
// Wait typed as 30 minutes to it, leaving the oven's 45; one gesture makes
// them all.
func TestMakeAConstantFromAWait(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindWait)
	f.withAnotherWorkflow()
	f.click("ui:constant-duration")
	f.typeText("shop.restock_minutes")
	f.click("declare:const.world.shop.restock_minutes")
	f.click("ui:picker-declare")
	declared, found := f.world().Find(worldstate.ScopeWorld, ids.ULID{}, "shop")
	if !found {
		t.Fatalf("the shop group is declared: %q", f.status())
	}
	restock, found := f.world().Find(worldstate.ScopeWorld, declared.ID, "restock_minutes")
	if !found || !restock.IsConstant() || restock.Zero().Integer != 30 {
		t.Fatalf("a constant starting at the 30 typed: %+v %q", restock, f.status())
	}
	if !f.has("constant:duration=const.world.shop.restock_minutes") {
		t.Fatal("it stands for the Wait's amount")
	}
	if !f.has("others=1") {
		t.Fatal("the other Wait typed as 30 minutes is offered, and not the oven's 45")
	}
	f.click("ui:other-1")
	f.click("ui:other-use")
	shelf, err := f.nodeIn(fetchID, shelfWaitID).Duration("duration")
	if err != nil || shelf.Constant.Value != restock.ID {
		t.Fatalf("the shelf's Wait uses it too: %+v %v", shelf, err)
	}
	if oven, _ := f.nodeIn(fetchID, ovenID).Duration("duration"); !oven.Constant.IsZero() {
		t.Fatal("the oven's Wait is left as typed")
	}
	gestures := map[string]bool{}
	for _, command := range f.requests.Commands[len(f.requests.Commands)-2:] {
		gestures[command.Command.Gesture] = true
	}
	if len(gestures) != 1 || gestures[""] {
		t.Fatalf("one gesture, so one Undo takes back the constant and the other places: %v", gestures)
	}
	if !f.has("unsaved=1") {
		t.Fatal("this Wait's own change waits for Save")
	}
	f.click("ui:save")
	if wait, _ := f.nth(flow.KindWait, 1).Duration("duration"); wait.Constant.Value != restock.ID {
		t.Fatalf("Save keeps it: %+v", wait)
	}
}

// TestAConstantFromANumberTyped makes a constant from minutes typed and not
// saved: it starts at them, the Wait keeps them, and the two count as one
// change. Revert then drops both.
func TestAConstantFromANumberTyped(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindWait)
	f.typeOnly("param-duration", "45")
	f.click("ui:constant-duration")
	f.typeText("tide_minutes")
	f.click("declare:const.world.tide_minutes")
	f.click("ui:picker-declare")
	tide, found := f.world().Find(worldstate.ScopeWorld, ids.ULID{}, "tide_minutes")
	if !found || tide.Zero().Integer != 45 {
		t.Fatalf("the constant starts at the 45 typed: %+v %q", tide, f.status())
	}
	if !f.has("constant:duration=const.world.tide_minutes") || !f.has("unsaved=1") {
		t.Fatal("it stands for the minutes typed: one change waiting for Save")
	}
	f.click("ui:revert")
	if !f.has("constant:duration=") || !f.has("unsaved=0") {
		t.Fatal("Revert drops the constant chosen and the minutes typed")
	}
	f.typeOnly("param-duration", "45")
	f.click("ui:constant-duration")
	f.click("valuerow:const.world.tide_minutes")
	f.click("ui:save")
	wait, err := f.nth(flow.KindWait, 1).Duration("duration")
	if err != nil || wait.Constant.Value != tide.ID || wait.Amount != 45 || !f.has("unsaved=0") {
		t.Fatalf("Save keeps the constant and the minutes typed under it: %+v %v", wait, err)
	}
}

// TestMakeAConstantFromWords selects words typed in a Say's line, makes
// them a text constant, which takes their place, and switches the other
// line with the same words to it.
func TestMakeAConstantFromWords(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSay)
	f.withAnotherWorkflow()
	f.typeInto("param-message", "Fresh bread! Come in.")
	f.click("ui:param-message")
	f.keys("Cmd+A")
	for range len(" Come in.") {
		f.keys("Shift+Left")
	}
	f.click("ui:make-constant")
	f.typeText("bread_call")
	f.click("declare:const.world.bread_call")
	f.click("ui:picker-declare")
	call, found := f.world().Find(worldstate.ScopeWorld, ids.ULID{}, "bread_call")
	if !found || !call.IsConstant() || call.Zero().Text != "Fresh bread!" {
		t.Fatalf("a text constant from the words selected: %+v %q", call, f.status())
	}
	if !f.has("line={const.world.bread_call} Come in.") || !f.has("unsaved=1") {
		t.Fatal("it takes the words' place, waiting for Save")
	}
	if !f.has("others=1") {
		t.Fatal("the other line with the same words is offered")
	}
	f.click("ui:other-1")
	f.click("ui:other-use")
	segments, err := f.nodeIn(fetchID, frametest.ID(74)).Message()
	if err != nil || len(segments) != 2 || segments[0].Ref == nil || segments[1].Text != " Come in." {
		t.Fatalf("the other line uses it too: %+v %v", segments, err)
	}
	f.click("ui:save")
	if !f.has("message={const.world.bread_call} Come in.") {
		t.Fatal("Save keeps this line's")
	}
}
