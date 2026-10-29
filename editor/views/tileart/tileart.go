// Package tileart draws tiles from the art documents a window holds, for the
// views that show them: the Pixel editor's canvas and the Map's cells
// (increment 11).
package tileart

import (
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Picture is a tile drawn at one image pixel per tile pixel, kept until the
// tile or one of its palettes changes.
type Picture struct {
	key     pictureKey
	drawn   *image.NRGBA
	image   *ebiten.Image
	missing int
}

// pictureKey says what a picture was drawn from: the tile and its revision,
// and the sum of its palettes' revisions, which grows with any change.
type pictureKey struct {
	tile      ids.ULID
	revision  uint64
	palettes  uint64
	available int
}

// NewPicture returns a picture with nothing drawn yet.
func NewPicture() *Picture {
	return &Picture{
		key:   pictureKey{tile: ids.ULID{}, revision: 0, palettes: 0, available: 0},
		drawn: nil, image: nil, missing: 0,
	}
}

// Palettes returns the palettes a tile is painted from: Default, and every
// other palette its colours name. A loose colour names none.
func Palettes(docs frame.Docs, tileID ids.ULID) []ids.ULID {
	palettes := []ids.ULID{art.DefaultPaletteID()}
	docs.View(tileID, func(document client.Document) {
		if tileDoc, isTile := document.Model.(*artplugin.TileDocument); isTile {
			for _, ref := range tileDoc.Tile.Colors() {
				if !ref.IsLoose() && !slices.Contains(palettes, ref.Palette) {
					palettes = append(palettes, ref.Palette)
				}
			}
		}
	})

	return palettes
}

// Refresh draws the picture again when the tile or one of its palettes has
// changed since it was last drawn, and reports whether there is a tile to
// show. It holds no document past the call.
func (p *Picture) Refresh(docs frame.Docs, tileID ids.ULID) bool {
	palettes := Palettes(docs, tileID)
	key := pictureKey{tile: tileID, revision: 0, palettes: 0, available: 0}
	for _, paletteID := range palettes {
		docs.View(paletteID, func(document client.Document) {
			key.palettes += document.Revision
			key.available++
		})
	}
	docs.View(tileID, func(document client.Document) { key.revision = document.Revision })
	if key == p.key && p.image != nil {
		return true
	}
	colors := art.Palettes{}
	for _, paletteID := range palettes {
		docs.View(paletteID, func(document client.Document) {
			if paletteDoc, ok := document.Model.(*artplugin.PaletteDocument); ok {
				colors[paletteID] = paletteDoc.Palette.Clone()
			}
		})
	}
	loaded := docs.View(tileID, func(document client.Document) {
		tileDoc, isTile := document.Model.(*artplugin.TileDocument)
		if !isTile {
			return
		}
		drawn, missing := tileDoc.Tile.Image(colors)
		if p.image == nil || p.image.Bounds().Size() != drawn.Bounds().Size() {
			if p.image != nil {
				p.image.Deallocate()
			}
			p.image = ebiten.NewImage(drawn.Bounds().Dx(), drawn.Bounds().Dy())
		}
		p.image.WritePixels(Premultiply(drawn.Pix))
		p.key, p.drawn, p.missing = key, drawn, missing
	})

	return loaded && p.image != nil
}

// Image returns the picture, drawn at one image pixel per tile pixel, or nil
// before it is first drawn.
func (p *Picture) Image() *ebiten.Image { return p.image }

// Size returns the drawn tile's size in pixels, or zero before it is drawn.
func (p *Picture) Size() int {
	if p.drawn == nil {
		return 0
	}

	return p.drawn.Bounds().Dx()
}

// Missing returns how many of the tile's pixels name a colour that cannot be
// found, drawn clear.
func (p *Picture) Missing() int { return p.missing }

// Pixel returns the colour drawn at one of the tile's pixels, with straight
// alpha, as a palette names it.
func (p *Picture) Pixel(col, row int) (color.NRGBA, bool) {
	if p.drawn == nil || !image.Pt(col, row).In(p.drawn.Bounds()) {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0}, false
	}

	return p.drawn.NRGBAAt(col, row), true
}

// Deallocate frees the picture's image.
func (p *Picture) Deallocate() {
	if p.image != nil {
		p.image.Deallocate()
	}
	*p = *NewPicture()
}

// Premultiply returns colours with straight alpha, as a tile draws them,
// with each channel scaled by its alpha, as Ebitengine takes them.
func Premultiply(straight []byte) []byte {
	const channels, alpha = 4, 3
	scaled := make([]byte, len(straight))
	for start := 0; start+alpha < len(straight); start += channels {
		opacity := uint32(straight[start+alpha])
		for channel := range alpha {
			value := uint32(straight[start+channel]) * opacity / math.MaxUint8
			scaled[start+channel] = byte(value) //nolint:gosec // A channel times an alpha, over 255, is at most 255.
		}
		scaled[start+alpha] = straight[start+alpha]
	}

	return scaled
}
