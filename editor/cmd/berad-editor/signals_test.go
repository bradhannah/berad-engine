package main_test

import (
	"path/filepath"
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestTheBellInterruptsTheClerk previews the sample and watches the clerk:
// the customer rings the bell at the counter, the clerk handles shop.bell_rung
// from the customer at once, saying Just a moment!, and Watch's log has
// both. Deleting the clerk's receiver then flags the customer's Send as a
// broken signal call.
func TestTheBellInterruptsTheClerk(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	watchShot, brokenShot := shotPath(t, "signal-watch.png"), shotPath(t, "signal-broken.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Clerk's day"), must("state:tabs:2", 600), wait(60),
		doubleClick("row:Customer shops"), must("state:tabs:3", 600), must("state:tab-tint:Clerk's day=green", 60),
		must("state:tab-tint:Customer shops=amber", 60), doubleClick("row:Sample zone"),
		must("state:tabs:4", 600), quitWhenAlone(),
	)
	clerkScript := script(
		must("state:loaded", 900), must("state:nodes:25", 60), wait(1500), click("ui:zoom-fit"), wait(10),
		click("node:"+flow.KindOnSignal+":1"),
		wait(5), shoot(shotPath(t, "signal-select.png")), click("ui:delete-node"), wait(10),
		shoot(shotPath(t, "signal-deleted.png")), must("state:nodes:24", 300), wait(600), quit(),
	)
	broken := "Broken signal call: Clerk's behaviour doesn't answer shop.bell_rung. Give it an On signal for it."
	customerScript := script(
		must("state:loaded", 900), must("state:problems:0", 600),
		must("state:problem:"+flow.KindSendSignal+":1="+broken, 3600), wait(10), shoot(brokenShot), wait(30), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("P"), click("cell:3,1"), must("state:previewing", 300), wait(10),
		click("live-npc:Clerk"), wait(5), click("live-npc:Clerk"), must("state:watching", 300), wait(10),
		click("ui:watch"), wait(1800), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 1800), must("state:watching:Clerk", 600), must("state:handling=shop.bell_rung", 1200),
		wait(5), shoot(watchShot), must("state:log-line=Heard shop.bell_rung from Customer", 300),
		must("state:log-line=Clerk says: Just a moment!", 300), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "workflow": {clerkScript, customerScript}, "map": {mapScript},
		"watch": {watchScript},
	})
	noCrashes(t, project)
	day, err := flow.Decode(readEnvelope(t, project, "workflows/clerk-s-day--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	if len(day.Receivers()) != 0 {
		t.Fatalf("the clerk's receiver is deleted: %v", day.Receivers())
	}
	t.Logf("screenshots: %s %s", watchShot, brokenShot)
}
