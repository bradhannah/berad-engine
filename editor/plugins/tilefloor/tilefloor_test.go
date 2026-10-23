package tilefloor_test

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
)

func paintArgs(t *testing.T, cells ...tilefloor.Cell) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(tilefloor.PaintArgs{Cells: cells})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestPaintMeetsUndoContract is the shared edit contract: apply then undo
// restores exactly the same data, including overlapping cells in one edit.
func TestPaintMeetsUndoContract(t *testing.T) {
	t.Parallel()
	document, err := tilefloor.NewDocument(16, 16, "grass")
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewPCG(1, 2))
	tiles := []string{"wall", "floor", "", "road"}
	for range 500 {
		cells := make([]tilefloor.Cell, 1+random.IntN(20))
		for index := range cells {
			cells[index] = tilefloor.Cell{Col: random.IntN(16), Row: random.IntN(16), Tile: tiles[random.IntN(len(tiles))]}
		}
		args := paintArgs(t, cells...)
		if err := model.VerifyUndo(document, tilefloor.Paint{}, args); err != nil {
			t.Fatal(err)
		}
		if _, err := (tilefloor.Paint{}).Apply(document, args); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPaintIsAllOrNothing(t *testing.T) {
	t.Parallel()
	document, err := tilefloor.NewDocument(4, 4, "grass")
	if err != nil {
		t.Fatal(err)
	}
	before, err := document.Encode()
	if err != nil {
		t.Fatal(err)
	}
	args := paintArgs(t, tilefloor.Cell{Col: 0, Row: 0, Tile: "wall"}, tilefloor.Cell{Col: 9, Row: 0, Tile: "wall"})
	if _, err := (tilefloor.Paint{}).Apply(document, args); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("err = %v", err)
	}
	after, err := document.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed paint changed the document")
	}
}

func TestPaintReportsTouchedCells(t *testing.T) {
	t.Parallel()
	document, err := tilefloor.NewDocument(4, 4, "grass")
	if err != nil {
		t.Fatal(err)
	}
	applied, err := (tilefloor.Paint{}).Apply(document, paintArgs(t, tilefloor.Cell{Col: 2, Row: 3, Tile: "wall"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(applied.Touched) != 1 || applied.Touched[0] != tilefloor.CellKey(2, 3) {
		t.Fatalf("touched = %v", applied.Touched)
	}
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(tilefloor.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Edit(tilefloor.PaintEdit); !ok {
		t.Fatal("paint edit not registered")
	}
	if _, ok := registry.Document("tilefloor"); !ok {
		t.Fatal("document type not registered")
	}
	if _, err := plugin.NewRegistry(tilefloor.Plugin(), tilefloor.Plugin()); !errors.Is(err, plugin.ErrDuplicate) {
		t.Fatalf("duplicate plug-in: %v", err)
	}
	renamed := tilefloor.Plugin()
	renamed.Name = "other"
	renamed.Documents = nil
	if _, err := plugin.NewRegistry(tilefloor.Plugin(), renamed); !errors.Is(err, plugin.ErrDuplicate) {
		t.Fatalf("duplicate edit: %v", err)
	}
}
