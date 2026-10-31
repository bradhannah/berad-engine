package playview_test

import (
	"image"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/playview"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

var (
	zoneID    = frametest.ID(1)
	groundID  = frametest.ID(2)
	clerkID   = frametest.ID(3)
	lookID    = frametest.ID(4)
	lookTile  = frametest.ID(5)
	gloomyID  = frametest.ID(6)
	fountain  = frametest.ID(7)
	placement = frametest.ID(8)
)

const tileSize = 8

type fixture struct {
	t        *testing.T
	state    *playview.State
	docs     *frametest.Docs
	requests *frametest.Requests
	ctx      *frame.Context
}

// newFixture is a project whose Town has one 12 by 10 grass floor, with a
// wall at 6,5, the fountain at 8,5 and Ada Clerk, who shows Look, at 9,8;
// the town plays its music throughout. Look also shows the player. The
// player starts at 5,5 when start is set.
func newFixture(t *testing.T, start bool) *fixture {
	t.Helper()
	docs := frametest.NewDocs()
	ground, err := tilefloor.NewDocument(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ground.Floor.SetTop(6, 5, "wall.a"); err != nil {
		t.Fatal(err)
	}
	if err := ground.Floor.InsertSound(0, floor.Sound{ID: fountain, Cue: "town/fountain", Col: 8, Row: 5}); err != nil {
		t.Fatal(err)
	}
	if err := ground.Floor.InsertPlacement(0, floor.Placement{ID: placement, NPC: clerkID, Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	town := zone.New()
	if err := town.Insert(0, groundID); err != nil {
		t.Fatal(err)
	}
	if err := town.InsertSound(0, "music/town"); err != nil {
		t.Fatal(err)
	}
	settings := game.DefaultSettings()
	if start {
		settings.Start = game.Start{Zone: zoneID, Floor: groundID, Col: 5, Row: 5}
	}
	putArt(t, docs)
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: zone.FormatName}, {ID: groundID, Name: "Ground", Format: floor.FormatName},
		{ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName},
		{ID: lookID, Name: "Look", Format: art.SpriteFormatName}, {ID: lookTile, Name: "Tile 1", Format: art.TileFormatName},
		{ID: gloomyID, Name: "gloomy", Format: art.PaletteFormatName},
	}))
	docs.Put(game.SettingsID(), game.SettingsName, &gameplugin.SettingsDocument{Settings: settings})
	docs.Put(zoneID, "Town", &zoneplugin.Document{Zone: town})
	docs.Put(groundID, "Ground", ground)
	docs.Put(clerkID, "Ada Clerk", &npcplugin.Document{NPC: npc.NPC{Behaviour: ids.ULID{}, Sprite: lookID, Notes: ""}})
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(1280, 800))
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	state := playview.New(frametest.ID(99), theme)
	f := &fixture{t: t, state: state, docs: docs, requests: requests, ctx: ctx}
	state.Start(ctx)
	f.frame(nil)

	return f
}

// putArt gives the project 8-pixel tiles and Look, a tile of one grey pixel,
// which shows the player.
func putArt(t *testing.T, docs *frametest.Docs) {
	t.Helper()
	ramp := art.Ramp{ID: frametest.ID(40), Name: "grey", Shades: []art.Shade{
		{ID: frametest.ID(41), Color: 0x7a7a7a, Opacity: art.MaxOpacity},
	}}
	palette := art.NewPalette()
	if err := palette.InsertRamp(0, ramp); err != nil {
		t.Fatal(err)
	}
	tile, err := art.NewTile(tileSize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tile.Set(0, 0, art.Named(gloomyID, ramp.ID, frametest.ID(41))); err != nil {
		t.Fatal(err)
	}
	standIns := art.NewStandInSprites()
	if _, err := standIns.Set(art.Player, lookID); err != nil {
		t.Fatal(err)
	}
	docs.Put(art.SettingsID(), art.SettingsName, &artplugin.SettingsDocument{Settings: art.Settings{TileSize: tileSize}})
	docs.Put(art.StandInsID(), art.StandInsName, &artplugin.StandInsDocument{Sprites: standIns})
	docs.Put(gloomyID, "gloomy", &artplugin.PaletteDocument{Palette: palette})
	docs.Put(lookTile, "Tile 1", &artplugin.TileDocument{Tile: tile})
	docs.Put(lookID, "Look", &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: lookTile}})
}

// frame runs a frame with keys held, then draws it.
func (f *fixture) frame(held []ui.Key) {
	input := ui.NewInput(image.Pt(0, 0))
	input.Keys = held
	f.state.Update(&input, f.ctx)
	f.state.Draw(ebiten.NewImage(1280, 800), f.ctx.Docs)
}

func (f *fixture) has(state string) bool {
	_, ok := f.state.Locate("state:"+state, f.ctx.Docs)

	return ok
}

// answerAudio answers the audio library question with the fountain's cue,
// heard within 3 cells, and the town's music.
func (f *fixture) answerAudio() {
	library := protocol.AudioLibrary{Cues: []protocol.AudioCue{
		{ID: "town/fountain", Name: "Fountain", Category: protocol.CategoryAmbient,
			Ambient: &protocol.AmbientSettings{RadiusTiles: 3, FadeInMs: 0, FadeOutMs: 0}},
		{ID: "music/town", Name: "Town", Category: protocol.CategoryMusic},
	}}
	for _, ask := range f.requests.AudioAsks {
		ask(library, nil)
	}
	f.requests.AudioAsks = nil
}

func (f *fixture) soundscape() *frametest.Soundscape { return f.ctx.Soundscape.(*frametest.Soundscape) }

// TestNoStartSaysHow says how to place a player start, and plays nothing.
func TestNoStartSaysHow(t *testing.T) {
	t.Parallel()
	f := newFixture(t, false)
	if f.has("playing") || !f.has("problem:No player start yet") {
		t.Fatal("with no start the game says how to place one")
	}
}

// TestLaunchPlaysTheGame launches at the player start with the player's and
// Ada's sprites, filling the tab while keeping its shape, and plays the town's music;
// walking towards the fountain, 3 cells away, brings it within hearing.
func TestLaunchPlaysTheGame(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	for _, document := range []ids.ULID{game.SettingsID(), zoneID, groundID, clerkID, lookID, lookTile, gloomyID} {
		if !slices.Contains(f.requests.Subscribed, document) {
			t.Fatalf("the launch reads %s", document)
		}
	}
	if !f.has("playing") || !f.has("player:5,5") || !f.has("sprite:player") ||
		!f.has("sprite:npc:"+clerkID.String()) {
		t.Fatal("the game starts at the start, with sprites")
	}
	if !f.has("scale:8.42") {
		t.Fatal("152 by 88 pixels fill 1280 by 800 across, 8.42 times their size")
	}
	f.answerAudio()
	f.frame(nil)
	if !f.has("layer:music/town") || f.has("layer:town/fountain") {
		t.Fatalf("the music plays, the fountain is out of hearing: %v", f.soundscape().Layers)
	}
	for range 12 {
		f.frame([]ui.Key{ui.KeyUp})
	}
	for range 2 * 12 {
		f.frame([]ui.Key{ui.KeyRight})
	}
	f.frame(nil)
	if !f.has("player:7,4") || !f.has("layer:town/fountain") {
		t.Fatalf("the fountain is heard within 3 cells: %v", f.soundscape().Layers)
	}
	gain := f.soundscape().Layers["ambient:"+fountain.String()].Gain
	if gain <= 0 || gain >= 1 {
		t.Fatalf("the fountain is quieter a cell and a half away: %v", gain)
	}
	f.state.SetShown(false)
	f.frame(nil)
	if len(f.soundscape().Layers) != 0 {
		t.Fatal("a game behind another tab goes quiet")
	}
}

// TestLaunchPlaysASnapshot keeps the game's floor as it was at launch while
// the map is edited, and launching again picks the edit up.
func TestLaunchPlaysASnapshot(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	f.docs.View(groundID, func(document client.Document) {
		if _, err := document.Model.(*tilefloor.Document).Floor.SetGround(4, 5, "road"); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.Touch(groundID)
	f.frame(nil)
	if !f.has("ground:4,5:grass") {
		t.Fatal("an edit changes nothing in the running game")
	}
	f.state.Changed(client.Update{Attention: true, Item: playview.LaunchItem}, f.ctx)
	f.frame(nil)
	if !f.has("launches:2") || !f.has("ground:4,5:road") || !f.has("player:5,5") {
		t.Fatal("launching again plays the project as it is now")
	}
}
