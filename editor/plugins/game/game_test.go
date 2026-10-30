package game_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
)

func encode(t *testing.T, args any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func id(number byte) ids.ULID {
	var made ids.ULID
	made[15] = number

	return made
}

func TestEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(gameplugin.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	document := &gameplugin.SettingsDocument{Settings: game.DefaultSettings()}
	start := game.Start{Zone: id(1), Floor: id(2), Col: 4, Row: -2}
	for _, check := range []struct {
		kind model.EditKind
		args json.RawMessage
	}{
		{gameplugin.SetScreen{}, encode(t, gameplugin.ScreenArgs{Columns: 15, Rows: 11})},
		{gameplugin.SetStart{}, encode(t, gameplugin.StartArgs{Start: start})},
	} {
		if err := model.VerifyUndo(document, check.kind, check.args, registry.Edit); err != nil {
			t.Fatalf("%s: %v", check.kind.Name(), err)
		}
		if _, err := check.kind.Apply(document, check.args); err != nil {
			t.Fatal(err)
		}
	}
	if document.Settings.Columns != 15 || document.Settings.Start != start {
		t.Fatalf("settings %+v", document.Settings)
	}
	references := document.References()
	if len(references) != 2 || references[0].Document != id(1) || references[1].Document != id(2) {
		t.Fatalf("the start refers to its zone and floor: %+v", references)
	}
	even := encode(t, gameplugin.ScreenArgs{Columns: 16, Rows: 11})
	if _, err := (gameplugin.SetScreen{}).Apply(document, even); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("an even screen: %v", err)
	}
	if document.Settings.Columns != 15 {
		t.Fatal("a refused edit leaves the settings alone")
	}
}

func TestEveryProjectHoldsGameSettings(t *testing.T) {
	t.Parallel()
	required := gameplugin.Plugin().Required
	if len(required) != 1 || required[0].ID != game.SettingsID() || required[0].ReadOnly {
		t.Fatalf("required %+v", required)
	}
	data, err := required[0].Data()
	if err != nil {
		t.Fatal(err)
	}
	settings, err := game.DecodeSettings(data)
	if err != nil || settings != game.DefaultSettings() {
		t.Fatalf("a new project's game settings %+v %v", settings, err)
	}
}
