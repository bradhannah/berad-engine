package mapview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestLinesInThePreview previews the clerk, whose workflow logs as it
// starts: the preview's state holds the line, numbered, with the clerk's
// placement and the workflow and node that wrote it.
func TestLinesInThePreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withWorkingClerk(t, f)
	f.docs.Put(worldstate.ID(), worldstate.Name, stateplugin.NewDocument())
	logged, err := flow.NewNode(flow.KindLog)
	if err == nil {
		logged, err = logged.WithParam(flow.ParamMessage, []flow.Segment{{Text: "Opening up", Ref: nil}})
	}
	if err != nil {
		t.Fatal(err)
	}
	logged.ID = frametest.ID(96)
	f.docs.View(routineID, func(document client.Document) {
		routine := document.Model.(*workflowplugin.Document).Workflow
		first, _ := routine.Node(frametest.ID(80))
		if err := routine.Insert([]flow.Node{logged.WithLink(flow.OutputNext, first.Next[flow.OutputNext])}, nil,
			[]flow.Link{{From: frametest.ID(80), Output: flow.OutputNext, To: logged.ID}}); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.Touch(routineID)
	f.startPreview("cell:3,3")
	f.idle(3)
	lines := f.lastState().Log
	if len(lines) == 0 || lines[0].Seq != 1 || lines[0].Text != "Opening up" || lines[0].Workflow != routineID ||
		lines[0].Node != logged.ID || lines[0].Entity != f.lastState().Entities[1].ID {
		t.Fatalf("the preview's state holds the clerk's line: %+v", lines)
	}
}
