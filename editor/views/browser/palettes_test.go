package browser_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/art"
)

var (
	gloomyID = frametest.ID(70)
	sunnyID  = frametest.ID(71)
)

// withPalettes adds two palettes of the game's own, gloomy and sunny, each
// with one ramp; gloomy's first colour is the same as Default's grey 3.
func withPalettes(f *fixture) {
	f.t.Helper()
	withArt(f)
	docs := f.ctx.Docs.(*frametest.Docs)
	putArtIndex(docs,
		project.Entry{ID: gloomyID, Name: "gloomy", Format: art.PaletteFormatName},
		project.Entry{ID: sunnyID, Name: "sunny", Format: art.PaletteFormatName})
	for at, id := range []byte{70, 71} {
		palette := art.NewPalette()
		if err := palette.InsertRamp(0, art.Ramp{ID: frametest.ID(80 + byte(at)), Name: "smoke", Shades: []art.Shade{
			{ID: frametest.ID(90 + byte(at)*2), Color: 0x222222, Opacity: 100},
			{ID: frametest.ID(91 + byte(at)*2), Color: 0x5D5A63, Opacity: 100},
		}}); err != nil {
			f.t.Fatal(err)
		}
		docs.Put(frametest.ID(id), []string{"gloomy", "sunny"}[at], &artplugin.PaletteDocument{Palette: palette})
	}
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestPalettesMakesAPalette makes an empty palette from the section's [+]
// and opens it in the Palette Editor.
func TestPalettesMakesAPalette(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.click("ui:new-palette", false)
	creates := f.requests.Of(project.CreateEdit)
	var args project.DocumentArgs
	if len(creates) != 1 || creates[0].Decode(&args) != nil || args.Format != art.PaletteFormatName ||
		args.Name != "Palette 1" || string(args.Data) != `{"ramps":[]}` {
		t.Fatalf("an empty palette: %+v", args)
	}
	made := frametest.ID(72)
	creates[0].Succeed(made)
	if opened := f.requests.Opened; len(opened) != 1 || opened[0].View != "palette" || opened[0].Target != made {
		t.Fatalf("it opens in the Palette Editor: %+v", opened)
	}
}

// TestZoneListsItsPalettes lists gloomy for Town from + Palette, which
// offers neither Default nor what Town already lists, and takes it off again.
func TestZoneListsItsPalettes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withPalettes(f)
	f.click("row:Town", false)
	f.click("ui:add-zone-palette", false)
	if _, offered := f.state.Locate("menu:"+art.DefaultPaletteID().String(), f.ctx.Docs); offered {
		t.Fatal("Default is not offered: it always comes after a zone's palettes")
	}
	f.click("menu:"+gloomyID.String(), false)
	inserts := f.requests.Of(zoneplugin.InsertPaletteEdit)
	var args zoneplugin.PaletteArgs
	if len(inserts) != 1 || inserts[0].Decode(&args) != nil || args.Palette != gloomyID ||
		inserts[0].Command.Document != zoneID {
		t.Fatalf("Town lists gloomy: %+v", args)
	}
	f.ctx.Docs.View(zoneID, func(document client.Document) {
		if err := document.Model.(*zoneplugin.Document).Zone.InsertPalette(0, gloomyID); err != nil {
			t.Fatal(err)
		}
	})
	f.ctx.Docs.(*frametest.Docs).Touch(zoneID)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.click("ui:add-zone-palette", false)
	if _, offered := f.state.Locate("menu:"+gloomyID.String(), f.ctx.Docs); offered {
		t.Fatal("a palette Town lists is not offered again")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.press("Escape")
	f.click("ui:remove-zone-palette:gloomy", false)
	if removes := f.requests.Of(zoneplugin.RemovePaletteEdit); len(removes) != 1 ||
		removes[0].Decode(&args) != nil || args.Palette != gloomyID {
		t.Fatalf("× takes gloomy off: %+v", args)
	}
}

// TestSpriteRemovesAPalette removes gloomy from Lantern's tile in one step:
// the pixel matching Default's grey 3 becomes it, and the other a loose
// colour.
func TestSpriteRemovesAPalette(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withPalettes(f)
	f.ctx.Docs.View(lanternTileID, func(document client.Document) {
		tile := document.Model.(*artplugin.TileDocument).Tile
		for col, shade := range []byte{90, 91} {
			if _, err := tile.Set(col, 0, art.Named(gloomyID, frametest.ID(80), frametest.ID(shade))); err != nil {
				t.Fatal(err)
			}
		}
	})
	f.click("row:Lantern", false)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.click("ui:remove-palette:gloomy", false)
	paints := f.requests.Of(artplugin.PaintEdit)
	var args artplugin.PaintArgs
	if len(paints) != 1 || paints[0].Decode(&args) != nil || len(args.Pixels) != 2 ||
		paints[0].Command.Document != lanternTileID {
		t.Fatalf("the tile's two pixels are repainted: %+v", args)
	}
	for _, pixel := range args.Pixels {
		want := art.Named(art.DefaultPaletteID(), art.DefaultRampID(1), art.DefaultShadeID(1, 3))
		if pixel.Col == 1 {
			want = art.Loose(0x5D5A63, 100)
		}
		if pixel.Color != want {
			t.Fatalf("pixel %d keeps its colour: %+v", pixel.Col, pixel.Color)
		}
	}
}

// TestUsedPaletteDeletesInItsEditor deletes a palette a tile uses by opening
// its Palette Editor at Delete palette…, which removes each use first.
func TestUsedPaletteDeletesInItsEditor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withPalettes(f)
	f.click("row:gloomy", false)
	f.click("ui:delete", false)
	for _, answer := range f.requests.Uses {
		answer([]protocol.Use{{Document: lanternTileID, Name: "Tile 1", Format: art.TileFormatName, Label: "painted with it"}},
			nil)
	}
	f.frame(ui.NewInput(image.Pt(0, 0)))
	opened := f.requests.Opened
	if len(opened) != 1 || opened[0].View != "palette" || opened[0].Target != gloomyID || opened[0].Item != "delete" {
		t.Fatalf("the Palette Editor opens at Delete palette…: %+v", opened)
	}
	if len(f.requests.Of(project.DeleteEdit)) != 0 {
		t.Fatal("nothing is deleted while it is used")
	}
}
