package main_test

import (
	"path/filepath"
	"testing"
)

// TestAPortalLeadsToAnotherZone makes portals by the Map's menus and walks
// Launch's game through them (increment 64). A zone is made for the walk,
// with an anchor. Its Map adds a portal back to the sample's Counter, the
// anchor picked in the sample's Map, and tells it to cut; the sample's Map
// adds one that leads to the new zone's anchor, picked in that zone's Map,
// which fades, as a new portal does. Launch's player then walks onto the
// sample's portal: the screen fades, the game enters the new zone, where
// the sample's tune stops, and fades in on the anchor; two cells on, the
// portal back cuts to the Counter, where the tune plays. It is one game
// throughout: launched once.
func TestAPortalLeadsToAnotherZone(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	marked, away, back := shotPath(t, "portal-marked.png"), shotPath(t, "portal-away.png"), shotPath(t, "portal-back.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("ui:new-zone"), must("row:Zone 1", 300), wait(10),
		doubleClick("row:Sample zone"), must("state:tabs:2", 600), wait(30),
		doubleClick("row:Zone 1"), must("state:tabs:3", 600), quitWhenAlone(),
	)
	lead := func(cell, zone string) []step {
		return script(
			rightClick(cell), must("menu:add-portal", 120), click("menu:add-portal"), must("ui:row:"+zone, 120),
			click("ui:row:"+zone), click("ui:lead"),
		)
	}
	zoneScript := script(
		must("state:loaded", 600), wait(30), rightClick("cell:1,1"), must("menu:add-anchor", 120),
		click("menu:add-anchor"), wait(30),
		lead("cell:3,1", "Sample zone"), must("state:portal:3,1:to:Counter", 1800), must("state:portal:3,1:fade", 60),
		rightClick("cell:3,1"), must("menu:portal-change:cut", 120), click("menu:portal-change:cut"),
		must("state:portal:3,1:cut", 300),
		must("state:picking", 3600), wait(10), click("cell:1,1"), wait(1500), quit(),
	)
	sampleScript := script(
		// The new zone's Map tells its portal to cut before it can answer a
		// pick, so this Map waits before asking for one.
		must("state:loaded", 600), must("state:picking", 3600), wait(10), click("cell:-3,1"), wait(240),
		lead("cell:2,2", "Zone 1"), must("state:portal:2,2:to:Anchor 1", 1800),
		must("state:portal:2,2:fade", 300), wait(20), shoot(marked),
		keys("Cmd+R"), must("state:launched", 1800), wait(1200), quit(),
	)
	// A tap walks exactly one cell, so the player never walks on past where
	// a portal that cuts puts them.
	twoCells := func(key string) []step { return script(walk(key), wait(20), walk(key)) }
	playScript := script(
		must("state:playing", 3600), must("state:zone:Sample zone", 60), must("state:layer:sample/tune", 900),
		must("state:player:0,2", 300), wait(30),
		twoCells("Right"), must("state:zone:Zone 1", 1200), must("state:light", 600), must("state:player:1,1", 120),
		must("state:floor:1", 60), wait(20), shoot(away),
		twoCells("Right"), must("state:zone:Sample zone", 1200), must("state:player:-3,1", 120),
		must("state:layer:sample/tune", 900), must("state:launches:1", 10), wait(20), shoot(back), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {sampleScript, zoneScript}, "play": {playScript},
	})
	noCrashes(t, project)
	ground, _ := readFloor(t, project, "ground-floor")
	fades, found := ground.PortalAt(2, 2)
	if !found || fades.Change != "fade" {
		t.Fatalf("the sample's portal is saved, and fades: %+v %v", fades, found)
	}
	brick := countColor(t, back, brickColor(), 0)
	if brick < 1000 {
		t.Fatalf("back in the sample zone its brick walls show: %d pixels", brick)
	}
	if got := countColor(t, away, brickColor(), 0); got*4 >= brick {
		t.Fatalf("in the new zone there are no brick walls: %d pixels", got)
	}
	t.Logf("screenshots: %s %s %s", marked, away, back)
}
