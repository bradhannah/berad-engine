// Package tilefloor is a minimal tile floor: a rectangular grid of named tiles.
//
// It is the first document type, used to prove the editor architecture. Richer
// world structure (zones, areas, anchors, placements) arrives as games need it.
package tilefloor

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/bradhannah/berad-engine/engine/content"
)

// FormatName identifies tile floors inside saved envelopes.
const FormatName = "tilefloor"

// FormatVersion is the tile floor format version this package writes.
const FormatVersion = 1

// MaxSide bounds a floor's width and height.
const MaxSide = 1024

// ErrSize reports invalid floor dimensions.
var ErrSize = errors.New("invalid floor size")

// ErrBounds reports coordinates outside the floor.
var ErrBounds = errors.New("position outside floor")

// ErrData reports saved floor data that does not match its dimensions.
var ErrData = errors.New("invalid floor data")

// Floor is a grid of tile names in row-major order. The empty name is no tile.
type Floor struct {
	width  int
	height int
	tiles  []string
}

type wireFloor struct {
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Tiles  []string `json:"tiles"`
}

// New returns a floor of the given size filled with one tile name.
func New(width, height int, fill string) (*Floor, error) {
	if width < 1 || height < 1 || width > MaxSide || height > MaxSide {
		return nil, fmt.Errorf("%w: %dx%d", ErrSize, width, height)
	}
	tiles := make([]string, width*height)
	for index := range tiles {
		tiles[index] = fill
	}

	return &Floor{width: width, height: height, tiles: tiles}, nil
}

// Width returns the number of columns.
func (f *Floor) Width() int { return f.width }

// Height returns the number of rows.
func (f *Floor) Height() int { return f.height }

// At returns the tile name at a position.
func (f *Floor) At(col, row int) (string, error) {
	index, err := f.index(col, row)
	if err != nil {
		return "", err
	}

	return f.tiles[index], nil
}

// Set changes the tile at a position and returns the previous name.
func (f *Floor) Set(col, row int, tile string) (string, error) {
	index, err := f.index(col, row)
	if err != nil {
		return "", err
	}
	previous := f.tiles[index]
	f.tiles[index] = tile

	return previous, nil
}

// Clone returns an independent copy.
func (f *Floor) Clone() *Floor {
	return &Floor{width: f.width, height: f.height, tiles: slices.Clone(f.tiles)}
}

// Equal reports whether two floors have the same size and tiles.
func (f *Floor) Equal(other *Floor) bool {
	return f.width == other.width && f.height == other.height && slices.Equal(f.tiles, other.tiles)
}

// Encode returns the floor's saved data.
func (f *Floor) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(wireFloor{Width: f.width, Height: f.height, Tiles: f.tiles})
	if err != nil {
		return nil, fmt.Errorf("encode floor: %w", err)
	}

	return data, nil
}

// Decode reads saved floor data.
func Decode(data json.RawMessage) (*Floor, error) {
	var wire wireFloor
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	floor, err := New(wire.Width, wire.Height, "")
	if err != nil {
		return nil, err
	}
	if len(wire.Tiles) != wire.Width*wire.Height {
		return nil, fmt.Errorf("%w: %d tiles for %dx%d", ErrData, len(wire.Tiles), wire.Width, wire.Height)
	}
	copy(floor.tiles, wire.Tiles)

	return floor, nil
}

// Format returns the tile floor format registration.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: nil}
}

func (f *Floor) index(col, row int) (int, error) {
	if col < 0 || row < 0 || col >= f.width || row >= f.height {
		return 0, fmt.Errorf("%w: (%d, %d) on %dx%d", ErrBounds, col, row, f.width, f.height)
	}

	return row*f.width + col, nil
}
