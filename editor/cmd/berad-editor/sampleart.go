package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The sample's lantern: a sprite shown as the small pickup, drawn in the
// starter greys with a ramp of amber glass, so the floor shows through it on
// the map. It is placeholder art for trying the editor, not game content.
const (
	lanternName   = "Lantern"
	lanternTile   = "Lantern"
	lanternShows  = "pickup.small"
	glassRampName = "glass"
	glassOpacity  = 40
	amberDark     = art.RGB(0xb8702a)
	amberLight    = art.RGB(0xf2b544)
	// glassIDs are the glass ramp's and its two shades'; starterGreys is
	// how many shades the starter ramp has.
	glassIDs     = 3
	starterGreys = 5
)

// errSample reports a sample document that is not what the sample expects.
var errSample = errors.New("unexpected sample document")

// lanternHalf is the left half of the lantern, a row of its tile on each
// line, mirrored for the right half: "." is clear, 1 to 5 the starter greys
// and a and b the glass's dark and light amber.
const lanternHalf = `
................
................
................
..............11
.............1..
.............1..
..............12
...........11333
.........1133344
........12222333
........11111111
.........12aaaaa
.........12aaaab
.........12aaabb
.........12aabbb
.........12aabbb
.........12aabb5
.........12aab55
.........12aab55
.........12aabb5
.........12aabbb
.........12aaabb
.........12aaaab
.........12aaaaa
.........12aaaaa
.........12aaaaa
........11111111
........12223333
.........1122222
..........111111
................
................
`

// sampleLantern adds the glass ramp to the shared palette, draws the lantern
// and makes it show the small pickup.
func sampleLantern(orch *orchestrator.Orchestrator) error {
	glass, err := glassRamp(orch)
	if err != nil {
		return err
	}
	err = orch.ChangeDocument(art.SharedPaletteID(), func(document model.Document) error {
		shared, ok := document.(*artplugin.PaletteDocument)
		if !ok {
			return fmt.Errorf("%w: the shared palette", errSample)
		}

		if err := shared.Palette.InsertRamp(len(shared.Palette.Ramps()), glass); err != nil {
			return fmt.Errorf("glass ramp: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("sample lantern: %w", err)
	}
	tile, err := lanternArt(glass)
	if err != nil {
		return err
	}
	tileID, err := orch.CreateDocument(lanternTile, &artplugin.TileDocument{Tile: tile})
	if err != nil {
		return fmt.Errorf("sample lantern: %w", err)
	}
	spriteID, err := orch.CreateDocument(lanternName, &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: tileID}})
	if err != nil {
		return fmt.Errorf("sample lantern: %w", err)
	}
	err = orch.ChangeDocument(art.StandInsID(), func(document model.Document) error {
		standIns, ok := document.(*artplugin.StandInsDocument)
		if !ok {
			return fmt.Errorf("%w: the stand-in sprites", errSample)
		}
		if _, err := standIns.Sprites.Set(lanternShows, spriteID); err != nil {
			return fmt.Errorf("lantern shows %s: %w", lanternShows, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("sample lantern: %w", err)
	}

	return nil
}

// glassRamp is the lantern's ramp of amber glass, with new IDs.
func glassRamp(orch *orchestrator.Orchestrator) (art.Ramp, error) {
	made := make([]ids.ULID, 0, glassIDs)
	for range glassIDs {
		id, err := orch.NewID()
		if err != nil {
			return art.Ramp{ID: ids.ULID{}, Name: "", Shades: nil}, fmt.Errorf("sample lantern: %w", err)
		}
		made = append(made, id)
	}

	return art.Ramp{ID: made[0], Name: glassRampName, Shades: []art.Shade{
		{ID: made[1], Color: amberDark, Opacity: glassOpacity},
		{ID: made[2], Color: amberLight, Opacity: glassOpacity},
	}}, nil
}

// lanternArt draws the lantern on a tile of the default tile size, which a
// new project starts with.
func lanternArt(glass art.Ramp) (*art.Tile, error) {
	tile, err := art.NewTile(art.DefaultTileSize)
	if err != nil {
		return nil, fmt.Errorf("sample lantern: %w", err)
	}
	colors := map[rune]art.ColorRef{
		'a': {Palette: art.SharedPaletteID(), Ramp: glass.ID, Shade: glass.Shades[0].ID},
		'b': {Palette: art.SharedPaletteID(), Ramp: glass.ID, Shade: glass.Shades[1].ID},
	}
	for number := 1; number <= starterGreys; number++ {
		colors[rune('0'+number)] = art.ColorRef{
			Palette: art.SharedPaletteID(), Ramp: art.StarterRampID(), Shade: art.StarterShadeID(number),
		}
	}
	for row, half := range strings.Fields(lanternHalf) {
		line := half + reverse(half)
		for col, mark := range line {
			ref, painted := colors[mark]
			if !painted {
				continue
			}
			if _, err := tile.Set(col, row, ref); err != nil {
				return nil, fmt.Errorf("sample lantern: %w", err)
			}
		}
	}

	return tile, nil
}

// reverse returns a row of the lantern mirrored.
func reverse(half string) string {
	var mirrored strings.Builder
	for index := len(half) - 1; index >= 0; index-- {
		mirrored.WriteByte(half[index])
	}

	return mirrored.String()
}
