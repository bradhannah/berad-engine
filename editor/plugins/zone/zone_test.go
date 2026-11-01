package zone_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
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

// TestPathAndValueEditsMeetUndoContract names a zone, declares a value of
// its own and applies a template, each undoing exactly.
func TestPathAndValueEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(zone.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	edit := func(name string) model.EditKind {
		kind, ok := registry.Edit(name)
		if !ok {
			t.Fatalf("no edit %s", name)
		}
		return kind
	}
	encode := func(value any) json.RawMessage {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	crowded := worldstate.New(floorID(9), "crowded", worldstate.ScopeZone, worldstate.TypeBool)
	crowded.ID[0] = 1
	document := zone.NewDocument()
	for _, check := range []struct {
		kind model.EditKind
		args json.RawMessage
	}{
		{zone.SetPath{}, encode(zone.SetPathArgs{Path: "market"})},
		{edit(zone.ChangeValuesEdit), encode(stateplugin.ChangeArgs{Add: []worldstate.Declaration{crowded}, Set: nil, Remove: nil})},
		{edit(zone.ApplyTemplateEdit), encode(stateplugin.TemplateArgs{Template: floorID(10), Index: 0})},
	} {
		if err := model.VerifyUndo(document, check.kind, check.args, registry.Edit); err != nil {
			t.Fatalf("%s: %v", check.kind.Name(), err)
		}
		if _, err := check.kind.Apply(document, check.args); err != nil {
			t.Fatal(err)
		}
	}
	if document.Zone.Path != "market" || document.Zone.Owned.Values.Len() != 1 || len(document.References()) != 1 {
		t.Fatalf("named, with a value and a template: %+v", document.Zone)
	}
	if _, err := (zone.SetPath{}).Apply(document, encode(zone.SetPathArgs{Path: "Market Square"})); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("a path name follows the rules for value names: %v", err)
	}
}
