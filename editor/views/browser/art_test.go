package browser_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
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
	palette, err := art.DefaultPalette()
	if err != nil {
		f.t.Fatal(err)
	}
	tile, err := art.NewTile(24)
	if err != nil {
		f.t.Fatal(err)
	}
	docs := f.ctx.Docs.(*frametest.Docs)
	putArtIndex(docs)
	docs.Put(art.DefaultPaletteID(), art.DefaultPaletteName, &artplugin.PaletteDocument{Palette: palette})
	docs.Put(art.SettingsID(), art.SettingsName, &artplugin.SettingsDocument{Settings: art.Settings{TileSize: 24}})
	docs.Put(art.StandInsID(), art.StandInsName, &artplugin.StandInsDocument{Sprites: art.NewStandInSprites()})
	docs.Put(lanternTileID, "Tile 1", &artplugin.TileDocument{Tile: tile})
	docs.Put(lanternID, "Lantern", &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: lanternTileID}})
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// putArtIndex lists the fixture's documents with the art, and any more.
func putArtIndex(docs *frametest.Docs, more ...project.Entry) {
	docs.Put(project.IndexID(), "Project", project.NewIndexWithFolders(append([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"},
		{ID: floorID, Name: "Ground", Format: floor.FormatName}, {ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName},
		{ID: dayID, Name: "Clerk day", Format: flow.FormatName},
		{ID: lanternID, Name: "Lantern", Format: art.SpriteFormatName, Folder: "Lights", Tags: []string{"light"}},
		{ID: lanternTileID, Name: "Tile 1", Format: art.TileFormatName},
		{ID: art.DefaultPaletteID(), Name: art.DefaultPaletteName, Format: art.PaletteFormatName},
		{ID: art.SettingsID(), Name: art.SettingsName, Format: art.SettingsFormatName},
		{ID: art.StandInsID(), Name: art.StandInsName, Format: art.StandInsFormatName},
	}, more...), []project.Folder{{Format: art.SpriteFormatName, Path: "Lights"}}))
}

// rowsIn shows a place and lists its rows as "label/indent".
func (f *fixture) rowsIn(place string) []string {
	f.state.Show(place)
	var shown []string
	for _, row := range f.state.Rows(f.ctx.Docs) {
		if !row.Heading {
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

// TestArtPlaces lists Sprites and Palettes among the places, each with how
// many it holds and a [+]. Sprites shows its folder, and the folder its
// sprite; Palettes shows Default, marked; Settings shows the tile size. A
// sprite's tile, the settings and the stand-in sprites are not listed as
// things. A search finds the sprite by its tag, and Settings by its words.
func TestArtPlaces(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	if !slices.Contains(f.requests.Subscribed, art.SettingsID()) {
		t.Fatal("the browser subscribes to the art settings")
	}
	places := map[string]ui.Row{}
	for _, row := range f.state.Places(f.ctx.Docs) {
		places[row.ID] = row
	}
	sprites, palettes := places["section:sprites"], places["section:palettes"]
	if sprites.Detail != "1" || !sprites.Add || palettes.Detail != "1" || !palettes.Add {
		t.Fatalf("Sprites and Palettes say how many and have a [+]: %+v %+v", sprites, palettes)
	}
	if got := f.rowsIn("section:sprites"); !slices.Equal(got, []string{"Lights/0"}) {
		t.Fatalf("Sprites holds its folder: %v", got)
	}
	if got := f.rowsIn("folder:sprite:Lights"); !slices.Equal(got, []string{"Lantern/0"}) {
		t.Fatalf("Lights holds Lantern: %v", got)
	}
	lantern, _ := f.row(lanternID.String())
	if lantern.Icon != ui.IconSprite || lantern.Aside != "#light" {
		t.Fatalf("the sprite's icon and tag: %+v", lantern)
	}
	if got := f.rowsIn("section:palettes"); !slices.Equal(got, []string{"Default/0"}) {
		t.Fatalf("palettes %v", got)
	}
	if defaults, _ := f.row(art.DefaultPaletteID().String()); defaults.Detail != "default · read only" ||
		defaults.Icon != ui.IconPalette {
		t.Fatalf("Default is marked: %+v", defaults)
	}
	f.state.Show("settings")
	if settings, _ := f.row("settings"); settings.Detail != "Tile size 24" || settings.Icon != ui.IconSettings {
		t.Fatalf("Settings says the tile size: %+v", settings)
	}
	if ui.IconColor(ui.IconSprite) != ui.ColorArt() || ui.IconColor(ui.IconPalette) != ui.ColorArt() {
		t.Fatal("sprites and palettes share the art colour")
	}
	for _, place := range []string{"section:sprites", "folder:sprite:Lights", "section:palettes", "section:templates"} {
		f.state.Show(place)
		for _, hidden := range []string{lanternTileID.String(), art.SettingsID().String(), art.StandInsID().String()} {
			if _, listed := f.row(hidden); listed {
				t.Fatalf("%s is not listed in %s", hidden, place)
			}
		}
	}
	f.typeInto("search", "#light")
	if !f.has("place=Matches") || !slices.Contains(f.matchLabels(), "Lantern") || slices.Contains(f.matchLabels(), "Settings") {
		t.Fatalf("a tag search finds the sprite and not the settings: %v", f.matchLabels())
	}
	f.press("Escape")
	f.typeInto("search", "tile")
	if got := f.matchLabels(); !slices.Equal(got, []string{"Settings"}) {
		t.Fatalf("searching for tile finds the settings only: %v", got)
	}
}

// matchLabels lists the things Matches shows, by name.
func (f *fixture) matchLabels() []string {
	var labels []string
	for _, row := range f.state.Rows(f.ctx.Docs) {
		if !row.Heading {
			labels = append(labels, row.Label)
		}
	}

	return labels
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
	if row, ok := f.state.SelectedRow(f.ctx.Docs); !ok || row.ID != newSprite.String() || !f.has("place=Lights") {
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
	if f.state.Unsaved(f.ctx.Docs) != 1 || f.state.UnsavedName(f.ctx.Docs) != "Settings" {
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
	if len(f.whereUsed()) != 2 {
		t.Fatalf("where-used questions %d", len(f.whereUsed()))
	}
	f.whereUsed()[0]([]protocol.Use{{Document: art.StandInsID(), Name: art.StandInsName, Label: "shows Small pickup"}}, nil)
	f.whereUsed()[1](nil, nil)
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

// TestDefaultPaletteKeepsItsName shows Default's name without a field, and
// offers no way to rename or delete it.
func TestDefaultPaletteKeepsItsName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.click("row:Default", false)
	if _, ok := f.state.Locate("ui:name", f.ctx.Docs); ok {
		t.Fatal("no name field")
	}
	f.click("ui:delete", false)
	f.keys("Cmd+Backspace")
	if len(f.whereUsed()) != 0 || !strings.HasPrefix(f.status(), "Every project holds “Default”") {
		t.Fatalf("not deleted: %d %q", len(f.whereUsed()), f.status())
	}
	f.rightClick("row:Default")
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

// TestSpriteOpensInThePixelEditor opens a sprite in a Pixel editor, from the
// inspector's button and from a double-click, and a palette in the Palette
// Editor.
func TestSpriteOpensInThePixelEditor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.click("row:Lantern", false)
	f.click("ui:open", false)
	f.click("row:Lantern", true)
	opened := f.requests.Opened
	if len(opened) != 2 || opened[0].View != "sprite" || opened[0].Target != lanternID || opened[1] != opened[0] {
		t.Fatalf("opened %+v", opened)
	}
	f.rightClick("row:Default")
	f.click("menu:context-open", false)
	if len(f.requests.Opened) != 3 || f.requests.Opened[2].View != "palette" ||
		f.requests.Opened[2].Target != art.DefaultPaletteID() {
		t.Fatalf("a palette opens in the Palette Editor: %+v", f.requests.Opened)
	}
}

// TestNPCShowsASprite gives the selected NPC a sprite from the project's
// sprites in a dialog, and Open opens it in the Pixel editor.
func TestNPCShowsASprite(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.click("row:Ada Clerk", false)
	f.click("ui:choose-sprite", false)
	f.click("row:Lantern", false)
	f.click("ui:use", false)
	set := f.requests.Of(npcplugin.SetSpriteEdit)
	var args npcplugin.SetSpriteArgs
	if len(set) != 1 || set[0].Command.Document != clerkID || set[0].Decode(&args) != nil || args.Sprite != lanternID {
		t.Fatalf("set sprite %d %+v", len(set), args)
	}
	f.ctx.Docs.(*frametest.Docs).Put(clerkID, "Ada Clerk", &npcplugin.Document{
		NPC: npc.NPC{Behaviour: ids.ULID{}, Sprite: lanternID, Notes: ""},
	})
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.click("ui:open-sprite", false)
	if opened := f.requests.Opened; len(opened) == 0 || opened[len(opened)-1].Target != lanternID {
		t.Fatalf("Open opens the NPC's sprite: %+v", opened)
	}
}

// TestScreenSetting types a screen size into the settings' inspector and
// saves it with Enter; an even number of cells is refused there.
func TestScreenSetting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withArt(f)
	f.ctx.Docs.(*frametest.Docs).Put(game.SettingsID(), game.SettingsName,
		&gameplugin.SettingsDocument{Settings: game.DefaultSettings()})
	f.click("row:Settings", false)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.click("ui:screen-columns", false)
	f.keys("Cmd+A")
	f.typeText("16")
	f.keys("Enter")
	if len(f.requests.Of(gameplugin.ScreenEdit)) != 0 || !strings.HasPrefix(f.status(), "Enter odd numbers") {
		t.Fatalf("an even screen is refused: %q", f.status())
	}
	f.click("ui:screen-columns", false)
	f.keys("Cmd+A")
	f.typeText("15")
	f.keys("Enter")
	sent := f.requests.Of(gameplugin.ScreenEdit)
	var args gameplugin.ScreenArgs
	if len(sent) != 1 || sent[0].Command.Document != game.SettingsID() || sent[0].Decode(&args) != nil ||
		args != (gameplugin.ScreenArgs{Columns: 15, Rows: 11}) {
		t.Fatalf("screen %d %+v %q", len(sent), args, f.status())
	}
}
