// Package game is a project played as a game: its game settings, which say
// how much of a map its screen shows and where the player starts, and the
// game runtime that plays it (increment 14). The editor's Play tab and a
// game's own program run the same code, so they always agree.
package game

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// SettingsFormatName identifies game settings inside saved envelopes. The
// document's folder is the format's name with an s.
const SettingsFormatName = "game-setting"

// SettingsVersion is the game settings format version this package writes.
const SettingsVersion = 1

// SettingsName is the name of the game settings every project holds.
const SettingsName = "Game settings"

// settingsID is the game settings' fixed ID. Its time part is zero, so no
// generated ID ever equals it.
const settingsID = "00000000000000000000000014"

// Screen sizes, in cells. The player stands in the middle cell, so a screen
// is an odd number of cells across and down.
const (
	DefaultColumns = 19
	DefaultRows    = 11
	MinScreen      = 3
	MaxScreen      = 63
)

// ErrData reports saved game settings that cannot be read.
var ErrData = errors.New("invalid game settings data")

// ErrSettings reports game settings that are not allowed.
var ErrSettings = errors.New("invalid game settings")

// SettingsID is the ID of a project's game settings.
func SettingsID() ids.ULID { return ids.MustParse(settingsID) }

// Start is where Launch puts the player: a cell of a floor of a zone. A zero
// zone means no start is placed yet.
type Start struct {
	Zone  ids.ULID `json:"zone"`
	Floor ids.ULID `json:"floor"`
	Col   int      `json:"col"`
	Row   int      `json:"row"`
}

// IsZero reports whether no start is placed.
func (s Start) IsZero() bool { return s.Zone.IsZero() }

// Settings are a project's game settings.
type Settings struct {
	// Columns and Rows are how many cells the screen shows across and down.
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
	// Start is the player start, zero until one is placed.
	Start Start `json:"start,omitzero"`
}

// DefaultSettings returns a new project's game settings: a screen of 19 by
// 11 cells, and no player start.
func DefaultSettings() Settings {
	return Settings{Columns: DefaultColumns, Rows: DefaultRows, Start: NoStart()}
}

// NoStart returns the start of a project with none placed.
func NoStart() Start { return Start{Zone: ids.ULID{}, Floor: ids.ULID{}, Col: 0, Row: 0} }

// CheckScreen reports a screen size that is not allowed.
func CheckScreen(columns, rows int) error {
	for _, size := range []int{columns, rows} {
		if size < MinScreen || size > MaxScreen || size%2 == 0 {
			return fmt.Errorf("%w: a screen is an odd number of cells from %d to %d across and down",
				ErrSettings, MinScreen, MaxScreen)
		}
	}

	return nil
}

// Validate reports settings that are not allowed: a screen of the wrong size,
// or a start in a zone without a floor.
func (s Settings) Validate() error {
	if err := CheckScreen(s.Columns, s.Rows); err != nil {
		return err
	}
	if !s.Start.IsZero() && s.Start.Floor.IsZero() {
		return fmt.Errorf("%w: the player start names no floor", ErrSettings)
	}

	return nil
}

// Encode returns the settings' saved data.
func (s Settings) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("encode game settings: %w", err)
	}

	return data, nil
}

// DecodeSettings reads saved game settings in the current format.
func DecodeSettings(data json.RawMessage) (Settings, error) {
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, fmt.Errorf("%w: %w", ErrData, err)
	}
	if err := settings.Validate(); err != nil {
		return settings, fmt.Errorf("%w: %w", ErrData, err)
	}

	return settings, nil
}

// SettingsFormat returns the game settings format registration.
func SettingsFormat() content.Format {
	return content.Format{Name: SettingsFormatName, Current: SettingsVersion, Migrations: nil}
}
