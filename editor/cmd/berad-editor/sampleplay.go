package main

import (
	"fmt"
	"strings"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// What makes the sample playable with Launch (increment 14): a player start
// inside the door, brick for the ground floor's walls, a figure for the
// player and another for a clerk standing by the desk, the hum placed in the
// building, heard within its distance, and the tune played throughout. It is
// placeholder art and sound for trying the editor, not game content.
const (
	startCol    = 0
	startRow    = 2
	clerkCol    = -4
	clerkRow    = -1
	humCol      = -3
	humRow      = 2
	humCue      = "sample/hum"
	tuneCue     = "sample/tune"
	brickShows  = "wall.a"
	brickCourse = 6
	brickLength = 12
	clerkName   = "Clerk"
	// The shelf the clerk tidies, and the cell inside the door where the
	// clerk waits at night.
	shelfCol      = -1
	shelfRow      = -3
	insideDoorRow = 3
)

// figureHalf is the left half of a standing figure, mirrored for the right
// half: "." is clear, o its outline, h its head, e its eyes and b its body,
// whose colour each figure chooses.
const figureHalf = `
............
............
........oooo
.......ohhhh
......ohhhhh
......ohheeh
......ohhhhh
.......ohhhh
........oooo
......oobbbb
.....obbbbbb
.....obbbbbb
.....obobbbb
.....obobbbb
.....obobbbb
.....ooobbbb
.......obbbb
.......obbbb
.......obbo.
.......obbo.
.......obbo.
.......oooo.
............
............
`

// sampleArt is the sample's palette, gloomy, and its ramps.
type sampleArt struct {
	palette     ids.ULID
	iron, glass art.Ramp
}

// ironShade is one of the iron greys, 1 to 5, as a colour of gloomy.
func (a sampleArt) ironShade(mark int) art.ColorRef {
	return art.Named(a.palette, a.iron.ID, a.iron.Shades[mark-1].ID)
}

// sampleSprite makes a sprite of one tile, named for what it shows.
func sampleSprite(orch *orchestrator.Orchestrator, name string, tile *art.Tile) (ids.ULID, error) {
	tileID, err := orch.CreateDocument(name, &artplugin.TileDocument{Tile: tile})
	if err != nil {
		return ids.ULID{}, fmt.Errorf("sample %s: %w", name, err)
	}
	spriteID, err := orch.CreateDocument(name, &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: tileID}})
	if err != nil {
		return ids.ULID{}, fmt.Errorf("sample %s: %w", name, err)
	}

	return spriteID, nil
}

// brickArt draws courses of iron brick with dark mortar, each course offset
// by half a brick.
func brickArt(sample sampleArt) (*art.Tile, error) {
	tile, err := art.NewTile(art.DefaultTileSize)
	if err != nil {
		return nil, fmt.Errorf("sample brick: %w", err)
	}
	for row := range art.DefaultTileSize {
		offset := (row / brickCourse % 2) * brickLength / 2 //nolint:mnd // Every other course.
		for col := range art.DefaultTileSize {
			mark := 3
			switch {
			case row%brickCourse == brickCourse-1, (col+offset)%brickLength == brickLength-1:
				mark = 1
			case row%brickCourse == 0:
				mark = 4
			}
			if _, err := tile.Set(col, row, sample.ironShade(mark)); err != nil {
				return nil, fmt.Errorf("sample brick: %w", err)
			}
		}
	}

	return tile, nil
}

// figureArt draws the figure with a body of one colour.
func figureArt(sample sampleArt, body art.ColorRef) (*art.Tile, error) {
	tile, err := art.NewTile(art.DefaultTileSize)
	if err != nil {
		return nil, fmt.Errorf("sample figure: %w", err)
	}
	colors := map[rune]art.ColorRef{
		'o': sample.ironShade(1), 'h': sample.ironShade(5), 'e': sample.ironShade(1), 'b': body, //nolint:mnd // Marks.
	}
	for row, half := range strings.Fields(figureHalf) {
		for col, mark := range half + reverse(half) {
			if ref, painted := colors[mark]; painted {
				if _, err := tile.Set(col, row, ref); err != nil {
					return nil, fmt.Errorf("sample figure: %w", err)
				}
			}
		}
	}

	return tile, nil
}

// samplePlayers makes brick show the walls of family A and a figure show the
// player, and the clerk, an NPC with a figure of their own. It returns the
// clerk's ID.
func samplePlayers(orch *orchestrator.Orchestrator, sample sampleArt) (ids.ULID, error) {
	brick, err := brickArt(sample)
	if err != nil {
		return ids.ULID{}, err
	}
	hero, err := figureArt(sample, sample.ironShade(3)) //nolint:mnd // A mid grey.
	if err != nil {
		return ids.ULID{}, err
	}
	apron, err := figureArt(sample, art.Named(sample.palette, sample.glass.ID, sample.glass.Shades[1].ID))
	if err != nil {
		return ids.ULID{}, err
	}
	shows := map[string]ids.ULID{}
	for _, made := range []struct {
		name, shows string
		tile        *art.Tile
	}{{"Brick", brickShows, brick}, {"Hero", art.Player, hero}, {clerkName, "", apron}} {
		spriteID, err := sampleSprite(orch, made.name, made.tile)
		if err != nil {
			return ids.ULID{}, err
		}
		shows[made.shows] = spriteID
	}
	clerkID, err := orch.CreateDocument(clerkName, &npcplugin.Document{
		NPC: npc.NPC{Behaviour: ids.ULID{}, Sprite: shows[""], Notes: ""},
	})
	if err != nil {
		return ids.ULID{}, fmt.Errorf("sample clerk: %w", err)
	}
	err = orch.ChangeDocument(art.StandInsID(), func(document model.Document) error {
		standIns, ok := document.(*artplugin.StandInsDocument)
		if !ok {
			return fmt.Errorf("%w: the stand-in sprites", errSample)
		}
		for _, shown := range []string{brickShows, art.Player} {
			if _, err := standIns.Sprites.Set(shown, shows[shown]); err != nil {
				return fmt.Errorf("sample shows %s: %w", shown, err)
			}
		}

		return nil
	})
	if err != nil {
		return ids.ULID{}, fmt.Errorf("sample sprites: %w", err)
	}

	return clerkID, nil
}

// samplePlace puts the clerk by the desk, the hum in the building, and the
// anchors the clerk goes to at the shelf and inside the door.
func samplePlace(floorData *floor.Floor, clerk, placement, hum ids.ULID, at sampleAnchors) error {
	for index, anchor := range []floor.Anchor{
		{ID: at.shelf, Name: "Shelf", Col: shelfCol, Row: shelfRow},
		{ID: at.door, Name: "Door", Col: doorCol, Row: insideDoorRow},
	} {
		if err := floorData.InsertAnchor(1+index, anchor); err != nil {
			return fmt.Errorf("sample anchors: %w", err)
		}
	}
	placed := floor.Placement{ID: placement, NPC: clerk, Col: clerkCol, Row: clerkRow}
	if err := floorData.InsertPlacement(0, placed); err != nil {
		return fmt.Errorf("sample clerk: %w", err)
	}
	if err := floorData.InsertSound(0, floor.Sound{ID: hum, Cue: humCue, Col: humCol, Row: humRow}); err != nil {
		return fmt.Errorf("sample hum: %w", err)
	}

	return nil
}

// sampleStart places the player start inside the door.
func sampleStart(orch *orchestrator.Orchestrator, zoneID, groundID ids.ULID) error {
	err := orch.ChangeDocument(game.SettingsID(), func(document model.Document) error {
		settings, ok := document.(*gameplugin.SettingsDocument)
		if !ok {
			return fmt.Errorf("%w: the game settings", errSample)
		}
		settings.Settings.Start = game.Start{Zone: zoneID, Floor: groundID, Col: startCol, Row: startRow}

		return nil
	})
	if err != nil {
		return fmt.Errorf("sample start: %w", err)
	}

	return nil
}
