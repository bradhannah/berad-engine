package main_test

import (
	"path/filepath"
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
// project browser, adds a world value, names it into a new group and makes
// it a list of choices: the saved world state holds world.quests.mood. The
// sample's NPCs and their shop template are in the tree.
func TestWorldStateDeclaresValues(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "world-state.png")
	browserScript := script(must("row:World state", 600), wait(5), doubleClick("row:World state"), quitWhenAlone())
	stateScript := script(
		must("state:loaded", 600), must("row:npc.shop.waiting", 600), wait(5), click("ui:add-value"),
		must("state:selected=world.value", 300), wait(5), click("ui:value-name"), keys("Cmd+A", "Backspace"),
		typed("quests.mood"), keys("Enter"), must("state:selected=world.quests.mood", 300), wait(5),
		click("ui:value-type-choice"), must("state:value=world.quests.mood:choice:first", 300), wait(30),
		shoot(shot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "world-state": {stateScript}})
	noCrashes(t, project)
	saved := readEnvelope(t, project, "world-states/*.json")
	decoded, err := stateplugin.Decode(saved.Data)
	if err != nil {
		t.Fatal(err)
	}
	values := decoded.(*stateplugin.Document).State.Values
	quests, found := values.Find(worldstate.ScopeWorld, ids.ULID{}, "quests")
	mood, held := values.Find(worldstate.ScopeWorld, quests.ID, "mood")
	if saved.ID != worldstate.ID() || !found || !held || mood.Type != worldstate.TypeChoice || len(mood.Choices) != 2 {
		t.Fatalf("the world state was saved as %+v", values.Values())
	}
	t.Logf("screenshot: %s", shot)
}

// TestValueNodesInAWorkflow builds a Set value and a Check value in a real
// Workflow: the Set value declares world.done from its chooser and sets it
// to 1, and the Check value checks it; then, with nothing selected, a local
// value is added. The world state holds world.done and the workflow the
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
		click("node:start:1"), wait(5), addNode("set-value"), must("state:selected:set-value", 300),
		click("ui:pick-value"), wait(5), typed("done"), wait(5), must("declare:world.done", 120),
		click("declare:world.done"), wait(5), click("ui:picker-declare"), must("state:value:value=world.done", 300),
		wait(5),
		click("ui:operand-to"), keys("Cmd+A"), typed("1"), keys("Enter"), must("state:operand:to=1", 300), wait(10),
		shoot(setShot),
		addNode("check-value"), must("state:selected:check-value", 300),
		click("ui:pick-value"), wait(5), must("valuerow:world.done", 120), click("valuerow:world.done"),
		must("state:value:value=world.done", 300), wait(10), shoot(checkShot),
		keys("Escape"), wait(5), click("ui:add-local"), must("state:local=value:integer:0", 300), wait(10),
		shoot(localShot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	state := readEnvelope(t, project, "world-states/*.json")
	decodedState, err := stateplugin.Decode(state.Data)
	if err != nil {
		t.Fatal(err)
	}
	if done, found := decodedState.(*stateplugin.Document).State.Values.Find(worldstate.ScopeWorld, ids.ULID{},
		"done"); !found || done.Type != worldstate.TypeInteger {
		t.Fatalf("the world state holds world.done: %+v", done)
	}
	logic := readEnvelope(t, project, "workflows/workflow-1--????????.json")
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

// TestShopkeeperServesACustomer builds a shop in the editor's windows
// alone. The World state window makes a stall template and applies it to
// two new NPCs. The customer's workflow looks for an NPC whose
// npc.stall.busy is 1 within two tiles: found, it stops queueing; none, it
// queues. The clerk's looks for a queueing customer within two tiles and is
// busy serving it. Both declare the template's values from their choosers.
// Placed side by side and previewed, the Watch window shows the customer no
// longer queueing, keeping the clerk it found.
func TestShopkeeperServesACustomer(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	customerShot, stateShot := shotPath(t, "shop-customer.png"), shotPath(t, "shop-state.png")
	mapShot, flowShot := shotPath(t, "shop-map.png"), shotPath(t, "shop-workflow.png")
	retype := func(field, text string) []step {
		return script(click("ui:"+field), wait(3), keys("Cmd+A"), typed(text), keys("Enter"), wait(10))
	}
	choose := func(param, name, row string) []step {
		picked := script(click("ui:pick-"+param), wait(5), typed(name), must(row, 3600), click(row), wait(10))
		if strings.HasPrefix(row, "declare:") {
			picked = append(picked, script(click("ui:picker-declare"), wait(10))...)
		}

		return picked
	}
	pause := func() []step {
		return script(addNode("wait"), wait(10), retype("param-duration", "1"), click("ui:duration-seconds"), wait(10))
	}
	browserScript := script(
		openZone(), wait(30), click("ui:new-workflow"), wait(30), click("ui:open"), wait(60),
		click("ui:new-workflow"), wait(30), click("ui:open"), wait(60),
		click("ui:new-npc"), wait(30), click("ui:choose-behaviour"), wait(10), click("row:Workflow 1"), wait(5),
		click("ui:use"), wait(30),
		click("ui:new-npc"), wait(30), click("ui:choose-behaviour"), wait(10), click("row:Workflow 2"), wait(5),
		click("ui:use"), wait(30), doubleClick("row:World state"), quitWhenAlone(),
	)
	stateScript := script(
		must("state:loaded", 600), must("row:npc.npc_1", 3600), must("row:npc.npc_2", 3600), wait(5),
		click("row:npc.npc_1"), wait(5), click("ui:add-template"), must("state:selected=npc.template", 300),
		retype("value-name", "stall"), must("state:selected=npc.stall", 300),
		click("row:npc.npc_1"), wait(5), click("ui:template-stall"), must("state:applied=npc.npc_1:stall", 300),
		click("row:npc.npc_2"), wait(5), click("ui:template-stall"), must("state:applied=npc.npc_2:stall", 300),
		click("toggle:npc.npc_2"), must("row:npc.npc_2.stall.busy", 3600), wait(10), shoot(stateShot), wait(10), quit(),
	)
	customerScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), addNode("find-npc"), must("state:selected:find-npc", 300),
		choose("where", "npc.stall.busy", "declare:npc.stall.busy"), must("state:value:where=npc.stall.busy", 300),
		retype("operand-is", "1"), must("state:operand:is=1", 300),
		choose("into", "clerk", "declare:local.clerk"), must("state:value:into=local.clerk", 300),
		addNode("set-value"), must("state:selected:set-value", 300),
		choose("value", "npc.stall.queued", "declare:npc.stall.queued"), must("state:operand:to=0", 300), pause(),
		click("node:find-npc:1"), wait(5), addNode("set-value"), must("state:selected:set-value", 300),
		choose("value", "queued", "valuerow:npc.stall.queued"), retype("operand-to", "1"),
		must("state:operand:to=1", 300), pause(), click("ui:arrange"), wait(20), shoot(flowShot), wait(10), quit(),
	)
	clerkScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), addNode("find-npc"), must("state:selected:find-npc", 300),
		choose("where", "queued", "valuerow:npc.stall.queued"), retype("operand-is", "1"),
		must("state:operand:is=1", 300),
		choose("into", "customer", "declare:local.customer"), must("state:value:into=local.customer", 300),
		addNode("set-value"), must("state:selected:set-value", 300),
		choose("value", "busy", "valuerow:npc.stall.busy"), retype("operand-to", "1"), must("state:operand:to=1", 300),
		pause(), click("node:find-npc:1"), wait(5), pause(), wait(10), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("N"), must("npc:NPC 2", 3600), click("npc:NPC 1"), wait(5),
		click("cell:1,1"), must("state:placement:1,1=NPC 1", 300), click("npc:NPC 2"), wait(5),
		click("cell:2,1"), must("state:placement:2,1=NPC 2", 300), wait(1500),
		keys("P"), click("cell:2,-2"), must("state:previewing", 300), wait(30),
		click("live-npc:NPC 1"), wait(5), click("live-npc:NPC 1"), must("state:watching", 300), wait(10),
		click("ui:watch"), wait(400), shoot(mapShot), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 3600), must("state:watching:NPC 1", 600),
		must("state:value:npc.stall.queued=0", 600), must("state:value:local.clerk=NPC 2", 300), wait(30),
		shoot(customerShot), wait(10), quit(),
	)
	clockScript := script(must("state:running", 3600), wait(30), quit())
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "world-state": {stateScript}, "workflow": {customerScript, clerkScript},
		"map": {mapScript}, "watch": {watchScript}, "clock": {clockScript},
	})
	noCrashes(t, project)
	state := readEnvelope(t, project, "world-states/*.json")
	decoded, err := stateplugin.Decode(state.Data)
	if err != nil {
		t.Fatal(err)
	}
	values := decoded.(*stateplugin.Document).State.Values
	stall, found := values.Find(worldstate.ScopeNPC, ids.ULID{}, "stall")
	_, busy := values.Find(worldstate.ScopeNPC, stall.ID, "busy")
	_, queued := values.Find(worldstate.ScopeNPC, stall.ID, "queued")
	if !found || !busy || !queued {
		t.Fatalf("the stall template holds busy and queued: %+v", values.Values())
	}
	t.Logf("screenshots: %s %s %s %s", stateShot, flowShot, mapShot, customerShot)
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
	values := decoded.(*stateplugin.Document).State.Values
	for _, value := range values.Values() {
		paths = append(paths, values.Path(value.ID))
	}
	if got := strings.Join(paths, " "); got != "npc.shop npc.shop.waiting npc.shop.serving npc.shop.served world.shop "+
		"world.shop.customers_served" {
		t.Fatalf("the sample declares the shop's values: %s", got)
	}
	sampleZone := readEnvelope(t, project, "zones/sample-zone--*.json").ID
	placed := map[ids.ULID]bool{}
	for _, placement := range ground.Placements() {
		placed[placement.NPC] = true
	}
	for _, each := range []struct {
		name, path, workflow string
		nodes                int
	}{{"clerk", "clerk", "clerk-s-day", 25}, {"customer", "customer", "customer-shops", 16}} {
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
		if made.Path != each.path || made.Behaviour != behaviour.ID || !placed[envelope.ID] || made.Zone != sampleZone ||
			len(workflow.Nodes()) != each.nodes || workflow.Locals().Len() != 1 {
			t.Fatalf("%s: %+v placed %v, %d nodes", each.name, made, placed[envelope.ID], len(workflow.Nodes()))
		}
	}
}

// TestSampleShopRuns previews the sample zone and watches its customer: it
// walks to the counter, the clerk at the desk finds it waiting there and
// serves it, and the world counts one customer served.
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
		must("state:value:world.shop.customers_served=1", 900), must("state:value:npc.shop.served=true", 60),
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
