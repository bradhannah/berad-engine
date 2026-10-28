package mapview

import (
	"fmt"
	"image"
	"maps"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/tileart"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// spriteArt is what the Map draws in place of a stand-in's flat colour: the
// tile of the sprite that shows it, chosen under Shows in the Pixel editor.
// Each picture is kept until its tile or a palette changes, so a stroke shows
// as it is drawn.
type spriteArt struct {
	pictures map[string]*tileart.Picture
	// shown is this frame's art by stand-in: only tiles of the project's
	// tile size, which every zoom level scales uniformly. A stand-in whose
	// tile is another size keeps its flat colour.
	shown map[string]*ebiten.Image
}

func newSpriteArt() *spriteArt {
	return &spriteArt{pictures: map[string]*tileart.Picture{}, shown: map[string]*ebiten.Image{}}
}

// standInSprites returns which sprite shows each stand-in.
func standInSprites(docs frame.Docs) map[string]ids.ULID {
	record := map[string]ids.ULID{}
	docs.View(art.StandInsID(), func(document client.Document) {
		if standIns, ok := document.Model.(*artplugin.StandInsDocument); ok {
			for _, standIn := range standIns.Sprites.StandIns() {
				record[standIn], _ = standIns.Sprites.Sprite(standIn)
			}
		}
	})

	return record
}

// spriteTile returns a sprite's tile, once the sprite is loaded.
func spriteTile(docs frame.Docs, sprite ids.ULID) (ids.ULID, bool) {
	var tile ids.ULID
	docs.View(sprite, func(document client.Document) {
		if spriteDoc, ok := document.Model.(*artplugin.SpriteDocument); ok {
			tile = spriteDoc.Sprite.Tile
		}
	})

	return tile, !tile.IsZero()
}

// projectTileSize returns the project's tile size, or the default until the
// art settings load.
func projectTileSize(docs frame.Docs) int {
	size := art.DefaultTileSize
	docs.View(art.SettingsID(), func(document client.Document) {
		if settings, ok := document.Model.(*artplugin.SettingsDocument); ok {
			size = settings.Settings.TileSize
		}
	})

	return size
}

// refresh redraws the art of any stand-in whose tile or palette changed. It
// runs before the floors are drawn, since a document is viewed one at a time.
func (a *spriteArt) refresh(docs frame.Docs, tileSize int) {
	clear(a.shown)
	sprites := standInSprites(docs)
	for standIn, sprite := range sprites {
		tileID, ok := spriteTile(docs, sprite)
		if !ok {
			continue
		}
		picture, made := a.pictures[standIn]
		if !made {
			picture = tileart.NewPicture()
			a.pictures[standIn] = picture
		}
		if picture.Refresh(docs, tileID) && picture.Size() == tileSize {
			a.shown[standIn] = picture.Image()
		}
	}
	for _, standIn := range slices.Collect(maps.Keys(a.pictures)) {
		if _, shown := sprites[standIn]; !shown {
			a.pictures[standIn].Deallocate()
			delete(a.pictures, standIn)
		}
	}
}

// image returns the art that shows a stand-in this frame, if a sprite does.
func (a *spriteArt) image(standIn string) (*ebiten.Image, bool) {
	picture, ok := a.shown[standIn]

	return picture, ok
}

// artParts is how many parts "art:" names after it: a stand-in, a pixel and
// a colour.
const artParts = 3

// hasPixel answers "art:<stand-in>:<col>,<row>:#RRGGBB" for input scripts:
// whether the art drawn for a stand-in has that colour at one of its pixels,
// whatever its opacity, or "clear".
func (a *spriteArt) hasPixel(value string) bool {
	parts := strings.Split(value, ":")
	if len(parts) != artParts {
		return false
	}
	picture, ok := a.pictures[parts[0]]
	if _, shown := a.shown[parts[0]]; !ok || !shown {
		return false
	}
	var col, row int
	if _, err := fmt.Sscanf(parts[1], "%d,%d", &col, &row); err != nil {
		return false
	}
	pixel, inside := picture.Pixel(col, row)
	if !inside {
		return false
	}
	if pixel.A == 0 {
		return parts[2] == "clear"
	}

	return strings.EqualFold(parts[2], fmt.Sprintf("#%02X%02X%02X", pixel.R, pixel.G, pixel.B))
}

// syncArt subscribes to the art the Map draws: the art settings for the tile
// size, the record of which sprite shows each stand-in, and each of those
// sprites with its tile and palettes. A sprite deleted since it was chosen is
// left alone, and its stand-in keeps its flat colour.
func (s *State) syncArt(ctx *frame.Context) {
	s.subscribe(art.SettingsID(), ctx)
	s.subscribe(art.StandInsID(), ctx)
	var index *project.Index
	ctx.Docs.View(project.IndexID(), func(document client.Document) {
		if found, ok := document.Model.(*project.Index); ok {
			index = found
		}
	})
	if index == nil {
		return
	}
	exists := func(id ids.ULID) bool {
		_, found := index.Entry(id)

		return found
	}
	for _, sprite := range standInSprites(ctx.Docs) {
		if !exists(sprite) {
			continue
		}
		s.subscribe(sprite, ctx)
		tileID, ok := spriteTile(ctx.Docs, sprite)
		if !ok || !exists(tileID) {
			continue
		}
		s.subscribe(tileID, ctx)
		for _, paletteID := range tileart.Palettes(ctx.Docs, tileID) {
			if exists(paletteID) {
				s.subscribe(paletteID, ctx)
			}
		}
	}
}

// drawSprite draws a stand-in's art to fill a cell: magnified by a whole
// number without smoothing, or reduced smoothly. alpha fades it, as for the
// floor below.
func drawSprite(area, picture *ebiten.Image, rect image.Rectangle, alpha uint8) {
	var place ebiten.DrawImageOptions
	scale := float64(rect.Dx()) / float64(picture.Bounds().Dx())
	place.GeoM.Scale(scale, scale)
	place.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	if scale < 1 {
		place.Filter = ebiten.FilterLinear
	}
	place.ColorScale.ScaleAlpha(float32(alpha) / fullAlpha)
	area.DrawImage(picture, &place)
}

// hasArt answers input scripts waiting for the Map's art:
// "sprite:<stand-in>" once a sprite's tile is drawn in its place, and
// "art:<stand-in>:<col>,<row>:<colour>" once that tile has the colour at a
// pixel.
func (s *State) hasArt(state string) bool {
	kind, value, _ := strings.Cut(state, ":")
	if kind == "sprite" {
		_, shown := s.sprites.image(value)

		return shown
	}

	return s.sprites.hasPixel(value)
}
