package main_test

import (
	"path/filepath"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestWorkflowComparesWithAnotherValue adds a Check value to a new workflow,
// declares world.stock from its chooser, compares it with ≥ and picks
// another value, world.stock itself, on the right: the saved logic holds the
// comparison and the reference.
func TestWorkflowComparesWithAnotherValue(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "workflow-compare.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:"+flow.KindCheckValue), must("state:nodes:2", 300), wait(10),
		click("ui:pick-value"), wait(5), typed("world.stock"), wait(10), click("declare:world.stock"),
		must("state:value:value=world.stock", 300), wait(10),
		click("ui:compare-"+flow.CompareLeast), wait(10), click("ui:source-value"), wait(10),
		click("valuerow:world.stock"), must("state:refers:against=world.stock", 300), wait(10),
		shoot(shot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	decoded, err := workflowplugin.Decode(readEnvelope(t, project, "workflows/workflow-1--*[^t].json").Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range decoded.(*workflowplugin.Document).Workflow.Nodes() {
		if node.Kind != flow.KindCheckValue {
			continue
		}
		compare, err := node.Choice(flow.ParamCompare)
		against, operandErr := node.Operand(flow.ParamAgainst)
		_, refers := flow.OperandRef(against)
		if err != nil || operandErr != nil || compare != flow.CompareLeast || !refers {
			t.Fatalf("the saved check compares ≥ another value: %q %s %v %v", compare, against, err, operandErr)
		}
		t.Logf("screenshot: %s", shot)

		return
	}
	t.Fatal("no Check value was saved")
}
