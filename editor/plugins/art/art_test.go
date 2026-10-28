package art_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

func encode(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func id(number int) ids.ULID {
	return ids.MustParse(fmt.Sprintf("01J0000000000000000000%04d", number))
}

func registry(t *testing.T) *plugin.Registry {
	t.Helper()
	registry, err := plugin.NewRegistry(artplugin.Plugin())
	if err != nil {
		t.Fatal(err)
	}

	return registry
}

func shared(t *testing.T) *artplugin.PaletteDocument {
	t.Helper()
	palette, err := art.StarterPalette()
	if err != nil {
		t.Fatal(err)
	}

	return &artplugin.PaletteDocument{Palette: palette}
}

func grey(number int) art.ColorRef {
	return art.ColorRef{Palette: art.SharedPaletteID(), Ramp: art.StarterRampID(), Shade: art.StarterShadeID(number)}
}

func blankTile(t *testing.T) *artplugin.TileDocument {
	t.Helper()
	tile, err := art.NewTile(art.MinTileSize)
	if err != nil {
		t.Fatal(err)
	}

	return &artplugin.TileDocument{Tile: tile}
}

type check struct {
	kind model.EditKind
	args json.RawMessage
}

// verify checks each edit meets the undo contract, then applies it, so the
// next check sees its result.
func verify(t *testing.T, document model.Document, checks []check) {
	t.Helper()
	edits := registry(t)
	for _, step := range checks {
		if err := model.VerifyUndo(document, step.kind, step.args, edits.Edit); err != nil {
			t.Fatalf("%s: %v", step.kind.Name(), err)
		}
		if _, err := step.kind.Apply(document, step.args); err != nil {
			t.Fatalf("%s: %v", step.kind.Name(), err)
		}
	}
}

func TestPaletteEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	sand := art.Ramp{ID: id(1), Name: "sand", Shades: []art.Shade{
		{ID: id(2), Color: 0xC2AE6A, Opacity: 100}, {ID: id(3), Color: 0xE0D2A0, Opacity: 100},
	}}
	verify(t, shared(t), []check{
		{artplugin.AddRamp{}, encode(t, artplugin.AddRampArgs{Index: 1, Ramp: sand})},
		{artplugin.RenameRamp{}, encode(t, artplugin.RenameRampArgs{Ramp: id(1), Name: "dune"})},
		{artplugin.AddShade{}, encode(t, artplugin.AddShadeArgs{
			Ramp: id(1), Index: 0, Shade: art.Shade{ID: id(4), Color: 0x8C7A3E, Opacity: 100},
		})},
		{artplugin.SetShade{}, encode(t, artplugin.SetShadeArgs{
			Ramp: id(1), Shade: art.Shade{ID: id(2), Color: 0x8AB4D4, Opacity: 40},
		})},
		{artplugin.RemoveShade{}, encode(t, artplugin.RemoveShadeArgs{Ramp: id(1), Shade: id(3)})},
		{artplugin.RemoveRamp{}, encode(t, artplugin.RemoveRampArgs{Ramp: art.StarterRampID()})},
	})
}

func TestOtherEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	verify(t, blankTile(t), []check{
		{artplugin.Paint{}, encode(t, artplugin.PaintArgs{Pixels: []artplugin.Pixel{
			{Col: 0, Row: 0, Color: grey(3)}, {Col: 1, Row: 0, Color: grey(1)}, {Col: 0, Row: 0, Color: grey(5)},
		}})},
		{artplugin.Paint{}, encode(t, artplugin.PaintArgs{Pixels: []artplugin.Pixel{{Col: 1, Row: 0, Color: art.ColorRef{}}}})},
	})
	verify(t, &artplugin.SettingsDocument{Settings: art.DefaultSettings()}, []check{
		{artplugin.SetTileSize{}, encode(t, artplugin.TileSizeArgs{Size: 24})},
	})
	verify(t, &artplugin.StandInsDocument{Sprites: art.NewStandInSprites()}, []check{
		{artplugin.SetStandIn{}, encode(t, artplugin.StandInArgs{StandIn: "pickup.small", Sprite: id(9)})},
		{artplugin.SetStandIn{}, encode(t, artplugin.StandInArgs{StandIn: "pickup.small", Sprite: ids.ULID{}})},
	})
}

func TestPrepareGivesPermanentIDs(t *testing.T) {
	t.Parallel()
	next := 100
	newID := func() (ids.ULID, error) {
		next++

		return id(next), nil
	}
	fresh := encode(t, artplugin.AddRampArgs{Index: 0, Ramp: art.Ramp{
		ID: ids.ULID{}, Name: "sand", Shades: []art.Shade{{ID: ids.ULID{}, Color: 1, Opacity: 100}},
	}})
	prepared, err := artplugin.AddRamp{}.Prepare(shared(t), fresh, newID)
	if err != nil {
		t.Fatal(err)
	}
	var add artplugin.AddRampArgs
	if err := json.Unmarshal(prepared, &add); err != nil {
		t.Fatal(err)
	}
	if add.Ramp.ID != id(101) || add.Ramp.Shades[0].ID != id(102) {
		t.Fatalf("prepared IDs: %+v", add.Ramp)
	}
	again, err := artplugin.AddRamp{}.Prepare(shared(t), prepared, newID)
	if err != nil || string(again) != string(prepared) {
		t.Fatal("preparing complete arguments must leave them unchanged")
	}
	shade := encode(t, artplugin.AddShadeArgs{Ramp: art.StarterRampID(), Index: 5, Shade: art.Shade{
		ID: ids.ULID{}, Color: 0xFFFFFF, Opacity: 100,
	}})
	prepared, err = artplugin.AddShade{}.Prepare(shared(t), shade, newID)
	if err != nil || string(prepared) == string(shade) {
		t.Fatalf("a new shade gets an ID: %v", err)
	}
}

func TestRemovalsNameWhatTilesReferTo(t *testing.T) {
	t.Parallel()
	tile := blankTile(t)
	if _, err := (artplugin.Paint{}).Apply(tile, encode(t, artplugin.PaintArgs{Pixels: []artplugin.Pixel{
		{Col: 2, Row: 2, Color: grey(3)},
	}})); err != nil {
		t.Fatal(err)
	}
	references := tile.References()
	shade := artplugin.ShadeItem(art.StarterShadeID(3))
	ramp := artplugin.RampItem(art.StarterRampID())
	for _, item := range []string{shade, ramp} {
		if !slices.ContainsFunc(references, func(reference model.Reference) bool {
			return reference.Document == art.SharedPaletteID() && reference.Item == item
		}) {
			t.Fatalf("a tile refers to %s: %+v", item, references)
		}
	}
	removes, err := artplugin.RemoveShade{}.Removes(shared(t), encode(t, artplugin.RemoveShadeArgs{
		Ramp: art.StarterRampID(), Shade: art.StarterShadeID(3),
	}))
	if err != nil || !slices.Equal(removes, []string{shade}) {
		t.Fatalf("removing a shade names it: %v %v", removes, err)
	}
	removes, err = artplugin.RemoveRamp{}.Removes(shared(t), encode(t, artplugin.RemoveRampArgs{Ramp: art.StarterRampID()}))
	if err != nil || !slices.Equal(removes, []string{ramp}) {
		t.Fatalf("removing a ramp names it: %v %v", removes, err)
	}
	sprite := &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: id(7)}}
	if !slices.Equal(sprite.Parts(), []ids.ULID{id(7)}) {
		t.Fatal("a sprite's tile is its part")
	}
	standIns := &artplugin.StandInsDocument{Sprites: art.NewStandInSprites()}
	if _, err := standIns.Sprites.Set("door", id(8)); err != nil {
		t.Fatal(err)
	}
	if got := standIns.References(); len(got) != 1 || got[0].Document != id(8) || got[0].Label != "shows Door" {
		t.Fatalf("stand-in references: %+v", got)
	}
}

func TestRefusedEditsLeaveDocumentsAlone(t *testing.T) {
	t.Parallel()
	tile := blankTile(t)
	before, err := tile.Encode()
	if err != nil {
		t.Fatal(err)
	}
	halfOutside := encode(t, artplugin.PaintArgs{Pixels: []artplugin.Pixel{
		{Col: 0, Row: 0, Color: grey(1)}, {Col: art.MinTileSize, Row: 0, Color: grey(1)},
	}})
	for name, args := range map[string]json.RawMessage{
		"a pixel outside the tile": halfOutside,
		"no pixels":                encode(t, artplugin.PaintArgs{Pixels: nil}),
		"not JSON":                 json.RawMessage(`{`),
	} {
		if _, err := (artplugin.Paint{}).Apply(tile, args); !errors.Is(err, model.ErrArgs) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	after, err := tile.Encode()
	if err != nil || string(after) != string(before) {
		t.Fatal("a refused paint changed the tile")
	}
	if _, err := (artplugin.Paint{}).Apply(shared(t), halfOutside); !errors.Is(err, model.ErrWrongDocument) {
		t.Fatalf("painting a palette: %v", err)
	}
	settings := &artplugin.SettingsDocument{Settings: art.DefaultSettings()}
	if _, err := (artplugin.SetTileSize{}).Apply(settings, encode(t, artplugin.TileSizeArgs{Size: 3})); err == nil {
		t.Fatal("a tile size of 3 was accepted")
	}
	standIns := &artplugin.StandInsDocument{Sprites: art.NewStandInSprites()}
	wall := encode(t, artplugin.StandInArgs{StandIn: "wall.a", Sprite: id(1)})
	if _, err := (artplugin.SetStandIn{}).Apply(standIns, wall); err == nil {
		t.Fatal("a wall was given a sprite")
	}
}

func TestRequiredDocuments(t *testing.T) {
	t.Parallel()
	edits := registry(t)
	required := edits.Required()
	if len(required) != 3 {
		t.Fatalf("required documents: %+v", required)
	}
	for _, document := range required {
		if !edits.IsRequired(document.ID) {
			t.Fatalf("%s is not marked required", document.Name)
		}
		documentType, ok := edits.Document(document.Format)
		if !ok {
			t.Fatalf("%s has no document type", document.Name)
		}
		data, err := document.Data()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := documentType.Decode(data); err != nil {
			t.Fatalf("%s: %v", document.Name, err)
		}
		again, err := document.Data()
		if err != nil || string(again) != string(data) {
			t.Fatalf("%s must be the same every time", document.Name)
		}
	}
	if edits.IsRequired(id(1)) {
		t.Fatal("an ordinary document is not required")
	}
}

func TestRegistryChecksRequiredDocuments(t *testing.T) {
	t.Parallel()
	data := func() (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
	unknown := plugin.Plugin{
		Name: "unknown", Documents: nil, Edits: nil,
		Required: []model.RequiredDocument{{ID: id(1), Format: "no-such-format", Name: "Nothing", Data: data}},
	}
	if _, err := plugin.NewRegistry(unknown); !errors.Is(err, plugin.ErrInvalid) {
		t.Fatalf("a required document of an unknown format: %v", err)
	}
	again := plugin.Plugin{
		Name: "again", Documents: nil, Edits: nil,
		Required: []model.RequiredDocument{{ID: art.SharedPaletteID(), Format: art.PaletteFormatName, Name: "Two", Data: data}},
	}
	if _, err := plugin.NewRegistry(artplugin.Plugin(), again); !errors.Is(err, plugin.ErrDuplicate) {
		t.Fatalf("a required document listed twice: %v", err)
	}
}
