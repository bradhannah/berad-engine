package orchestrator_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

func readEnvelopeFile(t *testing.T, path string) content.Envelope {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // A test's own temporary project.
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := content.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	return envelope
}

func onlyFile(t *testing.T, dir, pattern string) string {
	t.Helper()
	matches := files(t, dir, pattern)
	if len(matches) != 1 {
		t.Fatalf("%s: %v", pattern, matches)
	}

	return matches[0]
}

func boxOf(c *client.Client, workflowID, node ids.ULID) (flow.Box, bool) {
	var box flow.Box
	var placed bool
	c.View(workflowID, func(document client.Document) {
		box, placed = document.Model.(*workflow.Document).Workflow.Box(node)
	})

	return box, placed
}

// D4, D11, D17: a workflow's start node gets its ID when the workflow is
// created; its layout is saved in a file of its own beside it, so moving a
// node leaves the logic untouched; the layout is read back on opening,
// follows a rename, and comes back when a deletion is undone.
func TestWorkflowLayoutHasItsOwnFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	routine := createDocument(t, browser, flow.FormatName, "Daily routine", ids.ULID{}, workflow.NewData(), "")
	editor := h.dialView(t, "workflow", routine, "workflow:"+routine.String(), routine)
	var startID ids.ULID
	editor.View(routine, func(document client.Document) { startID = document.Model.(*workflow.Document).Workflow.Start() })
	if startID.IsZero() {
		t.Fatal("the start node has no ID")
	}
	wait, err := flow.NewNode(flow.KindWait)
	if err != nil {
		t.Fatal(err)
	}
	added := mustCommand(t, editor, routine, workflow.AddEdit, "", workflow.AddArgs{
		Nodes: []flow.Node{wait}, Boxes: map[ids.ULID]flow.Box{{}: {X: 160, Y: 40, W: 0, H: 0}}, Links: nil,
	})
	waitID := added.Created
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	logicPath := onlyFile(t, dir, "workflows/daily-routine--*[^t].json")
	layoutPath := onlyFile(t, dir, "workflows/daily-routine--*.layout.json")
	logic, layout := readEnvelopeFile(t, logicPath), readEnvelopeFile(t, layoutPath)
	if layout.Format != flow.LayoutFormatName || strings.Contains(string(logic.Data), "boxes") ||
		!strings.Contains(string(layout.Data), waitID.String()) || layout.Revision != 0 {
		t.Fatalf("logic and layout are not apart:\n%s\n%s", logic.Data, layout.Data)
	}

	again := start(t, dir, nil)
	reopened := again.dialView(t, "workflow", routine, "workflow:"+routine.String(), routine)
	if box, placed := boxOf(reopened, routine, waitID); !placed || box.X != 160 {
		t.Fatalf("the layout was not read back: %v %v", box, placed)
	}
	mustCommand(t, reopened, routine, workflow.MoveEdit, "", workflow.MoveArgs{
		Boxes: map[ids.ULID]flow.Box{waitID: {X: 400, Y: 40, W: 0, H: 0}},
	})
	if err := again.stop(); err != nil {
		t.Fatal(err)
	}
	if moved := readEnvelopeFile(t, logicPath); !bytes.Equal(moved.Data, logic.Data) {
		t.Fatal("moving a node changed the logic")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, readEnvelopeFile(t, layoutPath).Data); err != nil || !strings.Contains(compact.String(), `"x":400`) {
		t.Fatalf("the move is not in the layout file: %s %v", compact.String(), err)
	}

	third := start(t, dir, nil)
	browser = third.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	mustCommand(t, browser, project.IndexID(), project.RenameEdit, "", project.RenameArgs{ID: routine, Name: "Weekday"})
	mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: routine})
	if _, err := undo(t, browser); err != nil {
		t.Fatal(err)
	}
	restored := third.dialView(t, "workflow", routine, "workflow:"+routine.String(), routine)
	if box, placed := boxOf(restored, routine, waitID); !placed || box.X != 400 {
		t.Fatalf("undoing the deletion lost the layout: %v %v", box, placed)
	}
	if err := third.stop(); err != nil {
		t.Fatal(err)
	}
	onlyFile(t, dir, "workflows/weekday--*[^t].json")
	onlyFile(t, dir, "workflows/weekday--*.layout.json")
	if len(files(t, dir, "workflows/daily-routine--*")) != 0 || len(files(t, dir, ".berad/backups/removed/*layout*")) != 1 {
		t.Fatal("the renamed workflow's old files did not both move to the backups")
	}
}
