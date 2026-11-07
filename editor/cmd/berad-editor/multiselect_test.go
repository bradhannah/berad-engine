package main_test

import (
	"path/filepath"
	"strings"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// cmdClick presses and releases the main button on a named part of the
// window with Cmd held.
func cmdClick(at string) []step {
	return []step{
		{Repeat: 2, At: at, Down: true, Pressed: true, Held: "Cmd"}, {Repeat: 2, At: at, Released: true, Held: "Cmd"},
	}
}

// TestMultiSelect makes start → Wait → Say → Log, boxes the Wait and the
// Say, Cmd+clicks the Log, drags and nudges the three, colours them green,
// duplicates them, copies and pastes them, and arranges the pasted ones:
// the saved workflow holds three green runs of Wait → Say → Log.
func TestMultiSelect(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	selectedShot, pastedShot := shotPath(t, "multiselect-selected.png"), shotPath(t, "multiselect-pasted.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5), click("node:start:1"), wait(5),
		addNode("wait"), must("state:nodes:2", 300), wait(10), addNode("say"), must("state:nodes:3", 300), wait(10),
		addNode("log"), must("state:nodes:4", 300), wait(10),
		drag("canvas:200,170", "canvas:660,30"), must("state:selection=2", 60), wait(5),
		cmdClick("node:log:1"), must("state:selection=3", 60), wait(5),
		drag("node:wait:1", "canvas:260,140"), wait(10), keys("Right"), wait(5), keys("Shift+Down"), wait(10),
		click("ui:swatch:group:green"), must("state:tint:log:1=green", 300), wait(10), shoot(selectedShot),
		keys("Cmd+D"), must("state:nodes:7", 300), must("state:selection=3", 60), wait(10),
		keys("Cmd+C"), wait(5), keys("Cmd+V"), must("state:nodes:10", 300), must("state:selection=3", 60), wait(10),
		click("ui:arrange"), wait(20), shoot(pastedShot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	decoded, err := workflowplugin.Decode(readEnvelope(t, project, "workflows/workflow-1--????????.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	saved := decoded.(*workflowplugin.Document).Workflow
	runs := 0
	for _, node := range saved.Nodes() {
		if node.Kind != flow.KindWait {
			continue
		}
		say, ok := saved.Node(node.Next[flow.OutputNext])
		if log, found := saved.Node(say.Next[flow.OutputNext]); ok && found && say.Kind == flow.KindSay &&
			log.Kind == flow.KindLog {
			runs++
		}
	}
	layout := string(readEnvelope(t, project, "workflows/workflow-1--*.layout.json").Data)
	green := strings.Count(layout, `"green"`)
	if len(saved.Nodes()) != 10 || runs != 3 || green != 9 {
		t.Fatalf("the saved workflow holds three green runs of Wait → Say → Log: %d nodes, %d runs, %d green",
			len(saved.Nodes()), runs, green)
	}
	t.Logf("screenshots: %s %s", selectedShot, pastedShot)
}
