package zone_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/engine/ids"
)

func floorID(seed byte) ids.ULID {
	var id ids.ULID
	id[15] = seed
	return id
}

func args(t *testing.T, floor byte, index int) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(zone.FloorArgs{Floor: floorID(floor), Index: index})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestFloorEditsMeetUndoContract inserts, moves and removes floors, checking
// each edit restores the zone exactly when undone.
func TestFloorEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(zone.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	document := zone.NewDocument()
	for index, seed := range []byte{1, 2, 3} {
		insert := args(t, seed, index)
		if err := model.VerifyUndo(document, zone.InsertFloor{}, insert, registry.Edit); err != nil {
			t.Fatal(err)
		}
		if _, err := (zone.InsertFloor{}).Apply(document, insert); err != nil {
			t.Fatal(err)
		}
	}
	for _, check := range []struct {
		kind model.EditKind
		args json.RawMessage
	}{
		{zone.MoveFloor{}, args(t, 3, 0)},
		{zone.MoveFloor{}, args(t, 1, 2)},
		{zone.RemoveFloor{}, args(t, 2, 0)},
		{zone.InsertFloor{}, args(t, 4, 1)},
	} {
		if err := model.VerifyUndo(document, check.kind, check.args, registry.Edit); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := (zone.InsertFloor{}).Apply(document, args(t, 2, 0)); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("duplicate floor: %v", err)
	}
	if parts := document.Parts(); len(parts) != 3 || len(document.References()) != 3 {
		t.Fatalf("parts %v", parts)
	}
}
