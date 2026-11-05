package main_test

import (
	"path/filepath"
	"testing"
)

// TestTheInspectorFolds folds the inspector from a workflow's status bar:
// the Project tab's folds with it. Clicking the folded strip opens both
// again.
func TestTheInspectorFolds(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	foldedShot := shotPath(t, "inspector-folded.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Clerk's day"), must("state:tabs:2", 600),
		must("state:inspector-folded", 1200), must("state:inspector", 1200), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 900), wait(30), click("ui:inspector"), must("state:inspector-folded", 60), wait(10),
		shoot(foldedShot), click("ui:folded-inspector"), must("state:inspector", 60), wait(30), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", foldedShot)
}
