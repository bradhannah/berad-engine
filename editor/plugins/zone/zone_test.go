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

// TestPaletteEditsMeetUndoContract lists and unlists palettes, checking each
// edit restores the zone exactly, and that a listed palette is referred to.
func TestPaletteEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(zone.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	document := zone.NewDocument()
	listed := func(palette byte, index int) json.RawMessage {
		data, err := json.Marshal(zone.PaletteArgs{Palette: floorID(palette), Index: index})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for index, palette := range []byte{7, 8} {
		if err := model.VerifyUndo(document, zone.InsertPalette{}, listed(palette, index), registry.Edit); err != nil {
			t.Fatal(err)
		}
		if _, err := (zone.InsertPalette{}).Apply(document, listed(palette, index)); err != nil {
			t.Fatal(err)
		}
	}
	if err := model.VerifyUndo(document, zone.RemovePalette{}, listed(7, 0), registry.Edit); err != nil {
		t.Fatal(err)
	}
	if _, err := (zone.InsertPalette{}).Apply(document, listed(8, 0)); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("a palette listed twice: %v", err)
	}
	references := document.References()
	if len(references) != 2 || references[0].Document != floorID(7) || references[0].Label != "listed by the zone" {
		t.Fatalf("listed palettes are references: %+v", references)
	}
}

func TestSoundEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(zone.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	document := zone.NewDocument()
	listed := func(cue string, index int) json.RawMessage {
		data, err := json.Marshal(zone.SoundArgs{Cue: cue, Index: index})
		if err != nil {
			t.Fatal(err)
		}

		return data
	}
	for index, cue := range []string{"music/town", "bed/wind"} {
		if err := model.VerifyUndo(document, zone.InsertSound{}, listed(cue, index), registry.Edit); err != nil {
			t.Fatal(err)
		}
		if _, err := (zone.InsertSound{}).Apply(document, listed(cue, index)); err != nil {
			t.Fatal(err)
		}
	}
	if err := model.VerifyUndo(document, zone.RemoveSound{}, listed("music/town", 0), registry.Edit); err != nil {
		t.Fatal(err)
	}
	if _, err := (zone.InsertSound{}).Apply(document, listed("bed/wind", 0)); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("a sound listed twice: %v", err)
	}
	references := document.References()
	if len(references) != 2 || references[0].Document != model.AudioLibraryID() || references[0].Item != "music/town" {
		t.Fatalf("a zone's sounds are references to their cues: %+v", references)
	}
}
