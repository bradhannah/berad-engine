package mapview_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

var (
	lanternID     = frametest.ID(20)
	lanternTileID = frametest.ID(21)
	lawnID        = frametest.ID(22)
	lawnTileID    = frametest.ID(23)
	gloomyID      = frametest.ID(24)
	greyRampID    = frametest.ID(25)
)

const artTileSize = 8

// grey is a shade of the palette gloomy's ramp of five greys.
func grey(number int) art.ColorRef {
	return art.Named(gloomyID, greyRampID, frametest.ID(byte(30+number)))
}

func greyValue(number int) art.RGB {
	return []art.RGB{0x202020, 0x4A4A4A, 0x7A7A7A, 0xABABAB, 0xDCDCDC}[number-1]
}

func gloomyPalette(t *testing.T) *art.Palette {
	t.Helper()
	ramp := art.Ramp{ID: greyRampID, Name: "grey", Shades: nil}
	for number := 1; number <= 5; number++ {
		ramp.Shades = append(ramp.Shades, art.Shade{ID: grey(number).Shade, Color: greyValue(number), Opacity: 100})
	}
	palette := art.NewPalette()
	if err := palette.InsertRamp(0, ramp); err != nil {
		t.Fatal(err)
	}

	return palette
}

// greyHex is a grey's colour as "art:" states name it.
func greyHex(t *testing.T, number int) string {
	t.Helper()

	return fmt.Sprintf("#%06X", uint32(greyValue(number)))
}

// withArt gives the project 8-pixel tiles and the palette gloomy. The sprite
// Lantern shows the door, with grey 3 at its tile's top-left pixel; the
// sprite Lawn shows grass, but its tile is 16 pixels, another size.
func withArt(t *testing.T, f *fixture) {
	t.Helper()
	palette := gloomyPalette(t)
	lantern, err := art.NewTile(artTileSize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lantern.Set(0, 0, grey(3)); err != nil {
		t.Fatal(err)
	}
	lawn, err := art.NewTile(2 * artTileSize)
	if err != nil {
		t.Fatal(err)
	}
	standIns := art.NewStandInSprites()
	for standIn, sprite := range map[string]ids.ULID{"door": lanternID, "grass": lawnID} {
		if _, err := standIns.Set(standIn, sprite); err != nil {
			t.Fatal(err)
		}
	}
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"}, {ID: groundID, Name: "Ground", Format: floor.FormatName},
		{ID: upperID, Name: "Upper", Format: floor.FormatName},
		{ID: lanternID, Name: "Lantern", Format: art.SpriteFormatName},
		{ID: lanternTileID, Name: "Tile 1", Format: art.TileFormatName},
		{ID: lawnID, Name: "Lawn", Format: art.SpriteFormatName},
		{ID: lawnTileID, Name: "Tile 1", Format: art.TileFormatName},
		{ID: gloomyID, Name: "gloomy", Format: art.PaletteFormatName},
	}))
	f.docs.Put(art.SettingsID(), art.SettingsName, &artplugin.SettingsDocument{
		Settings: art.Settings{TileSize: artTileSize},
	})
	f.docs.Put(gloomyID, "gloomy", &artplugin.PaletteDocument{Palette: palette})
	f.docs.Put(lanternTileID, "Tile 1", &artplugin.TileDocument{Tile: lantern})
	f.docs.Put(lanternID, "Lantern", &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: lanternTileID}})
	f.docs.Put(lawnTileID, "Tile 1", &artplugin.TileDocument{Tile: lawn})
	f.docs.Put(lawnID, "Lawn", &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: lawnTileID}})
	f.docs.Put(art.StandInsID(), art.StandInsName, &artplugin.StandInsDocument{Sprites: standIns})
	snapshot := art.StandInsID()
	f.state.Changed(client.Update{Snapshot: &snapshot}, f.ctx)
	f.draw()
}

func (f *fixture) draw() { f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs) }

// apply applies an art edit to a document, as an event from another window
// would.
func (f *fixture) apply(document ids.ULID, kind model.EditKind, args any) {
	f.t.Helper()
	data, err := json.Marshal(args)
	if err != nil {
		f.t.Fatal(err)
	}
	f.docs.View(document, func(found client.Document) { _, err = kind.Apply(found.Model, data) })
	if err != nil {
		f.t.Fatal(err)
	}
	f.docs.Touch(document)
	f.draw()
}

// TestStandInsShowTheirSprites draws the door with Lantern's tile, at the
// project's tile size, and leaves grass its flat colour, since Lawn's tile is
// another size. A stroke and a shade's new colour show at once, and taking
// the sprite away brings the door's flat colour back.
func TestStandInsShowTheirSprites(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(t, f)
	for _, document := range []ids.ULID{
		art.SettingsID(), art.StandInsID(), lanternID, lanternTileID, gloomyID, lawnID, lawnTileID,
	} {
		if !slices.Contains(f.requests.Subscribed, document) {
			t.Fatalf("not subscribed to %s", document)
		}
	}
	if f.cellSize() != artTileSize || !f.has("zoom:100") {
		t.Fatalf("100%% is the tile size: %d", f.cellSize())
	}
	if !f.has("sprite:door") || f.has("sprite:grass") {
		t.Fatal("the door shows Lantern, and grass, whose tile is another size, its flat colour")
	}
	if !f.has("art:door:0,0:"+greyHex(t, 3)) || !f.has("art:door:1,0:clear") {
		t.Fatal("Lantern's tile is drawn from its palette")
	}
	f.apply(lanternTileID, artplugin.Paint{}, artplugin.PaintArgs{Pixels: []artplugin.Pixel{
		{Col: 1, Row: 0, Color: grey(5)},
	}})
	if !f.has("art:door:1,0:" + greyHex(t, 5)) {
		t.Fatal("a stroke shows on the map as it is drawn")
	}
	f.apply(gloomyID, artplugin.SetShade{}, artplugin.SetShadeArgs{
		Ramp: greyRampID, Shade: art.Shade{ID: grey(3).Shade, Color: 0xff00ff, Opacity: art.MaxOpacity},
	})
	if !f.has("art:door:0,0:#FF00FF") {
		t.Fatal("a shade's new colour recolours the tile on the map")
	}
	f.apply(art.StandInsID(), artplugin.SetStandIn{}, artplugin.StandInArgs{StandIn: "door", Sprite: ids.ULID{}})
	if f.has("sprite:door") {
		t.Fatal("without its sprite the door is its flat colour again")
	}
}

// TestZoomFollowsTheTileSize steps through a quarter, a half, and one to
// four times a 24-pixel tile.
func TestZoomFollowsTheTileSize(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.docs.Put(art.SettingsID(), art.SettingsName, &artplugin.SettingsDocument{Settings: art.Settings{TileSize: 24}})
	f.click("ui:zoom-out")
	f.click("ui:zoom-out")
	sizes := []int{f.cellSize()}
	for range 5 {
		f.click("ui:zoom-in")
		sizes = append(sizes, f.cellSize())
	}
	if want := []int{6, 12, 24, 48, 72, 96}; !slices.Equal(sizes, want) {
		t.Fatalf("zoom steps %v, want %v", sizes, want)
	}
}
