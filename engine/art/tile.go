package art

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
)

// Tile size bounds, in pixels. Tiles are square.
const (
	MinTileSize = 8
	MaxTileSize = 128
)

// MaxTileColors bounds how many colours one tile uses, besides clear: a
// saved pixel is two hexadecimal digits, and 00 is clear.
const MaxTileColors = 255

// Tile is one cell-sized piece of art: a square of pixels, each naming its
// colour.
//
// A tile is saved with a table of the colours it uses, in the order they first
// appear, and one line of two hexadecimal digits per pixel for each row: 00 is
// clear, and 01 is the table's first colour. Changing one pixel changes one
// line of the file. Equal tiles always save the same way.
type Tile struct {
	size   int
	pixels []ColorRef
}

type wireTile struct {
	Size   int        `json:"size"`
	Colors []ColorRef `json:"colors"`
	Rows   []string   `json:"rows"`
}

// NewTile returns a clear tile of a size.
func NewTile(size int) (*Tile, error) {
	if size < MinTileSize || size > MaxTileSize {
		return nil, fmt.Errorf("%w: %d", ErrSize, size)
	}

	return &Tile{size: size, pixels: make([]ColorRef, size*size)}, nil
}

// Size returns the tile's width and height, in pixels.
func (t *Tile) Size() int { return t.size }

// At returns a pixel's colour.
func (t *Tile) At(col, row int) (ColorRef, error) {
	index, err := t.index(col, row)
	if err != nil {
		var none ColorRef

		return none, err
	}

	return t.pixels[index], nil
}

// Set paints a pixel, returning its previous colour.
func (t *Tile) Set(col, row int, ref ColorRef) (ColorRef, error) {
	var none ColorRef
	index, err := t.index(col, row)
	if err != nil {
		return none, err
	}
	if !ref.valid() {
		return none, fmt.Errorf("%w: colour %+v", ErrData, ref)
	}
	previous := t.pixels[index]
	if ref != previous && !ref.IsClear() && !slices.Contains(t.pixels, ref) {
		count := len(t.Colors())
		if !previous.IsClear() && t.Count(func(other ColorRef) bool { return other == previous }) == 1 {
			count--
		}
		if count >= MaxTileColors {
			return none, fmt.Errorf("%w: %d", ErrTooManyColors, MaxTileColors)
		}
	}
	t.pixels[index] = ref

	return previous, nil
}

// Colors returns the colours the tile uses, besides clear, in the order they
// first appear, row by row.
func (t *Tile) Colors() []ColorRef {
	var colors []ColorRef
	for _, ref := range t.pixels {
		if !ref.IsClear() && !slices.Contains(colors, ref) {
			colors = append(colors, ref)
		}
	}

	return colors
}

// Count returns how many pixels' colours match.
func (t *Tile) Count(match func(ref ColorRef) bool) int {
	count := 0
	for _, ref := range t.pixels {
		if match(ref) {
			count++
		}
	}

	return count
}

// Uses reports whether any pixel's colour matches.
func (t *Tile) Uses(match func(ref ColorRef) bool) bool {
	return slices.ContainsFunc(t.pixels, match)
}

// Clone returns an independent copy.
func (t *Tile) Clone() *Tile {
	return &Tile{size: t.size, pixels: slices.Clone(t.pixels)}
}

// Equal reports whether two tiles are the same size with the same pixels.
func (t *Tile) Equal(other *Tile) bool {
	return t.size == other.size && slices.Equal(t.pixels, other.pixels)
}

// Encode returns the tile's saved data.
func (t *Tile) Encode() (json.RawMessage, error) {
	colors := t.Colors()
	numbers := make(map[ColorRef]byte, len(colors))
	for index, ref := range colors {
		numbers[ref] = byte(index + 1)
	}
	rows := make([]string, 0, t.size)
	line := make([]byte, t.size)
	for start := 0; start < len(t.pixels); start += t.size {
		for col, ref := range t.pixels[start : start+t.size] {
			line[col] = numbers[ref]
		}
		rows = append(rows, hex.EncodeToString(line))
	}
	if colors == nil {
		colors = []ColorRef{}
	}
	data, err := json.Marshal(wireTile{Size: t.size, Colors: colors, Rows: rows})
	if err != nil {
		return nil, fmt.Errorf("encode tile: %w", err)
	}

	return data, nil
}

// DecodeTile reads saved tile data in the current format.
func DecodeTile(data json.RawMessage) (*Tile, error) {
	var wire wireTile
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	tile, err := NewTile(wire.Size)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	if err := checkTable(wire.Colors); err != nil {
		return nil, err
	}
	if len(wire.Rows) != wire.Size {
		return nil, fmt.Errorf("%w: %d rows for size %d", ErrData, len(wire.Rows), wire.Size)
	}
	for row, text := range wire.Rows {
		line, err := hex.DecodeString(text)
		if err != nil || len(line) != wire.Size {
			return nil, fmt.Errorf("%w: row %d is not %d pixels", ErrData, row, wire.Size)
		}
		for col, number := range line {
			if int(number) > len(wire.Colors) {
				return nil, fmt.Errorf("%w: row %d names colour %d of %d", ErrData, row, number, len(wire.Colors))
			}
			if number > 0 {
				tile.pixels[row*wire.Size+col] = wire.Colors[number-1]
			}
		}
	}

	return tile, nil
}

func checkTable(colors []ColorRef) error {
	if len(colors) > MaxTileColors {
		return fmt.Errorf("%w: %d colours", ErrData, len(colors))
	}
	for index, ref := range colors {
		if ref.IsClear() || !ref.valid() || slices.Contains(colors[:index], ref) {
			return fmt.Errorf("%w: colour %d of the table", ErrData, index+1)
		}
	}

	return nil
}

func (t *Tile) index(col, row int) (int, error) {
	if col < 0 || row < 0 || col >= t.size || row >= t.size {
		return 0, fmt.Errorf("%w: %d,%d in %d", ErrBounds, col, row, t.size)
	}

	return row*t.size + col, nil
}
