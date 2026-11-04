package main_test

import (
	"path/filepath"
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestShowingThePlayer gives the customer's behaviour a Say, a Show message
// the player closes and a Shake screen after its start, and previews the
// sample: the clerk says Shop's open!, the customer says its line, the box
// shows until a click closes it, the screen shakes, and Watch's log has the
// line and the box.
func TestShowingThePlayer(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	nodesShot, speechShot, shakeShot := shotPath(t, "show-nodes.png"), shotPath(t, "show-speech.png"),
		shotPath(t, "show-shake.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Customer"), must("state:tabs:2", 600), wait(60),
		doubleClick("row:Sample zone"), must("state:tabs:3", 600), quitWhenAlone(),
	)
	customerScript := script(
		must("state:loaded", 600), wait(5), click("node:start:1"), wait(5), addNode(flow.KindSay),
		must("state:selected:"+flow.KindSay, 300), click("ui:param-message"), wait(5), typed("Good morning!"),
		keys("Enter"), must("state:message=Good morning!", 300), wait(5),
		addNode(flow.KindShowMessage), must("state:selected:"+flow.KindShowMessage, 300), click("ui:param-message"),
		wait(5), typed("A customer arrives."), keys("Enter"), must("state:message=A customer arrives.", 300),
		click("ui:"+flow.ParamAway+"-"+flow.AwayClosed), wait(5), click("ui:"+flow.ParamCarry+"-"+flow.CarryWhenDone),
		wait(5), addNode(flow.KindShake), must("state:selected:"+flow.KindShake, 300), wait(10), shoot(nodesShot),
		wait(30), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(900), keys("P"), click("cell:3,1"), must("state:previewing", 300),
		click("ui:watch"), wait(30),
		must("state:speaking:Clerk=Shop's open!", 1200), must("state:speaking:Customer=Good morning!", 600),
		must("state:box=A customer arrives.", 600), wait(30), shoot(speechShot), click("box"),
		must("state:shaking", 300), wait(10), shoot(shakeShot), wait(120), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 3600), must("state:log-line=Customer says: Good morning!", 1800),
		must("state:log-line=Box: A customer arrives.", 600), wait(30), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "workflow": {customerScript}, "map": {mapScript}, "watch": {watchScript},
	})
	noCrashes(t, project)
	shops, err := flow.Decode(readEnvelope(t, project, "workflows/customer-shops--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	start, _ := shops.Node(shops.Start())
	say, _ := shops.Node(start.Next[flow.OutputNext])
	box, _ := shops.Node(say.Next[flow.OutputNext])
	shake, _ := shops.Node(box.Next[flow.OutputNext])
	if say.Kind != flow.KindSay || box.Kind != flow.KindShowMessage || shake.Kind != flow.KindShake {
		t.Fatalf("the start leads to the Say, the box and the shake: %+v", shops.Nodes())
	}
	t.Logf("screenshots: %s %s %s", nodesShot, speechShot, shakeShot)
}
