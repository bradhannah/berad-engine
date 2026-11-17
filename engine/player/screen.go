// Package player shows a game and lets it be played (increment 62): it
// draws what the engine's game runtime runs, the game's screen of a fixed
// number of cells with the player in the middle, fitted to the room it is
// given. The editor's Play tab and a game's own program both draw through it,
// so a game looks the same in both. It is the part of the engine that uses
// Ebitengine; the game runtime and everything under it do not.
package player

import (
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Drawing, in tile pixels: a character without a sprite is a disc this much
// of a cell across; a small or medium pickup without one fills the middle of
// its cell, leaving these eighths around it, and other things on top one.
const (
	discShare    = 0.36
	ringWidth    = 1.5
	eighths      = 8
	smallPickup  = 3
	mediumPickup = 2
	otherTop     = 1
	half         = 2
	hexToken     = 0x1d2433
	hexPlayer    = 0xffffff
	hexRing      = 0x1f5fbf
	hexDark      = 0x000000
)

// Screen draws a game: its screen at one pixel per tile pixel, placed as
// large as its room allows.
type Screen struct {
	art *Art
	// npcSprites is the sprite that shows each NPC as its details name it,
	// to tell it from one a workflow switched the NPC to.
	npcSprites map[ids.ULID]ids.ULID
	tile       int
	// canvas is the game's screen at one pixel per tile pixel; scale is how
	// many of the room's pixels each of its pixels takes; sharp is the canvas
	// enlarged by a whole number, for a scale that is not one.
	canvas *ebiten.Image
	sharp  *ebiten.Image
	scale  float64
	// origin is where the canvas's top-left corner is drawn in the room.
	origin image.Point
}

// NewScreen returns a screen with no game to draw yet.
func NewScreen() *Screen {
	return &Screen{
		art: NewArt(), npcSprites: map[ids.ULID]ids.ULID{}, tile: 0, canvas: nil, sharp: nil, scale: 1,
		origin: image.Pt(0, 0),
	}
}

// Start readies the screen for a game that is starting: its tile size, and
// its art, drawn now from the project's documents.
func (s *Screen) Start(loaded gameload.Loaded, source gameload.Source) {
	s.tile, s.npcSprites = loaded.TileSize, loaded.NPCSprites
	s.art.Draw(source, loaded.Tiles, loaded.TileSize)
}

// Art is the art the game shows.
func (s *Screen) Art() *Art { return s.art }

// Scale is how many of the room's pixels each of the game's pixels took
// when it was last drawn.
func (s *Screen) Scale() float64 { return s.scale }

// Origin is where the game's top-left corner was last drawn in its room.
func (s *Screen) Origin() image.Point { return s.origin }

// Tile is the game's tile size, in its own pixels.
func (s *Screen) Tile() int { return s.tile }

// Draw draws a game into its room as large as the room allows, keeping its
// shape and centring it; the room around it is left as it was.
func (s *Screen) Draw(room *ebiten.Image, played *game.Game) {
	view := played.View()
	width, height := view.Columns*s.tile, view.Rows*s.tile
	if s.canvas == nil || s.canvas.Bounds().Dx() != width || s.canvas.Bounds().Dy() != height {
		if s.canvas != nil {
			s.canvas.Deallocate()
		}
		s.canvas = ebiten.NewImage(width, height)
	}
	s.canvas.Fill(hex(hexDark))
	shaken := view
	shakeX, shakeY := played.Stage().Offset()
	shaken.Left -= shakeX / float64(s.tile)
	shaken.Top -= shakeY / float64(s.tile)
	s.drawCells(played, shaken)
	s.drawCharacters(played, shaken)
	s.present(room, width, height)
}

// present draws the canvas as large as the room allows, in the room's own
// pixels, keeping its shape and centring it. A whole-number scale is drawn
// as it is; any other is first enlarged by the next whole number with hard
// edges, then shrunk to fit smoothly, so every pixel stays the same size and
// only their edges soften.
func (s *Screen) present(room *ebiten.Image, width, height int) {
	bounds := room.Bounds()
	s.scale = min(float64(bounds.Dx())/float64(width), float64(bounds.Dy())/float64(height))
	source, shrink := s.canvas, s.scale
	if whole := math.Ceil(s.scale); whole != s.scale {
		size := image.Pt(width*int(whole), height*int(whole))
		if s.sharp == nil || s.sharp.Bounds().Size() != size {
			if s.sharp != nil {
				s.sharp.Deallocate()
			}
			s.sharp = ebiten.NewImage(size.X, size.Y)
		}
		var enlarge ebiten.DrawImageOptions
		enlarge.GeoM.Scale(whole, whole)
		s.sharp.DrawImage(s.canvas, &enlarge)
		source, shrink = s.sharp, s.scale/whole
	}
	var place ebiten.DrawImageOptions
	place.GeoM.Scale(shrink, shrink)
	s.origin = image.Pt(int(math.Round((float64(bounds.Dx())-float64(width)*s.scale)/half)),
		int(math.Round((float64(bounds.Dy())-float64(height)*s.scale)/half)))
	place.GeoM.Translate(float64(s.origin.X), float64(s.origin.Y))
	if shrink != 1 {
		place.Filter = ebiten.FilterLinear
	}
	room.DrawImage(source, &place)
}

// CellAt is where a cell's top-left corner falls on the game's screen, in
// its own pixels.
func (s *Screen) CellAt(view game.View, x, y float64) image.Point {
	return image.Pt(int(math.Round((x-view.Left)*float64(s.tile))), int(math.Round((y-view.Top)*float64(s.tile))))
}

// drawCells draws each cell the view covers, and a cell either side for
// a view between cells: its ground, then what stands on it.
func (s *Screen) drawCells(played *game.Game, view game.View) {
	left, top := int(math.Floor(view.Left)), int(math.Floor(view.Top))
	played.Floor(view.Floor, func(shown *tilefloor.Floor) {
		for row := top; row <= top+view.Rows; row++ {
			for col := left; col <= left+view.Columns; col++ {
				if !shown.In(col, row) {
					continue
				}
				corner := s.CellAt(view, float64(col), float64(row))
				rect := image.Rectangle{Min: corner, Max: corner.Add(image.Pt(s.tile, s.tile))}
				ground, _ := shown.Ground(col, row) //nolint:errcheck // The cell is on the floor.
				top, _ := shown.Top(col, row)       //nolint:errcheck // The cell is on the floor.
				s.drawGround(rect, ground)
				if top != "" {
					s.drawTop(rect, top)
				}
			}
		}
	})
}

// drawGround draws a cell's ground: its sprite, or its stand-in's colour.
// Unassigned ground stays dark.
func (s *Screen) drawGround(rect image.Rectangle, ground string) {
	if picture, shown := s.art.Image(ground); shown {
		drawArt(s.canvas, picture, rect.Min)

		return
	}
	if ground != "" {
		fillRect(s.canvas, rect, hex(tiles.ColorOf(ground)))
	}
}

// drawTop draws what stands on a cell: its sprite, or its stand-in's colour,
// a wall or a door filling the cell and pickups smaller.
func (s *Screen) drawTop(rect image.Rectangle, top string) {
	if picture, shown := s.art.Image(top); shown {
		drawArt(s.canvas, picture, rect.Min)

		return
	}
	definition, _ := tiles.Lookup(top)
	inset := 0
	switch definition.Category {
	case tiles.CategoryPickupSmall:
		inset = smallPickup
	case tiles.CategoryPickupMedium:
		inset = mediumPickup
	case tiles.CategoryStairsUp, tiles.CategoryStairsDown:
		inset = otherTop
	case tiles.CategoryGround, tiles.CategoryWall, tiles.CategoryDoor:
	}
	fillRect(s.canvas, rect.Inset(rect.Dx()*inset/eighths), hex(tiles.ColorOf(top)))
}

// figure is a character or the player, drawn in order of the row it stands
// on, so whoever is lower on the screen is drawn in front.
type figure struct {
	key    string
	x, y   float64
	player bool
}

// spriteOf is the key of the art an NPC shows now: the sprite in its
// details, or the one its workflows set its sprite to; "" for none.
func (s *Screen) spriteOf(played *game.Game, npcID ids.ULID) string {
	values := played.Values()
	if values == nil {
		return gameload.NPCKey(npcID)
	}
	declared, known := values.Catalog().Lookup(worldstate.ScopeNPC, npcID, worldstate.SpriteID())
	if !known {
		return gameload.NPCKey(npcID)
	}
	switch now := values.Get(declared, npcID).Ref; {
	case now == s.npcSprites[npcID]:
		return gameload.NPCKey(npcID)
	case now.IsZero():
		return ""
	default:
		return gameload.SpriteKey(now)
	}
}

// drawCharacters draws the NPCs on the player's floor and the player.
func (s *Screen) drawCharacters(played *game.Game, view game.View) {
	player := played.Player()
	playerX, playerY := player.At()
	figures := []figure{{key: art.Player, x: playerX, y: playerY, player: true}}
	for _, character := range played.Characters() {
		if character.Floor == view.Floor {
			figures = append(figures, figure{
				key: s.spriteOf(played, character.NPC), x: character.X, y: character.Y, player: false,
			})
		}
	}
	slices.SortStableFunc(figures, func(a, b figure) int {
		switch {
		case a.y < b.y:
			return -1
		case a.y > b.y:
			return 1
		}

		return 0
	})
	for _, each := range figures {
		corner := s.CellAt(view, each.x, each.y)
		if picture, shown := s.art.Image(each.key); shown {
			drawArt(s.canvas, picture, corner)

			continue
		}
		centreX, centreY := float32(corner.X)+float32(s.tile)/half, float32(corner.Y)+float32(s.tile)/half
		radius := float32(s.tile) * discShare
		if each.player {
			vector.FillCircle(s.canvas, centreX, centreY, radius, hex(hexPlayer), true)
			vector.StrokeCircle(s.canvas, centreX, centreY, radius, ringWidth, hex(hexRing), true)

			continue
		}
		vector.FillCircle(s.canvas, centreX, centreY, radius, hex(hexToken), true)
	}
}

// drawArt draws a tile's art at a canvas point, one pixel for one.
func drawArt(canvas, picture *ebiten.Image, at image.Point) {
	var place ebiten.DrawImageOptions
	place.GeoM.Translate(float64(at.X), float64(at.Y))
	canvas.DrawImage(picture, &place)
}

// fillRect fills a rectangle of whole pixels.
func fillRect(canvas *ebiten.Image, rect image.Rectangle, fill color.Color) {
	vector.FillRect(canvas, float32(rect.Min.X), float32(rect.Min.Y), float32(rect.Dx()), float32(rect.Dy()), fill, false)
}

// hex is an opaque colour from a 0xRRGGBB value.
func hex(rgb uint32) color.RGBA {
	const (
		red, green = 16, 8
		mask       = 0xff
	)

	return color.RGBA{
		R: uint8(rgb >> red & mask), G: uint8(rgb >> green & mask), B: uint8(rgb & mask), A: mask,
	}
}
