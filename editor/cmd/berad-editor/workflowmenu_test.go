package main_test

import (
	"path/filepath"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
)

// TestWorkflowRightClickDeletes adds a Wait to a new workflow, right-clicks
// it and chooses Delete node: the saved workflow holds only its start.
func TestWorkflowRightClickDeletes(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "workflow-right-click.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5), click("node:start:1"), wait(5), addNode("wait"),
		must("state:nodes:2", 300), wait(10), rightClick("node:wait:1"), must("menu:delete-node", 60), wait(5),
		shoot(shot), click("menu:delete-node"), must("state:nodes:1", 300), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	decoded, err := workflowplugin.Decode(readEnvelope(t, project, "workflows/workflow-1--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	if nodes := decoded.(*workflowplugin.Document).Workflow.Nodes(); len(nodes) != 1 {
		t.Fatalf("the saved workflow holds %d nodes", len(nodes))
	}
	t.Logf("screenshot: %s", shot)
}
