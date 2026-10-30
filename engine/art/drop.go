package art

import "github.com/bradhannah/berad-engine/engine/ids"

// Dropped returns what each of a tile's colours from a palette becomes when
// the palette is removed from the tile, so every pixel keeps its colour: the
// first of Default's shades with exactly the same colour and opacity, or else
// a loose colour of its own value. A colour the palette no longer holds was
// already drawn clear, and becomes clear.
func Dropped(tile *Tile, paletteID ids.ULID, palette, defaults *Palette) map[ColorRef]ColorRef {
	changes := map[ColorRef]ColorRef{}
	for _, ref := range tile.Colors() {
		if ref.Palette != paletteID || ref.IsLoose() {
			continue
		}
		shade, _, found := palette.Resolve(ref.Ramp, ref.Shade)
		if !found {
			changes[ref] = ColorRef{Palette: ids.ULID{}, Ramp: ids.ULID{}, Shade: ids.ULID{}, Value: 0, Opacity: 0}

			continue
		}
		changes[ref] = match(shade, defaults)
	}

	return changes
}

// match finds a shade's exact match in Default, or makes it a loose colour.
func match(shade Shade, defaults *Palette) ColorRef {
	for _, ramp := range defaults.ramps {
		for _, candidate := range ramp.Shades {
			if candidate.Color == shade.Color && candidate.Opacity == shade.Opacity {
				return Named(DefaultPaletteID(), ramp.ID, candidate.ID)
			}
		}
	}

	return Loose(shade.Color, shade.Opacity)
}
