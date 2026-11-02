package workflowview_test

import (
	"encoding/json"
	"image"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

var (
	moduleID  = frametest.ID(70)
	placeIn   = frametest.ID(71)
	opensAt   = frametest.ID(72)
	triesOut  = frametest.ID(73)
	arrivedID = frametest.ID(74)
	closedID  = frametest.ID(75)
)

// withModule adds Go to work, a workflow to call: its inputs place and
// opens_at, which starts at 9, its output tries, and its ways out Arrived
// and Closed.
func (f *fixture) withModule() {
	f.t.Helper()
	module, err := flow.New(frametest.ID(76))
	if err != nil {
		f.t.Fatal(err)
	}
	place := worldstate.New(placeIn, "place", worldstate.ScopeLocal, worldstate.TypePlace)
	opens := worldstate.New(opensAt, "opens_at", worldstate.ScopeLocal, worldstate.TypeInteger)
	opens.Default = json.RawMessage("9")
	tries := worldstate.New(triesOut, "tries", worldstate.ScopeLocal, worldstate.TypeInteger)
	place.Role, opens.Role, tries.Role = worldstate.RoleInput, worldstate.RoleInput, worldstate.RoleOutput
	if err := module.Locals().InsertAll([]worldstate.Declaration{place, opens, tries}); err != nil {
		f.t.Fatal(err)
	}
	arrived := returnNode(f.t, arrivedID, "Arrived", "w1")
	closed := returnNode(f.t, closedID, "Closed", "w2")
	if err := module.Insert([]flow.Node{arrived, closed}, nil, nil); err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(moduleID, "Go to work", &workflowplugin.Document{Workflow: module})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: workflowID, Name: "Daily routine", Format: flow.FormatName},
		{ID: moduleID, Name: "Go to work", Format: flow.FormatName},
		{ID: floorID, Name: "Office", Format: floor.FormatName},
		{ID: adaID, Name: "Ada Clerk", Format: npc.FormatName},
	}))
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// returnNode is a Return of a way out.
func returnNode(t *testing.T, nodeID ids.ULID, label, port string) flow.Node {
	t.Helper()
	made, err := flow.NewNode(flow.KindReturn)
	if err == nil {
		made, err = made.WithParam(flow.ParamWayName, label)
	}
	if err == nil {
		made, err = made.WithParam(flow.ParamWayPort, port)
	}
	if err != nil {
		t.Fatal(err)
	}
	made.ID = nodeID

	return made
}

// module returns Go to work as it is now.
func (f *fixture) module() *flow.Workflow {
	f.t.Helper()
	var found *flow.Workflow
	f.docs.View(moduleID, func(document client.Document) {
		found = document.Model.(*workflowplugin.Document).Workflow
	})

	return found
}

// withCall adds a Call after the start, calling Go to work.
func (f *fixture) withCall() {
	f.t.Helper()
	f.withModule()
	f.click("node:start:1")
	f.click("palette:" + flow.KindCall)
	f.click("ui:call-workflow")
	f.click("menu:workflow:" + moduleID.String())
}

// TestCallingAWorkflow chooses the workflow a Call calls, which gives it an
// output for each way out and done; types a number for one input, picks a
// place on a map for another and gives the first another value instead;
// keeps an output; and opens the module from the inspector and by a
// double-click on the node.
func TestCallingAWorkflow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withCall()
	call := f.nth(flow.KindCall, 1)
	if called, ok := call.Called(); !ok || called != moduleID {
		t.Fatalf("the Call calls Go to work: %v", call.Params)
	}
	names := []string{}
	for _, output := range flow.Outputs(call) {
		names = append(names, output.Label)
	}
	if !slices.Equal(names, []string{"Arrived", "Closed", "done"}) {
		t.Fatalf("one output per way out, then done: %v", names)
	}
	f.typeInto("input-opens_at", "10")
	if given := f.nth(flow.KindCall, 1).Inputs()[opensAt]; string(given) != "10" {
		t.Fatalf("opens_at is given 10: %s", given)
	}
	f.click("ui:input-place-pick")
	f.answerPick()
	given := f.nth(flow.KindCall, 1).Inputs()[placeIn]
	var place worldstate.Place
	if err := json.Unmarshal(given, &place); err != nil || place != (worldstate.Place{Floor: floorID, Anchor: deskID}) {
		t.Fatalf("place is given the desk: %s", given)
	}
	f.keys("Escape")
	f.click("ui:add-local")
	f.click("node:call-workflow:1")
	f.click("ui:input-opens_at-another")
	f.click("valuerow:local.value")
	if ref, refers := flow.OperandRef(f.nth(flow.KindCall, 1).Inputs()[opensAt]); !refers || ref.Scope != worldstate.ScopeLocal {
		t.Fatalf("opens_at is given another value: %s", f.nth(flow.KindCall, 1).Inputs()[opensAt])
	}
	f.click("ui:keep-tries")
	f.click("valuerow:local.value")
	if kept := f.nth(flow.KindCall, 1).Keeps()[triesOut]; kept.Scope != worldstate.ScopeLocal {
		t.Fatalf("tries is kept in local.value: %+v", kept)
	}
	f.click("ui:keep-tries-leave")
	if len(f.nth(flow.KindCall, 1).Keeps()) != 0 {
		t.Fatal("Leave it keeps it nowhere")
	}
	f.click("ui:call-open")
	opened := f.requests.Opened
	if len(opened) != 1 || opened[0].Target != moduleID || opened[0].View != workflowview.ViewName {
		t.Fatalf("Open opens Go to work: %+v", opened)
	}
	at := f.at("node:call-workflow:1")
	f.press(at)
	f.release(at)
	input := ui.NewInput(at)
	input.Pressed, input.Down, input.DoubleClick = true, true, true
	f.frame(input)
	if len(f.requests.Opened) != 2 {
		t.Fatalf("a double-click opens it too: %+v", f.requests.Opened)
	}
}

// TestACallsLinesFollowItsModule draws a line from a way out, keeps it
// when the way out is renamed, strikes it through when the module loses
// it, and shows a way out added to the module at once.
func TestACallsLinesFollowItsModule(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withCall()
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:700,400"))
	f.drag(f.at("port:call-workflow:1:w2"), f.at("node:wait:1"))
	call := f.nth(flow.KindCall, 1)
	if call.Next["w2"] != f.nth(flow.KindWait, 1).ID {
		t.Fatalf("Closed leads to the wait: %v", call.Next)
	}
	f.renameWay(closedID, "Shut")
	if !f.has("output:call-workflow:1:w2=Shut") {
		t.Fatal("a renamed way out keeps its line and shows its new name")
	}
	f.carryOutOn(moduleID, workflowplugin.RemoveEdit, workflowplugin.RemoveArgs{Nodes: []ids.ULID{closedID}})
	if !f.has("missing:call-workflow:1:w2") {
		t.Fatal("a way out the module lost keeps its line, struck through")
	}
	added := returnNode(t, frametest.ID(77), "Lunch", "w3")
	f.carryOutOn(moduleID, workflowplugin.AddEdit, workflowplugin.AddArgs{
		Nodes: []flow.Node{added}, Boxes: map[ids.ULID]flow.Box{}, Links: nil,
	})
	if !f.has("output:call-workflow:1:w3=Lunch") {
		t.Fatal("a way out added to the module shows at once")
	}
	f.drag(f.at("port:call-workflow:1:w3"), f.at("node:wait:1"))
	ways, err := f.nth(flow.KindCall, 1).Ways()
	if err != nil || len(ways) != 3 || f.nth(flow.KindCall, 1).Next["w3"].IsZero() {
		t.Fatalf("drawing a line from it saves the ways out with it: %v %v", ways, err)
	}
}

// renameWay renames a Return of Go to work.
func (f *fixture) renameWay(nodeID ids.ULID, label string) {
	f.t.Helper()
	node, ok := f.module().Node(nodeID)
	if !ok {
		f.t.Fatal("no such Return")
	}
	renamed, err := node.WithParam(flow.ParamWayName, label)
	if err != nil {
		f.t.Fatal(err)
	}
	f.carryOutOn(moduleID, workflowplugin.ReplaceEdit, workflowplugin.ReplaceArgs{Node: renamed})
}

// TestCallProblems raises and mends each of a Call's problems: no workflow
// chosen, a way out the module lost, a module that calls back, an input
// whose type changed and a module deleted.
func TestCallProblems(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withModule()
	f.click("node:start:1")
	f.click("palette:" + flow.KindCall)
	if !f.has("problem:call-workflow:1=No workflow chosen yet.") {
		t.Fatal("a Call with no workflow is flagged")
	}
	f.click("ui:call-workflow")
	f.click("menu:workflow:" + moduleID.String())
	if f.has("problem:call-workflow:1") {
		t.Fatal("calling Go to work is fine")
	}
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:700,400"))
	f.drag(f.at("port:call-workflow:1:w2"), f.at("node:wait:1"))
	f.carryOutOn(moduleID, workflowplugin.RemoveEdit, workflowplugin.RemoveArgs{Nodes: []ids.ULID{closedID}})
	if !f.has("problem:call-workflow:1=Go to work has no way out called Closed any more. Remove the line, " +
		"or add the Return back.") {
		t.Fatal("a way out the module lost is flagged")
	}
	f.carryOutOn(moduleID, workflowplugin.AddEdit, workflowplugin.AddArgs{
		Nodes: []flow.Node{returnNode(t, closedID, "Closed", "w2")}, Boxes: map[ids.ULID]flow.Box{}, Links: nil,
	})
	if f.has("problem:call-workflow:1") {
		t.Fatal("putting the Return back mends it")
	}
	back := frametest.ID(78)
	callsBack, err := flow.NewNode(flow.KindCall)
	if err == nil {
		callsBack, err = callsBack.WithParam(flow.ParamWorkflow, workflowID.String())
	}
	if err != nil {
		t.Fatal(err)
	}
	callsBack.ID = back
	f.carryOutOn(moduleID, workflowplugin.AddEdit, workflowplugin.AddArgs{
		Nodes: []flow.Node{callsBack}, Boxes: map[ids.ULID]flow.Box{}, Links: nil,
	})
	if !f.has("problem:call-workflow:1=Calls itself: Daily routine calls Go to work, which calls Daily routine. " +
		"Choose another workflow.") {
		t.Fatal("a module that calls back is flagged")
	}
	f.carryOutOn(moduleID, workflowplugin.RemoveEdit, workflowplugin.RemoveArgs{Nodes: []ids.ULID{back}})
	f.click("node:call-workflow:1")
	f.typeInto("input-opens_at", "10")
	opens, _ := f.module().Locals().Value(opensAt)
	text := worldstate.New(opens.ID, opens.Name, worldstate.ScopeLocal, worldstate.TypeText)
	text.Role = worldstate.RoleInput
	f.carryOutOn(moduleID, workflowplugin.ChangeLocalsEdit, stateplugin.ChangeArgs{
		Add: nil, Set: []worldstate.Declaration{text}, Remove: nil,
	})
	if !f.has("problem:call-workflow:1=The input opens_at is now text; 10 no longer fits. Type text, " +
		"or choose another value.") {
		t.Fatal("an input whose type changed is flagged")
	}
	f.typeInto("input-opens_at", "ten")
	if f.has("problem:call-workflow:1") {
		t.Fatal("typing text mends it")
	}
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: workflowID, Name: "Daily routine", Format: flow.FormatName},
	}))
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("problem:call-workflow:1=The workflow it calls was deleted. Choose another workflow.") {
		t.Fatal("a deleted module is flagged")
	}
}
