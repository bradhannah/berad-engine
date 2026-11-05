package main_test

import (
	"path/filepath"
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestLoggingInWatch adds a Log to the customer's behaviour after its
// start, its name put in with Insert value…, and a Set value after the
// clerk's start that dresses the clerk as the lantern. Previewing the sample, the Map shows the clerk as
// the lantern, and Watch, watching the customer once it stands at the
// counter, reads its line, opens the customer's behaviour at the Log, and
// shows every NPC's lines.
func TestLoggingInWatch(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	logShot, spriteShot, watchShot := shotPath(t, "log-node.png"), shotPath(t, "log-sprite.png"),
		shotPath(t, "log-watch.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Customer"), must("state:tabs:2", 600), wait(60),
		doubleClick("row:Clerk"), must("state:tabs:3", 600), wait(60), doubleClick("row:Sample zone"),
		must("state:tabs:4", 600), quitWhenAlone(),
	)
	customerScript := script(
		must("state:loaded", 600), wait(5), click("node:start:1"), wait(5),
		insertAfter(flow.KindLog, flow.OutputNext, "start:1:next", flow.KindCheckValue+":1"),
		must("state:selected:log", 300), wait(10), click("ui:param-message"), wait(5), typed("Shopping as "), wait(5),
		click("ui:insert-value"), must("valuerow:npc.name", 300), click("valuerow:npc.name"),
		must("state:message=Shopping as {npc.name}", 300), must("state:reads=Shopping as Customer", 300), wait(10),
		shoot(logShot), click("node:start:1"), must("state:selected:start", 60), must("state:selected:log", 3600),
		wait(30), quit(),
	)
	clerkScript := script(
		must("state:loaded", 900), wait(5), click("node:start:1"), wait(5),
		insertAfter(flow.KindSetValue, flow.OutputNext, "start:1:next", flow.KindTimeSwitch+":1"),
		must("state:selected:set-value", 300), wait(10), click("ui:pick-value"), must("valuerow:npc.sprite", 300),
		click("valuerow:npc.sprite"), must("state:value:value=npc.sprite", 300), click("ui:pick-to"),
		must("state:dialog-row=Lantern", 300), click("row:Lantern"), wait(5), click("ui:use"),
		must("state:holds:to=Lantern", 300), wait(30), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(900), keys("P"), click("cell:3,1"), must("state:previewing", 300),
		must("state:npc-sprite:Clerk=Lantern", 1200), wait(10), shoot(spriteShot),
		must("state:npc:Customer=-3,1", 1800), click("live-npc:Customer"), wait(5), click("live-npc:Customer"),
		must("state:watching", 300), click("ui:watch"), wait(1800), keys("P"),
		wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 3600), must("state:watching:Customer", 1200),
		must("state:log-line=Shopping as Customer", 1800), wait(10), shoot(watchShot), click("log-line:Shopping as Customer"),
		wait(30), click("ui:log-every"), must("state:log-line=Shopping as Customer", 60), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "workflow": {customerScript, clerkScript}, "map": {mapScript},
		"watch": {watchScript},
	})
	noCrashes(t, project)
	shops, err := flow.Decode(readEnvelope(t, project, "workflows/customer-shops--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	logged := 0
	for _, node := range shops.Nodes() {
		message, err := node.Message()
		if node.Kind != flow.KindLog || err != nil || len(message) != 2 || message[0].Text != "Shopping as " ||
			message[1].Ref == nil || message[1].Ref.Value != worldstate.NameID() {
			continue
		}
		logged++
	}
	if logged != 1 {
		t.Fatal("the customer's behaviour logs its name")
	}
	day, err := flow.Decode(readEnvelope(t, project, "workflows/clerk-s-day--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	lantern, dressed := readEnvelope(t, project, "sprites/lantern--*.json").ID, false
	for _, node := range day.Nodes() {
		ref, refErr := node.Ref(flow.ParamValue)
		to, toErr := node.Operand("to")
		dressed = dressed || (node.Kind == flow.KindSetValue && refErr == nil && toErr == nil &&
			ref.Value == worldstate.SpriteID() && string(to) == `"`+lantern.String()+`"`)
	}
	if !dressed {
		t.Fatal("the clerk's day dresses the clerk as the lantern")
	}
	t.Logf("screenshots: %s %s %s", logShot, spriteShot, watchShot)
}
