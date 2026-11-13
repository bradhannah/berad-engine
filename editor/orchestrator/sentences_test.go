package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugin"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	"github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	viewsplugin "github.com/bradhannah/berad-engine/editor/plugins/views"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TestChangesAreSaidAndUndoSaysWhich paints a stroke of two cells as one
// gesture: the paint is said as "Painted Floor 1", and Undo and Redo answer
// with that sentence (increment 39).
func TestChangesAreSaidAndUndoSaysWhich(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	view := h.dial(t, "map:said")
	if result := mustPaint(t, view, h.floor, 1, 1, "wall", "stroke"); result.Said != "Painted Floor 1" {
		t.Fatalf("a paint is said: %q", result.Said)
	} else if touched := result.Touched; touched.Document != h.floor || len(touched.Items) == 0 {
		t.Fatalf("a change says what it touched, for History to select (increment 45): %+v", touched)
	}
	mustPaint(t, view, h.floor, 2, 1, "wall", "stroke")
	result, err := undo(t, view)
	if err != nil || result.Said != "Painted Floor 1" || tileAt(t, view, h.floor, 2, 1) == "wall" {
		t.Fatalf("undo says which change it took back, the whole stroke: %+v %v", result, err)
	}
	if result.Touched.Document != h.floor || len(result.Touched.Items) == 0 {
		t.Fatalf("undo says what it touched: %+v", result.Touched)
	}
	result, err = redo(t, view)
	if err != nil || result.Said != "Painted Floor 1" {
		t.Fatalf("redo says which change it put back: %+v %v", result, err)
	}
	if result, err = undo(t, view); err != nil || result.Said != "Painted Floor 1" {
		t.Fatalf("and undo again: %+v %v", result, err)
	}
	if result, err = undo(t, view); err == nil || result.Said != "" {
		t.Fatalf("with nothing left to undo, nothing is said: %+v", result)
	}
}

// TestEveryEditHasWords checks that every kind of edit the editor's plug-ins
// make is said in words of its own, not by its code name.
func TestEveryEditHasWords(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(
		project.Plugin(), tilefloor.Plugin(), zone.Plugin(), workflow.Plugin(), npc.Plugin(), artplugin.Plugin(),
		gameplugin.Plugin(), stateplugin.Plugin(), viewsplugin.Plugin(),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range registry.EditNames() {
		if !orchestrator.EditHasWords(edit) {
			t.Errorf("%s has no words", edit)
		}
	}
}

// paintSaying paints a cell with a sentence of the view's own.
func paintSaying(t *testing.T, c *client.Client, floor ids.ULID, col int, gesture, says string) protocol.Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.Command(ctx, protocol.Command{
		Document: floor, Edit: tilefloor.PaintEdit, Gesture: gesture,
		Args: paintArgs(t, tilefloor.Cell{Col: col, Row: 1, Layer: tilefloor.LayerGround, Tile: "wall"}), Says: says,
	})
	if err != nil {
		t.Fatal(err)
	}

	return result
}

// TestAViewSaysItsOwnSentence says a change in the view's words in place of
// the orchestrator's, and the latest of a stroke's sentences is the step's,
// which Undo answers with (increment 51).
func TestAViewSaysItsOwnSentence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	view := h.dial(t, "map:own")
	if result := paintSaying(t, view, h.floor, 1, "stroke", "Painted 1 cell of Wall on Floor 1"); result.Said !=
		"Painted 1 cell of Wall on Floor 1" {
		t.Fatalf("the view's sentence is said: %q", result.Said)
	}
	paintSaying(t, view, h.floor, 2, "stroke", "Painted 2 cells of Wall on Floor 1")
	if result, err := undo(t, view); err != nil || result.Said != "Painted 2 cells of Wall on Floor 1" {
		t.Fatalf("undo says the whole stroke: %+v %v", result, err)
	}
}
