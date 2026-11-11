package main_test

import (
	"path/filepath"
	"testing"
)

// TestALineDroppedOnEmptyCanvasOffersANode makes a workflow and drags a line
// from its start onto empty canvas: the node menu opens, and choosing Wait
// adds one there, joined by that line (increment 40).
func TestALineDroppedOnEmptyCanvasOffersANode(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "node-menu.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"),
		must("state:tabs:2", 600), must("state:tabs:1", 3600), wait(5), keys("Cmd+Q"), must("ui:quit", 300),
		wait(5), click("ui:quit"), wait(60),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(30), drag("port:start:1:next", "canvas:520,160"),
		must("menu:node-menu", 60), click("menu:node-group:Action"), must("menu:node-kind:wait", 60),
		step{Repeat: 2, Screenshot: shot, Front: false}, click("menu:node-kind:wait"), must("state:nodes:2", 120),
		must("state:entries:wait:1=1", 120), wait(10), keys("Cmd+W"), wait(30),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}
