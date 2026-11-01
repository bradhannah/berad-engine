// Package game is the plug-in for a project's game settings: the screen's
// size in cells and the player start (increment 14).
package game

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/engine/game"
)

// PluginName identifies the plug-in.
const PluginName = "game"

// Edit kind names.
const (
	ScreenEdit = "game.settings.screen"
	StartEdit  = "game.settings.start"
)

// Undo-clash items.
const (
	screenItem = "screen"
	startItem  = "start"
)

// ErrNotSettings reports a document that is not game settings.
var ErrNotSettings = errors.New("document is not game settings")

// SettingsDocument is a project's game settings held by the editor.
type SettingsDocument struct {
	Settings game.Settings
}

// Format names the document format.
func (d *SettingsDocument) Format() string { return game.SettingsFormatName }

// Encode returns the saved data.
func (d *SettingsDocument) Encode() (json.RawMessage, error) {
	data, err := d.Settings.Encode()
	if err != nil {
		return nil, fmt.Errorf("game settings document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *SettingsDocument) Clone() model.Document { return &SettingsDocument{Settings: d.Settings} }

// References lists the player start's zone and floor, so deleting either
// shows that the start is there.
func (d *SettingsDocument) References() []model.Reference {
	start := d.Settings.Start
	if start.IsZero() {
		return nil
	}

	return []model.Reference{
		{Document: start.Zone, Item: "", Label: "player start"},
		{Document: start.Floor, Item: "", Label: "player start"},
	}
}

// DecodeSettings reads saved game settings.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func DecodeSettings(data json.RawMessage) (model.Document, error) {
	settings, err := game.DecodeSettings(data)
	if err != nil {
		return nil, fmt.Errorf("game settings document: %w", err)
	}

	return &SettingsDocument{Settings: settings}, nil
}

// ScreenArgs sets how many cells the screen shows across and down.
type ScreenArgs struct {
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
}

// StartArgs places the player start, or with a zero zone takes it away.
type StartArgs struct {
	Start game.Start `json:"start"`
}

// SetScreen is the edit kind that sets the screen's size.
type SetScreen struct{}

// Name identifies the edit kind.
func (SetScreen) Name() string { return ScreenEdit }

// Format names the document format the edit applies to.
func (SetScreen) Format() string { return game.SettingsFormatName }

// Apply sets the screen's size; its inverse restores the previous size.
func (SetScreen) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var set ScreenArgs

	return change(document, args, &set, screenItem, func(settings *game.Settings) any {
		previous := ScreenArgs{Columns: settings.Columns, Rows: settings.Rows}
		settings.Columns, settings.Rows = set.Columns, set.Rows

		return previous
	})
}

// SetStart is the edit kind that places the player start.
type SetStart struct{}

// Name identifies the edit kind.
func (SetStart) Name() string { return StartEdit }

// Format names the document format the edit applies to.
func (SetStart) Format() string { return game.SettingsFormatName }

// Apply places the start; its inverse puts the previous one back.
func (SetStart) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	var set StartArgs

	return change(document, args, &set, startItem, func(settings *game.Settings) any {
		previous := StartArgs{Start: settings.Start}
		settings.Start = set.Start

		return previous
	})
}

// change decodes an edit's arguments into target, applies it to a copy of
// the settings, and keeps the copy only if it is valid.
func change(
	document model.Document, args json.RawMessage, target any, item string, apply func(*game.Settings) any,
) (model.Applied, error) {
	settingsDoc, ok := document.(*SettingsDocument)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotSettings)
	}
	if err := json.Unmarshal(args, target); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	changed := settingsDoc.Settings
	inverse := apply(&changed)
	if err := changed.Validate(); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	inverseArgs, err := json.Marshal(inverse)
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}
	settingsDoc.Settings = changed

	return model.Applied{Inverse: inverseArgs, InverseEdit: "", Touched: []string{item}, Covers: nil}, nil
}

// Plugin returns the game plug-in.
func Plugin() plugin.Plugin {
	return plugin.Plugin{
		Name: PluginName,
		Documents: []model.DocumentType{
			{Format: game.SettingsFormat(), Decode: DecodeSettings, Layout: nil, Prepare: nil},
		},
		Edits: []model.EditKind{SetScreen{}, SetStart{}},
		Required: []model.RequiredDocument{{
			ID: game.SettingsID(), Format: game.SettingsFormatName, Name: game.SettingsName,
			Data: game.DefaultSettings().Encode, ReadOnly: false,
		}},
	}
}
