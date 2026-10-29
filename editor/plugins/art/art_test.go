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

// paletteID is the ID the tests' own palette goes by; greyRamp is its one
// ramp, of five greys, id(21) to id(25).
func paletteID() ids.ULID { return id(50) }

func greyRamp() ids.ULID { return id(20) }

func shared(t *testing.T) *artplugin.PaletteDocument {
	t.Helper()
	ramp := art.Ramp{ID: greyRamp(), Name: "grey", Shades: nil}
	for number, value := range []art.RGB{0x202020, 0x4A4A4A, 0x7A7A7A, 0xABABAB, 0xDCDCDC} {
		ramp.Shades = append(ramp.Shades, art.Shade{ID: id(21 + number), Color: value, Opacity: art.MaxOpacity})
	}
	palette := art.NewPalette()
	if err := palette.InsertRamp(0, ramp); err != nil {
		t.Fatal(err)
	}

	return &artplugin.PaletteDocument{Palette: palette}
}

func grey(number int) art.ColorRef {
	return art.Named(paletteID(), greyRamp(), id(20+number))
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
			Ramp: id(1), Shade: art.Shade{ID: id(2), Color: 0x8AB4D4, Opacity: 40, Label: "wet sand"},
		})},
		{artplugin.MoveRamp{}, encode(t, artplugin.MoveRampArgs{Ramp: id(1), Index: 0})},
		{artplugin.MoveShade{}, encode(t, artplugin.MoveShadeArgs{Shade: id(23), Ramp: id(1), Index: 1, Moved: nil})},
		{artplugin.MoveShade{}, encode(t, artplugin.MoveShadeArgs{Shade: id(23), Ramp: greyRamp(), Index: 0, Moved: nil})},
		{artplugin.MoveShade{}, encode(t, artplugin.MoveShadeArgs{Shade: id(2), Ramp: id(1), Index: 2, Moved: nil})},
		{artplugin.RemoveShade{}, encode(t, artplugin.RemoveShadeArgs{Ramp: id(1), Shade: id(3)})},
		{artplugin.RemoveRamp{}, encode(t, artplugin.RemoveRampArgs{Ramp: greyRamp()})},
	})
}

func TestOtherEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	verify(t, blankTile(t), []check{
		{artplugin.Paint{}, encode(t, artplugin.PaintArgs{Pixels: []artplugin.Pixel{
			{Col: 0, Row: 0, Color: grey(3)}, {Col: 1, Row: 0, Color: grey(1)}, {Col: 0, Row: 0, Color: grey(5)},
		}})},
		{artplugin.Paint{}, encode(t, artplugin.PaintArgs{Pixels: []artplugin.Pixel{{Col: 1, Row: 0, Color: art.ColorRef{}}}})},
		{artplugin.Paint{}, encode(t, artplugin.PaintArgs{Pixels: []artplugin.Pixel{{Col: 2, Row: 0, Color: art.Loose(0x5D5A63, 60)}}})},
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
	shade := encode(t, artplugin.AddShadeArgs{Ramp: greyRamp(), Index: 5, Shade: art.Shade{
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
	shade := artplugin.ShadeItem(id(23))
	ramp := artplugin.RampItem(greyRamp())
	for _, item := range []string{shade, ramp} {
		if !slices.ContainsFunc(references, func(reference model.Reference) bool {
			return reference.Document == paletteID() && reference.Item == item
		}) {
			t.Fatalf("a tile refers to %s: %+v", item, references)
		}
	}
	removes, err := artplugin.RemoveShade{}.Removes(shared(t), encode(t, artplugin.RemoveShadeArgs{
		Ramp: greyRamp(), Shade: id(23),
	}))
	if err != nil || !slices.Equal(removes, []string{shade}) {
		t.Fatalf("removing a shade names it: %v %v", removes, err)
	}
	removes, err = artplugin.RemoveRamp{}.Removes(shared(t), encode(t, artplugin.RemoveRampArgs{Ramp: greyRamp()}))
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
	if !edits.IsReadOnly(art.DefaultPaletteID()) || edits.IsReadOnly(art.SettingsID()) || edits.IsReadOnly(art.StandInsID()) {
		t.Fatal("only Default is read only")
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
		Required: []model.RequiredDocument{{ID: art.DefaultPaletteID(), Format: art.PaletteFormatName, Name: "Two", Data: data}},
	}
	if _, err := plugin.NewRegistry(artplugin.Plugin(), again); !errors.Is(err, plugin.ErrDuplicate) {
		t.Fatalf("a required document listed twice: %v", err)
	}
}

func TestLooseColorsReferToNoPalette(t *testing.T) {
	t.Parallel()
	tile := blankTile(t)
	if _, err := (artplugin.Paint{}).Apply(tile, encode(t, artplugin.PaintArgs{Pixels: []artplugin.Pixel{
		{Col: 0, Row: 0, Color: art.Loose(0x101010, 100)}, {Col: 1, Row: 0, Color: grey(2)},
	}})); err != nil {
		t.Fatal(err)
	}
	for _, reference := range tile.References() {
		if reference.Document != paletteID() {
			t.Fatalf("a loose colour refers to nothing: %+v", reference)
		}
	}
}

func TestMovedShadeKeepsTilesFindingIt(t *testing.T) {
	t.Parallel()
	palette := shared(t)
	sand := art.Ramp{ID: id(1), Name: "sand", Shades: nil}
	if _, err := (artplugin.AddRamp{}).Apply(palette, encode(t, artplugin.AddRampArgs{Index: 1, Ramp: sand})); err != nil {
		t.Fatal(err)
	}
	applied, err := (artplugin.MoveShade{}).Apply(palette, encode(t, artplugin.MoveShadeArgs{Shade: id(23), Ramp: id(1), Index: 0}))
	if err != nil {
		t.Fatal(err)
	}
	if _, moved, ok := palette.Palette.Resolve(greyRamp(), id(23)); !ok || !moved {
		t.Fatal("a tile naming grey 3 finds it in sand through the shadow record")
	}
	if _, err := (artplugin.MoveShade{}).Apply(palette, applied.Inverse); err != nil {
		t.Fatal(err)
	}
	if len(palette.Palette.MovedShades()) != 0 {
		t.Fatalf("undoing the move drops its record: %+v", palette.Palette.MovedShades())
	}
}
