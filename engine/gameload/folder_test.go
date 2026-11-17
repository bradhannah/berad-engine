package gameload_test

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// file is a document as a project folder keeps it.
func file(t *testing.T, format string, version int, number byte, name string, revision uint64, data json.RawMessage) *fstest.MapFile {
	t.Helper()
	raw, err := content.Encode(content.Envelope{
		Format: format, Version: version, ID: id(number), Name: name, Revision: revision, Tags: nil,
		Changed: time.Time{}, Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}

	return &fstest.MapFile{Data: raw, Mode: 0o600, ModTime: time.Time{}, Sys: nil}
}

func encoded(t *testing.T, encode func() (json.RawMessage, error)) json.RawMessage {
	t.Helper()
	data, err := encode()
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// townFiles is a small project folder: a zone of one floor, where the
// player starts, with an NPC, beside the editor's own folder, the audio
// folder, a layout and a file that is no document.
func townFiles(t *testing.T) fstest.MapFS {
	t.Helper()
	ground, err := tilefloor.New(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	if err := ground.InsertPlacement(0, tilefloor.Placement{ID: id(40), NPC: id(clerkID), Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	zoned := zone.New()
	if err := zoned.Insert(0, id(groundID)); err != nil {
		t.Fatal(err)
	}
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: id(townID), Floor: id(groundID), Col: 5, Row: 5}
	clerk := npc.New()

	return fstest.MapFS{
		"game-settings/game-settings--X.json": {
			Data: envelopeOf(t, game.SettingsFormatName, game.SettingsVersion, game.SettingsID(), "Game settings",
				encoded(t, settings.Encode)),
			Mode: 0o600, ModTime: time.Time{}, Sys: nil,
		},
		"zones/town--1.json":            file(t, zone.FormatName, zone.FormatVersion, townID, "Town", 3, encoded(t, zoned.Encode)),
		"tilefloors/ground--2.json":     file(t, tilefloor.FormatName, tilefloor.FormatVersion, groundID, "Ground", 1, encoded(t, ground.Encode)),
		"npcs/People/ada-clerk--4.json": file(t, npc.FormatName, npc.FormatVersion, clerkID, "Ada Clerk", 1, encoded(t, clerk.Encode)),
		".berad/backups/town--1.json":   file(t, zone.FormatName, zone.FormatVersion, townID, "Old town", 9, encoded(t, zone.New().Encode)),
		"audio/library.json":            {Data: []byte(`{"schema":4}`), Mode: 0o600, ModTime: time.Time{}, Sys: nil},
		"workflows/day--5.layout.json":  {Data: []byte(`{"format":"workflow-layout"}`), Mode: 0o600, ModTime: time.Time{}, Sys: nil},
		"sprites/figure.png":            {Data: []byte("not json"), Mode: 0o600, ModTime: time.Time{}, Sys: nil},
		"views/views--9.json":           file(t, "view", 1, 90, "Project views", 1, json.RawMessage(`{"views":[]}`)),
	}
}

func envelopeOf(t *testing.T, format string, version int, documentID ids.ULID, name string, data json.RawMessage) []byte {
	t.Helper()
	raw, err := content.Encode(content.Envelope{
		Format: format, Version: version, ID: documentID, Name: name, Revision: 1, Tags: nil, Changed: time.Time{}, Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

func readFolder(t *testing.T, files fstest.MapFS) *gameload.Folder {
	t.Helper()
	folder, err := gameload.ReadFolder(files)
	if err != nil {
		t.Fatal(err)
	}

	return folder
}

// TestFolderReadsAProject reads a project's documents and nothing else of
// the folder: not the editor's own folder, the audio folder, layouts, files
// that are not JSON, or documents of the editor's own formats. The game
// starts from it.
func TestFolderReadsAProject(t *testing.T) {
	t.Parallel()
	folder := readFolder(t, townFiles(t))
	if unread := folder.Unread(); len(unread) != 0 {
		t.Fatalf("every document reads: %v", unread)
	}
	if got := folder.Name(id(townID)); got != "Town" {
		t.Fatalf("the zone is the project's, not the backup's: %q", got)
	}
	if folder.View(id(90), func(gameload.Document) {}) != gameload.Missing {
		t.Fatal("a document of the editor's own format is passed over")
	}
	loaded, ready := gameload.Load(folder, nil)
	if !ready {
		t.Fatal("a folder never waits")
	}
	if _, err := game.New(loaded.Project); err != nil {
		t.Fatalf("the game starts from the folder: %v", err)
	}
	if _, read := loaded.Project.NPCs[id(clerkID)]; !read {
		t.Fatal("an NPC in a folder within its kind's folder is read")
	}
}

// TestFolderHoldsWhatEveryProjectHolds gives a folder with no documents the
// ones every project holds, and reads Default as the engine's whatever a
// file says.
func TestFolderHoldsWhatEveryProjectHolds(t *testing.T) {
	t.Parallel()
	folder := readFolder(t, fstest.MapFS{
		"palettes/default--X.json": {
			Data: envelopeOf(t, art.PaletteFormatName, art.PaletteVersion, art.DefaultPaletteID(), "Mine",
				encoded(t, art.NewPalette().Encode)),
			Mode: 0o600, ModTime: time.Time{}, Sys: nil,
		},
	})
	for _, documentID := range []ids.ULID{game.SettingsID(), art.SettingsID(), art.StandInsID(), worldstate.ID()} {
		if folder.View(documentID, func(gameload.Document) {}) != gameload.Held {
			t.Fatalf("every project holds %s", documentID)
		}
	}
	want, err := art.DefaultPalette()
	if err != nil {
		t.Fatal(err)
	}
	folder.View(art.DefaultPaletteID(), func(document gameload.Document) {
		palette, ok := document.Value.(*art.Palette)
		if !ok || !palette.Equal(want) || document.Name != art.DefaultPaletteName {
			t.Fatalf("Default is the engine's: %q", document.Name)
		}
	})
	loaded, ready := gameload.Load(folder, nil)
	if !ready || !loaded.Project.Settings.Start.IsZero() {
		t.Fatal("an empty folder is a project with no player start")
	}
}

// TestFolderKeepsTheNewerOfTwoFiles reads the higher revision of a document
// two files hold, and at the same revision the file written later.
func TestFolderKeepsTheNewerOfTwoFiles(t *testing.T) {
	t.Parallel()
	files := townFiles(t)
	files["zones/town-renamed--1.json"] = file(t, zone.FormatName, zone.FormatVersion, townID, "Market town", 4,
		encoded(t, zone.New().Encode))
	if got := readFolder(t, files).Name(id(townID)); got != "Market town" {
		t.Fatalf("the higher revision is kept: %q", got)
	}
	later := file(t, zone.FormatName, zone.FormatVersion, townID, "Later town", 3, encoded(t, zone.New().Encode))
	later.ModTime = time.Unix(1000, 0)
	files["zones/town-renamed--1.json"] = later
	if got := readFolder(t, files).Name(id(townID)); got != "Later town" {
		t.Fatalf("at one revision, the file written later is kept: %q", got)
	}
}

// TestFolderUpgradesOldFilesInMemory reads a document saved in an older
// version of its format, and leaves out, saying why, one that is newer than
// the engine reads, one that is not a document, and one whose data is wrong.
func TestFolderUpgradesOldFilesInMemory(t *testing.T) {
	t.Parallel()
	files := townFiles(t)
	files["zones/old--20.json"] = file(t, zone.FormatName, 1, 20, "Old zone", 1, encoded(t, zone.New().Encode))
	files["zones/new--21.json"] = file(t, zone.FormatName, zone.FormatVersion+1, 21, "New zone", 1, encoded(t, zone.New().Encode))
	files["zones/broken--22.json"] = &fstest.MapFile{Data: []byte("{"), Mode: 0o600, ModTime: time.Time{}, Sys: nil}
	files["zones/wrong--23.json"] = file(t, zone.FormatName, zone.FormatVersion, 23, "Wrong zone", 1, json.RawMessage(`{"floors":7}`))
	before := len(files)
	folder := readFolder(t, files)
	if folder.View(id(20), func(gameload.Document) {}) != gameload.Held {
		t.Fatal("a document in an older version is read")
	}
	unread := folder.Unread()
	if len(unread) != 3 {
		t.Fatalf("three files cannot be read: %v", unread)
	}
	for index, part := range []string{"zones/broken--22.json", "zones/new--21.json", "zones/wrong--23.json"} {
		if unread[index].Path != part || unread[index].Reason == "" {
			t.Fatalf("unread file %d is %s, with a reason: %v", index, part, unread[index])
		}
	}
	if !strings.Contains(unread[1].Reason, "newer") {
		t.Fatalf("a newer document says so: %q", unread[1].Reason)
	}
	if len(files) != before {
		t.Fatal("reading writes nothing")
	}
}
