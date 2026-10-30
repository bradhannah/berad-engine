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

// The sample's lantern: a sprite shown as the small pickup, painted only from
// a palette of the sample's own, "gloomy": a ramp of dim iron greys and a ramp
// of smoky amber glass, so the floor shows through it on the map. The sample
// zone lists that palette. It is placeholder art for trying the editor, not
// game content.
const (
	lanternName   = "Lantern"
	lanternTile   = "Lantern"
	lanternShows  = "pickup.small"
	paletteName   = "gloomy"
	ironRampName  = "iron"
	glassRampName = "glass"
	glassOpacity  = 40
	amberDark     = art.RGB(0x6b4a2a)
	amberLight    = art.RGB(0x9c7440)
	// The iron ramp, dark to light: greys with a little violet, none lighter
	// than Default's grey 6.
	iron1 = art.RGB(0x16151a)
	iron2 = art.RGB(0x221f28)
	iron3 = art.RGB(0x302c38)
	iron4 = art.RGB(0x403b4a)
	iron5 = art.RGB(0x544e60)
)

// ironColors are the iron ramp's colours, the lantern's marks 1 to 5.
func ironColors() []art.RGB { return []art.RGB{iron1, iron2, iron3, iron4, iron5} }

// errSample reports a sample document that is not what the sample expects.
var errSample = errors.New("unexpected sample document")

// lanternHalf is the left half of the lantern, a row of its tile on each
// line, mirrored for the right half: "." is clear, 1 to 5 the iron greys and
// a and b the glass's dark and light amber.
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

// sampleLantern makes the sample's palette, gloomy, draws the lantern with it
// and makes it show the small pickup. It returns the palette's ID.
func sampleLantern(orch *orchestrator.Orchestrator) (ids.ULID, error) {
	iron, err := sampleRamp(orch, ironRampName, art.MaxOpacity, ironColors())
	if err != nil {
		return ids.ULID{}, err
	}
	glass, err := sampleRamp(orch, glassRampName, glassOpacity, []art.RGB{amberDark, amberLight})
	if err != nil {
		return ids.ULID{}, err
	}
	palette := art.NewPalette()
	for at, ramp := range []art.Ramp{iron, glass} {
		if err := palette.InsertRamp(at, ramp); err != nil {
			return ids.ULID{}, fmt.Errorf("sample lantern: %w", err)
		}
	}
	paletteID, err := orch.CreateDocument(paletteName, &artplugin.PaletteDocument{Palette: palette})
	if err != nil {
		return ids.ULID{}, fmt.Errorf("sample lantern: %w", err)
	}
	tile, err := lanternArt(paletteID, iron, glass)
	if err != nil {
		return ids.ULID{}, err
	}
	tileID, err := orch.CreateDocument(lanternTile, &artplugin.TileDocument{Tile: tile})
	if err != nil {
		return ids.ULID{}, fmt.Errorf("sample lantern: %w", err)
	}
	spriteID, err := orch.CreateDocument(lanternName, &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: tileID}})
	if err != nil {
		return ids.ULID{}, fmt.Errorf("sample lantern: %w", err)
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
		return ids.ULID{}, fmt.Errorf("sample lantern: %w", err)
	}

	return paletteID, nil
}

// sampleRamp is a ramp of the sample's palette, its colours at one opacity,
// with new IDs.
func sampleRamp(orch *orchestrator.Orchestrator, name string, opacity int, colors []art.RGB) (art.Ramp, error) {
	rampID, err := orch.NewID()
	if err != nil {
		return art.Ramp{ID: ids.ULID{}, Name: "", Shades: nil}, fmt.Errorf("sample lantern: %w", err)
	}
	shades := make([]art.Shade, 0, len(colors))
	for _, color := range colors {
		shadeID, err := orch.NewID()
		if err != nil {
			return art.Ramp{ID: ids.ULID{}, Name: "", Shades: nil}, fmt.Errorf("sample lantern: %w", err)
		}
		shades = append(shades, art.Shade{ID: shadeID, Color: color, Opacity: opacity, Label: ""})
	}

	return art.Ramp{ID: rampID, Name: name, Shades: shades}, nil
}

// lanternArt draws the lantern on a tile of the default tile size, which a
// new project starts with.
func lanternArt(paletteID ids.ULID, iron, glass art.Ramp) (*art.Tile, error) {
	tile, err := art.NewTile(art.DefaultTileSize)
	if err != nil {
		return nil, fmt.Errorf("sample lantern: %w", err)
	}
	colors := map[rune]art.ColorRef{
		'a': art.Named(paletteID, glass.ID, glass.Shades[0].ID),
		'b': art.Named(paletteID, glass.ID, glass.Shades[1].ID),
	}
	for mark, shade := range iron.Shades {
		colors[rune('1'+mark)] = art.Named(paletteID, iron.ID, shade.ID)
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
