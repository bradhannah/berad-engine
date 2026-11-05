package main_test

import (
	"path/filepath"
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestFindingAndPlacingNodes searches a new workflow's palette for
// "value", reads Check value's help and adds it after the start with Add
// after; rests on Wait for its tooltip; then clears the search and drags a
// Wait onto the line from the start, where it lands connected to nothing.
func TestFindingAndPlacingNodes(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	searchShot, helpShot, tooltipShot, dropShot := shotPath(t, "palette-search.png"), shotPath(t, "palette-help.png"),
		shotPath(t, "palette-tooltip.png"), shotPath(t, "palette-drop.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5), click("node:start:1"), wait(5), click("ui:palette-search"), wait(5),
		typed("value"), must("result:"+flow.KindCheckValue, 120), wait(10), shoot(searchShot),
		click("result:"+flow.KindCheckValue), must("state:help="+flow.KindCheckValue, 120), wait(10), shoot(helpShot),
		click("ui:help-add"), must("state:selected:"+flow.KindCheckValue, 300), must("state:nodes:2", 60), wait(10),
		click("ui:palette-search"), keys("Escape"), wait(5),
		step{Repeat: 45, At: "palette:" + flow.KindWait}, must("state:tooltip="+flow.KindWait, 30), shoot(tooltipShot),
		drag("palette:"+flow.KindWait, "line:start:1:next"), must("state:nodes:3", 300),
		must("state:selected:"+flow.KindWait, 120), wait(10), shoot(dropShot), wait(30), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	saved, err := flow.Decode(readEnvelope(t, project, "workflows/workflow-1--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	start, _ := saved.Node(saved.Start())
	check, checked := saved.Node(start.Next[flow.OutputNext])
	if !checked || check.Kind != flow.KindCheckValue {
		t.Fatalf("the start still leads to Check value: %+v", saved.Nodes())
	}
	for _, node := range saved.Nodes() {
		if node.Kind == flow.KindWait && len(node.Next) != 0 {
			t.Fatalf("the dropped Wait is connected to nothing: %+v", node)
		}
	}
	t.Logf("screenshots: %s %s %s %s", searchShot, helpShot, tooltipShot, dropShot)
}
