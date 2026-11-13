package main_test

import (
	"path/filepath"
	"testing"
)

// TestDividersDragAndAreKept drags a Map's inspector and left panel wider,
// quits, and opens the project again: the Map comes back with the widths as
// they were left (increment 53).
func TestDividersDragAndAreKept(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "dividers.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), must("state:tabs:2", 600),
		quitWhenAlone(),
	)
	dragScript := script(
		must("state:loaded", 600), wait(30), drag("ui:left-divider", "point:260,300"), wait(10),
		must("state:left-width=260", 60), drag("ui:inspector-divider", "point:700,300"), wait(10),
		must("state:inspector-width=500", 60), wait(30), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {dragScript}})
	keptScript := script(
		must("state:loaded", 600), wait(30), must("state:left-width=260", 120), must("state:inspector-width=500", 120),
		step{Repeat: 2, Screenshot: shot, Front: false}, wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {keptScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestAMapKeepsAnArrangementForPreviewing folds a Map's inspector while it
// previews: stopping the preview brings the inspector back as it was for
// editing, and the next preview folds it again (increment 54).
func TestAMapKeepsAnArrangementForPreviewing(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "preview-arrangement.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), must("state:tabs:2", 600),
		quitWhenAlone(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(30), keys("P"), click("cell:3,1"), must("state:previewing", 300), wait(10),
		keys("Cmd+I"), must("state:inspector-folded", 60), wait(10), keys("P"), wait(20), must("state:inspector", 60),
		keys("P"), click("cell:3,1"), must("state:previewing", 300), must("state:inspector-folded", 60), wait(10),
		step{Repeat: 2, Screenshot: shot, Front: false}, keys("P"), wait(20), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {mapScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestTheWindowReopensAsItWasLeft opens the sample zone's Map and the Fetch
// stock workflow, quits, and opens the project again: both tabs reopen, in
// order, and the status bar says so (increment 55).
func TestTheWindowReopensAsItWasLeft(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "reopened.png")
	firstBrowser := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), must("state:tabs:2", 600), wait(60),
		doubleClick("row:Fetch stock"), must("state:tabs:3", 600), wait(120), keys("Cmd+Q"), must("ui:quit", 300),
		wait(5), click("ui:quit"), wait(60),
	)
	idle := script(must("state:loaded", 900), wait(30))
	runEditor(t, binary, project, scripts{
		"browser": {firstBrowser}, "map": {idle}, "workflow": {idle},
	})
	againBrowser := script(
		must("state:tabs:3", 1800), wait(60), keys("Cmd+Q"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	reopened := script(
		must("state:loaded", 900), must("state:said:Reopened as it was left", 900), wait(10),
		step{Repeat: 2, Screenshot: shot, Front: false}, wait(30),
	)
	runEditor(t, binary, project, scripts{
		"browser": {againBrowser}, "map": {idle}, "workflow": {reopened},
	})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}
