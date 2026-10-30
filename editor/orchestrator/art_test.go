package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
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

// The tests' own palette, "gloomy", has one ramp, "grey", of five shades.
func greyRamp() ids.ULID { return ids.MustParse("01J00000000000000000000020") }

func greyShade(number int) ids.ULID {
	return ids.MustParse(fmt.Sprintf("01J00000000000000000000%03d", 20+number))
}

func grey(palette ids.ULID, number int) art.ColorRef {
	return art.Named(palette, greyRamp(), greyShade(number))
}

func gloomy(t testing.TB) *artplugin.PaletteDocument {
	t.Helper()
	ramp := art.Ramp{ID: greyRamp(), Name: "grey", Shades: nil}
	for number, value := range []art.RGB{0x202020, 0x4A4A4A, 0x7A7A7A, 0xABABAB, 0xDCDCDC} {
		ramp.Shades = append(ramp.Shades, art.Shade{ID: greyShade(number + 1), Color: value, Opacity: art.MaxOpacity})
	}
	palette := art.NewPalette()
	if err := palette.InsertRamp(0, ramp); err != nil {
		t.Fatal(err)
	}
	return &artplugin.PaletteDocument{Palette: palette}
}

// lantern is a project with the palette gloomy and a sprite, "Lantern",
// whose tile is painted with gloomy's grey 3.
type lantern struct {
	tile, palette ids.ULID
}

// dialArt connects a window that holds a palette, Default and a tile.
func dialArt(t testing.TB, h *harness, group string, made lantern) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := client.Dial(ctx, h.socket, protocol.Hello{
		Build: testBuild, View: "map", Target: made.tile, UndoGroup: group,
	}, artRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	for _, document := range []ids.ULID{made.palette, art.DefaultPaletteID(), made.tile} {
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
			art.DefaultPaletteID(): art.DefaultPaletteName, art.SettingsID(): art.SettingsName,
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
	for folder, format := range map[string]content.Format{
		"palettes": art.PaletteFormat(), "art-settings": art.SettingsFormat(),
		"standin-sprites": art.StandInsFormat(),
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
		if err != nil || envelope.Format != format.Name || envelope.Version != format.Current {
			t.Fatalf("%s: %+v %v", folder, envelope, err)
		}
	}
}

// projectWithLantern makes a project with the palette gloomy and a sprite,
// "Lantern", whose tile is painted with gloomy's grey 3.
func projectWithLantern(t *testing.T, dir string) lantern {
	t.Helper()
	createProject(t, dir)
	cfg := baseConfig(t, dir, shortSocket(t))
	cfg.Registry = artRegistry(t)
	orch, err := orchestrator.New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	paletteID, err := orch.CreateDocument("gloomy", gloomy(t))
	if err != nil {
		t.Fatal(err)
	}
	tile, err := art.NewTile(16)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tile.Set(0, 0, grey(paletteID, 3)); err != nil {
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
	return lantern{tile: tileID, palette: paletteID}
}

func TestColourInUseIsNotRemoved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	made := projectWithLantern(t, dir)
	h := start(t, dir, withArt(t))
	view := dialArt(t, h, "", made)
	palette := made.palette
	result, err := command(t, view, palette, artplugin.RemoveShadeEdit, "", artplugin.RemoveShadeArgs{
		Ramp: greyRamp(), Shade: greyShade(3),
	})
	refused(t, result, err, "used by Lantern")
	result, err = command(t, view, palette, artplugin.RemoveRampEdit, "", artplugin.RemoveRampArgs{Ramp: greyRamp()})
	refused(t, result, err, "used by Lantern")
	result, err = command(t, view, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: palette})
	refused(t, result, err, "used by Lantern")
	if _, err := command(t, view, palette, artplugin.RemoveShadeEdit, "", artplugin.RemoveShadeArgs{
		Ramp: greyRamp(), Shade: greyShade(1),
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
		{project.DeleteEdit, project.DocumentArgs{ID: art.DefaultPaletteID()}},
		{project.RenameEdit, project.RenameArgs{ID: art.DefaultPaletteID(), Name: "Mine"}},
		{project.RenameEdit, project.RenameArgs{ID: art.SettingsID(), Name: "Mine"}},
	} {
		result, err := command(t, view, project.IndexID(), change.edit, "", change.args)
		refused(t, result, err, "every project holds")
	}
}

func TestUndoNeverRemovesAColourInUse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	made := projectWithLantern(t, dir)
	tileID := made.tile
	h := start(t, dir, withArt(t))
	palettes := dialArt(t, h, "", made)
	painter := dialArt(t, h, "painter", made)
	sand := art.Ramp{ID: ids.MustParse("01J00000000000000000000001"), Name: "sand", Shades: []art.Shade{
		{ID: ids.MustParse("01J00000000000000000000002"), Color: 0xC2AE6A, Opacity: 100},
	}}
	if _, err := command(t, palettes, made.palette, artplugin.AddRampEdit, "", artplugin.AddRampArgs{
		Index: 1, Ramp: sand,
	}); err != nil {
		t.Fatal(err)
	}
	sand1 := art.Named(made.palette, sand.ID, sand.Shades[0].ID)
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

// TestDefaultIsReadOnly refuses every edit to Default, and a project whose
// saved Default was changed gets the engine's back when it opens.
func TestDefaultIsReadOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	made := projectWithLantern(t, dir)
	files, err := filepath.Glob(filepath.Join(dir, "palettes", "default--*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("Default is saved: %v %v", files, err)
	}
	raw, err := os.ReadFile(files[0]) //nolint:gosec // A file the test just wrote.
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(raw), `"grey"`, `"gray"`, 1)
	if changed == string(raw) {
		t.Fatal("the test expects Default's grey ramp in its file")
	}
	if err := os.WriteFile(files[0], []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	h := start(t, dir, withArt(t))
	view := dialArt(t, h, "", made)
	var name string
	view.View(art.DefaultPaletteID(), func(document client.Document) {
		ramp, _ := document.Model.(*artplugin.PaletteDocument).Palette.Ramp(art.DefaultRampID(1))
		name = ramp.Name
	})
	if name != "grey" {
		t.Fatalf("a changed Default is restored when the project opens: %q", name)
	}
	for _, change := range []struct {
		edit string
		args any
	}{
		{artplugin.RenameRampEdit, artplugin.RenameRampArgs{Ramp: art.DefaultRampID(1), Name: "gray"}},
		{artplugin.SetShadeEdit, artplugin.SetShadeArgs{
			Ramp: art.DefaultRampID(1), Shade: art.Shade{ID: art.DefaultShadeID(1, 1), Color: 1, Opacity: 100, Label: ""},
		}},
		{artplugin.MoveRampEdit, artplugin.MoveRampArgs{Ramp: art.DefaultRampID(1), Index: 3}},
	} {
		result, err := command(t, view, art.DefaultPaletteID(), change.edit, "", change.args)
		refused(t, result, err, "read only")
	}
}
