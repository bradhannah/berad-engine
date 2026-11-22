package player_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/player"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// twoZones is the field, with a cellar beside it as a zone of its own. In
// the field a portal at 7,5 fades to the cellar's foot of the stairs, 2,2,
// and one at 1,3 leads to an anchor that is gone. In the cellar a portal at
// 4,2 cuts back to the field's gate, 1,1, and one at 2,4 cuts to the
// cellar's own far corner, 6,6.
func twoZones(t *testing.T) fstest.MapFS {
	t.Helper()
	files := field(t)
	ground, err := tilefloor.New(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	cellar, err := tilefloor.New(8, 8, "stone")
	if err != nil {
		t.Fatal(err)
	}
	leads := func(floor, anchor byte) tilefloor.AnchorRef {
		return tilefloor.AnchorRef{Floor: id(floor), Anchor: id(anchor)}
	}
	for index, anchor := range []tilefloor.Anchor{{ID: id(10), Name: "Gate", Col: 1, Row: 1}} {
		if err := ground.InsertAnchor(index, anchor); err != nil {
			t.Fatal(err)
		}
	}
	for index, anchor := range []tilefloor.Anchor{
		{ID: id(11), Name: "Foot of the stairs", Col: 2, Row: 2}, {ID: id(12), Name: "Far corner", Col: 6, Row: 6},
	} {
		if err := cellar.InsertAnchor(index, anchor); err != nil {
			t.Fatal(err)
		}
	}
	for index, portal := range []tilefloor.Portal{
		{ID: id(20), Col: 7, Row: 5, To: leads(4, 11), Change: tilefloor.ChangeFade},
		{ID: id(21), Col: 1, Row: 3, To: leads(4, 99), Change: tilefloor.ChangeCut},
	} {
		if err := ground.InsertPortal(index, portal); err != nil {
			t.Fatal(err)
		}
	}
	for index, portal := range []tilefloor.Portal{
		{ID: id(22), Col: 4, Row: 2, To: leads(2, 10), Change: tilefloor.ChangeCut},
		{ID: id(23), Col: 2, Row: 4, To: leads(4, 12), Change: tilefloor.ChangeCut},
	} {
		if err := cellar.InsertPortal(index, portal); err != nil {
			t.Fatal(err)
		}
	}
	below := zone.New()
	if err := below.Insert(0, id(4)); err != nil {
		t.Fatal(err)
	}
	files["tilefloors/ground--2.json"] = document(t, tilefloor.FormatName, tilefloor.FormatVersion, id(2), "Ground",
		ground.Encode)
	files["zones/cellar--3.json"] = document(t, zone.FormatName, zone.FormatVersion, id(3), "Cellar", below.Encode)
	files["tilefloors/cellar--4.json"] = document(t, tilefloor.FormatName, tilefloor.FormatVersion, id(4), "Cellar floor",
		cellar.Encode)

	return files
}

// TestWindowTravelsByPortals walks a game through its portals by a script:
// a fade into another zone, a cut to a cell of the zone it is in, a cut
// back to the first zone, and a portal that leads nowhere, which leaves the
// player on it and is reported once however often it is stepped on.
func TestWindowTravelsByPortals(t *testing.T) {
	t.Parallel()
	var reported []string
	hold := func(key, must string) player.ScriptStep {
		return player.ScriptStep{Repeat: 300, Hold: []string{key}, Must: must}
	}
	wait := func(must string) player.ScriptStep { return player.ScriptStep{Repeat: 300, Must: must} }
	cells := func(key string, count int) player.ScriptStep {
		return player.ScriptStep{Repeat: count*play.StepTicks - play.StepTicks/2, Hold: []string{key}}
	}
	window, err := player.Open(player.Options{
		Project: twoZones(t), Title: "Field", Seed: 1, Report: func(problem string) { reported = append(reported, problem) },
		Script: []player.ScriptStep{
			wait("zone:Field"), wait("light"),
			hold("Right", "leaving"), wait("player:7,5"), hold("Right", "dark"),
			hold("Right", "zone:Cellar"), wait("player:2,2"), wait("floor:1"), wait("ground:2,2:stone"),
			hold("Right", "light"), wait("player:2,2"),
			hold("Down", "player:6,6"), wait("zone:Cellar"),
			cells("Up", 4), wait("player:6,2"), hold("Left", "zone:Field"), wait("player:1,1"), wait("light"),
			cells("Down", 2), wait("player:1,3"), wait("zone:Field"),
			cells("Up", 1), wait("player:1,2"), cells("Down", 1), wait("player:1,3"), cells("Up", 1), wait("player:1,2"),
			{Repeat: 1, Quit: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run(t, window)
	if window.Err() != nil {
		t.Fatalf("the walk reaches every state: %v", window.Err())
	}
	if len(reported) != 1 || !strings.Contains(reported[0], "The portal at cell 1,3 cannot take the player") ||
		!strings.Contains(reported[0], "its anchor is gone") {
		t.Fatalf("the portal that leads nowhere is reported, once: %v", reported)
	}
	if err := window.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestWindowStartsOnAPortalWithoutLeaving starts the player on a portal's
// cell: nothing happens until they step onto it.
func TestWindowStartsOnAPortalWithoutLeaving(t *testing.T) {
	t.Parallel()
	files := twoZones(t)
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: id(1), Floor: id(2), Col: 7, Row: 5}
	files["game-settings/game-settings.json"] = document(t, game.SettingsFormatName, game.SettingsVersion,
		game.SettingsID(), game.SettingsName, settings.Encode)
	window, err := player.Open(player.Options{
		Project: files, Title: "Field", Seed: 1, Report: nil,
		Script: []player.ScriptStep{
			{Repeat: 60}, {Repeat: 1, Must: "zone:Field"}, {Repeat: 1, Must: "player:7,5"}, {Repeat: 1, Quit: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	run(t, window)
	if window.Err() != nil {
		t.Fatalf("the player stands on the portal, in the field: %v", window.Err())
	}
}
