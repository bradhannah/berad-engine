package main_test

import (
	"path/filepath"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestWorldStateMovesAndFinds declares world.done from a workflow's Set
// value, then in World state's Values window moves it by its right-click menu
// into a new group, asked first as the workflow uses it, and names the
// group progress; adds a bool named ready from progress's menu; finds done's
// type locked; adds a value in progress and drags it out onto World;
// finds done by
// Enter; and declares world.shop.open_hour from Find. The saved world state
// holds them there, and the Set value still sets done, its box showing the
// path it moved to.
func TestWorldStateMovesAndFinds(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	menuShot, askShot, findShot, declareShot := shotPath(t, "state-menu.png"), shotPath(t, "state-move-ask.png"),
		shotPath(t, "state-find.png"), shotPath(t, "state-find-declare.png")
	addShot, lockShot := shotPath(t, "state-group-menu.png"), shotPath(t, "state-type-locked.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), wait(60),
		doubleClick("row:Values"), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), addNode(""+flow.KindSetValue), must("state:selected:set-value", 300),
		click("ui:pick-value"), must("state:picker", 120), typed("done"), wait(5), must("declare:world.done", 120),
		click("declare:world.done"), wait(5), click("ui:picker-declare"), must("state:value:value=world.done", 300),
		must("state:value:value=world.progress.done", 3600), wait(10), quit(),
	)
	stateScript := script(
		must("state:loaded", 600), must("row:world.done", 1200), wait(10),
		rightClick("row:world.done"), wait(5), click("menu:move-into"), wait(5), must("menu:into-new", 60),
		shoot(menuShot), click("menu:into-new"), must("dialog:move", 300), wait(5), shoot(askShot),
		click("dialog:move"), must("state:selected=world.group", 300), wait(5),
		keys("Cmd+A"), typed("progress"), keys("Enter"), must("row:world.progress.done", 300), wait(10),
		rightClick("row:world.progress"), wait(5), click("menu:add-value"), must("menu:add:bool", 60), wait(5),
		shoot(addShot), click("menu:add:bool"), must("state:selected=world.progress.value", 300), wait(5),
		typed("ready"), keys("Enter"), must("row:world.progress.ready", 300), wait(10),
		click("row:world.progress.done"), wait(30), click("ui:value-type-bool"), wait(10), shoot(lockShot),
		click("ui:add-value"), must("state:selected=world.progress.value", 300), wait(10),
		drag("row:world.progress.value", "section:world", "section:world"), wait(5),
		must("row:world.value", 300), wait(10),
		keys("Cmd+F"), must("state:find", 120), typed("done"), must("valuerow:world.progress.done", 120), wait(10),
		shoot(findShot), keys("Enter"), must("state:selected=world.progress.done", 300), wait(10),
		keys("Cmd+F"), must("state:find", 120), typed("world.shop.open_hour"),
		must("declare:world.shop.open_hour", 120), wait(10), shoot(declareShot), click("ui:picker-declare"),
		must("state:selected=world.shop.open_hour", 300), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "workflow": {workflowScript}, "world-state": {stateScript},
	})
	noCrashes(t, project)
	decoded, err := stateplugin.Decode(readEnvelope(t, project, "world-states/*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	values := decoded.(*stateplugin.Document).State.Values
	progress, grouped := values.Find(worldstate.ScopeWorld, ids.ULID{}, "progress")
	done, moved := values.Find(worldstate.ScopeWorld, progress.ID, "done")
	_, dragged := values.Find(worldstate.ScopeWorld, ids.ULID{}, "value")
	shop, _ := values.Find(worldstate.ScopeWorld, ids.ULID{}, "shop")
	_, declared := values.Find(worldstate.ScopeWorld, shop.ID, "open_hour")
	ready, added := values.Find(worldstate.ScopeWorld, progress.ID, "ready")
	if !grouped || !moved || !dragged || !declared || !added || ready.Type != worldstate.TypeBool ||
		done.Type != worldstate.TypeInteger {
		t.Fatalf("the world state holds progress.done, still an integer, progress.ready, value and shop.open_hour: %+v",
			values.Values())
	}
	logic, err := workflowplugin.Decode(readEnvelope(t, project, "workflows/workflow-1--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range logic.(*workflowplugin.Document).Workflow.Nodes() {
		if ref, refErr := node.Ref(flow.ParamValue); node.Kind == flow.KindSetValue && (refErr != nil ||
			ref.Value != done.ID) {
			t.Fatalf("the Set value still sets done: %+v %v", ref, refErr)
		}
	}
	t.Logf("screenshots: %s %s %s %s %s %s", menuShot, askShot, findShot, declareShot, addShot, lockShot)
}
