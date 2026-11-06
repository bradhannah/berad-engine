package main_test

import (
	"path/filepath"
	"testing"

	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// freshBread is the clerk's own greeting, written with markup.
const freshBread = "Fresh bread, [i]still warm[/i]!"

// TestConstants writes the clerk's own greeting, a constant of the sample's
// shop template, in the writing sheet from her inspector in the Project
// window. World state's Constants window lists the greeting, opens it in
// the sheet on the default and switches to the clerk's own, and refuses to
// delete the serving time the clerk's day waits for. In a new workflow, a
// Wait makes a constant from its typed-in minutes, saved with Save, and
// uses it in the other Wait typed the same. Previewed, the clerk says her
// own greeting.
func TestConstants(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	sheetShot, inspectorShot := shotPath(t, "constants-sheet.png"), shotPath(t, "constants-inspector.png")
	windowShot, holderShot := shotPath(t, "constants-window.png"), shotPath(t, "constants-holder.png")
	refusedShot, othersShot := shotPath(t, "constants-refused.png"), shotPath(t, "constants-others.png")
	waitShot, speechShot := shotPath(t, "constants-wait.png"), shotPath(t, "constants-speech.png")
	greeting := "state:constant:npc.shop.greeting="
	// Each tab works while it is the only one besides the Project window,
	// which opens the next once the last has closed.
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("row:Clerk"), must(greeting+`"Shop's open!"`, 300), wait(5),
		click("ui:constant-write-npc.shop.greeting"), must("state:sheet", 300), wait(5), keys("Cmd+A", "Backspace"),
		typed(freshBread), wait(10), shoot(sheetShot), click("ui:sheet-done"), must(greeting+`"`+freshBread+`"`, 300),
		wait(10), shoot(inspectorShot),
		doubleClick("row:Constants"), must("state:tabs:2", 600), must("state:tabs:1", 3600), wait(5),
		click("ui:new-workflow"), wait(30), click("ui:open"), must("state:tabs:2", 600), must("state:tabs:1", 3600),
		wait(5), doubleClick("row:Sample zone"), must("state:tabs:2", 600), quitWhenAlone(),
	)
	constantsScript := script(
		must("state:loaded", 600), must("row:npc.shop.greeting", 600), wait(5), click("row:npc.shop.greeting"),
		must(`state:value=npc.shop.greeting:text:"Shop's open!"`, 300), wait(10), shoot(windowShot),
		click("ui:write"), must("state:sheet", 300), must("state:sheet-holder=Default", 60), wait(5),
		click("ui:sheet-holder"), wait(5), click("menu:sheet-holder-1"), must("state:sheet-holder=Clerk's own", 300),
		must("state:sheet-text="+freshBread, 60), wait(10), shoot(holderShot), keys("Escape"), wait(10),
		click("row:world.shop.serve_minutes"), must("state:selected=world.shop.serve_minutes", 300), wait(5),
		click("ui:value-delete"), must("state:says=“world.shop.serve_minutes” is in use", 600), wait(10),
		shoot(refusedShot), click("dialog:cancel"), wait(10), quit(),
	)
	waitNode := "node:" + flow.KindWait
	workflowScript := script(
		must("state:loaded", 600), wait(5), click("node:start:1"), wait(5), addNode(flow.KindWait),
		must("state:selected:"+flow.KindWait, 300), wait(10), addNode(flow.KindWait), wait(10),
		must(waitNode+":2", 300), click(waitNode+":2"), wait(5),
		click("ui:constant-duration"), wait(5), typed("tide_minutes"), must("declare:const.world.tide_minutes", 300),
		click("declare:const.world.tide_minutes"), wait(5), click("ui:picker-declare"),
		must("state:constant:duration=const.world.tide_minutes", 300), must("state:others=1", 600), wait(10),
		shoot(othersShot), click("ui:other-1"), wait(5), click("ui:other-use"), wait(10), must("state:unsaved=1", 60),
		click("ui:save"), must("state:unsaved=0", 300), wait(5),
		click(waitNode+":1"), must("state:constant:duration=const.world.tide_minutes", 300), wait(10),
		shoot(waitShot), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(120), keys("P"), click("cell:3,1"), must("state:previewing", 300),
		must("state:speaking:Clerk="+freshBread, 1200), wait(30), shoot(speechShot), keys("P"), wait(30), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "world-constants": {constantsScript}, "workflow": {workflowScript},
		"map": {mapScript},
	})
	noCrashes(t, project)
	checkConstantsSaved(t, project)
	t.Logf("screenshots: %s %s %s %s %s %s %s %s", sheetShot, inspectorShot, windowShot, holderShot, refusedShot,
		othersShot, waitShot, speechShot)
}

// checkConstantsSaved finds the new constant in the world state, starting at
// the minutes typed; the clerk's own greeting in her file; and both of the
// new workflow's Waits using the constant: each file at its version.
func checkConstantsSaved(t *testing.T, project string) {
	t.Helper()
	stateFile := readEnvelope(t, project, "world-states/*.json")
	decoded, err := stateplugin.Decode(stateFile.Data)
	if err != nil {
		t.Fatal(err)
	}
	values := decoded.(*stateplugin.Document).State.Values
	tide, found := values.Find(worldstate.ScopeWorld, ids.ULID{}, "tide_minutes")
	if stateFile.Version != worldstate.FormatVersion || !found || !tide.IsConstant() ||
		tide.Type != worldstate.TypeInteger || tide.Zero().Integer != 30 {
		t.Fatalf("the world state, version %d, holds world.tide_minutes from 30: %+v", stateFile.Version, tide)
	}
	shop, _ := values.Find(worldstate.ScopeNPC, ids.ULID{}, "shop")
	greeting, _ := values.Find(worldstate.ScopeNPC, shop.ID, "greeting")
	clerkFile := readEnvelope(t, project, "npcs/clerk--*.json")
	clerk, err := npc.Decode(clerkFile.Data)
	if err != nil {
		t.Fatal(err)
	}
	if own, has := clerk.Owned.ConstantValue(greeting); clerkFile.Version != npc.FormatVersion || !has ||
		own.Text != freshBread {
		t.Fatalf("the clerk's file, version %d, holds her own greeting: %+v", clerkFile.Version, own)
	}
	logic := readEnvelope(t, project, "workflows/workflow-1--????????.json")
	routine, err := flow.Decode(logic.Data)
	if err != nil {
		t.Fatal(err)
	}
	using := 0
	for _, node := range routine.Nodes() {
		if duration, err := node.Duration("duration"); err == nil && node.Kind == flow.KindWait &&
			duration.Constant.Value == tide.ID && duration.Amount == 30 {
			using++
		}
	}
	if logic.Version != flow.FormatVersion || using != 2 {
		t.Fatalf("both Waits of the workflow, version %d, use world.tide_minutes: %d", logic.Version, using)
	}
}
