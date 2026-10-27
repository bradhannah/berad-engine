package browser_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var (
	lanternID     = frametest.ID(60)
	lanternTileID = frametest.ID(61)
)

// withArt gives the fixture's project the documents every project holds,
// with a tile size of 24, and the sprite Lantern, tagged "light", in the
// sprite folder Lights.
func withArt(f *fixture) {
	f.t.Helper()
	palette, err := art.StarterPalette()
	if err != nil {
		f.t.Fatal(err)
	}
	tile, err := art.NewTile(24)
	if err != nil {
		f.t.Fatal(err)
	}
	docs := f.ctx.Docs.(*frametest.Docs)
	putArtIndex(docs)
	docs.Put(art.SharedPaletteID(), art.SharedPaletteName, &artplugin.PaletteDocument{Palette: palette})
	docs.Put(art.SettingsID(), art.SettingsName, &artplugin.SettingsDocument{Settings: art.Settings{TileSize: 24}})
	docs.Put(art.StandInsID(), art.StandInsName, &artplugin.StandInsDocument{Sprites: art.NewStandInSprites()})
	docs.Put(lanternTileID, "Tile 1", &artplugin.TileDocument{Tile: tile})
	docs.Put(lanternID, "Lantern", &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: lanternTileID}})
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// putArtIndex lists the fixture's documents with the art, and any more.
func putArtIndex(docs *frametest.Docs, more ...project.Entry) {
	docs.Put(project.IndexID(), "Project", project.NewIndexWithFolders(append([]project.Entry{
		{ID: textID, Name: "Notes", Format: textdoc.FormatName}, {ID: zoneID, Name: "Town", Format: "zone"},
		{ID: floorID, Name: "Ground", Format: floor.FormatName}, {ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName},
		{ID: dayID, Name: "Clerk day", Format: flow.FormatName},
		{ID: lanternID, Name: "Lantern", Format: art.SpriteFormatName, Folder: "Lights", Tags: []string{"light"}},
		{ID: lanternTileID, Name: "Tile 1", Format: art.TileFormatName},
		{ID: art.SharedPaletteID(), Name: art.SharedPaletteName, Format: art.PaletteFormatName},
		{ID: art.SettingsID(), Name: art.SettingsName, Format: art.SettingsFormatName},
		{ID: art.StandInsID(), Name: art.StandInsName, Format: art.StandInsFormatName},
	}, more...), []project.Folder{{Format: art.SpriteFormatName, Path: "Lights"}}))
}

// rowsIn lists a section's rows as "label/indent".
func (f *fixture) rowsIn(section string) []string {
	var shown []string
	inside := false
	for _, row := range f.state.Rows(f.ctx.Docs) {
		if row.Section {
			inside = row.ID == section

			continue
		}
		if inside {
			shown = append(shown, row.Label+"/"+string(rune('0'+row.Indent)))
		}
	}

	return shown
}

func (f *fixture) row(id string) (ui.Row, bool) {
	for _, row := range f.state.Rows(f.ctx.Docs) {
		if row.ID == id {
			return row, true
		}
	}

	return ui.Row{}, false
}

func (f *fixture) status() string {
	message, _ := f.ctx.Frame.Message()

	return message
}

// TestArtSections lists the Settings row first with the tile size, then,
// after Audio, Sprites with its folders and a [+], and Palettes without one.
// A sprite's tile, the settings and the stand-in sprites are not listed as
// documents.
func TestArtSections(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	rows := f.state.Rows(f.ctx.Docs)
	if first := rows[0]; first.ID != "settings" || first.Label != "Settings" || first.Detail != "Tile size 24" ||
		first.Icon != ui.IconSettings {
		t.Fatalf("the Settings row comes first: %+v", first)
	}
	if !slices.Contains(f.requests.Subscribed, art.SettingsID()) {
		t.Fatal("the browser subscribes to the art settings")
	}
	sprites, _ := f.row("section:sprites")
	palettes, _ := f.row("section:palettes")
	if sprites.Detail != "1" || !sprites.Add || palettes.Detail != "1" || palettes.Add {
		t.Fatalf("Sprites has a [+], Palettes none: %+v %+v", sprites, palettes)
	}
	if got := f.rowsIn("section:sprites"); !slices.Equal(got, []string{"Lights/0", "Lantern/1"}) {
		t.Fatalf("sprites %v", got)
	}
	if got := f.rowsIn("section:palettes"); !slices.Equal(got, []string{"Shared/0"}) {
		t.Fatalf("palettes %v", got)
	}
	lantern, _ := f.row(lanternID.String())
	shared, _ := f.row(art.SharedPaletteID().String())
	if lantern.Icon != ui.IconSprite || lantern.Aside != "#light" || shared.Icon != ui.IconPalette {
		t.Fatalf("icons and tags: %+v %+v", lantern, shared)
	}
	if ui.IconColor(ui.IconSprite) != ui.ColorArt() || ui.IconColor(ui.IconPalette) != ui.ColorArt() {
		t.Fatal("sprites and palettes share the art colour")
	}
	for _, hidden := range []string{lanternTileID.String(), art.SettingsID().String(), art.StandInsID().String()} {
		if _, listed := f.row(hidden); listed {
			t.Fatalf("%s is not listed", hidden)
		}
	}
	f.typeInto("search", "#light")
	if _, ok := f.row("settings"); ok || !slices.Equal(f.rowsIn("section:sprites"), []string{"Lights/0", "Lantern/1"}) {
		t.Fatalf("a tag search finds the sprite and not the settings: %v", f.rowsIn("section:sprites"))
	}
	f.press("Escape")
	f.typeInto("search", "tile")
	if _, ok := f.row("settings"); !ok || len(f.rowsIn("section:sprites")) != 0 {
		t.Fatalf("searching for tile finds the settings only: %v", f.rowsIn("section:sprites"))
	}
}

// TestNewSpriteMakesItsTile makes a sprite in the selected folder: first its
// tile at the project's tile size, then the sprite naming it, in one
// gesture. A refused sprite takes its tile away again.
func TestNewSpriteMakesItsTile(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.click("row:Lights", false)
	f.click("ui:new-sprite", false)
	creates := f.requests.Of(project.CreateEdit)
	var tileArgs project.DocumentArgs
	if len(creates) != 1 || creates[0].Decode(&tileArgs) != nil {
		t.Fatalf("creates %d", len(creates))
	}
	tile, err := art.DecodeTile(tileArgs.Data)
	if err != nil || tileArgs.Format != art.TileFormatName || tileArgs.Name != "Tile 1" || tile.Size() != 24 ||
		tileArgs.Folder != "" {
		t.Fatalf("the tile first, at the tile size: %+v %v", tileArgs, err)
	}
	newTile, newSprite := frametest.ID(70), frametest.ID(71)
	creates[0].Succeed(newTile)
	creates = f.requests.Of(project.CreateEdit)
	var spriteArgs project.DocumentArgs
	if len(creates) != 2 || creates[1].Decode(&spriteArgs) != nil {
		t.Fatalf("creates %d", len(creates))
	}
	sprite, err := art.DecodeSprite(spriteArgs.Data)
	if err != nil || spriteArgs.Format != art.SpriteFormatName || spriteArgs.Name != "Sprite 1" ||
		spriteArgs.Folder != "Lights" || sprite.Tile != newTile {
		t.Fatalf("then the sprite, in Lights, naming its tile: %+v %v", spriteArgs, err)
	}
	gesture := creates[0].Command.Gesture
	if gesture == "" || creates[1].Command.Gesture != gesture {
		t.Fatal("the tile and the sprite are one undo step")
	}
	putArtIndex(f.ctx.Docs.(*frametest.Docs),
		project.Entry{ID: newTile, Name: "Tile 1", Format: art.TileFormatName},
		project.Entry{ID: newSprite, Name: "Sprite 1", Format: art.SpriteFormatName, Folder: "Lights"})
	creates[1].Succeed(newSprite)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if row, ok := f.state.SelectedRow(); !ok || row.ID != newSprite.String() || row.Indent != 1 {
		t.Fatalf("the new sprite is selected, in Lights: %+v", row)
	}
	if !slices.Contains(f.requests.Subscribed, newSprite) {
		t.Fatal("the browser subscribes to the new sprite, to know its tile")
	}
	if f.status() != "Created “Sprite 1” with one tile, 24 × 24 pixels" {
		t.Fatalf("status %q", f.status())
	}

	f.click("ui:new-sprite", false)
	creates = f.requests.Of(project.CreateEdit)
	creates[2].Succeed(frametest.ID(72))
	creates = f.requests.Of(project.CreateEdit)
	creates[3].Done(protocol.Result{OK: false, Reason: "no folder"}, client.ErrRejected)
	deletes := f.requests.Of(project.DeleteEdit)
	var deletion project.DocumentArgs
	if len(deletes) != 1 || deletes[0].Decode(&deletion) != nil || deletion.ID != frametest.ID(72) ||
		deletes[0].Command.Gesture != creates[2].Command.Gesture {
		t.Fatalf("a refused sprite deletes its tile in the same step: %+v", deletes)
	}
}

// TestTileSizeSetting types a tile size into the settings' inspector and
// saves it with the save bar; a size a tile cannot be is refused there.
func TestTileSizeSetting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.click("row:Settings", false)
	for _, absent := range []string{"ui:name", "ui:delete", "ui:tags"} {
		if _, ok := f.state.Locate(absent, f.ctx.Docs); ok {
			t.Fatalf("the settings have no %s", absent)
		}
	}
	f.click("ui:tile-size", false)
	f.keys("Cmd+A")
	f.typeText("16")
	if f.state.Unsaved(f.ctx.Docs) != 1 || f.state.UnsavedName(f.ctx.Docs) != art.SettingsName {
		t.Fatalf("unsaved %d in %q", f.state.Unsaved(f.ctx.Docs), f.state.UnsavedName(f.ctx.Docs))
	}
	f.click("ui:save", false)
	sets := f.requests.Of(artplugin.TileSizeEdit)
	var args artplugin.TileSizeArgs
	if len(sets) != 1 || sets[0].Decode(&args) != nil || args.Size != 16 || sets[0].Command.Document != art.SettingsID() {
		t.Fatalf("tile size %d %+v", len(sets), args)
	}
	docs := f.ctx.Docs.(*frametest.Docs)
	docs.Put(art.SettingsID(), art.SettingsName, &artplugin.SettingsDocument{Settings: art.Settings{TileSize: 16}})
	sets[0].Succeed(art.SettingsID())
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if row, _ := f.row("settings"); row.Detail != "Tile size 16" || f.state.Unsaved(f.ctx.Docs) != 0 {
		t.Fatalf("saved: %+v, unsaved %d", row, f.state.Unsaved(f.ctx.Docs))
	}
	for _, bad := range []string{"7", "129", "big"} {
		f.click("ui:tile-size", false)
		f.keys("Cmd+A")
		f.typeText(bad)
		f.keys("Enter")
		if len(f.requests.Of(artplugin.TileSizeEdit)) != 1 || !strings.HasPrefix(f.status(), "Enter a whole number") ||
			f.state.Unsaved(f.ctx.Docs) != 1 {
			t.Fatalf("%q is refused and kept to correct: %q", bad, f.status())
		}
	}
}

// TestDeleteSpriteTakesItsTile asks where the sprite and its tile are used,
// then deletes both in one step, the sprite first.
func TestDeleteSpriteTakesItsTile(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.click("row:Lantern", false)
	f.click("ui:delete", false)
	if len(f.requests.Uses) != 2 {
		t.Fatalf("where-used questions %d", len(f.requests.Uses))
	}
	f.requests.Uses[0]([]protocol.Use{{Document: art.StandInsID(), Name: art.StandInsName, Label: "shows Small pickup"}}, nil)
	f.requests.Uses[1](nil, nil)
	lines := strings.Join(f.state.DialogLines(), "\n")
	if !strings.Contains(lines, "shows Small pickup") || strings.Contains(lines, "floor") {
		t.Fatalf("dialog %q", lines)
	}
	f.click("ui:delete", false)
	deletes := f.requests.Of(project.DeleteEdit)
	var first, second project.DocumentArgs
	if len(deletes) != 2 || deletes[0].Decode(&first) != nil || deletes[1].Decode(&second) != nil ||
		first.ID != lanternID || second.ID != lanternTileID || deletes[0].Command.Gesture != deletes[1].Command.Gesture {
		t.Fatalf("deletes %+v", deletes)
	}
}

// TestSharedPaletteKeepsItsName shows the shared palette's name without a
// field, and offers no way to rename or delete it, or to make a palette.
func TestSharedPaletteKeepsItsName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.click("row:Shared", false)
	if _, ok := f.state.Locate("ui:name", f.ctx.Docs); ok {
		t.Fatal("no name field")
	}
	f.click("ui:delete", false)
	f.keys("Cmd+Backspace")
	if len(f.requests.Uses) != 0 || !strings.HasPrefix(f.status(), "Every project holds “Shared”") {
		t.Fatalf("not deleted: %d %q", len(f.requests.Uses), f.status())
	}
	f.rightClick("row:Shared")
	f.click("menu:context-rename", false)
	if f.state.Focused() != nil {
		t.Fatal("Rename is disabled")
	}
	f.press("Escape")
	f.rightClick("section:palettes")
	if _, ok := f.state.Locate("menu:new-folder-palette", f.ctx.Docs); !ok {
		t.Fatal("the Palettes heading offers a folder")
	}
	f.click("menu:new-folder-palette", false)
	var change project.FolderArgs
	folders := f.requests.Of(project.FolderEdit)
	if len(folders) != 1 || folders[0].Decode(&change) != nil || change.Format != art.PaletteFormatName {
		t.Fatalf("a palette folder: %+v", change)
	}
}
