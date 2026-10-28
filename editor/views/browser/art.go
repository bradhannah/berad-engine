package browser

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The Sprites and Palettes sections list the project's art, and the
// Settings row, the first in the list, holds its art settings. A sprite's
// tile is reached through its sprite, so tiles have no section.
const (
	sectionSprites  = sectionPrefix + "sprites"
	sectionPalettes = sectionPrefix + "palettes"
	settingsRow     = "settings"
	// newSprite, from New ▾ or the section's [+], makes a sprite with its
	// tile.
	newSprite = "new-sprite"
	// firstTile names a new sprite's tile, which need only be unique within
	// its sprite.
	firstTile = "Tile 1"
	// The tile size field is this wide and holds this many bytes.
	tileSizeWidth = 64
	tileSizeBytes = 8
)

// settingsRows is the Settings row, showing the tile size. A search leaves
// it out unless its own words match.
func (s *State) settingsRows(docs frame.Docs, match matcher) []ui.Row {
	if match.active() && !match.words("Settings", "Tile size") {
		return nil
	}
	row := ui.NewRow(settingsRow, "Settings")
	row.Icon = ui.IconSettings
	if size, ok := s.projectTileSize(docs); ok {
		row.Detail = "Tile size " + strconv.Itoa(size)
	}

	return []ui.Row{row}
}

// projectTileSize reads the project's tile size from its art settings.
func (s *State) projectTileSize(docs frame.Docs) (int, bool) {
	size := 0
	docs.View(art.SettingsID(), func(document client.Document) {
		if settings, ok := document.Model.(*artplugin.SettingsDocument); ok {
			size = settings.Settings.TileSize
		}
	})

	return size, size > 0
}

// tileOf returns a sprite's tile.
func (s *State) tileOf(docs frame.Docs, spriteID ids.ULID) (ids.ULID, bool) {
	var tile ids.ULID
	docs.View(spriteID, func(document client.Document) {
		if sprite, ok := document.Model.(*artplugin.SpriteDocument); ok {
			tile = sprite.Sprite.Tile
		}
	})

	return tile, !tile.IsZero()
}

// everyProjectHolds reports a document every project holds: it keeps its
// name and cannot be deleted.
func everyProjectHolds(documentID ids.ULID) bool {
	return documentID == art.SharedPaletteID() || documentID == art.SettingsID() || documentID == art.StandInsID()
}

// createSprite makes a sprite with one tile at the project's tile size, as
// one undo step. The tile is made first, since a sprite names its tile.
func (s *State) createSprite(ctx *frame.Context) {
	size, ok := s.projectTileSize(ctx.Docs)
	if !ok {
		size = art.DefaultTileSize
	}
	tile, err := art.NewTile(size)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	data, err := tile.Encode()
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	name := uniqueName("Sprite", names(s.entries(ctx.Docs, art.SpriteFormatName)))
	folder := s.targetFolder(ctx.Docs, art.SpriteFormatName)
	gesture := s.gesture()
	args := newDocument(art.TileFormatName, firstTile, ids.ULID{}, data)
	made := fmt.Sprintf("Created “%s” with one tile, %d × %d pixels", name, size, size)
	s.command(ctx, project.IndexID(), project.CreateEdit, gesture, args, func(result protocol.Result, _ error) {
		s.addSprite(ctx, newArt{name: name, folder: folder, gesture: gesture, made: made}, result.Created)
	}, nil)
}

// newArt is a sprite on its way: its name and folder, the gesture that makes
// it with its tile, and what the status bar says once it is made.
type newArt struct {
	name, folder, gesture, made string
}

// addSprite makes the sprite that shows a new tile. When the sprite is
// refused, the tile is deleted again, so no tile is left without a sprite.
func (s *State) addSprite(ctx *frame.Context, sprite newArt, tileID ids.ULID) {
	gesture := sprite.gesture
	removeTile := func() {
		deletion := newDocument("", "", ids.ULID{}, nil).withID(tileID)
		s.command(ctx, project.IndexID(), project.DeleteEdit, gesture, deletion, nil, nil)
	}
	data, err := art.Sprite{Tile: tileID}.Encode()
	if err != nil {
		ctx.Frame.Status(err.Error())
		removeTile()

		return
	}
	args := newDocument(art.SpriteFormatName, sprite.name, ids.ULID{}, data)
	args.Folder = sprite.folder
	s.command(ctx, project.IndexID(), project.CreateEdit, gesture, args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
		s.subscribe(result.Created, ctx)
		ctx.Frame.Status(sprite.made)
	}, removeTile)
}

// syncSettings keeps the tile size field showing the saved tile size.
func (s *State) syncSettings(docs frame.Docs) {
	if size, ok := s.projectTileSize(docs); ok {
		s.tileSize.Field.SetSaved(strconv.Itoa(size))
	}
}

// layoutSettings places the settings' inspector: the tile size field, and
// the save bar at the foot.
func (s *State) layoutSettings(docs frame.Docs, left, width int) {
	s.tileSize.Rect = image.Rect(left, nameFieldTop, left+tileSizeWidth, nameFieldTop+ui.ButtonH)
	bottom := frame.Inspector(s.size).Max.Y - ui.Inset
	s.save.Rect = image.Rect(left, bottom-ui.SaveBarHeight(width), left+width, bottom)
	s.save.Unsaved = s.Unsaved(docs)
}

// commitTileSize saves the typed tile size, a whole number of pixels that a
// tile can be.
func (s *State) commitTileSize(ctx *frame.Context) {
	typed, changed := s.tileSize.Field.Commit()
	if !changed {
		s.focus.Set(nil)

		return
	}
	size, err := strconv.Atoi(strings.TrimSpace(typed))
	if err != nil || (art.Settings{TileSize: size}).Validate() != nil {
		s.tileSize.Field.Failed()
		ctx.Frame.Status(fmt.Sprintf("Enter a whole number of pixels from %d to %d", art.MinTileSize, art.MaxTileSize))

		return
	}
	s.command(ctx, art.SettingsID(), artplugin.TileSizeEdit, "", artplugin.TileSizeArgs{Size: size},
		func(protocol.Result, error) {
			s.focus.Set(nil)
			s.tileSize.Field.Revert()
			ctx.Frame.Status(fmt.Sprintf("Tile size set to %d. New sprites have tiles of this size.", size))
		},
		func() { s.tileSize.Field.Failed() })
}

// drawSettings draws the settings' inspector: the tile size and what it
// decides.
func (s *State) drawSettings(screen *ebiten.Image, left int) {
	s.drawHeader(screen, ui.IconSettings, "Settings", art.SettingsName, left)
	s.theme.Heading(screen, "TILE SIZE", left, nameFieldTop-ui.LineHeight)
	s.tileSize.Draw(screen)
	s.theme.DrawText(screen, "pixels square", s.tileSize.Rect.Max.X+ui.Gap*ui.Half,
		s.tileSize.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorMuted())
	top := s.tileSize.Rect.Max.Y + ui.Gap
	for _, line := range []string{
		fmt.Sprintf("From %d to %d pixels.", art.MinTileSize, art.MaxTileSize),
		"A new sprite's tile is this size.", "Tiles of another size are flagged.",
	} {
		s.theme.DrawText(screen, line, left, top, ui.ColorSubdued())
		top += ui.LineHeight
	}
	s.save.Draw(screen, s.theme)
}
