package playview

import (
	"image"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/tileart"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
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
	hexToken     = 0x1d2433
	hexPlayer    = 0xffffff
	hexRing      = 0x1f5fbf
	hexDark      = 0x000000
	hexMessage   = 0xd8dce3
)

// artSet is the art a launch draws: the tile each stand-in, the player and
// each NPC shows, drawn once when the game launches, so edits made while it
// runs change nothing until it launches again.
type artSet struct {
	want     map[string]ids.ULID
	pictures map[string]*tileart.Picture
	shown    map[string]*ebiten.Image
}

func newArtSet() *artSet {
	return &artSet{want: map[string]ids.ULID{}, pictures: map[string]*tileart.Picture{}, shown: map[string]*ebiten.Image{}}
}

// draw draws the art the launch wants, keeping only tiles of the project's
// tile size.
func (a *artSet) draw(docs frame.Docs, tile int) {
	for key, picture := range a.pictures {
		picture.Deallocate()
		delete(a.pictures, key)
	}
	clear(a.shown)
	for key, tileID := range a.want {
		picture := tileart.NewPicture()
		a.pictures[key] = picture
		if picture.Refresh(docs, tileID) && picture.Size() == tile {
			a.shown[key] = picture.Image()
		}
	}
}

// image returns the art that shows a key, if a sprite does.
func (a *artSet) image(key string) (*ebiten.Image, bool) {
	picture, ok := a.shown[key]

	return picture, ok
}

// Draw draws the game's screen, filling the tab while keeping its shape, or
// why there is no game yet.
func (s *State) Draw(screen *ebiten.Image, _ frame.Docs) {
	screen.Fill(ui.Hex(hexDark))
	if s.game == nil {
		message := "Loading the game…"
		if s.problem != "" {
			message = s.problem
		}
		bounds := screen.Bounds()
		width := s.theme.TextWidth(message)
		s.theme.DrawText(screen, message, (bounds.Dx()-width)/ui.Half, bounds.Dy()/ui.Half, ui.Hex(hexMessage))

		return
	}
	view := s.game.View()
	width, height := view.Columns*s.tile, view.Rows*s.tile
	if s.canvas == nil || s.canvas.Bounds().Dx() != width || s.canvas.Bounds().Dy() != height {
		if s.canvas != nil {
			s.canvas.Deallocate()
		}
		s.canvas = ebiten.NewImage(width, height)
	}
	s.canvas.Fill(ui.Hex(hexDark))
	s.drawCells(view)
	s.drawCharacters(view)
	s.present(screen, width, height)
}

// present draws the canvas as large as the tab allows, keeping its shape and
// centring it on black. A whole-number scale is drawn as it is; any other is
// first enlarged by the next whole number with hard edges, then shrunk to fit
// smoothly, so every pixel stays the same size and only their edges soften.
func (s *State) present(screen *ebiten.Image, width, height int) {
	bounds := screen.Bounds()
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
	place.GeoM.Translate(math.Round((float64(bounds.Dx())-float64(width)*s.scale)/ui.Half),
		math.Round((float64(bounds.Dy())-float64(height)*s.scale)/ui.Half))
	if shrink != 1 {
		place.Filter = ebiten.FilterLinear
	}
	screen.DrawImage(source, &place)
}

// cellAt is where a cell's top-left corner falls on the canvas.
func (s *State) cellAt(view game.View, x, y float64) image.Point {
	return image.Pt(int(math.Round((x-view.Left)*float64(s.tile))), int(math.Round((y-view.Top)*float64(s.tile))))
}

// drawCells draws each cell the view covers, and a cell either side for
// a view between cells: its ground, then what stands on it.
func (s *State) drawCells(view game.View) {
	left, top := int(math.Floor(view.Left)), int(math.Floor(view.Top))
	s.game.Floor(view.Floor, func(shown *floor.Floor) {
		for row := top; row <= top+view.Rows; row++ {
			for col := left; col <= left+view.Columns; col++ {
				if !shown.In(col, row) {
					continue
				}
				corner := s.cellAt(view, float64(col), float64(row))
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
func (s *State) drawGround(rect image.Rectangle, ground string) {
	if picture, shown := s.art.image(ground); shown {
		drawArt(s.canvas, picture, rect.Min)

		return
	}
	if ground != "" {
		ui.FillRect(s.canvas, rect, ui.Hex(tiles.ColorOf(ground)))
	}
}

// drawTop draws what stands on a cell: its sprite, or its stand-in's colour,
// a wall or a door filling the cell and pickups smaller.
func (s *State) drawTop(rect image.Rectangle, top string) {
	if picture, shown := s.art.image(top); shown {
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
	ui.FillRect(s.canvas, rect.Inset(rect.Dx()*inset/eighths), ui.Hex(tiles.ColorOf(top)))
}

// figure is a character or the player, drawn in order of the row it stands
// on, so whoever is lower on the screen is drawn in front.
type figure struct {
	key    string
	x, y   float64
	player bool
}

// drawCharacters draws the NPCs on the player's floor and the player.
func (s *State) drawCharacters(view game.View) {
	player := s.game.Player()
	playerX, playerY := player.At()
	figures := []figure{{key: art.Player, x: playerX, y: playerY, player: true}}
	for _, character := range s.game.Characters() {
		if character.Floor == view.Floor {
			figures = append(figures, figure{key: npcKey(character.NPC), x: character.X, y: character.Y, player: false})
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
		corner := s.cellAt(view, each.x, each.y)
		if picture, shown := s.art.image(each.key); shown {
			drawArt(s.canvas, picture, corner)

			continue
		}
		centreX, centreY := float32(corner.X)+float32(s.tile)/ui.Half, float32(corner.Y)+float32(s.tile)/ui.Half
		radius := float32(s.tile) * discShare
		if each.player {
			vector.FillCircle(s.canvas, centreX, centreY, radius, ui.Hex(hexPlayer), true)
			vector.StrokeCircle(s.canvas, centreX, centreY, radius, ringWidth, ui.Hex(hexRing), true)

			continue
		}
		vector.FillCircle(s.canvas, centreX, centreY, radius, ui.Hex(hexToken), true)
	}
}

// drawArt draws a tile's art at a canvas point, one pixel for one.
func drawArt(canvas, picture *ebiten.Image, at image.Point) {
	var place ebiten.DrawImageOptions
	place.GeoM.Translate(float64(at.X), float64(at.Y))
	canvas.DrawImage(picture, &place)
}
