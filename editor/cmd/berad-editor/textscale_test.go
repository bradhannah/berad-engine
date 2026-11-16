package main_test

import (
	"path/filepath"
	"testing"
)

// TestTextAndControlsScale makes text and controls larger with
// Cmd+Shift+=, chooses 125% from the status bar's Text button, quits, and
// opens the project again: the Map comes back at 125%, and Cmd+Shift+0 puts
// it back at 100% (increment 56).
func TestTextAndControlsScale(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	larger, kept := shotPath(t, "text-150.png"), shotPath(t, "text-125-kept.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), must("state:tabs:2", 600),
		quitWhenAlone(),
	)
	scaleScript := script(
		must("state:loaded", 600), wait(30), must("state:text-scale=100", 60), keys("Cmd+Shift+Equal"), wait(5),
		keys("Cmd+Shift+Equal"), wait(5), keys("Cmd+Shift+Equal"), must("state:text-scale=150", 60),
		must("state:zoom:100", 60), wait(20), step{Repeat: 2, Screenshot: larger, Front: false}, wait(10),
		click("ui:text-scale"), wait(5), click("ui:text-125"), must("state:text-scale=125", 60), wait(30), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {scaleScript}})
	keptScript := script(
		must("state:loaded", 600), wait(30), must("state:text-scale=125", 120), must("state:zoom:100", 60),
		step{Repeat: 2, Screenshot: kept, Front: false}, wait(10), keys("Cmd+Shift+0"),
		must("state:text-scale=100", 60), wait(20), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {keptScript}})
	noCrashes(t, project)
	t.Logf("screenshots: %s %s", larger, kept)
}
