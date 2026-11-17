package player_test

import (
	"encoding/json"
	"errors"
	"testing"
	"testing/fstest"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/player"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

func document(t *testing.T, format string, version int, documentID ids.ULID, name string, encode func() (json.RawMessage, error)) *fstest.MapFile {
	t.Helper()
	data, err := encode()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := content.Encode(content.Envelope{
		Format: format, Version: version, ID: documentID, Name: name, Revision: 1, Tags: nil, Changed: time.Time{}, Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}

	return &fstest.MapFile{Data: raw, Mode: 0o600, ModTime: time.Time{}, Sys: nil}
}

// field is a project folder with one floor of grass, 12 by 10, where the
// player starts at 5,5, and no sounds.
func field(t *testing.T) fstest.MapFS {
	t.Helper()
	ground, err := tilefloor.New(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	zoned := zone.New()
	if err := zoned.Insert(0, id(2)); err != nil {
		t.Fatal(err)
	}
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: id(1), Floor: id(2), Col: 5, Row: 5}

	return fstest.MapFS{
		"game-settings/game-settings.json": document(t, game.SettingsFormatName, game.SettingsVersion, game.SettingsID(),
			game.SettingsName, settings.Encode),
		"zones/field--1.json":       document(t, zone.FormatName, zone.FormatVersion, id(1), "Field", zoned.Encode),
		"tilefloors/ground--2.json": document(t, tilefloor.FormatName, tilefloor.FormatVersion, id(2), "Ground", ground.Encode),
	}
}

// run plays a window by its script until the script ends it.
func run(t *testing.T, window *player.Window) {
	t.Helper()
	for range 10000 {
		if err := window.Update(); errors.Is(err, ebiten.Termination) {
			return
		} else if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("the script never ended the game")
}

// TestWindowPlaysAFolderByItsScript opens a project folder, starts its game
// at the player start, and walks the player by a script's held keys.
func TestWindowPlaysAFolderByItsScript(t *testing.T) {
	t.Parallel()
	var reported []string
	window, err := player.Open(player.Options{
		Project: field(t), Title: "Field", Seed: 1, Report: func(problem string) { reported = append(reported, problem) },
		Script: []player.ScriptStep{
			{Repeat: 10, Must: "playing"}, {Repeat: 10, Must: "player:5,5"}, {Repeat: 10, Must: "floor:1"},
			{Repeat: 10, Must: "ground:5,5:grass"},
			{Repeat: 2*play.StepTicks - play.StepTicks/2, Hold: []string{"Right"}}, {Repeat: 60, Must: "player:7,5"},
			{Repeat: 1, Quit: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if width, height := window.Size(); width != 19*24 || height != 11*24 || window.Problem() != "" {
		t.Fatalf("the game's screen is 19 by 11 cells of 24 pixels: %d by %d, %q", width, height, window.Problem())
	}
	run(t, window)
	if window.Err() != nil || len(reported) != 0 {
		t.Fatalf("the walk reaches every state, with nothing to report: %v %v", window.Err(), reported)
	}
	if err := window.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestWindowSaysWhyNoGameRuns opens a project with no player start: the
// window opens and says so; and a script's state never reached is an error.
func TestWindowSaysWhyNoGameRuns(t *testing.T) {
	t.Parallel()
	window, err := player.Open(player.Options{
		Project: fstest.MapFS{}, Title: "Empty", Seed: 1, Report: nil,
		Script: []player.ScriptStep{{Repeat: 5, Must: "problem:This project has no player start"}, {Repeat: 5, Must: "playing"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if width, height := window.Size(); width != 0 || height != 0 || window.Problem() == "" {
		t.Fatalf("no game, and why: %d by %d, %q", width, height, window.Problem())
	}
	run(t, window)
	if !errors.Is(window.Err(), player.ErrScript) {
		t.Fatalf("a state the game never reaches fails the script: %v", window.Err())
	}
}

// TestWindowReportsWhatItCannotRead opens a folder with a file that is no
// document: the window opens, and says which file and why.
func TestWindowReportsWhatItCannotRead(t *testing.T) {
	t.Parallel()
	files := field(t)
	files["zones/broken--9.json"] = &fstest.MapFile{Data: []byte("{"), Mode: 0o600, ModTime: time.Time{}, Sys: nil}
	var reported []string
	window, err := player.Open(player.Options{
		Project: files, Title: "Field", Seed: 1, Report: func(problem string) { reported = append(reported, problem) },
		Script: nil,
	})
	if err != nil || window.Problem() != "" {
		t.Fatalf("the game still starts: %v %q", err, window.Problem())
	}
	if len(reported) != 1 || reported[0][:len("zones/broken--9.json")] != "zones/broken--9.json" {
		t.Fatalf("the unread file is reported: %v", reported)
	}
}
