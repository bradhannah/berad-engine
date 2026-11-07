package main_test

import (
	"path/filepath"
	"testing"
)

// TestFindingByWords searches the sample's Project window for "open": the
// shop's greeting, "Shop's open!", is found and opens selected in the
// Constants window. A search for "no stock" finds the customer's Say and
// opens Customer shops with it selected.
func TestFindingByWords(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	matchesShot, linesShot := shotPath(t, "find-constant.png"), shotPath(t, "find-line.png")
	constantShot, sayShot := shotPath(t, "find-constant-opened.png"), shotPath(t, "find-line-opened.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("ui:search"), wait(5), typed("open"),
		must("row:npc.shop.greeting", 300), wait(10), click("row:npc.shop.greeting"), wait(10), shoot(matchesShot),
		click("ui:open"), must("state:tabs:2", 600), must("state:tabs:1", 3600), wait(5),
		click("ui:search"), wait(5), keys("Cmd+A"), typed("no stock"), must("row:Customer shops · Say", 300),
		wait(10), click("row:Customer shops · Say"), wait(10), shoot(linesShot),
		click("ui:open"), must("state:tabs:2", 600), quitWhenAlone(),
	)
	constantsScript := script(
		must("state:loaded", 600), must("state:selected=npc.shop.greeting", 600), wait(10), shoot(constantShot),
		quit(),
	)
	workflowScript := script(
		must("state:loaded", 600), must("state:selected:say", 600), wait(10), shoot(sayShot), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "world-constants": {constantsScript}, "workflow": {workflowScript},
	})
	noCrashes(t, project)
	t.Logf("screenshots: %s %s %s %s", matchesShot, constantShot, linesShot, sayShot)
}
