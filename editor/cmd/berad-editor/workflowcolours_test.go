package main_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestWorkflowLegsAndColoursAreSaved makes start → random choice → Wait with
// a line from the Wait back to the random choice, auto-arranges it, drags
// the loop's end leg to the left, colours the random choice's first outcome
// amber and the Wait green, and shows the legend: the saved layout holds
// the end leg's offset and both tints.
func TestWorkflowLegsAndColoursAreSaved(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "workflow-colours.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), addNode("random"), must("state:nodes:2", 300), wait(10),
		addNode("wait"), must("state:nodes:3", 300), wait(10),
		drag("port:wait:1:next", "node:random:1"), must("state:entries:random:1=2", 300), wait(10),
		click("ui:arrange"), wait(20),
		drag("bend:wait:1:next:end", "canvas:150,300"), must("state:bent:wait:1:next", 300), wait(10),
		click("node:random:1"), wait(10), click("ui:swatch:outcome:o1:amber"),
		must("state:outcome-tint:random:1:o1=amber", 300), wait(10),
		click("node:wait:1"), wait(10), click("ui:swatch:node:green"), must("state:tint:wait:1=green", 300), wait(10),
		click("ui:legend"), must("state:legend", 60), wait(10), shoot(shot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	layout := string(readEnvelope(t, project, "workflows/workflow-1--*.layout.json").Data)
	for _, want := range []string{`"end"`, `"tints"`, `"amber"`, `"green"`} {
		if !strings.Contains(layout, want) {
			t.Fatalf("the saved layout holds %s: %s", want, layout)
		}
	}
	t.Logf("screenshot: %s", shot)
}
