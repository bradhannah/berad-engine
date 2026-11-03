package main_test

import (
	"path/filepath"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestASampleZoneLoop makes a zone loop from New ▾, sets world.rings to 1
// in it, makes it the sample zone's loop, and previews the zone: Watch
// watches the loop under the zone's name and finds the value it set.
func TestASampleZoneLoop(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	loopShot, detailsShot, watchShot := shotPath(t, "zone-loop.png"), shotPath(t, "zone-loop-details.png"),
		shotPath(t, "zone-loop-watch.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("ui:new"), wait(5), click("menu:new-zone-loop"), wait(30),
		must("state:row-loop:Zone loop 1", 300), click("ui:open"), must("state:tabs:2", 600), wait(30),
		click("row:Sample zone"), wait(30), click("ui:choose-zone-loop"), must("state:dialog-row=Zone loop 1", 300),
		click("row:Zone loop 1"), wait(5), click("ui:use"), wait(30), shoot(detailsShot),
		doubleClick("row:Sample zone"), must("state:tabs:3", 600), quitWhenAlone(),
	)
	loopScript := script(
		must("state:loaded", 600), wait(5), click("node:start:1"), wait(5), click("palette:set-value"),
		must("state:selected:set-value", 300), click("ui:pick-value"), wait(5), typed("rings"), wait(5),
		must("declare:world.rings", 120), click("declare:world.rings"), wait(5), click("ui:picker-declare"),
		must("state:value:value=world.rings", 300), wait(5),
		click("ui:operand-to"), keys("Cmd+A"), typed("1"), keys("Enter"), must("state:operand:to=1", 300), wait(10),
		click("palette:wait"), must("state:selected:wait", 300), wait(10), keys("Escape"),
		must("state:strip=Zone loop · Runs in any zone · Loop of Sample zone", 1800), wait(10), shoot(loopShot),
		wait(1200), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(600), keys("P"), click("cell:3,1"), must("state:previewing", 300), wait(30),
		click("ui:watch"), wait(1800), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 1800), must("ui:watch-loop", 1200), click("ui:watch-loop"),
		must("state:watching:Sample zone", 600), must("state:value:world.rings=1", 1200), wait(10), shoot(watchShot),
		quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "workflow": {loopScript}, "map": {mapScript}, "watch": {watchScript},
	})
	noCrashes(t, project)
	decoded, err := zoneplugin.Decode(readEnvelope(t, project, "zones/sample-zone--*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	loop := readEnvelope(t, project, "workflows/zone-loop-1--????????.json")
	read, err := workflowplugin.Decode(loop.Data)
	if err != nil || decoded.(*zoneplugin.Document).Zone.Loop != loop.ID ||
		read.(*workflowplugin.Document).Workflow.Purpose() != flow.PurposeZoneLoop {
		t.Fatalf("the sample zone runs Zone loop 1, a zone loop: %v", err)
	}
	t.Logf("screenshots: %s %s %s", loopShot, detailsShot, watchShot)
}

// TestRunningTheCustomerElsewhere runs the customer's behaviour in a new
// zone: the Runs in box asks first, naming the customer and the counter,
// then the Go to and the customer's details are flagged; Undo runs it in
// the sample zone again, the flags gone.
func TestRunningTheCustomerElsewhere(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	askShot, flaggedShot, detailsShot := shotPath(t, "runs-in-ask.png"), shotPath(t, "runs-in-flagged.png"),
		shotPath(t, "runs-in-npc-flag.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("ui:new-zone"), wait(30), click("row:Customer"), wait(60),
		doubleClick("row:Customer"), must("state:tabs:2", 600), wait(30), click("row:Customer"),
		must("state:flag=Customer shops runs in Zone 1, and Customer lives in Sample zone.", 3600), wait(10),
		shoot(detailsShot), quitWhenAlone(),
	)
	shopsScript := script(
		must("state:loaded", 900), wait(10), must("state:runs-in=Sample zone", 600), click("ui:runs-in"),
		must("state:zone-picker", 120), wait(10), click("zone:Zone 1"), wait(5), keys("Enter"),
		must("state:dialog", 300), must("state:dialog-line=Customer lives elsewhere", 60),
		must("state:dialog-row=Counter", 60), wait(10), shoot(askShot), click("ui:runs-in-change"),
		must("state:runs-in=Zone 1", 600), must("state:problem:go-to:1", 600), wait(10), shoot(flaggedShot),
		wait(900), keys("Cmd+Z"), must("state:runs-in=Sample zone", 600), must("state:problems:0", 3600), wait(10),
		quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {shopsScript}})
	noCrashes(t, project)
	decoded, err := workflowplugin.Decode(readEnvelope(t, project, "workflows/customer-shops--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	if runs := decoded.(*workflowplugin.Document).Workflow.RunsIn(); runs != readEnvelope(t, project,
		"zones/sample-zone--*.json").ID {
		t.Fatalf("Undo runs the customer's behaviour in the sample zone again: %v", runs)
	}
	t.Logf("screenshots: %s %s %s", askShot, flaggedShot, detailsShot)
}
