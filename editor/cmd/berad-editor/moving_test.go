package main_test

import (
	"image/color"
	"path/filepath"
	"testing"
)

// brickColor is a colour of the brick that shows the sample's walls.
func brickColor() color.RGBA { return color.RGBA{R: 0x30, G: 0x2c, B: 0x38, A: 0xff} }

// TestMovingThePlayer moves the player while a game runs (increment 63).
// In a preview of the sample zone, a cell's menu moves the player to that
// cell and the preview goes on. Then Launch plays the game, and the same
// menu, on the sample zone's Map, moves its player within the zone; on the
// Map of a zone made for the walk, it moves the game into that zone, where
// the sample's tune stops; and on the sample's Map again, it moves the game
// back, where the tune plays. It is one game throughout: launched once.
func TestMovingThePlayer(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	away, back := shotPath(t, "moved-away.png"), shotPath(t, "moved-back.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("ui:new-zone"), must("row:Zone 1", 300), wait(10),
		doubleClick("row:Sample zone"), must("state:tabs:2", 600), wait(30),
		doubleClick("row:Zone 1"), must("state:tabs:3", 600), quitWhenAlone(),
	)
	move := func(cell string) []step {
		return script(rightClick(cell), must("menu:move-here", 120), click("menu:move-here"))
	}
	sampleScript := script(
		must("state:loaded", 600), wait(30), keys("P"), click("cell:3,1"), must("state:previewing", 300), wait(30),
		move("cell:0,2"), must("state:player:0,2", 120), must("state:previewing", 10), wait(10), keys("P"), wait(30),
		keys("Cmd+R"), must("state:launched", 1800), wait(60), move("cell:3,1"),
		wait(600), move("cell:1,2"), wait(600), quit(),
	)
	zoneScript := script(
		must("state:loaded", 600), must("state:launched", 3600), wait(360), move("cell:1,1"), wait(900), quit(),
	)
	playScript := script(
		must("state:playing", 1800), must("state:zone:Sample zone", 60), must("state:layer:sample/tune", 900),
		must("state:player:3,1", 900),
		must("state:zone:Zone 1", 1200), must("state:player:1,1", 60), must("state:floor:1", 60), wait(20), shoot(away),
		must("state:zone:Sample zone", 1200), must("state:player:1,2", 60), must("state:layer:sample/tune", 900),
		must("state:launches:1", 10), wait(20), shoot(back), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {sampleScript, zoneScript}, "play": {playScript},
	})
	noCrashes(t, project)
	brick := countColor(t, back, brickColor(), 0)
	if brick < 1000 {
		t.Fatalf("back in the sample zone its brick walls show: %d pixels", brick)
	}
	if got := countColor(t, away, brickColor(), 0); got*4 >= brick {
		t.Fatalf("in the new zone there are no brick walls: %d pixels", got)
	}
	t.Logf("screenshots: %s %s", away, back)
}
