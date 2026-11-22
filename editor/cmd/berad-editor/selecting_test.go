package main_test

import (
	"path/filepath"
	"testing"
)

// TestSelectThenInspect selects each kind of thing on the sample zone's Map
// with the Select tool, and sets it in the inspector (increment 65). The
// hum is selected and dragged to another cell. The cell inside the door
// holds the Door anchor and the customer: a click there asks which, and
// each is selected in turn. The player start is selected and dragged. A
// portal is added from the cell's menu, its anchor picked in this same Map;
// it is then selected, told to cut in the inspector, dragged, and removed
// from the menu, which only adds and removes.
func TestSelectThenInspect(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	sound, which := shotPath(t, "select-sound.png"), shotPath(t, "select-which.png")
	anchor, npc := shotPath(t, "select-anchor.png"), shotPath(t, "select-npc.png")
	start, portal := shotPath(t, "select-start.png"), shotPath(t, "select-portal.png")
	mapScript := script(
		must("state:loaded", 600), wait(30), keys("S"), wait(5),
		click("cell:-3,2"), must("state:selected:sound:-3,2", 120), wait(10), shoot(sound),
		drag("cell:-3,2", "cell:-2,2"), must("state:ambient:-2,2:sample/hum", 300),
		must("state:selected:sound:-2,2", 60),
		click("cell:0,3"), must("menu:select:npc", 120), must("menu:select:anchor", 10), wait(10), shoot(which),
		click("menu:select:anchor"), must("state:selected:anchor:0,3", 120), wait(10), shoot(anchor),
		click("cell:0,3"), must("menu:select:npc", 120), click("menu:select:npc"),
		must("state:selected:npc:0,3", 120), wait(10), shoot(npc),
		click("cell:0,2"), must("state:selected:start:0,2", 120), wait(10), shoot(start),
		drag("cell:0,2", "cell:1,2"), must("state:player-start:1,2", 300), must("state:selected:start:1,2", 60),
		rightClick("cell:2,2"), must("menu:add-portal", 120), click("menu:add-portal"),
		must("ui:row:Sample zone", 120), click("ui:row:Sample zone"), click("ui:lead"),
		must("state:picking", 600), wait(10), click("cell:-3,1"), must("state:portal:2,2:to:Counter", 600),
		click("cell:2,2"), must("state:selected:portal:2,2", 120), must("state:portal:2,2:fade", 10), wait(10),
		shoot(portal),
		click("ui:portal-cut"), must("state:portal:2,2:cut", 300),
		drag("cell:2,2", "cell:3,2"), must("state:portal:3,2:cut", 300), must("state:selected:portal:3,2", 60),
		rightClick("cell:3,2"), must("menu:remove-portal", 120), click("menu:remove-portal"),
		must("state:selected:none", 300), wait(20), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browseZone()}, "map": {mapScript}})
	noCrashes(t, project)
	ground, _ := readFloor(t, project, "ground-floor")
	if hum, found := ground.SoundAt(-2, 2); !found || hum.Cue != "sample/hum" {
		t.Fatalf("the hum was dragged a cell to the right: %+v %v", hum, found)
	}
	if left := ground.Portals(); len(left) != 0 {
		t.Fatalf("the portal was removed from the menu: %+v", left)
	}
	t.Logf("screenshots: %s %s %s %s %s %s", sound, which, anchor, npc, start, portal)
}
