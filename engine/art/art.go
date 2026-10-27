// Package art is a project's art: the palettes it is painted with, the tiles
// drawn in the pixel editor, the sprites that definitions show, the game's
// tile size, and which sprite shows each stand-in until definitions arrive.
//
// A tile is one cell-sized piece of art and a sprite is one or more tiles.
// A pixel names its colour, a shade of a palette's ramp, by permanent IDs, so
// renaming a ramp or changing a shade never breaks a tile, and changing a
// shade changes every tile that uses it. Games draw tiles with this package
// too, so the editor and the game always agree.
package art

import (
	"errors"

	"github.com/bradhannah/berad-engine/engine/content"
)

// Format names inside saved envelopes.
const (
	PaletteFormatName  = "palette"
	TileFormatName     = "tile"
	SpriteFormatName   = "sprite"
	SettingsFormatName = "art-settings"
	StandInsFormatName = "standin-sprites"
)

// Format versions this package writes.
const (
	PaletteVersion  = 1
	TileVersion     = 1
	SpriteVersion   = 1
	SettingsVersion = 1
	StandInsVersion = 1
)

// MaxNameLength bounds a ramp's name, in bytes.
const MaxNameLength = 200

// ErrData reports saved art data that is not valid.
var ErrData = errors.New("invalid art data")

// ErrName reports an empty, untrimmed, overlong or duplicate ramp name.
var ErrName = errors.New("invalid ramp name")

// ErrNoRamp reports a ramp ID the palette does not hold.
var ErrNoRamp = errors.New("no such ramp")

// ErrNoShade reports a shade ID the palette does not hold.
var ErrNoShade = errors.New("no such shade")

// ErrShade reports an invalid shade: a duplicate ID, or a colour or opacity
// out of range.
var ErrShade = errors.New("invalid shade")

// ErrSize reports a tile size outside the supported range.
var ErrSize = errors.New("invalid tile size")

// ErrBounds reports a pixel outside the tile.
var ErrBounds = errors.New("pixel outside tile")

// ErrTooManyColors reports a tile that would use more colours than it can save.
var ErrTooManyColors = errors.New("too many colours in one tile")

// ErrStandIn reports a stand-in that cannot be shown by a sprite: an unknown
// one, or a wall, whose family needs a sprite for each neighbour arrangement.
var ErrStandIn = errors.New("stand-in cannot show a sprite")

// PaletteFormat returns the palette format registration.
func PaletteFormat() content.Format {
	return content.Format{Name: PaletteFormatName, Current: PaletteVersion, Migrations: nil}
}

// TileFormat returns the tile format registration.
func TileFormat() content.Format {
	return content.Format{Name: TileFormatName, Current: TileVersion, Migrations: nil}
}

// SpriteFormat returns the sprite format registration.
func SpriteFormat() content.Format {
	return content.Format{Name: SpriteFormatName, Current: SpriteVersion, Migrations: nil}
}

// SettingsFormat returns the art settings format registration.
func SettingsFormat() content.Format {
	return content.Format{Name: SettingsFormatName, Current: SettingsVersion, Migrations: nil}
}

// StandInsFormat returns the stand-in sprites format registration.
func StandInsFormat() content.Format {
	return content.Format{Name: StandInsFormatName, Current: StandInsVersion, Migrations: nil}
}
