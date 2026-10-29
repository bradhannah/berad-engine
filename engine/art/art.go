// Package art is a project's art: the palettes it is painted with, the tiles
// drawn in the pixel editor, the sprites that definitions show, the game's
// tile size, and which sprite shows each stand-in until definitions arrive.
//
// A tile is one cell-sized piece of art and a sprite is one or more tiles.
// A pixel names its colour, a shade of a palette's ramp, by permanent IDs, so
// renaming a ramp or changing a shade never breaks a tile, and changing a
// shade changes every tile that uses it. Every project holds Default, the
// engine's own palette, which never changes; a game adds palettes of its own.
// Games draw tiles with this package too, so the editor and the game always
// agree.
package art

import (
	"encoding/json"
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

// Format versions this package writes. Version 2 of palettes added labels
// and moved colours' shadow records, and of tiles loose colours; both read
// version 1 unchanged.
const (
	PaletteVersion  = 2
	TileVersion     = 2
	SpriteVersion   = 1
	SettingsVersion = 1
	StandInsVersion = 1
)

// Names of the documents every project holds.
const (
	DefaultPaletteName = "Default"
	SettingsName       = "Art settings"
	StandInsName       = "Stand-in sprites"
)

// Fixed IDs. Their time part is zero, so no generated ID ever equals them.
// Default's ramps and shades are numbered within its own range.
const (
	settingsID       = "00000000000000000000000011"
	standInsID       = "00000000000000000000000012"
	defaultPaletteID = "00000000000000000000000013"
	defaultRampIDs   = "0000000000000000000002%02d00"
	defaultShadeIDs  = "0000000000000000000002%02d%02d"
)

// DefaultPaletteID is the ID of Default, the engine's own palette, which every
// project holds and nothing changes.
func DefaultPaletteID() ids.ULID { return ids.MustParse(defaultPaletteID) }

// SettingsID is the ID of a project's art settings.
func SettingsID() ids.ULID { return ids.MustParse(settingsID) }

// StandInsID is the ID of the record of which sprite shows each stand-in.
func StandInsID() ids.ULID { return ids.MustParse(standInsID) }

// DefaultRampID is the ID of one of Default's ramps, by its place from 1.
func DefaultRampID(place int) ids.ULID { return ids.MustParse(fmt.Sprintf(defaultRampIDs, place)) }

// DefaultShadeID is the ID of one of Default's shades, by its ramp's place
// and its number, both from 1.
func DefaultShadeID(place, number int) ids.ULID {
	return ids.MustParse(fmt.Sprintf(defaultShadeIDs, place, number))
}

// MaxNameLength bounds a ramp's name, in bytes.
const MaxNameLength = 200

// MaxLabelLength bounds a colour's label, in characters.
const MaxLabelLength = 15

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

// ErrLabel reports a label that is untrimmed, too long or another colour's in
// the same palette.
var ErrLabel = errors.New("invalid label")

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
	return content.Format{Name: PaletteFormatName, Current: PaletteVersion, Migrations: unchanged()}
}

// TileFormat returns the tile format registration.
func TileFormat() content.Format {
	return content.Format{Name: TileFormatName, Current: TileVersion, Migrations: unchanged()}
}

// unchanged upgrades version 1 to 2 as it is: version 2 only added fields
// that version 1 leaves out.
func unchanged() map[int]content.Migration {
	return map[int]content.Migration{1: func(data json.RawMessage) (json.RawMessage, error) { return data, nil }}
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
