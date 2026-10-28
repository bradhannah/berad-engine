package art

import (
	"image"
	"image/color"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Resolver gives the colour a reference names, with its opacity as alpha.
type Resolver interface {
	Color(ref ColorRef) (color.NRGBA, bool)
}

// Palettes resolves colours from palettes by their document IDs.
type Palettes map[ids.ULID]*Palette

// Color returns the colour a reference names, if its palette, ramp and
// shade exist.
func (p Palettes) Color(ref ColorRef) (color.NRGBA, bool) {
	none := color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	palette, ok := p[ref.Palette]
	if !ok || palette == nil {
		return none, false
	}
	shade, _, ok := palette.Lookup(ref.Ramp, ref.Shade)
	if !ok {
		return none, false
	}

	return shade.Color.NRGBA(shade.Opacity), true
}

// Image draws the tile at one image pixel per tile pixel. A see-through
// shade keeps its opacity as alpha, so the image composites over whatever it
// is drawn on; clear pixels are fully transparent. A pixel whose colour cannot
// be found is drawn clear too, and Image reports how many there were.
func (t *Tile) Image(colors Resolver) (*image.NRGBA, int) {
	picture := image.NewNRGBA(image.Rect(0, 0, t.size, t.size))
	found := map[ColorRef]color.NRGBA{}
	missing := 0
	for index, ref := range t.pixels {
		if ref.IsClear() {
			continue
		}
		value, known := found[ref]
		if !known {
			resolved, ok := colors.Color(ref)
			if !ok {
				missing++

				continue
			}
			value = resolved
			found[ref] = value
		}
		picture.SetNRGBA(index%t.size, index/t.size, value)
	}

	return picture, missing
}
