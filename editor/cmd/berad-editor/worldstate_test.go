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

// TestShopkeeperServesACustomer builds a shop from nodes alone. The
// customer's workflow looks for a clerk serving within two tiles: found, it
// stops waiting; none, it waits. The clerk's looks for a waiting customer
// within two tiles and serves it. Both declare their values from the
// choosers. Placed side by side and previewed, the Watch window shows the
// customer no longer waiting, keeping the clerk it found: it stops only on
// finding a clerk serving, which the clerk is only once it has found the
// customer waiting.
func TestShopkeeperServesACustomer(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	customerShot := shotPath(t, "shop-customer.png")
	mapShot, flowShot := shotPath(t, "shop-map.png"), shotPath(t, "shop-workflow.png")
	retype := func(field, text string) []step {
		return script(click("ui:"+field), wait(3), keys("Cmd+A"), typed(text), keys("Enter"), wait(10))
	}
	choose := func(param, name, row string) []step {
		return script(click("ui:pick-"+param), wait(5), typed(name), must(row, 1800), click(row), wait(10))
	}
	pause := func() []step {
		return script(click("palette:wait"), wait(10), retype("param-duration", "1"), click("ui:duration-seconds"), wait(10))
	}
	browserScript := script(
		openZone(), wait(30), click("ui:new-workflow"), wait(30), click("ui:open"), wait(60),
		click("ui:new-workflow"), wait(30), click("ui:open"), wait(60),
		click("ui:new-npc"), wait(30), click("ui:choose-behaviour"), wait(10), click("row:Workflow 1"), wait(5),
		click("ui:use"), wait(30),
		click("ui:new-npc"), wait(30), click("ui:choose-behaviour"), wait(10), click("row:Workflow 2"), wait(5),
		click("ui:use"), wait(30), quitWhenAlone(),
	)
	customerScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:find-npc"), must("state:selected:find-npc", 300),
		choose("where", "serving", "declare:npc"), must("state:value:where=npc.serving", 300),
		retype("operand-is", "1"), must("state:operand:is=1", 300),
		choose("into", "clerk", "declare:local"), must("state:value:into=local.clerk", 300),
		click("palette:set-value"), must("state:selected:set-value", 300),
		choose("value", "waiting", "declare:npc"), must("state:operand:to=0", 300), pause(),
		click("node:find-npc:1"), wait(5), click("palette:set-value"), must("state:selected:set-value", 300),
		choose("value", "waiting", "valuerow:npc.waiting"), retype("operand-to", "1"), must("state:operand:to=1", 300),
		pause(), click("ui:arrange"), wait(20), shoot(flowShot), wait(10), quit(),
	)
	clerkScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:find-npc"), must("state:selected:find-npc", 300),
		choose("where", "waiting", "valuerow:npc.waiting"), retype("operand-is", "1"), must("state:operand:is=1", 300),
		choose("into", "customer", "declare:local"), must("state:value:into=local.customer", 300),
		click("palette:set-value"), must("state:selected:set-value", 300),
		choose("value", "serving", "valuerow:npc.serving"), retype("operand-to", "1"), must("state:operand:to=1", 300),
		pause(), click("node:find-npc:1"), wait(5), pause(), wait(10), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("N"), must("npc:NPC 2", 3600), click("npc:NPC 1"), wait(5),
		click("cell:1,1"), must("state:placement:1,1=NPC 1", 300), click("npc:NPC 2"), wait(5),
		click("cell:2,1"), must("state:placement:2,1=NPC 2", 300), wait(900),
		keys("P"), click("cell:2,-2"), must("state:previewing", 300), wait(30),
		click("live-npc:NPC 1"), wait(5), click("live-npc:NPC 1"), must("state:watching", 300), wait(10),
		click("ui:watch"), wait(400), shoot(mapShot), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 1800), must("state:watching:NPC 1", 600),
		must("state:value:npc.waiting=0", 600), must("state:value:local.clerk=NPC 2", 300), wait(30),
		shoot(customerShot), wait(10), quit(),
	)
	clockScript := script(must("state:running", 3600), wait(30), quit())
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "workflow": {customerScript, clerkScript}, "map": {mapScript},
		"watch": {watchScript}, "clock": {clockScript},
	})
	noCrashes(t, project)
	state := readEnvelope(t, project, "world-states/*.json")
	decoded, err := stateplugin.Decode(state.Data)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, value := range decoded.(*stateplugin.Document).State.Values.Values() {
		paths = append(paths, value.Path())
	}
	if len(paths) != 2 || paths[0] != "npc.serving" || paths[1] != "npc.waiting" {
		t.Fatalf("the world state holds %v", paths)
	}
	t.Logf("screenshots: %s %s %s", flowShot, mapShot, customerShot)
}
