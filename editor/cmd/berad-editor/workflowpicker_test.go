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

// TestWorkflowValuePicker declares world.quests.dragon_slain from a Set
// value's picker, a bool starting true, and world.gold by Enter; opens a
// Check value's picker by Enter, opens the world.quests group, searches for
// gold and picks it. The saved world state holds both values, and the saved
// logic checks world.gold.
func TestWorkflowValuePicker(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	declareShot, treeShot, searchShot := shotPath(t, "picker-declare.png"), shotPath(t, "picker-tree.png"),
		shotPath(t, "picker-search.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:"+flow.KindSetValue), must("state:selected:set-value", 300),
		click("ui:pick-value"), must("state:picker", 120), typed("world.quests.dragon_slain"), wait(10),
		must("declare:world.quests.dragon_slain", 120), click("declare:world.quests.dragon_slain"), wait(5),
		click("ui:declare-type-bool"), wait(5), click("ui:declare-start-true"), wait(10), shoot(declareShot), wait(5),
		click("ui:picker-declare"), must("state:value:value=world.quests.dragon_slain", 300), wait(10),
		click("ui:pick-value"), must("state:picker", 120), typed("world.gold"), wait(10), keys("Enter"),
		must("state:value:value=world.gold", 300), wait(10),
		click("palette:"+flow.KindCheckValue), must("state:selected:check-value", 300), wait(5),
		keys("Enter"), must("state:picker", 120), must("picker:group:world.quests", 120),
		click("picker:group:world.quests"), must("valuerow:world.quests.dragon_slain", 120), wait(10),
		shoot(treeShot), wait(5),
		typed("gold"), must("valuerow:world.gold", 120), wait(10), shoot(searchShot), wait(5),
		click("valuerow:world.gold"), must("state:value:value=world.gold", 300), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	decodedState, err := stateplugin.Decode(readEnvelope(t, project, "world-states/*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	values := decodedState.(*stateplugin.Document).State.Values
	quests, grouped := values.Find(worldstate.ScopeWorld, ids.ULID{}, "quests")
	slain, slainFound := values.Find(worldstate.ScopeWorld, quests.ID, "dragon_slain")
	gold, goldFound := values.Find(worldstate.ScopeWorld, ids.ULID{}, "gold")
	if !grouped || !slainFound || slain.Type != worldstate.TypeBool || string(slain.Default) != "true" ||
		!goldFound || gold.Type != worldstate.TypeInteger {
		t.Fatalf("the world state holds world.quests.dragon_slain, true, and world.gold: %+v %+v", slain, gold)
	}
	decoded, err := workflowplugin.Decode(readEnvelope(t, project, "workflows/workflow-1--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range decoded.(*workflowplugin.Document).Workflow.Nodes() {
		if ref, refErr := node.Ref(flow.ParamValue); node.Kind == flow.KindCheckValue && (refErr != nil ||
			ref.Value != gold.ID) {
			t.Fatalf("the saved Check value checks world.gold: %+v %v", ref, refErr)
		}
	}
	t.Logf("screenshots: %s %s %s", declareShot, treeShot, searchShot)
}
