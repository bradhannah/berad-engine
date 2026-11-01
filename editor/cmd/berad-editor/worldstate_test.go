package main_test

import (
	"path/filepath"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestWorldStateDeclaresValues opens the World state window from the
// project browser, declares an NPC value, names it and makes it a list of
// choices: the saved world state holds npc.mood.
func TestWorldStateDeclaresValues(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "world-state.png")
	browserScript := script(must("row:World state", 600), wait(5), doubleClick("row:World state"), quitWhenAlone())
	stateScript := script(
		must("state:loaded", 600), wait(5), click("scope:npc"), wait(5), click("ui:add-value"),
		must("state:selected=npc.value", 300), wait(5), click("ui:value-name"), keys("Cmd+A", "Backspace"),
		typed("mood"), keys("Enter"), must("state:selected=npc.mood", 300), wait(5),
		click("ui:value-type-choice"), must("state:value=npc.mood:choice:first", 300), wait(30),
		shoot(shot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "world-state": {stateScript}})
	noCrashes(t, project)
	saved := readEnvelope(t, project, "world-states/*.json")
	decoded, err := stateplugin.Decode(saved.Data)
	if err != nil {
		t.Fatal(err)
	}
	values := decoded.(*stateplugin.Document).State.Values.Values()
	if saved.ID != worldstate.ID() || len(values) != 1 || values[0].Path() != "npc.mood" ||
		values[0].Type != worldstate.TypeChoice || len(values[0].Choices) != 2 {
		t.Fatalf("the world state was saved as %+v", values)
	}
	t.Logf("screenshot: %s", shot)
}

// TestValueNodesInAWorkflow builds a Set value and a Check value in a real
// Workflow: the Set value declares npc.served from its chooser and sets it
// to 1, and the Check value checks it; then, with nothing selected, a local
// value is added. The world state holds npc.served and the workflow the
// local and both nodes.
func TestValueNodesInAWorkflow(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	setShot, checkShot, localShot := shotPath(t, "set-value.png"), shotPath(t, "check-value.png"),
		shotPath(t, "locals.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:set-value"), must("state:selected:set-value", 300),
		click("ui:pick-value"), wait(5), typed("served"), wait(5), must("declare:npc", 120), click("declare:npc"),
		must("state:value:value=npc.served", 300), wait(5),
		click("ui:operand-to"), keys("Cmd+A"), typed("1"), keys("Enter"), must("state:operand:to=1", 300), wait(10),
		shoot(setShot),
		click("palette:check-value"), must("state:selected:check-value", 300),
		click("ui:pick-value"), wait(5), must("valuerow:npc.served", 120), click("valuerow:npc.served"),
		must("state:value:value=npc.served", 300), wait(10), shoot(checkShot),
		keys("Escape"), wait(5), click("ui:add-local"), must("state:local=value:number:0", 300), wait(10),
		shoot(localShot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	state := readEnvelope(t, project, "world-states/*.json")
	decodedState, err := stateplugin.Decode(state.Data)
	if err != nil {
		t.Fatal(err)
	}
	values := decodedState.(*stateplugin.Document).State.Values.Values()
	if len(values) != 1 || values[0].Path() != "npc.served" {
		t.Fatalf("the world state holds %+v", values)
	}
	logic := readEnvelope(t, project, "workflows/workflow-1--*[^t].json")
	decoded, err := workflowplugin.Decode(logic.Data)
	if err != nil {
		t.Fatal(err)
	}
	routine := decoded.(*workflowplugin.Document).Workflow
	kinds := map[string]int{}
	for _, node := range routine.Nodes() {
		kinds[node.Kind]++
	}
	if kinds[flow.KindSetValue] != 1 || kinds[flow.KindCheckValue] != 1 || routine.Locals().Len() != 1 {
		t.Fatalf("the workflow holds %v and %d locals", kinds, routine.Locals().Len())
	}
	t.Logf("screenshots: %s %s %s", setShot, checkShot, localShot)
}
