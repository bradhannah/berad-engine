package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// resubscribe loads a document again, as a window does for one that was
// deleted and has come back.
func resubscribe(t *testing.T, c *client.Client, document ids.ULID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Subscribe(ctx, document); err != nil {
		t.Fatal(err)
	}
}

// A refused undo or redo changes nothing and keeps its step. A stroke across
// two floors is one step; while another window has one of the floors
// deleted, the step cannot be undone or redone. The floor it did reach is
// put back as it was, and the step is still there to undo or redo once the
// deleted floor is back.
func TestRefusedUndoAndRedoKeepTheirStep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "browser", project.IndexID())
	first := createDocument(t, browser, floor.FormatName, "First", ids.ULID{}, newFloorData(t), "")
	second := createDocument(t, browser, floor.FormatName, "Second", ids.ULID{}, newFloorData(t), "")
	painter := h.dialView(t, "map", project.IndexID(), "painter", first, second)
	tiles := func() [2]string {
		return [2]string{tileAt(t, painter, first, 1, 1), tileAt(t, painter, second, 1, 1)}
	}
	mustPaint(t, painter, first, 1, 1, "wall", "stroke")
	mustPaint(t, painter, second, 1, 1, "wall", "stroke")

	mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: first})
	if _, err := undo(t, painter); err == nil {
		t.Fatal("a stroke on a deleted floor was undone")
	}
	if got := tileAt(t, painter, second, 1, 1); got != "wall" {
		t.Fatalf("a refused undo leaves the other floor as it was: %q", got)
	}
	if _, err := undo(t, browser); err != nil {
		t.Fatal(err)
	}
	resubscribe(t, painter, first)
	if _, err := undo(t, painter); err != nil {
		t.Fatalf("the refused step is still there to undo: %v", err)
	}
	if got := tiles(); got != [2]string{"grass", "grass"} {
		t.Fatalf("after undoing the stroke: %v", got)
	}

	mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: second})
	if _, err := redo(t, painter); err == nil {
		t.Fatal("a stroke on a deleted floor was redone")
	}
	if got := tileAt(t, painter, first, 1, 1); got != "grass" {
		t.Fatalf("a refused redo leaves the other floor as it was: %q", got)
	}
	if _, err := undo(t, browser); err != nil {
		t.Fatal(err)
	}
	resubscribe(t, painter, second)
	if _, err := redo(t, painter); err != nil {
		t.Fatalf("the refused step is still there to redo: %v", err)
	}
	if got := tiles(); got != [2]string{"wall", "wall"} {
		t.Fatalf("after redoing the stroke: %v", got)
	}
	if _, err := undo(t, painter); err != nil {
		t.Fatalf("the redone step undoes again: %v", err)
	}
	if got := tiles(); got != [2]string{"grass", "grass"} {
		t.Fatalf("after undoing the redone stroke: %v", got)
	}
}
