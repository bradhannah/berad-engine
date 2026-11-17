package player

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Art is the art a game shows: the tile each stand-in, the player and each
// NPC shows, drawn once when the game starts, so a change made to a tile
// while the game runs changes nothing until it starts again.
type Art struct {
	shown map[string]*ebiten.Image
	drawn []*ebiten.Image
}

// NewArt returns art with nothing drawn yet.
func NewArt() *Art {
	return &Art{shown: map[string]*ebiten.Image{}, drawn: nil}
}

// Draw draws the tiles a game shows again, from a project's documents: each
// tile under the key it shows, as gameload gathers them. Only a tile of the
// project's tile size is kept; anything else shows as its stand-in does
// without art.
func (a *Art) Draw(source gameload.Source, tiles map[string]ids.ULID, tileSize int) {
	a.Deallocate()
	for key, tileID := range tiles {
		colors := art.Palettes{}
		for _, paletteID := range gameload.TilePalettes(source, tileID) {
			source.View(paletteID, func(document gameload.Document) {
				if palette, ok := document.Value.(*art.Palette); ok {
					colors[paletteID] = palette.Clone()
				}
			})
		}
		source.View(tileID, func(document gameload.Document) {
			tile, ok := document.Value.(*art.Tile)
			if !ok || tile.Size() != tileSize {
				return
			}
			drawn, _ := tile.Image(colors)
			picture := ebiten.NewImage(drawn.Bounds().Dx(), drawn.Bounds().Dy())
			picture.WritePixels(Premultiply(drawn.Pix))
			a.shown[key] = picture
			a.drawn = append(a.drawn, picture)
		})
	}
}

// Image returns the art that shows a key, if a tile does.
func (a *Art) Image(key string) (*ebiten.Image, bool) {
	picture, ok := a.shown[key]

	return picture, ok
}

// Deallocate frees the art's images.
func (a *Art) Deallocate() {
	for _, picture := range a.drawn {
		picture.Deallocate()
	}
	clear(a.shown)
	a.drawn = nil
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
