package main_test

import (
	"path/filepath"
	"testing"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
)

// TestMovingTheCustomer makes a new zone and moves the sample's customer
// into it with the zone picker: the move dialog lists the counter and the
// door its behaviour goes to, the behaviour's Go to is flagged once it
// lives in the new zone, and Undo puts it back in the sample zone, its Go to
// no longer flagged.
func TestMovingTheCustomer(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	pickerShot, dialogShot, flagShot := shotPath(t, "zone-picker.png"), shotPath(t, "zone-move.png"),
		shotPath(t, "zone-flag.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("ui:new-zone"), wait(30), click("row:Customer"), wait(60),
		doubleClick("row:Customer"),
		must("state:tabs:2", 600), wait(30), click("row:Customer"), must("state:npc-zone=Sample zone", 600), wait(10),
		click("ui:npc-zone"), must("state:zone-picker", 120), wait(10), click("zone:Zone 1"), wait(10),
		shoot(pickerShot), keys("Enter"), must("state:dialog", 900), must("state:dialog-row=Counter", 60), wait(10),
		shoot(dialogShot), click("ui:zone-move"), must("state:npc-zone=Zone 1", 600), wait(600), keys("Cmd+Z"),
		must("state:npc-zone=Sample zone", 600), quitWhenAlone(),
	)
	shopsScript := script(
		must("state:loaded", 900), must("state:problem:go-to:1", 3600), wait(10), click("node:go-to:1"), wait(10),
		shoot(flagShot), must("state:problems:0", 3600), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {shopsScript}})
	noCrashes(t, project)
	decoded, err := npcplugin.Decode(readEnvelope(t, project, "npcs/customer--*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	if home := decoded.(*npcplugin.Document).NPC.Zone; home != readEnvelope(t, project, "zones/sample-zone--*.json").ID {
		t.Fatalf("Undo put the customer back in the sample zone: %v", home)
	}
	t.Logf("screenshots: %s %s %s", pickerShot, dialogShot, flagShot)
}

// TestTheCustomerTakesTheStairs adds a place on the sample building's
// upper floor and sends the customer there: previewing the zone, the
// customer walks to the stairs and up them, the map follows it to the upper
// floor, and Watch finds it at the new place there.
func TestTheCustomerTakesTheStairs(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	walkShot, mapShot, watchShot := shotPath(t, "stairs-walk.png"), shotPath(t, "stairs-map.png"),
		shotPath(t, "stairs-watch.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("row:Customer"), wait(60), doubleClick("row:Customer"),
		must("state:tabs:2", 600), wait(30), doubleClick("row:Sample zone"), must("state:tabs:3", 600), quitWhenAlone(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(10), keys("PageUp"), must("state:floor:2", 120), keys("A"), wait(5),
		click("cell:2,0"), wait(30), keys("Escape"), wait(5), keys("PageDown"),
		must("state:floor:1", 120), wait(400), keys("P"), click("cell:3,1"), must("state:previewing", 300), wait(10),
		click("live-npc:Customer"), wait(5), click("live-npc:Customer"), must("state:watching", 300), wait(10),
		shoot(walkShot), click("ui:watch"), must("state:floor:2", 1800), wait(30), shoot(mapShot), keys("P"), wait(30),
		quit(),
	)
	shopsScript := script(
		must("state:loaded", 900), wait(10), click("ui:zoom-fit"), wait(20), click("node:go-to:1"),
		must("state:selected:go-to", 120), wait(10), click("ui:place"), must("place:Anchor 1", 1800), wait(5),
		click("place:Anchor 1"), wait(60), quit(),
	)
	watchScript := script(
		must("state:previewing", 1800), must("state:watching:Customer", 600),
		must("state:at=cell 2, 0 on Upper floor", 1200), wait(10), shoot(watchShot), quit(),
	)
	clockScript := script(must("state:running", 3600), wait(30), quit())
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {mapScript}, "workflow": {shopsScript}, "watch": {watchScript},
		"clock": {clockScript},
	})
	noCrashes(t, project)
	t.Logf("screenshots: %s %s %s", walkShot, mapShot, watchShot)
}
