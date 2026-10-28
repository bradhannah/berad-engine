package orchestrator_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugin"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

func artRegistry(t testing.TB) *plugin.Registry {
	t.Helper()
	registry, err := plugin.NewRegistry(
		project.Plugin(), tilefloor.Plugin(), zone.Plugin(), textdoc.Plugin(), workflow.Plugin(), npc.Plugin(),
		artplugin.Plugin(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func withArt(t testing.TB) func(*orchestrator.Config) {
	return func(cfg *orchestrator.Config) { cfg.Registry = artRegistry(t) }
}

func grey(number int) art.ColorRef {
	return art.ColorRef{Palette: art.SharedPaletteID(), Ramp: art.StarterRampID(), Shade: art.StarterShadeID(number)}
}

// dialArt connects a window that holds the shared palette and a tile.
func dialArt(t testing.TB, h *harness, group string, tile ids.ULID) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := client.Dial(ctx, h.socket, protocol.Hello{
		Build: testBuild, View: "map", Target: art.SharedPaletteID(), UndoGroup: group,
	}, artRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	for _, document := range []ids.ULID{art.SharedPaletteID(), tile} {
		if err := connection.Subscribe(ctx, document); err != nil {
			t.Fatal(err)
		}
	}
	return connection
}

func refused(t testing.TB, result protocol.Result, err error, reason string) {
	t.Helper()
	if !errors.Is(err, client.ErrRejected) || result.OK || !strings.Contains(result.Reason, reason) {
		t.Fatalf("want refused for %q, got %+v %v", reason, result, err)
	}
}

func TestOlderProjectGetsItsArtDocuments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	for range 2 {
		cfg := baseConfig(t, dir, shortSocket(t))
		cfg.Registry = artRegistry(t)
		orch, err := orchestrator.New(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		documents, err := orch.Documents()
		if err != nil {
			t.Fatal(err)
		}
		found := map[ids.ULID]string{}
		for _, document := range documents {
			found[document.ID] = document.Name
		}
		for fixed, name := range map[ids.ULID]string{
			art.SharedPaletteID(): art.SharedPaletteName, art.SettingsID(): art.SettingsName,
			art.StandInsID(): art.StandInsName,
		} {
			if found[fixed] != name {
				t.Fatalf("%s: found %q among %+v", name, found[fixed], documents)
			}
		}
		if len(documents) != 4 {
			t.Fatalf("the floor and three art documents, each once: %+v", documents)
		}
		if err := orch.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for folder, format := range map[string]string{
		"palettes": art.PaletteFormatName, "art-settings": art.SettingsFormatName,
		"standin-sprites": art.StandInsFormatName,
	} {
		files, err := filepath.Glob(filepath.Join(dir, folder, "*.json"))
		if err != nil || len(files) != 1 {
			t.Fatalf("%s: %v %v", folder, files, err)
		}
		raw, err := os.ReadFile(files[0]) //nolint:gosec // A file the test just wrote.
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := content.Decode(raw)
		if err != nil || envelope.Format != format || envelope.Version != 1 {
			t.Fatalf("%s: %+v %v", folder, envelope, err)
		}
	}
}

// projectWithLantern makes a project whose sprite "Lantern" has a tile
// painted with grey 3, and returns the tile's ID.
func projectWithLantern(t *testing.T, dir string) ids.ULID {
	t.Helper()
	createProject(t, dir)
	cfg := baseConfig(t, dir, shortSocket(t))
	cfg.Registry = artRegistry(t)
	orch, err := orchestrator.New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	tile, err := art.NewTile(16)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tile.Set(0, 0, grey(3)); err != nil {
		t.Fatal(err)
	}
	tileID, err := orch.CreateDocument("Frame 1", &artplugin.TileDocument{Tile: tile})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orch.CreateDocument("Lantern", &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: tileID}}); err != nil {
		t.Fatal(err)
	}
	if err := orch.Close(); err != nil {
		t.Fatal(err)
	}
	return tileID
}

func TestColourInUseIsNotRemoved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tileID := projectWithLantern(t, dir)
	h := start(t, dir, withArt(t))
	view := dialArt(t, h, "", tileID)
	palette := art.SharedPaletteID()
	result, err := command(t, view, palette, artplugin.RemoveShadeEdit, "", artplugin.RemoveShadeArgs{
		Ramp: art.StarterRampID(), Shade: art.StarterShadeID(3),
	})
	refused(t, result, err, "used by Lantern")
	result, err = command(t, view, palette, artplugin.RemoveRampEdit, "", artplugin.RemoveRampArgs{Ramp: art.StarterRampID()})
	refused(t, result, err, "used by Lantern")
	if _, err := command(t, view, palette, artplugin.RemoveShadeEdit, "", artplugin.RemoveShadeArgs{
		Ramp: art.StarterRampID(), Shade: art.StarterShadeID(1),
	}); err != nil {
		t.Fatalf("a shade no tile uses can be removed: %v", err)
	}
	if _, err := undo(t, view); err != nil {
		t.Fatalf("undoing the removal: %v", err)
	}
	for _, change := range []struct {
		edit string
		args any
	}{
		{project.DeleteEdit, project.DocumentArgs{ID: palette}},
		{project.RenameEdit, project.RenameArgs{ID: art.SettingsID(), Name: "Mine"}},
	} {
		result, err := command(t, view, project.IndexID(), change.edit, "", change.args)
		refused(t, result, err, "every project holds")
	}
}

func TestUndoNeverRemovesAColourInUse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tileID := projectWithLantern(t, dir)
	h := start(t, dir, withArt(t))
	palettes := dialArt(t, h, "", tileID)
	painter := dialArt(t, h, "painter", tileID)
	sand := art.Ramp{ID: ids.MustParse("01J00000000000000000000001"), Name: "sand", Shades: []art.Shade{
		{ID: ids.MustParse("01J00000000000000000000002"), Color: 0xC2AE6A, Opacity: 100},
	}}
	if _, err := command(t, palettes, art.SharedPaletteID(), artplugin.AddRampEdit, "", artplugin.AddRampArgs{
		Index: 1, Ramp: sand,
	}); err != nil {
		t.Fatal(err)
	}
	sand1 := art.ColorRef{Palette: art.SharedPaletteID(), Ramp: sand.ID, Shade: sand.Shades[0].ID}
	paintAt := func(color art.ColorRef) {
		if _, err := command(t, painter, tileID, artplugin.PaintEdit, "", artplugin.PaintArgs{
			Pixels: []artplugin.Pixel{{Col: 1, Row: 1, Color: color}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	paintAt(sand1)
	result, err := undo(t, palettes)
	refused(t, result, err, "used by Lantern")
	paintAt(art.ColorRef{})
	if _, err := undo(t, palettes); err != nil {
		t.Fatalf("a refused undo stays on the list until nothing uses the colour: %v", err)
	}
	if _, err := redo(t, palettes); err != nil {
		t.Fatal(err)
	}
}

// TestChangeDocumentBeforeRun changes a document every project holds before
// the orchestrator runs, as the sample does, and saves it at once; a change
// that fails and an unknown document are reported.
func TestChangeDocumentBeforeRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	cfg := baseConfig(t, dir, shortSocket(t))
	cfg.Registry = artRegistry(t)
	orch, err := orchestrator.New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = orch.Close() })
	lantern := ids.MustParse("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	err = orch.ChangeDocument(art.StandInsID(), func(document model.Document) error {
		_, setErr := document.(*artplugin.StandInsDocument).Sprites.Set("pickup.small", lantern)

		return setErr
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "standin-sprites", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("saved at once: %v %v", files, err)
	}
	raw, err := os.ReadFile(files[0]) //nolint:gosec // A file the test just wrote.
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := content.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := art.DecodeStandInSprites(envelope.Data)
	if shown, ok := saved.Sprite("pickup.small"); err != nil || !ok || shown != lantern {
		t.Fatalf("the saved stand-in sprites: %v %v %v", shown, ok, err)
	}
	failing := errors.New("refused")
	if err := orch.ChangeDocument(art.StandInsID(), func(model.Document) error { return failing }); !errors.Is(err, failing) {
		t.Fatalf("a failed change is reported: %v", err)
	}
	if err := orch.ChangeDocument(ids.MustParse("01ARZ3NDEKTSV4RRFFQ69G5FAW"), func(model.Document) error { return nil }); !errors.Is(err, orchestrator.ErrUnknownDocument) {
		t.Fatalf("an unknown document is reported: %v", err)
	}
}
