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

// Color returns the colour a reference names, if its palette and shade
// exist, found through a shadow record if the shade has moved. A loose colour
// is its own value.
func (p Palettes) Color(ref ColorRef) (color.NRGBA, bool) {
	none := color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	if ref.IsLoose() {
		return ref.Value.NRGBA(ref.Opacity), true
	}
	palette, ok := p[ref.Palette]
	if !ok || palette == nil {
		return none, false
	}
	shade, _, ok := palette.Resolve(ref.Ramp, ref.Shade)
	if !ok {
		return none, false
	}

	return shade.Color.NRGBA(shade.Opacity), true
}

// Moved reports whether a reference finds its shade only through a shadow
// record, because the shade has moved to another ramp since it was painted.
func (p Palettes) Moved(ref ColorRef) bool {
	palette, ok := p[ref.Palette]
	if !ok || palette == nil || ref.IsLoose() || ref.IsClear() {
		return false
	}
	_, moved, found := palette.Resolve(ref.Ramp, ref.Shade)

	return found && moved
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
