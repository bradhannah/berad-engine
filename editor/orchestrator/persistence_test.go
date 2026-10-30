package orchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

func readFloorFile(t *testing.T, dir string) content.Envelope {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "tilefloors", "*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("floor files: %v %v", matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := content.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

// D11: one readable file per document, named "<name>--<short id>.json".
func TestDocumentFileIsReadableAndNamed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id := createProject(t, dir)
	want := filepath.Join(dir, "tilefloors", "floor-1--"+id.Short(8)+".json")
	raw, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\"format\": \"tilefloor\"") || !strings.Contains(string(raw), fmt.Sprintf("\"version\": %d", floor.FormatVersion)) {
		t.Fatalf("file is not a readable, versioned envelope:\n%.200s", raw)
	}
}

// D9: always saved. A graceful stop writes every change to its file and empties
// the journal.
func TestStopSavesFilesAndEmptiesJournal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	view := h.dial(t, "")
	mustPaint(t, view, h.floor, 7, 7, "wall", "")
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	envelope := readFloorFile(t, dir)
	if envelope.Revision != 1 {
		t.Fatalf("saved revision %d", envelope.Revision)
	}
	floor, err := tilefloor.Decode(envelope.Data)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := floor.(*tilefloor.Document).Floor.Ground(7, 7); got != "wall" {
		t.Fatalf("saved tile %q", got)
	}
	journal, err := os.ReadFile(filepath.Join(dir, ".berad", "journal.log"))
	if err != nil || len(journal) != 0 {
		t.Fatalf("journal not emptied: %q %v", journal, err)
	}
}

// D9 and increment 1's plan: killing the orchestrator process loses no
// acknowledged edit. The child process never flushes, so only the journal
// holds the edits; it is killed with SIGKILL, then the project is reopened.
func TestKilledOrchestratorLosesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floor := createProject(t, dir)
	socket := shortSocket(t)
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), envRole+"="+roleOrchestrator, envDir+"="+dir, envSocket+"="+socket)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	var view *client.Client
	waitFor(t, "child orchestrator", func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var err error
		view, err = client.Dial(ctx, socket, protocol.Hello{View: "map"}, newRegistry(t))
		return err == nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := view.Subscribe(ctx, floor); err != nil {
		t.Fatal(err)
	}
	const painted = 50
	for index := range painted {
		mustPaint(t, view, floor, index%16, index/16, "wall", "")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	_ = view.Close()

	h := start(t, dir, nil)
	reopened := h.dial(t, "")
	if got := revision(reopened, floor); got != painted {
		t.Fatalf("recovered revision %d, want %d", got, painted)
	}
	for index := range painted {
		if got := tileAt(t, reopened, floor, index%16, index/16); got != "wall" {
			t.Fatalf("cell %d = %q after recovery", index, got)
		}
	}
}

// A crash mid-write can cut the last journal line short; it is ignored.
func TestTornJournalLineIsIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floor := createProject(t, dir)
	args := paintArgs(t, tilefloor.Cell{Col: 0, Row: 0, Layer: tilefloor.LayerGround, Tile: "wall"})
	entry := map[string]any{"v": 1, "seq": 1, "document": floor, "revision": 1, "edit": tilefloor.PaintEdit, "args": json.RawMessage(args)}
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	journal := append(append(line, '\n'), []byte(`{"v":1,"seq":2,"docu`)...)
	if err := os.WriteFile(filepath.Join(dir, ".berad", "journal.log"), journal, 0o600); err != nil {
		t.Fatal(err)
	}
	h := start(t, dir, nil)
	view := h.dial(t, "")
	if got := tileAt(t, view, floor, 0, 0); got != "wall" || revision(view, floor) != 1 {
		t.Fatalf("tile %q revision %d", got, revision(view, floor))
	}
}

func TestJournalGapRefusesToOpen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floor := createProject(t, dir)
	entry := map[string]any{"v": 1, "seq": 1, "document": floor, "revision": 5, "edit": tilefloor.PaintEdit,
		"args": json.RawMessage(paintArgs(t, tilefloor.Cell{Col: 0, Row: 0, Layer: tilefloor.LayerGround, Tile: "wall"}))}
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".berad", "journal.log"), append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = orchestrator.New(context.Background(), baseConfig(t, dir, shortSocket(t)))
	if !errors.Is(err, orchestrator.ErrJournalGap) {
		t.Fatalf("err = %v", err)
	}
}

// nextVersionPlugin re-registers the tile floor format one version ahead with
// an upgrade step, standing in for a future format change.
func nextVersionPlugin() plugin.Plugin {
	base := tilefloor.Plugin()
	documentType := base.Documents[0]
	migrations := map[int]content.Migration{}
	for version, step := range documentType.Format.Migrations {
		migrations[version] = step
	}
	migrations[floor.FormatVersion] = func(data json.RawMessage) (json.RawMessage, error) { return data, nil }
	documentType.Format.Current = floor.FormatVersion + 1
	documentType.Format.Migrations = migrations
	base.Documents = []model.DocumentType{documentType}
	return base
}

// D17: an old file is backed up, upgraded on load and saved in the new version.
func TestOldFileIsBackedUpAndUpgraded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	registry, err := plugin.NewRegistry(project.Plugin(), nextVersionPlugin())
	if err != nil {
		t.Fatal(err)
	}
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.Registry = registry })
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	if envelope := readFloorFile(t, dir); envelope.Version != floor.FormatVersion+1 {
		t.Fatalf("saved version %d", envelope.Version)
	}
	pattern := fmt.Sprintf("*.v%d.*.bak", floor.FormatVersion)
	backups, err := filepath.Glob(filepath.Join(dir, ".berad", "backups", pattern))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups %v %v", backups, err)
	}
	original, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if envelope, err := content.Decode(original); err != nil || envelope.Version != floor.FormatVersion {
		t.Fatalf("backup is not the original: %v %v", envelope.Version, err)
	}
}

// The first real format upgrade: a version 1 floor file opens in the current
// version, with walls moved to the top layer and its top-left cell at the
// origin, and the original is kept as a backup.
func TestVersionOneFloorIsUpgraded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id := createProject(t, dir)
	matches, err := filepath.Glob(filepath.Join(dir, "tilefloors", "*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("floor files %v %v", matches, err)
	}
	tiles := make([]string, 16*16)
	for index := range tiles {
		tiles[index] = "grass"
	}
	tiles[0] = "wall"
	data, err := json.Marshal(map[string]any{"width": 16, "height": 16, "tiles": tiles})
	if err != nil {
		t.Fatal(err)
	}
	old, err := content.Encode(content.Envelope{
		Format: floor.FormatName, Version: 1, ID: id, Name: "Floor 1", Revision: 2, Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(matches[0], old, 0o600); err != nil {
		t.Fatal(err)
	}
	h := start(t, dir, nil)
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	envelope := readFloorFile(t, dir)
	upgraded, err := floor.Decode(envelope.Data)
	if err != nil || envelope.Version != floor.FormatVersion || envelope.Revision != 2 ||
		upgraded.Bounds() != image.Rect(0, 0, 16, 16) {
		t.Fatalf("upgraded file: version %d revision %d %v", envelope.Version, envelope.Revision, err)
	}
	ground, _ := upgraded.Ground(0, 0) //nolint:errcheck // On the floor.
	top, _ := upgraded.Top(0, 0)       //nolint:errcheck // On the floor.
	if ground != "floor" || top != "wall" {
		t.Fatalf("wall cell became %q under %q", ground, top)
	}
	backups, err := filepath.Glob(filepath.Join(dir, ".berad", "backups", "*.v1.*.bak"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups %v %v", backups, err)
	}
}

func TestNewerFileRefusesToOpen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	envelope := readFloorFile(t, dir)
	envelope.Version = 99
	encoded, err := content.Encode(envelope)
	if err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "tilefloors", "*.json"))
	if err := os.WriteFile(matches[0], encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = orchestrator.New(context.Background(), baseConfig(t, dir, shortSocket(t)))
	if !errors.Is(err, content.ErrNewerVersion) {
		t.Fatalf("err = %v", err)
	}
}

// D10: one orchestrator per project.
func TestSecondOrchestratorIsRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	_, err := orchestrator.New(context.Background(), baseConfig(t, dir, shortSocket(t)))
	if !errors.Is(err, orchestrator.ErrProjectOpen) || !strings.Contains(err.Error(), h.socket) {
		t.Fatalf("err = %v", err)
	}
	socket, err := orchestrator.RunningSocket(dir)
	if err != nil || socket != h.socket {
		t.Fatalf("running socket %q %v", socket, err)
	}
}

// D6: the wire tap records every message as a readable JSON line.
func TestWireTapRecordsMessages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	tapPath := filepath.Join(t.TempDir(), "tap.jsonl")
	tap, err := os.Create(tapPath)
	if err != nil {
		t.Fatal(err)
	}
	defer tap.Close()
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.Tap = tap })
	view := h.dial(t, "")
	mustPaint(t, view, h.floor, 0, 0, "wall", "")
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(tapPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{`"kind":"welcome"`, `"kind":"command"`, `"kind":"event"`, `"kind":"result"`} {
		if !strings.Contains(string(raw), kind) {
			t.Fatalf("tap lacks %s", kind)
		}
	}
	var first orchestrator.Recorded
	if err := json.Unmarshal([]byte(strings.SplitN(string(raw), "\n", 2)[0]), &first); err != nil || first.Window == (ids.ULID{}) {
		t.Fatalf("tap line not decodable: %v", err)
	}
}
