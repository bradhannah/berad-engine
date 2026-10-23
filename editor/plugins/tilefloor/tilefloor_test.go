package tilefloor_test

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

func registry(t *testing.T) *plugin.Registry {
	t.Helper()
	registry, err := plugin.NewRegistry(tilefloor.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func encode(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func paintArgs(t *testing.T, cells ...tilefloor.Cell) json.RawMessage {
	t.Helper()
	return encode(t, tilefloor.PaintArgs{Cells: cells})
}

func anchorID(seed byte) ids.ULID {
	var id ids.ULID
	id[15] = seed
	return id
}

// TestPaintMeetsUndoContract is the shared edit contract: apply then undo
// restores exactly the same data, including overlapping cells and both layers
// in one edit.
func TestPaintMeetsUndoContract(t *testing.T) {
	t.Parallel()
	lookup := registry(t).Edit
	document, err := tilefloor.NewDocument(16, 16, "grass")
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewPCG(1, 2))
	tiles := []string{"wall", "floor", "", "road", "door"}
	layers := []string{tilefloor.LayerGround, tilefloor.LayerTop}
	for range 500 {
		cells := make([]tilefloor.Cell, 1+random.IntN(20))
		for index := range cells {
			cells[index] = tilefloor.Cell{
				Col: random.IntN(16), Row: random.IntN(16),
				Layer: layers[random.IntN(2)], Tile: tiles[random.IntN(len(tiles))],
			}
		}
		args := paintArgs(t, cells...)
		if err := model.VerifyUndo(document, tilefloor.Paint{}, args, lookup); err != nil {
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
	for _, args := range []json.RawMessage{
		paintArgs(t,
			tilefloor.Cell{Col: 0, Row: 0, Layer: tilefloor.LayerTop, Tile: "wall"},
			tilefloor.Cell{Col: 9, Row: 0, Layer: tilefloor.LayerTop, Tile: "wall"}),
		paintArgs(t,
			tilefloor.Cell{Col: 0, Row: 0, Layer: tilefloor.LayerTop, Tile: "wall"},
			tilefloor.Cell{Col: 1, Row: 0, Layer: "sky", Tile: "cloud"}),
	} {
		if _, err := (tilefloor.Paint{}).Apply(document, args); !errors.Is(err, model.ErrArgs) {
			t.Fatalf("err = %v", err)
		}
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
	args := paintArgs(t, tilefloor.Cell{Col: 2, Row: 3, Layer: tilefloor.LayerTop, Tile: "wall"})
	applied, err := (tilefloor.Paint{}).Apply(document, args)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied.Touched) != 1 || applied.Touched[0] != tilefloor.CellKey(tilefloor.LayerTop, 2, 3) {
		t.Fatalf("touched = %v", applied.Touched)
	}
	if tilefloor.CellKey(tilefloor.LayerGround, 2, 3) == applied.Touched[0] {
		t.Fatal("the two layers of a cell must not clash")
	}
}

// TestAnchorEditsMeetUndoContract covers adding, moving, renaming and removing
// anchors, whose inverses use a different edit kind.
func TestAnchorEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	lookup := registry(t).Edit
	document, err := tilefloor.NewDocument(8, 8, "grass")
	if err != nil {
		t.Fatal(err)
	}
	add := func(seed byte, name string, col, row, index int) {
		t.Helper()
		args := encode(t, tilefloor.AddAnchorArgs{
			Anchor: floor.Anchor{ID: anchorID(seed), Name: name, Col: col, Row: row}, Index: index,
		})
		if err := model.VerifyUndo(document, tilefloor.AddAnchor{}, args, lookup); err != nil {
			t.Fatal(err)
		}
		if _, err := (tilefloor.AddAnchor{}).Apply(document, args); err != nil {
			t.Fatal(err)
		}
	}
	add(1, "Door", 1, 1, 0)
	add(2, "Desk", 2, 2, 0)
	add(3, "Bed", 3, 3, 1)
	update := encode(t, tilefloor.UpdateAnchorArgs{Anchor: floor.Anchor{ID: anchorID(3), Name: "Cot", Col: 7, Row: 7}})
	if err := model.VerifyUndo(document, tilefloor.UpdateAnchor{}, update, lookup); err != nil {
		t.Fatal(err)
	}
	for _, seed := range []byte{1, 2, 3} {
		remove := encode(t, tilefloor.RemoveAnchorArgs{ID: anchorID(seed)})
		if err := model.VerifyUndo(document, tilefloor.RemoveAnchor{}, remove, lookup); err != nil {
			t.Fatal(err)
		}
	}
	clash := encode(t, tilefloor.AddAnchorArgs{
		Anchor: floor.Anchor{ID: anchorID(4), Name: "door", Col: 0, Row: 0}, Index: 0,
	})
	if _, err := (tilefloor.AddAnchor{}).Apply(document, clash); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("duplicate name: %v", err)
	}
}

// TestAddAnchorPrepareAssignsID checks that the orchestrator-side step gives a
// new anchor its permanent ID once, and leaves a complete argument set alone.
func TestAddAnchorPrepareAssignsID(t *testing.T) {
	t.Parallel()
	document, err := tilefloor.NewDocument(4, 4, "grass")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	source := func() (ids.ULID, error) {
		calls++
		return anchorID(9), nil
	}
	args := encode(t, tilefloor.AddAnchorArgs{Anchor: floor.Anchor{ID: ids.ULID{}, Name: "Spot", Col: 0, Row: 0}, Index: 0})
	prepared, err := (tilefloor.AddAnchor{}).Prepare(document, args, source)
	if err != nil {
		t.Fatal(err)
	}
	var add tilefloor.AddAnchorArgs
	if err := json.Unmarshal(prepared, &add); err != nil || add.Anchor.ID != anchorID(9) {
		t.Fatalf("prepared %s %v", prepared, err)
	}
	again, err := (tilefloor.AddAnchor{}).Prepare(document, prepared, source)
	if err != nil || string(again) != string(prepared) || calls != 1 {
		t.Fatalf("preparing twice changed the arguments: %s %d %v", again, calls, err)
	}
	if document.CanPick(tilefloor.PickAnchor) {
		t.Fatal("an empty floor has nothing to pick")
	}
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	t.Parallel()
	registry := registry(t)
	for _, name := range []string{
		tilefloor.PaintEdit, tilefloor.AddAnchorEdit, tilefloor.RemoveAnchorEdit, tilefloor.UpdateAnchorEdit,
	} {
		if _, ok := registry.Edit(name); !ok {
			t.Fatalf("%s not registered", name)
		}
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
