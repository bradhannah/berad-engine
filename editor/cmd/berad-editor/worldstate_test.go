package main_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
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
	mood := slices.IndexFunc(values, func(value worldstate.Declaration) bool { return value.Path() == "npc.mood" })
	if saved.ID != worldstate.ID() || mood < 0 || values[mood].Type != worldstate.TypeChoice ||
		len(values[mood].Choices) != 2 {
		t.Fatalf("the world state was saved as %+v", values)
	}
	t.Logf("screenshot: %s", shot)
}

// TestValueNodesInAWorkflow builds a Set value and a Check value in a real
// Workflow: the Set value declares npc.done from its chooser and sets it
// to 1, and the Check value checks it; then, with nothing selected, a local
// value is added. The world state holds npc.done and the workflow the
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
		click("ui:pick-value"), wait(5), typed("done"), wait(5), must("declare:npc", 120), click("declare:npc"),
		must("state:value:value=npc.done", 300), wait(5),
		click("ui:operand-to"), keys("Cmd+A"), typed("1"), keys("Enter"), must("state:operand:to=1", 300), wait(10),
		shoot(setShot),
		click("palette:check-value"), must("state:selected:check-value", 300),
		click("ui:pick-value"), wait(5), must("valuerow:npc.done", 120), click("valuerow:npc.done"),
		must("state:value:value=npc.done", 300), wait(10), shoot(checkShot),
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
	if !slices.ContainsFunc(values, func(value worldstate.Declaration) bool { return value.Path() == "npc.done" }) {
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
// choosers, as npc.busy and npc.queued, since the sample's own shop
// declares npc.serving and npc.waiting. Placed side by side and previewed,
// the Watch window shows the customer no longer waiting, keeping the clerk
// it found: it stops only on finding a clerk serving, which the clerk is
// only once it has found the customer waiting.
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
		choose("where", "busy", "declare:npc"), must("state:value:where=npc.busy", 300),
		retype("operand-is", "1"), must("state:operand:is=1", 300),
		choose("into", "clerk", "declare:local"), must("state:value:into=local.clerk", 300),
		click("palette:set-value"), must("state:selected:set-value", 300),
		choose("value", "queued", "declare:npc"), must("state:operand:to=0", 300), pause(),
		click("node:find-npc:1"), wait(5), click("palette:set-value"), must("state:selected:set-value", 300),
		choose("value", "queued", "valuerow:npc.queued"), retype("operand-to", "1"), must("state:operand:to=1", 300),
		pause(), click("ui:arrange"), wait(20), shoot(flowShot), wait(10), quit(),
	)
	clerkScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:find-npc"), must("state:selected:find-npc", 300),
		choose("where", "queued", "valuerow:npc.queued"), retype("operand-is", "1"), must("state:operand:is=1", 300),
		choose("into", "customer", "declare:local"), must("state:value:into=local.customer", 300),
		click("palette:set-value"), must("state:selected:set-value", 300),
		choose("value", "busy", "valuerow:npc.busy"), retype("operand-to", "1"), must("state:operand:to=1", 300),
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
		must("state:value:npc.queued=0", 600), must("state:value:local.clerk=NPC 2", 300), wait(30),
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
	if !slices.Contains(paths, "npc.busy") || !slices.Contains(paths, "npc.queued") {
		t.Fatalf("the world state holds %v", paths)
	}
	t.Logf("screenshots: %s %s %s", flowShot, mapShot, customerShot)
}

// checkSampleShop checks the sample's shop: its four values declared, and a
// clerk and a customer, each with its path name, following its own
// workflow with one local value, and placed on the ground floor.
func checkSampleShop(t *testing.T, project string, ground *floor.Floor) {
	t.Helper()
	decoded, err := stateplugin.Decode(readEnvelope(t, project, "world-states/*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, value := range decoded.(*stateplugin.Document).State.Values.Values() {
		paths = append(paths, value.Path())
	}
	if got := strings.Join(paths, " "); got != "npc.waiting npc.serving npc.served world.customers_served" {
		t.Fatalf("the sample declares the shop's values: %s", got)
	}
	placed := map[ids.ULID]bool{}
	for _, placement := range ground.Placements() {
		placed[placement.NPC] = true
	}
	for _, each := range []struct {
		name, path, workflow string
		nodes                int
	}{{"clerk", "clerk", "clerk-serves", 5}, {"customer", "customer", "customer-shops", 12}} {
		envelope := readEnvelope(t, project, "npcs/"+each.name+"--*.json")
		document, err := npcplugin.Decode(envelope.Data)
		if err != nil {
			t.Fatal(err)
		}
		made := document.(*npcplugin.Document).NPC
		behaviour := readEnvelope(t, project, "workflows/"+each.workflow+"--????????.json")
		logic, err := workflowplugin.Decode(behaviour.Data)
		if err != nil {
			t.Fatal(err)
		}
		workflow := logic.(*workflowplugin.Document).Workflow
		if made.Path != each.path || made.Behaviour != behaviour.ID || !placed[envelope.ID] ||
			len(workflow.Nodes()) != each.nodes || workflow.Locals().Len() != 1 {
			t.Fatalf("%s: %+v placed %v, %d nodes", each.name, made, placed[envelope.ID], len(workflow.Nodes()))
		}
	}
}

// TestSampleShopRuns previews the sample zone and watches its customer: it
// walks to the desk, the clerk finds it waiting there and serves it, and the
// world counts one customer served.
func TestSampleShopRuns(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	mapShot, watchShot := shotPath(t, "sample-shop-map.png"), shotPath(t, "sample-shop-watch.png")
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("P"), click("cell:3,1"), must("state:previewing", 300), wait(10),
		click("live-npc:Customer"), wait(5), click("live-npc:Customer"), must("state:watching", 300), wait(10),
		click("ui:watch"), wait(900), shoot(mapShot), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 1800), must("state:watching:Customer", 600),
		must("state:value:world.customers_served=1", 900), must("state:value:npc.served=yes", 60),
		shoot(watchShot), wait(10), quit(),
	)
	clockScript := script(must("state:running", 3600), wait(30), quit())
	runEditor(t, binary, project, scripts{
		"browser": {script(openZone(), quitWhenAlone())}, "map": {mapScript}, "watch": {watchScript},
		"clock": {clockScript},
	})
	noCrashes(t, project)
	t.Logf("screenshots: %s %s", mapShot, watchShot)
}
