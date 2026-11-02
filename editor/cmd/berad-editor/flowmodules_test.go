package main_test

import (
	"path/filepath"
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestMakingAndCallingAModule makes a new workflow a module in its tab: two
// Returns, each a way out, and a place input. In the clerk's day it adds a
// Call of it beside its Call of Fetch stock, whose outputs are its ways out
// and done; back in the module,
// the strip names the clerk's day as calling it, and the input's type is
// fixed. The saved files hold the module's interface and the Call.
func TestMakingAndCallingAModule(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	moduleShot, callShot, calledShot := shotPath(t, "module.png"), shotPath(t, "call.png"),
		shotPath(t, "module-called.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), wait(120),
		doubleClick("row:Clerk's day"), quitWhenAlone(),
	)
	moduleScript := script(
		must("state:loaded", 600), wait(5), click("node:start:1"), wait(5), click("palette:"+flow.KindReturn),
		must("state:selected:return", 300), wait(5), drag("palette:"+flow.KindReturn, "canvas:520,360", "canvas:520,360"),
		wait(10), keys("Escape"), wait(5), click("ui:add-local"), wait(10), click("ui:value-role-input"), wait(5),
		click("ui:value-type-place"), must("state:local=value:place:no place yet", 300), wait(10), shoot(moduleShot),
		must("state:calledby=Called by 1: Clerk's day · Ways out: finished, finished 2, done", 3600), wait(10),
		click("ui:value-type-bool"), wait(10), must("state:local=value:place:no place yet", 60), shoot(calledShot),
		quit(),
	)
	dayScript := script(
		must("state:loaded", 900), must("state:nodes:19", 60), wait(10), keys("Escape"), wait(5),
		click("palette:"+flow.KindCall), must("state:selected:call-workflow", 300), wait(5), click("ui:call-workflow"),
		must("menu:workflow-named:Workflow 1", 120), click("menu:workflow-named:Workflow 1"), wait(60), shoot(callShot),
		must("state:output:call-workflow:2:w2=finished 2", 600), wait(240), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {moduleScript, dayScript}})
	noCrashes(t, project)
	module, err := flow.Decode(readEnvelope(t, project, "workflows/workflow-1--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	inputs := module.Inputs()
	if len(module.WaysOut()) != 2 || len(inputs) != 1 || inputs[0].Type != worldstate.TypePlace {
		t.Fatalf("the module has two ways out and a place input: %v %+v", module.WaysOut(), inputs)
	}
	day, err := flow.Decode(readEnvelope(t, project, "workflows/clerk-s-day--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	calls, moduleID := 0, readEnvelope(t, project, "workflows/workflow-1--????????.json").ID
	for _, node := range day.Nodes() {
		if called, ok := node.Called(); ok && called == moduleID {
			calls++
		}
	}
	if calls != 1 {
		t.Fatalf("the clerk's day calls the module as well as Fetch stock: %d", calls)
	}
	t.Logf("screenshots: %s %s %s", moduleShot, callShot, calledShot)
}

// TestClerkCallsFetchStock previews the sample and watches the clerk: its
// day calls Fetch stock, giving it the shelf, and Watch names the path down
// through the call while the clerk walks there, then the clerk comes back
// to its day. Fetch stock's tab names the clerk's day as calling it.
func TestClerkCallsFetchStock(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	watchShot, backShot, stripShot := shotPath(t, "watch-call.png"), shotPath(t, "watch-back.png"),
		shotPath(t, "fetch-stock.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Fetch stock"), must("state:tabs:2", 600), wait(10),
		doubleClick("row:Sample zone"), quitWhenAlone(),
	)
	fetchScript := script(
		must("state:loaded", 900), must("state:calledby=Called by 1: Clerk's day · Ways out: Fetched, Out of stock, done",
			3600), wait(10), shoot(stripShot), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("P"), click("cell:3,1"), must("state:previewing", 300), wait(10),
		click("live-npc:Clerk"), wait(5), click("live-npc:Clerk"), must("state:watching", 300), wait(10),
		click("ui:watch"), wait(3000), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 1800), must("state:watching:Clerk", 600),
		must("state:path=Clerk's day › Fetch stock › Go to", 2400), must("state:value:local.place=Shelf", 60), wait(10),
		shoot(watchShot),
		must("state:path=", 600), wait(10), shoot(backShot), quit(),
	)
	clockScript := script(must("state:running", 3600), wait(30), quit())
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "workflow": {fetchScript}, "map": {mapScript}, "watch": {watchScript},
		"clock": {clockScript},
	})
	noCrashes(t, project)
	t.Logf("screenshots: %s %s %s", watchShot, backShot, stripShot)
}

// TestPickingACallsPlace picks the place the clerk's day gives Fetch stock
// on the Sample zone map: the map comes forward in picking mode with the
// shelf marked now, a name in its list answers, and the clerk's day comes
// back in front giving the Counter.
func TestPickingACallsPlace(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	mapShot, dayShot := shotPath(t, "picking-map.png"), shotPath(t, "picking-back.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Sample zone"), must("state:tabs:2", 600), wait(30),
		doubleClick("row:Clerk's day"), quitWhenAlone(),
	)
	dayScript := script(
		must("state:loaded", 900), wait(10), click("ui:zoom-fit"), wait(20), click("node:call-workflow:1"),
		must("state:selected:call-workflow", 120), wait(10), click("ui:input-place-pick"), must("state:hidden", 600),
		must("state:shown", 1800), must("state:given:place=Counter", 300), wait(10), shoot(dayShot), quit(),
	)
	mapScript := script(
		must("state:loaded", 900), must("state:picking", 1500), must("state:pick-now=Shelf", 60), wait(10),
		step{Repeat: 2, Screenshot: mapShot}, click("place:Counter"), wait(600), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {dayScript}, "map": {mapScript}})
	noCrashes(t, project)
	t.Logf("screenshots: %s %s", mapShot, dayShot)
}
