package art

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// DefaultTileSize is the tile size a project starts with.
const DefaultTileSize = 32

// Sprite is what a definition shows. For now a sprite is one frame of one
// tile, one cell in size; bigger and animated sprites come later.
type Sprite struct {
	Tile ids.ULID `json:"tile"`
}

// Encode returns the sprite's saved data.
func (s Sprite) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("encode sprite: %w", err)
	}

	return data, nil
}

// DecodeSprite reads saved sprite data in the current format.
func DecodeSprite(data json.RawMessage) (Sprite, error) {
	var sprite Sprite
	if err := json.Unmarshal(data, &sprite); err != nil {
		return sprite, fmt.Errorf("%w: %w", ErrData, err)
	}
	if sprite.Tile.IsZero() {
		return sprite, fmt.Errorf("%w: sprite has no tile", ErrData)
	}

	return sprite, nil
}

// Settings are a game's art settings: for now, its tile size.
type Settings struct {
	TileSize int `json:"tileSize"`
}

// DefaultSettings returns the settings a project starts with.
func DefaultSettings() Settings { return Settings{TileSize: DefaultTileSize} }

// Validate reports settings out of range.
func (s Settings) Validate() error {
	if s.TileSize < MinTileSize || s.TileSize > MaxTileSize {
		return fmt.Errorf("%w: %d", ErrSize, s.TileSize)
	}

	return nil
}

// Encode returns the settings' saved data.
func (s Settings) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("encode art settings: %w", err)
	}

	return data, nil
}

// DecodeSettings reads saved art settings in the current format.
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

// CanShowSprite reports whether a stand-in can be shown by a sprite: any
// built-in stand-in but a wall, whose family needs a sprite for each
// neighbour arrangement.
func CanShowSprite(standIn string) bool {
	definition, ok := tiles.Lookup(standIn)

	return ok && !definition.IsWall()
}

// StandInSprites records which sprite shows each stand-in, until definitions
// can be edited and carry their own sprites.
type StandInSprites struct {
	sprites map[string]ids.ULID
}

type wireStandIns struct {
	Sprites map[string]ids.ULID `json:"sprites"`
}

// NewStandInSprites returns a record with no stand-in shown by a sprite.
func NewStandInSprites() *StandInSprites {
	return &StandInSprites{sprites: map[string]ids.ULID{}}
}

// Sprite returns the sprite that shows a stand-in.
func (s *StandInSprites) Sprite(standIn string) (ids.ULID, bool) {
	sprite, ok := s.sprites[standIn]

	return sprite, ok
}

// StandIns returns the stand-ins shown by a sprite, in sorted order.
func (s *StandInSprites) StandIns() []string {
	return slices.Sorted(maps.Keys(s.sprites))
}

// Set chooses the sprite that shows a stand-in, or with a zero sprite goes
// back to its flat colour. It returns the sprite that showed it before, or
// zero.
func (s *StandInSprites) Set(standIn string, sprite ids.ULID) (ids.ULID, error) {
	if !CanShowSprite(standIn) {
		var none ids.ULID

		return none, fmt.Errorf("%w: %q", ErrStandIn, standIn)
	}
	previous := s.sprites[standIn]
	if sprite.IsZero() {
		delete(s.sprites, standIn)
	} else {
		s.sprites[standIn] = sprite
	}

	return previous, nil
}

// Clone returns an independent copy.
func (s *StandInSprites) Clone() *StandInSprites {
	return &StandInSprites{sprites: maps.Clone(s.sprites)}
}

// Equal reports whether two records show the same stand-ins by the same
// sprites.
func (s *StandInSprites) Equal(other *StandInSprites) bool {
	return maps.Equal(s.sprites, other.sprites)
}

// Encode returns the record's saved data, stand-ins in sorted order.
func (s *StandInSprites) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(wireStandIns{Sprites: s.sprites})
	if err != nil {
		return nil, fmt.Errorf("encode stand-in sprites: %w", err)
	}

	return data, nil
}

// DecodeStandInSprites reads saved stand-in sprites in the current format.
func DecodeStandInSprites(data json.RawMessage) (*StandInSprites, error) {
	var wire wireStandIns
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	record := NewStandInSprites()
	for _, standIn := range slices.Sorted(maps.Keys(wire.Sprites)) {
		sprite := wire.Sprites[standIn]
		if sprite.IsZero() {
			return nil, fmt.Errorf("%w: %q has no sprite", ErrData, standIn)
		}
		if _, err := record.Set(standIn, sprite); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrData, err)
		}
	}

	return record, nil
}
