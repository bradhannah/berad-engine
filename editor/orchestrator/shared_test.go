package orchestrator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugin"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	"github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/game"
)

// withGame uses a registry that holds the game's settings, a document every
// project holds, so a project never has no documents.
func withGame(t *testing.T) func(*orchestrator.Config) {
	t.Helper()
	registry, err := plugin.NewRegistry(
		project.Plugin(), tilefloor.Plugin(), zone.Plugin(), notePlugin(), workflow.Plugin(), npc.Plugin(),
		gameplugin.Plugin(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return func(cfg *orchestrator.Config) {
		cfg.Registry = registry
		cfg.FlushInterval = 20 * time.Millisecond
	}
}

// createWithGame makes a project with a floor and the game's settings.
func createWithGame(t *testing.T, dir string, configure func(*orchestrator.Config)) {
	t.Helper()
	createProject(t, dir)
	cfg := baseConfig(t, dir)
	configure(&cfg)
	orch, err := orchestrator.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := orch.Close(); err != nil {
		t.Fatal(err)
	}
}

// nextOpening waits for the question a project that cannot open as it is
// asks.
func nextOpening(t *testing.T, c *client.Client) protocol.Opening {
	t.Helper()
	var asked protocol.Opening
	nextUpdate(t, c, "the opening question", func(update client.Update) bool {
		if update.Session == nil || update.Session.Topic != protocol.TopicOpening {
			return false
		}
		if err := json.Unmarshal(update.Session.Value, &asked); err != nil {
			t.Fatal(err)
		}
		return true
	})
	return asked
}

// floorPath is the floor's file in a project made with createProject.
func floorPath(t *testing.T, dir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "tilefloors", "*.json"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("floor file: %v %v", matches, err)
	}
	for _, match := range matches {
		if !strings.HasSuffix(match, ".layout.json") {
			return match
		}
	}
	t.Fatal("no floor file")
	return ""
}

// renameOnDisk changes the floor's name in its file, as another program
// would, and returns the bytes written.
func renameOnDisk(t *testing.T, dir, name string) []byte {
	t.Helper()
	path := floorPath(t, dir)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := content.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	envelope.Name = name
	encoded, err := content.Encode(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return encoded
}

// fast flushes every few milliseconds.
func fast(cfg *orchestrator.Config) { cfg.FlushInterval = 20 * time.Millisecond }

// A file changed outside the editor, while nothing for it waits in the
// editor, is taken in: windows showing it are sent it again and say so,
// and its undo steps go, since they were for the version it replaces.
func TestAFileChangedOutsideIsTakenIn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floor := createProject(t, dir)
	h := start(t, dir, fast)
	view := h.dial(t, "")
	mustPaint(t, view, floor, 0, 0, "wall", "")
	waitFor(t, "the paint to be saved", func() bool {
		raw, err := os.ReadFile(floorPath(t, dir))
		return err == nil && strings.Contains(string(raw), `"revision": 1`)
	})
	renameOnDisk(t, dir, "Cellar")
	update := nextUpdate(t, view, "the floor sent again", func(update client.Update) bool {
		return update.Snapshot != nil && *update.Snapshot == floor && update.Said != ""
	})
	if update.Said != "Took in Cellar as changed on disk" {
		t.Fatalf("the window is told why: %q", update.Said)
	}
	view.View(floor, func(document client.Document) {
		if document.Name != "Cellar" {
			t.Errorf("the window shows the file's version: %q", document.Name)
		}
	})
	if _, err := undo(t, view); err == nil {
		t.Fatal("the undo steps for the version replaced are gone")
	}
}

// A file changed outside the editor while a window had typed changes for it
// is not written over: its author is asked, and Keep mine writes the
// editor's version, keeping the file's in the backups.
func TestAFileChangedInBothPlacesKeepsMineWhenAsked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, fast)
	view := h.dial(t, "")
	if err := view.Unsaved(protocol.Unsaved{Count: 1, Name: "Floor 1"}); err != nil {
		t.Fatal(err)
	}
	theirs := renameOnDisk(t, dir, "Cellar")
	notice := nextNotice(t, view)
	if notice.Kind != protocol.NoticeChangedOnDisk || len(notice.Actions) != 2 {
		t.Fatalf("its author is asked which to keep: %+v", notice)
	}
	time.Sleep(100 * time.Millisecond)
	if raw, err := os.ReadFile(floorPath(t, dir)); err != nil || string(raw) != string(theirs) {
		t.Fatalf("nothing is written over the file until its author chooses: %v", err)
	}
	if err := view.NoticeChoice(notice.Key, "keep-mine"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the editor's version to be written", func() bool {
		raw, err := os.ReadFile(floorPath(t, dir))
		return err == nil && strings.Contains(string(raw), `"name": "Floor 1"`)
	})
	kept, err := filepath.Glob(filepath.Join(dir, ".berad", "backups", "changed", "*.bak"))
	if err != nil || len(kept) != 1 {
		t.Fatalf("the file's version is kept in the backups: %v %v", kept, err)
	}
	if data, err := os.ReadFile(kept[0]); err != nil || string(data) != string(theirs) {
		t.Fatalf("the backup is the file's version: %v", err)
	}
}

// Take the file's takes in the file's version, keeping the editor's in the
// backups, and the window showing it says so.
func TestAFileChangedInBothPlacesTakesTheFilesWhenAsked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floor := createProject(t, dir)
	h := start(t, dir, fast)
	view := h.dial(t, "")
	if err := view.Unsaved(protocol.Unsaved{Count: 1, Name: "Floor 1"}); err != nil {
		t.Fatal(err)
	}
	renameOnDisk(t, dir, "Cellar")
	notice := nextNotice(t, view)
	if err := view.NoticeChoice(notice.Key, "take-file"); err != nil {
		t.Fatal(err)
	}
	update := nextUpdate(t, view, "the floor sent again", func(update client.Update) bool {
		return update.Snapshot != nil && *update.Snapshot == floor && update.Said != ""
	})
	if !strings.Contains(update.Said, "your version is in the backups") {
		t.Fatalf("the window says where the editor's version went: %q", update.Said)
	}
	kept, err := filepath.Glob(filepath.Join(dir, ".berad", "backups", "changed", "*.bak"))
	if err != nil || len(kept) != 1 {
		t.Fatalf("the editor's version is kept in the backups: %v %v", kept, err)
	}
	if data, err := os.ReadFile(kept[0]); err != nil || !strings.Contains(string(data), "grass") {
		t.Fatalf("the backup is the editor's version of the floor: %v", err)
	}
}

// A field the editor does not know is said when the project opens, and the
// file is copied to the backups before the save that drops it.
func TestFieldsTheEditorDoesNotKnowAreToldAndKept(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floor := createProject(t, dir)
	path := floorPath(t, dir)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	file["note"] = "added by hand"
	file["data"].(map[string]any)["mood"] = "tired"
	changed, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	h := start(t, dir, fast)
	view := h.dial(t, "")
	notice := nextNotice(t, view)
	if notice.Kind != protocol.NoticeUnknownFields || !strings.Contains(notice.Text, "note, mood") {
		t.Fatalf("the notice names the file and its fields: %+v", notice)
	}
	mustPaint(t, view, floor, 0, 0, "wall", "")
	waitFor(t, "the paint to be saved", func() bool {
		saved, err := os.ReadFile(path)
		return err == nil && !strings.Contains(string(saved), "mood")
	})
	kept, err := filepath.Glob(filepath.Join(dir, ".berad", "backups", "unknown-fields", "*.bak"))
	if err != nil || len(kept) != 1 {
		t.Fatalf("the file is kept in the backups first: %v %v", kept, err)
	}
	if data, err := os.ReadFile(kept[0]); err != nil || !strings.Contains(string(data), "mood") {
		t.Fatalf("the backup holds what was dropped: %v", err)
	}
}

// A document every project holds whose file cannot be read stands in for
// it: the project asks, then says so, and never writes over the file.
func TestARequiredDocumentThatCannotBeReadStandsIn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configure := withGame(t)
	createWithGame(t, dir, configure)
	matches, err := filepath.Glob(filepath.Join(dir, "*", "*--"+game.SettingsID().Short(8)+".json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("the settings' file: %v %v", matches, err)
	}
	broken := []byte("{ not json")
	if err := os.WriteFile(matches[0], broken, 0o600); err != nil {
		t.Fatal(err)
	}
	h := start(t, dir, configure)
	view := h.dial(t, "")
	if asked := nextOpening(t, view); len(asked.Files) != 1 {
		t.Fatalf("the question names the settings' file: %+v", asked)
	}
	if err := view.NoticeChoice(protocol.OpeningKey, protocol.OpenAnyway); err != nil {
		t.Fatal(err)
	}
	saw := map[string]bool{}
	for range 2 {
		saw[nextNotice(t, view).Kind] = true
	}
	if !saw[protocol.NoticeStandIn] || !saw[protocol.NoticeFilesLeftOut] {
		t.Fatalf("both the file left out and the stand-in are told: %v", saw)
	}
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	if after, err := os.ReadFile(matches[0]); err != nil || string(after) != string(broken) {
		t.Fatalf("nothing is written over the file: %q %v", after, err)
	}
}

// Notices last side by side: the newest shows, with how many others last.
func TestNoticesWaitTheirTurn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, fast)
	view := h.dial(t, "")
	if err := view.Unsaved(protocol.Unsaved{Count: 1, Name: "Floor 1"}); err != nil {
		t.Fatal(err)
	}
	renameOnDisk(t, dir, "Cellar")
	first := nextNotice(t, view)
	if err := view.Crashed("a test failure"); err != nil {
		t.Fatal(err)
	}
	second := nextNotice(t, view)
	if second.Kind != protocol.NoticeWindowCrashed || second.More != 1 {
		t.Fatalf("the newest shows, with one more beside it: %+v", second)
	}
	if err := view.NoticeChoice(second.Key, "dismiss"); err != nil {
		t.Fatal(err)
	}
	if again := nextNotice(t, view); again.Key != first.Key || again.More != 0 {
		t.Fatalf("the one before shows again once the newest ends: %+v", again)
	}
}
