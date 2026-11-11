package orchestrator_test

import (
	"testing"

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
	}
	mustPaint(t, view, h.floor, 2, 1, "wall", "stroke")
	result, err := undo(t, view)
	if err != nil || result.Said != "Painted Floor 1" || tileAt(t, view, h.floor, 2, 1) == "wall" {
		t.Fatalf("undo says which change it took back, the whole stroke: %+v %v", result, err)
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
