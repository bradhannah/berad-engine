package main_test

import (
	"path/filepath"
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestLanesRunTogether puts a Together after the customer's start, its
// three lanes a chime that waits until it has played, a long shake and a
// line of speech, and previews the sample: Watch shows the chime's and the
// line's lanes done while the shake's runs on, and the customer comes round
// to say its line again only once the shake is done. It first takes away
// the customer's receiver, as the clerk's out_of_stock at the start of the
// preview would interrupt the lanes and add its own lines to the log.
func TestLanesRunTogether(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	canvasShot, watchShot := shotPath(t, "together-canvas.png"), shotPath(t, "together-watch.png")
	together := "node:" + flow.KindTogether + ":1"
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Customer"), must("state:tabs:2", 600), wait(60),
		doubleClick("row:Sample zone"), must("state:tabs:3", 600), quitWhenAlone(),
	)
	customerScript := script(
		must("state:loaded", 600), wait(60), click("ui:zoom-fit"), wait(10), click("node:"+flow.KindOnSignal+":1"),
		wait(5), click("ui:delete-node"), wait(10), click("node:start:1"), wait(5),
		insertAfter(flow.KindTogether, flow.OutputDone, "start:1:next", flow.KindCheckValue+":1"),
		must("state:selected:"+flow.KindTogether, 300), must("state:lanes:"+flow.KindTogether+":1=2", 60),
		click("ui:add-lane"), must("state:lanes:"+flow.KindTogether+":1=3", 300),
		addNode(flow.KindPlaySound), must("state:selected:"+flow.KindPlaySound, 300), click("ui:cue"),
		must("state:chooser", 300), click("cue:sample/chime"), wait(5),
		click("ui:"+flow.ParamCarry+"-"+flow.CarryWhenDone), wait(5),
		click(together), wait(5), addNode(flow.KindShake), must("state:selected:"+flow.KindShake, 300),
		click("ui:param-seconds"), wait(5), keys("Cmd+A"), typed("10"), keys("Enter"), wait(5),
		click(together), wait(5), addNode(flow.KindSay), must("state:selected:"+flow.KindSay, 300),
		click("ui:param-message"), wait(5), typed("Hold on!"), keys("Enter"), must("state:message=Hold on!", 300),
		wait(10), shoot(canvasShot), wait(30), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(1000), keys("P"), click("cell:3,1"), must("state:previewing", 300),
		must("state:npc:Customer=-3,1", 1800), click("live-npc:Customer"), wait(5), click("live-npc:Customer"),
		must("state:watching", 300), click("ui:watch"), wait(1500), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 3600), must("state:watching:Customer", 1200),
		must("state:lane:2=Shake screen", 600), must("state:lane-done:1", 600), must("state:lane-done:3", 600),
		must("state:lane:2=Shake screen", 60), wait(10), shoot(watchShot),
		must("state:log-lines=2", 1200), wait(10), quit(),
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
	lanes, _ := shops.Node(start.Next[flow.OutputNext])
	after, _ := shops.Node(lanes.Next[flow.OutputDone])
	sound, _ := shops.Node(lanes.Next["l1"])
	shake, _ := shops.Node(lanes.Next["l2"])
	say, _ := shops.Node(lanes.Next["l3"])
	if lanes.Kind != flow.KindTogether || after.Kind != flow.KindCheckValue || sound.Kind != flow.KindPlaySound ||
		shake.Kind != flow.KindShake || say.Kind != flow.KindSay {
		t.Fatalf("the start leads to the Together, its lanes to the chime, the shake and the line: %+v", shops.Nodes())
	}
	if carry, err := sound.Choice(flow.ParamCarry); err != nil || carry != flow.CarryWhenDone {
		t.Fatalf("the chime waits until it has played: %q %v", carry, err)
	}
	t.Logf("screenshots: %s %s", canvasShot, watchShot)
}
