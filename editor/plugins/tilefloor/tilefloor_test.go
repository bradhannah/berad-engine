package tilefloor_test

import (
	"encoding/json"
	"errors"
	"image"
	"math/rand/v2"
	"slices"
	"strings"
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

// TestResizeMeetsUndoContract resizes a painted floor with anchors at random,
// growing and shrinking on every side: each resize, and the inverse it
// returns, restores exactly what was there.
func TestResizeMeetsUndoContract(t *testing.T) {
	t.Parallel()
	lookup := registry(t).Edit
	document, err := tilefloor.NewDocumentAt(floor.Centered(8, 6), "grass")
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewPCG(3, 4))
	for seed := range byte(6) {
		bounds := document.Floor.Bounds()
		col, row := bounds.Min.X+random.IntN(bounds.Dx()), bounds.Min.Y+random.IntN(bounds.Dy())
		if _, err := document.Floor.SetTop(col, row, "wall.a"); err != nil {
			t.Fatal(err)
		}
		if _, err := document.Floor.SetGround(col, row, "road"); err != nil {
			t.Fatal(err)
		}
		if _, taken := document.Floor.AnchorAt(col, row); !taken {
			spot := floor.Anchor{ID: anchorID(seed + 1), Name: "Spot " + string(rune('A'+seed)), Col: col, Row: row}
			if err := document.Floor.InsertAnchor(int(seed), spot); err != nil {
				t.Fatal(err)
			}
		}
		if _, taken := document.Floor.PlacementAt(col, row); !taken {
			placed := floor.Placement{ID: anchorID(seed + 20), NPC: anchorID(seed + 40), Col: col, Row: row}
			if err := document.Floor.InsertPlacement(int(seed), placed); err != nil {
				t.Fatal(err)
			}
		}
	}
	grounds := []string{"grass", "", "floor"}
	for range 200 {
		bounds := document.Floor.Bounds()
		next := image.Rect(
			bounds.Min.X+random.IntN(5)-2, bounds.Min.Y+random.IntN(5)-2,
			bounds.Max.X+random.IntN(5)-2, bounds.Max.Y+random.IntN(5)-2,
		)
		if next == bounds || next.Dx() < 1 || next.Dy() < 1 {
			continue
		}
		args := encode(t, tilefloor.ResizeArgs{
			Bounds: tilefloor.BoundsOf(next), Ground: grounds[random.IntN(len(grounds))], Cells: nil, Anchors: nil,
			Placements: nil,
		})
		if err := model.VerifyUndo(document, tilefloor.Resize{}, args, lookup); err != nil {
			t.Fatal(err)
		}
		applied, err := (tilefloor.Resize{}).Apply(document, args)
		if err != nil {
			t.Fatal(err)
		}
		if err := model.VerifyUndo(document, tilefloor.Resize{}, applied.Inverse, lookup); err != nil {
			t.Fatalf("the inverse of a resize must itself undo exactly: %v", err)
		}
	}
}

// TestResizeRestoresWhatItRemoved shrinks a floor past a wall and an anchor,
// then applies the inverse: both come back in the same place.
func TestResizeRestoresWhatItRemoved(t *testing.T) {
	t.Parallel()
	document, err := tilefloor.NewDocument(4, 3, "grass")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Floor.SetTop(3, 1, "wall.b"); err != nil {
		t.Fatal(err)
	}
	if err := document.Floor.InsertAnchor(0, floor.Anchor{ID: anchorID(1), Name: "Edge", Col: 3, Row: 2}); err != nil {
		t.Fatal(err)
	}
	shrink := tilefloor.ResizeArgs{Bounds: tilefloor.Bounds{Col: -1, Row: 0, Width: 4, Height: 3}, Ground: "road"}
	applied, err := (tilefloor.Resize{}).Apply(document, encode(t, shrink))
	if err != nil {
		t.Fatal(err)
	}
	if document.Floor.In(3, 1) || len(document.Floor.Anchors()) != 0 {
		t.Fatal("the right column should be gone with its anchor")
	}
	if ground, _ := document.Floor.Ground(-1, 0); ground != "road" { //nolint:errcheck // On the floor.
		t.Fatalf("new column ground %q", ground)
	}
	if !slices.Contains(applied.Touched, tilefloor.BoundsKey) ||
		!slices.Contains(applied.Touched, tilefloor.CellKey(tilefloor.LayerTop, 3, 1)) ||
		!slices.Contains(applied.Touched, tilefloor.AnchorKey(anchorID(1))) {
		t.Fatalf("touched %v", applied.Touched)
	}
	if _, err := (tilefloor.Resize{}).Apply(document, applied.Inverse); err != nil {
		t.Fatal(err)
	}
	if top, _ := document.Floor.Top(3, 1); top != "wall.b" { //nolint:errcheck // On the floor.
		t.Fatalf("restored wall %q", top)
	}
	if _, found := document.Floor.AnchorAt(3, 2); !found || document.Floor.In(-1, 0) {
		t.Fatal("undo should restore the anchor and remove the added column")
	}
}

func TestResizeRejectsBadArguments(t *testing.T) {
	t.Parallel()
	document, err := tilefloor.NewDocument(4, 4, "grass")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []tilefloor.ResizeArgs{
		{Bounds: tilefloor.Bounds{Col: 0, Row: 0, Width: 4, Height: 4}},
		{Bounds: tilefloor.Bounds{Col: 0, Row: 0, Width: 0, Height: 4}},
		{Bounds: tilefloor.Bounds{Col: 0, Row: 0, Width: 5, Height: 4}, Cells: []tilefloor.Cell{
			{Col: 1, Row: 1, Layer: tilefloor.LayerTop, Tile: "wall"},
		}},
		{Bounds: tilefloor.Bounds{Col: 0, Row: 0, Width: 5, Height: 4}, Anchors: []tilefloor.AddAnchorArgs{
			{Anchor: floor.Anchor{ID: anchorID(2), Name: "Inside", Col: 0, Row: 0}, Index: 0},
		}},
		{Bounds: tilefloor.Bounds{Col: -300, Row: 0, Width: 304, Height: 300}},
	} {
		if _, err := (tilefloor.Resize{}).Apply(document, encode(t, bad)); !errors.Is(err, model.ErrArgs) {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	if document.Floor.Bounds() != image.Rect(0, 0, 4, 4) {
		t.Fatal("a refused resize changed the floor")
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
// TestPlacementEditsMeetUndoContract places, moves and removes an NPC,
// checking each edit restores the floor exactly when undone, and that the
// floor lists the NPCs it places for where-used.
func TestPlacementEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	lookup := registry(t).Edit
	document, err := tilefloor.NewDocumentAt(floor.Centered(6, 6), "grass")
	if err != nil {
		t.Fatal(err)
	}
	clerk := floor.Placement{ID: anchorID(1), NPC: anchorID(9), Col: 0, Row: 0}
	add := encode(t, tilefloor.AddPlacementArgs{Placement: clerk, Index: 0})
	if err := model.VerifyUndo(document, tilefloor.AddPlacement{}, add, lookup); err != nil {
		t.Fatal(err)
	}
	if _, err := (tilefloor.AddPlacement{}).Apply(document, add); err != nil {
		t.Fatal(err)
	}
	moved := clerk
	moved.Col = 2
	for _, check := range []struct {
		kind model.EditKind
		args json.RawMessage
	}{
		{tilefloor.UpdatePlacement{}, encode(t, tilefloor.UpdatePlacementArgs{Placement: moved})},
		{tilefloor.RemovePlacement{}, encode(t, tilefloor.RemovePlacementArgs{ID: clerk.ID})},
	} {
		if err := model.VerifyUndo(document, check.kind, check.args, lookup); err != nil {
			t.Fatalf("%s: %v", check.kind.Name(), err)
		}
	}
	crowded := floor.Placement{ID: anchorID(2), NPC: anchorID(10), Col: 0, Row: 0}
	if _, err := (tilefloor.AddPlacement{}).Apply(document, encode(t, tilefloor.AddPlacementArgs{
		Placement: crowded, Index: 1,
	})); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("one NPC per cell: %v", err)
	}
	references := document.References()
	if len(references) != 1 || references[0].Document != clerk.NPC || references[0].Label != "placed at 0, 0" {
		t.Fatalf("references %+v", references)
	}
	prepared, err := (tilefloor.AddPlacement{}).Prepare(document, encode(t, tilefloor.AddPlacementArgs{
		Placement: floor.Placement{ID: anchorID(0), NPC: anchorID(10), Col: 1, Row: 1}, Index: 1,
	}), func() (ids.ULID, error) { return anchorID(30), nil })
	if err != nil || !strings.Contains(string(prepared), anchorID(30).String()) {
		t.Fatalf("a new placement gets its ID when prepared: %s %v", prepared, err)
	}
}

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
