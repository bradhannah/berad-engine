package main_test

import (
	"path/filepath"
	"testing"
)

// TestUndoSaysWhatItTookBack paints a stroke of road on the sample's ground
// floor and undoes it: the status bar names the tool in hand, Undo answers
// with the sentence the paint was said with, and History lists both
// (increment 39).
func TestUndoSaysWhatItTookBack(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "history.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("row:Zones"), wait(20), doubleClick("row:Sample zone"),
		must("state:tabs:2", 600), must("state:tabs:1", 3600), wait(5), keys("Cmd+Q"), must("ui:quit", 300),
		wait(5), click("ui:quit"), wait(60),
	)
	mapScript := script(
		must("state:loaded", 600), wait(30), keys("3"), drag("cell:4,8", "cell:5,8"), wait(10),
		must("state:hand:Ground floor · Ground · Road · cell 5, 8: Road", 60),
		must("state:said:Painted Ground floor", 120), keys("Cmd+Z"), must("state:said:Undid: Painted Ground floor", 120),
		click("ui:history"), must("state:history-open", 60), step{Repeat: 2, Screenshot: shot, Front: false},
		keys("Escape"), wait(5), keys("Cmd+W"), wait(30),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {mapScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}
