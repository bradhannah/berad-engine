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
	"fmt"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Format names inside saved envelopes. A document's folder is its format's
// name with an s.
const (
	PaletteFormatName  = "palette"
	TileFormatName     = "tile"
	SpriteFormatName   = "sprite"
	SettingsFormatName = "art-setting"
	StandInsFormatName = "standin-sprite"
)

// Format versions this package writes.
const (
	PaletteVersion  = 1
	TileVersion     = 1
	SpriteVersion   = 1
	SettingsVersion = 1
	StandInsVersion = 1
)

// Names of the documents every project holds.
const (
	SharedPaletteName = "Shared"
	SettingsName      = "Art settings"
	StandInsName      = "Stand-in sprites"
)

// Fixed IDs. Their time part is zero, so no generated ID ever equals them.
const (
	sharedPaletteID = "00000000000000000000000010"
	settingsID      = "00000000000000000000000011"
	standInsID      = "00000000000000000000000012"
	starterRampID   = "00000000000000000000000020"
	starterShades   = "000000000000000000000000%02d"
	starterFirst    = 20
)

// SharedPaletteID is the ID of the palette every project holds, which its art
// starts from.
func SharedPaletteID() ids.ULID { return ids.MustParse(sharedPaletteID) }

// SettingsID is the ID of a project's art settings.
func SettingsID() ids.ULID { return ids.MustParse(settingsID) }

// StandInsID is the ID of the record of which sprite shows each stand-in.
func StandInsID() ids.ULID { return ids.MustParse(standInsID) }

// StarterRampID is the ID of the shared palette's starter ramp, "grey".
func StarterRampID() ids.ULID { return ids.MustParse(starterRampID) }

// StarterShadeID is the ID of one shade of the starter ramp, by its number
// from 1 to 5.
func StarterShadeID(number int) ids.ULID {
	return ids.MustParse(fmt.Sprintf(starterShades, starterFirst+number))
}

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
