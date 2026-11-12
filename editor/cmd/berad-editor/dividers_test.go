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
