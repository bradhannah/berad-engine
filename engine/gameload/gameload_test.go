package gameload_test

import (
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

const (
	townID    = 1
	groundID  = 2
	upperID   = 3
	clerkID   = 4
	dayID     = 5
	chimeID   = 6
	loopID    = 7
	spriteID  = 8
	tileID    = 9
	paletteID = 10
	elsewhere = 11
	strayID   = 12
	counterID = 13
)

func id(number byte) ids.ULID {
	var made ids.ULID
	made[len(made)-1] = number

	return made
}

// held is one document of a project in memory; waiting says it has not
// arrived.
type held struct {
	name    string
	format  string
	value   any
	waiting bool
}

// memory is a project in memory.
type memory map[ids.ULID]held

func (m memory) View(documentID ids.ULID, fn func(gameload.Document)) gameload.State {
	document, ok := m[documentID]
	switch {
	case !ok:
		return gameload.Missing
	case document.waiting:
		return gameload.Waiting
	}
	fn(gameload.Document{Name: document.name, Value: document.value})

	return gameload.Held
}

func (m memory) Name(documentID ids.ULID) string { return m[documentID].name }

func (m memory) List(format string) []ids.ULID {
	var listed []ids.ULID
	for documentID, document := range m {
		if document.format == format {
			listed = append(listed, documentID)
		}
	}
	slices.SortFunc(listed, func(a, b ids.ULID) int { return a.Compare(b) })

	return listed
}

// wait marks documents as not arrived yet.
func (m memory) wait(waiting bool, documents ...byte) {
	for _, number := range documents {
		document := m[id(number)]
		document.waiting = waiting
		m[id(number)] = document
	}
}

func node(t *testing.T, kind string, number byte, params map[string]any) flow.Node {
	t.Helper()
	made, err := flow.NewNode(kind)
	if err != nil {
		t.Fatal(err)
	}
	made.ID = id(number)
	for name, value := range params {
		if made, err = made.WithParam(name, value); err != nil {
			t.Fatal(err)
		}
	}

	return made
}

// workflow is a workflow of nodes one after another from its start.
func workflow(t *testing.T, start byte, nodes ...flow.Node) *flow.Workflow {
	t.Helper()
	made, err := flow.New(id(start))
	if err != nil {
		t.Fatal(err)
	}
	links := make([]flow.Link, 0, len(nodes))
	from := id(start)
	for _, each := range nodes {
		links = append(links, flow.Link{From: from, Output: flow.OutputNext, To: each.ID})
		from = each.ID
	}
	if err := made.Insert(nodes, nil, links); err != nil {
		t.Fatal(err)
	}

	return made
}

// town is a project in memory: a zone of two floors with a loop, Ada Clerk
// placed on the ground floor, whose day rings a bell and calls a workflow
// that plays a chime, a sprite for her, and a zone elsewhere.
func town(t *testing.T) memory {
	t.Helper()
	ground, err := tilefloor.New(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	upper, err := tilefloor.New(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	if err := ground.InsertPlacement(0, tilefloor.Placement{ID: id(40), NPC: id(clerkID), Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	if err := ground.InsertPlacement(1, tilefloor.Placement{ID: id(41), NPC: id(strayID), Col: 1, Row: 1}); err != nil {
		t.Fatal(err)
	}
	if err := ground.InsertAnchor(0, tilefloor.Anchor{ID: id(counterID), Name: "Counter", Col: 3, Row: 3}); err != nil {
		t.Fatal(err)
	}
	zoned := zone.New()
	for index, floorID := range []byte{groundID, upperID} {
		if err := zoned.Insert(index, id(floorID)); err != nil {
			t.Fatal(err)
		}
	}
	zoned.Loop = id(loopID)
	clerk := npc.New()
	clerk.Behaviour, clerk.Sprite, clerk.Zone = id(dayID), id(spriteID), id(townID)
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: id(townID), Floor: id(groundID), Col: 5, Row: 5}
	tile, err := art.NewTile(art.DefaultTileSize)
	if err != nil {
		t.Fatal(err)
	}
	artSettings := art.DefaultSettings()
	artSettings.TileSize = 16
	standIns := art.NewStandInSprites()
	if _, err := standIns.Set(art.Player, id(spriteID)); err != nil {
		t.Fatal(err)
	}

	return memory{
		game.SettingsID(): {name: "Game settings", format: game.SettingsFormatName, value: settings, waiting: false},
		id(townID):        {name: "Town", format: zone.FormatName, value: zoned, waiting: false},
		id(elsewhere):     {name: "Elsewhere", format: zone.FormatName, value: zone.New(), waiting: false},
		id(groundID):      {name: "Ground floor", format: tilefloor.FormatName, value: ground, waiting: false},
		id(upperID):       {name: "Upper floor", format: tilefloor.FormatName, value: upper, waiting: false},
		id(clerkID):       {name: "Ada Clerk", format: npc.FormatName, value: clerk, waiting: false},
		id(dayID): {
			name: "Clerk's day", format: flow.FormatName, waiting: false,
			value: workflow(t, 50, node(t, flow.KindPlaySound, 51, map[string]any{"cue": "bell"}),
				node(t, flow.KindCall, 52, map[string]any{flow.ParamWorkflow: id(chimeID).String()})),
		},
		id(chimeID): {
			name: "Chime", format: flow.FormatName, waiting: false,
			value: workflow(t, 60, node(t, flow.KindPlaySound, 61, map[string]any{"cue": "chime"}),
				node(t, flow.KindPlaySound, 62, map[string]any{"cue": "bell"})),
		},
		id(loopID):             {name: "Town loop", format: flow.FormatName, value: workflow(t, 70), waiting: false},
		worldstate.ID():        {name: "World state", format: worldstate.FormatName, value: worldstate.NewDocument(), waiting: false},
		art.SettingsID():       {name: "Art settings", format: art.SettingsFormatName, value: artSettings, waiting: false},
		art.StandInsID():       {name: "Stand-in sprites", format: art.StandInsFormatName, value: standIns, waiting: false},
		id(spriteID):           {name: "Figure", format: art.SpriteFormatName, value: art.Sprite{Tile: id(tileID)}, waiting: false},
		id(tileID):             {name: "Figure", format: art.TileFormatName, value: tile, waiting: false},
		art.DefaultPaletteID(): {name: "Default", format: art.PaletteFormatName, value: art.NewPalette(), waiting: false},
	}
}

// TestLoadGathersWhatTheStartNeeds gathers the start's zone, its floors,
// the NPCs placed there, their behaviours with what those call, the zone's
// loop, the cues, the tile size and the art; an NPC the project no longer
// holds is skipped.
func TestLoadGathersWhatTheStartNeeds(t *testing.T) {
	t.Parallel()
	loaded, ready := gameload.Load(town(t), nil)
	if !ready {
		t.Fatal("everything has arrived, so the game is ready")
	}
	project := loaded.Project
	if project.Zone == nil || !slices.Equal(project.Zone.Floors(), []ids.ULID{id(groundID), id(upperID)}) {
		t.Fatalf("the start's zone: %v", project.Zone)
	}
	if len(project.Floors) != 2 || project.Floors[id(groundID)] == nil || project.Floors[id(upperID)] == nil {
		t.Fatalf("the zone's floors: %v", project.Floors)
	}
	if _, read := project.NPCs[id(clerkID)]; !read || len(project.NPCs) != 1 {
		t.Fatalf("the NPCs placed, less the one the project lacks: %v", project.NPCs)
	}
	for _, workflowID := range []byte{dayID, chimeID, loopID} {
		if project.Workflows[id(workflowID)] == nil {
			t.Fatalf("workflow %d is gathered: %v", workflowID, project.Workflows)
		}
	}
	if !slices.Equal(loaded.Cues, []string{"bell", "chime"}) {
		t.Fatalf("each cue once, in order: %v", loaded.Cues)
	}
	if loaded.TileSize != 16 {
		t.Fatalf("the project's tile size: %d", loaded.TileSize)
	}
	wantTiles := map[string]ids.ULID{art.Player: id(tileID), gameload.NPCKey(id(clerkID)): id(tileID)}
	if len(loaded.Tiles) != len(wantTiles) || loaded.Tiles[art.Player] != id(tileID) ||
		loaded.Tiles[gameload.NPCKey(id(clerkID))] != id(tileID) {
		t.Fatalf("the tiles shown: got %v, want %v", loaded.Tiles, wantTiles)
	}
	if loaded.NPCSprites[id(clerkID)] != id(spriteID) {
		t.Fatalf("the clerk's sprite: %v", loaded.NPCSprites)
	}
	if project.Values == nil {
		t.Fatal("the values the project declares are gathered")
	}
	if _, err := game.New(project); err != nil {
		t.Fatalf("the game starts from what was gathered: %v", err)
	}
}

// TestLoadCopiesWhatItGathers changes the source after loading: the project
// gathered does not change.
func TestLoadCopiesWhatItGathers(t *testing.T) {
	t.Parallel()
	source := town(t)
	loaded, _ := gameload.Load(source, nil)
	zoned, ok := source[id(townID)].value.(*zone.Zone)
	if !ok {
		t.Fatal("the town is a zone")
	}
	if _, err := zoned.Remove(id(upperID)); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Project.Zone.Floors()) != 2 {
		t.Fatal("the gathered zone keeps both floors")
	}
}

// TestLoadWaitsForWhatHasNotArrived is not ready while a document the
// project holds has not arrived, whichever it is, and is once it has.
func TestLoadWaitsForWhatHasNotArrived(t *testing.T) {
	t.Parallel()
	for _, number := range []byte{townID, upperID, clerkID, chimeID, loopID, spriteID, tileID, elsewhere} {
		source := town(t)
		source.wait(true, number)
		if _, ready := gameload.Load(source, nil); ready {
			t.Fatalf("document %d has not arrived, so the game is not ready", number)
		}
		source.wait(false, number)
		if _, ready := gameload.Load(source, nil); !ready {
			t.Fatalf("document %d arrived, so the game is ready", number)
		}
	}
	for _, documentID := range []ids.ULID{game.SettingsID(), worldstate.ID(), art.SettingsID(), art.DefaultPaletteID()} {
		source := town(t)
		document := source[documentID]
		document.waiting = true
		source[documentID] = document
		if _, ready := gameload.Load(source, nil); ready {
			t.Fatalf("%s has not arrived, so the game is not ready", document.name)
		}
	}
}

// TestLoadStartsWhereTold puts the player at a cell in place of the player
// start, for one load.
func TestLoadStartsWhereTold(t *testing.T) {
	t.Parallel()
	from := game.Start{Zone: id(townID), Floor: id(upperID), Col: 1, Row: 2}
	loaded, ready := gameload.Load(town(t), &from)
	if !ready || loaded.Project.Settings.Start != from {
		t.Fatalf("the start is where it was told: %v, ready %v", loaded.Project.Settings.Start, ready)
	}
}

// TestLoadWithNoStart gathers no zone for a project with no player start,
// and is ready, so whoever starts the game can say why it cannot start.
func TestLoadWithNoStart(t *testing.T) {
	t.Parallel()
	source := town(t)
	source[game.SettingsID()] = held{
		name: "Game settings", format: game.SettingsFormatName, value: game.DefaultSettings(), waiting: false,
	}
	loaded, ready := gameload.Load(source, nil)
	if !ready || loaded.Project.Zone != nil || !loaded.Project.Settings.Start.IsZero() {
		t.Fatalf("no start, no zone, and ready: zone %v, ready %v", loaded.Project.Zone, ready)
	}
}

// TestNamesNameDocumentsAndPlaces names a document by its name, and a place
// by its anchor, floor and zone.
func TestNamesNameDocumentsAndPlaces(t *testing.T) {
	t.Parallel()
	names := gameload.NewNames(town(t))
	if got := names.Document(id(clerkID)); got != "Ada Clerk" {
		t.Fatalf("a document's name: %q", got)
	}
	if got := names.Document(id(99)); got != "" {
		t.Fatalf("a document the project lacks has no name: %q", got)
	}
	place := worldstate.Place{Floor: id(groundID), Anchor: id(counterID)}
	if got := names.Place(place); got != "Counter, Ground floor, Town" {
		t.Fatalf("a place's name: %q", got)
	}
	if got := names.Place(worldstate.Place{Floor: id(groundID), Anchor: id(99)}); got != "" {
		t.Fatalf("a place with no anchor has no name: %q", got)
	}
}

// TestCatalogWaitsForTheWorldState is not ready until the world state has
// arrived, and then holds each NPC and zone that has.
func TestCatalogWaitsForTheWorldState(t *testing.T) {
	t.Parallel()
	source := town(t)
	document := source[worldstate.ID()]
	document.waiting = true
	source[worldstate.ID()] = document
	if _, ready := gameload.Catalog(source); ready {
		t.Fatal("no catalog before the world state arrives")
	}
	document.waiting = false
	source[worldstate.ID()] = document
	catalog, ready := gameload.Catalog(source)
	if !ready || catalog == nil {
		t.Fatal("a catalog once the world state has arrived")
	}
	if !slices.Equal(gameload.Owners(source), []ids.ULID{id(clerkID), id(townID), id(elsewhere)}) {
		t.Fatalf("NPCs, then zones: %v", gameload.Owners(source))
	}
}
