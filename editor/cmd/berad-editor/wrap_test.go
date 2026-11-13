package main_test

import (
	"path/filepath"
	"testing"
)

// TestASayWrapsOnItsNode makes a workflow, adds a Say after its start from
// the node menu and types it a long line: the Say grows to show the line in
// three lines (increment 50).
func TestASayWrapsOnItsNode(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "say-wraps.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"),
		must("state:tabs:2", 600), must("state:tabs:1", 3600), wait(5), keys("Cmd+Q"), must("ui:quit", 300),
		wait(5), click("ui:quit"), wait(60),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(30), drag("port:start:1:next", "canvas:520,160"),
		must("menu:node-menu", 60), click("menu:node-group:Show"), must("menu:node-kind:say", 60),
		click("menu:node-kind:say"), must("state:nodes:2", 120), click("node:say:1"), must("state:selected:say", 60),
		click("ui:param-message"), wait(5), typed("Fresh bread, still warm from the oven this morning!"),
		keys("Enter"), must("state:say-lines:say:1=3", 120), wait(10),
		step{Repeat: 2, Screenshot: shot, Front: false}, keys("Cmd+W"), wait(30),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}
