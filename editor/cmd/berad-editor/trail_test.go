package main_test

import (
	"path/filepath"
	"testing"
)

// TestWatchKeepsATrail previews the sample zone, watches the clerk and
// moves the clock on an hour: Watch's trail says the clerk started again
// because the clock moved, and the strip shows the trail and the values,
// each from its tab (increment 46).
func TestWatchKeepsATrail(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	trailShot, valuesShot := shotPath(t, "watch-trail.png"), shotPath(t, "watch-values.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), must("state:tabs:2", 600),
		quitWhenAlone(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("P"), click("cell:3,1"), must("state:previewing", 300), wait(10),
		click("live-npc:Clerk"), wait(5), click("live-npc:Clerk"), must("state:watching", 300), wait(10),
		click("ui:watch"), wait(1800), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 1800), must("state:watching:Clerk", 600), click("ui:strip-trail"),
		must("state:strip=trail", 60), must("state:trail:Started again: the clock moved to", 1200), wait(30),
		shoot(trailShot), click("ui:strip-values"), must("state:strip=values", 60), wait(10), shoot(valuesShot),
		quit(),
	)
	clockScript := script(must("state:running", 3600), wait(120), click("ui:hour-on"), wait(30), quit())
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {mapScript}, "watch": {watchScript}, "clock": {clockScript},
	})
	noCrashes(t, project)
	t.Logf("screenshots: %s %s", trailShot, valuesShot)
}
