package art_test

import (
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const size = 8

// glassPalette returns a palette with an opaque blue and a 40% glass, and
// references to each.
func glassPalette(t *testing.T) (*art.Palette, art.ColorRef, art.ColorRef) {
	t.Helper()
	palette := art.NewPalette()
	ramp := art.Ramp{ID: id(10), Name: "blue", Shades: []art.Shade{
		{ID: id(11), Color: 0x0000FF, Opacity: 100},
		{ID: id(12), Color: 0x0000FF, Opacity: 40},
	}}
	if err := palette.InsertRamp(0, ramp); err != nil {
		t.Fatal(err)
	}
	solid := art.ColorRef{Palette: id(1), Ramp: id(10), Shade: id(11)}
	glass := art.ColorRef{Palette: id(1), Ramp: id(10), Shade: id(12)}

	return palette, solid, glass
}

func newTile(t *testing.T) *art.Tile {
	t.Helper()
	tile, err := art.NewTile(size)
	if err != nil {
		t.Fatal(err)
	}

	return tile
}

func set(t *testing.T, tile *art.Tile, col, row int, ref art.ColorRef) {
	t.Helper()
	if _, err := tile.Set(col, row, ref); err != nil {
		t.Fatal(err)
	}
}

func rows(t *testing.T, tile *art.Tile) []string {
	t.Helper()
	data, err := tile.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Rows []string `json:"rows"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}

	return wire.Rows
}

func TestTileSaving(t *testing.T) {
	t.Parallel()
	_, solid, glass := glassPalette(t)
	tile := newTile(t)
	set(t, tile, 1, 0, glass)
	set(t, tile, 2, 3, solid)
	before := rows(t, tile)
	if before[0] != "0001000000000000" || before[3] != "0000020000000000" {
		t.Fatalf("rows: %q", before)
	}
	set(t, tile, 5, 3, solid)
	after := rows(t, tile)
	changed := 0
	for row := range before {
		if before[row] != after[row] {
			changed++
		}
	}
	if changed != 1 {
		t.Fatalf("changing one pixel changed %d rows", changed)
	}
	data, err := tile.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := art.DecodeTile(data)
	if err != nil || !decoded.Equal(tile) {
		t.Fatalf("round trip: %v", err)
	}
	again, err := decoded.Encode()
	if err != nil || string(again) != string(data) {
		t.Fatal("equal tiles must save the same way")
	}
	if previous, err := tile.Set(1, 0, art.ColorRef{}); err != nil || previous != glass {
		t.Fatalf("erasing returns what was there: %+v %v", previous, err)
	}
	if colors := tile.Colors(); len(colors) != 1 || colors[0] != solid {
		t.Fatalf("a colour no pixel uses leaves the table: %+v", colors)
	}
}

func TestTileDecodeRefusesBadData(t *testing.T) {
	t.Parallel()
	ref := `{"palette":"01J00000000000000000000001","ramp":"01J00000000000000000000010","shade":"01J00000000000000000000011"}`
	clear := `{"palette":"00000000000000000000000000","ramp":"00000000000000000000000000","shade":"00000000000000000000000000"}`
	row := `"0000000000000000"`
	eight := func(first string) string {
		return `[` + first + `,` + row + `,` + row + `,` + row + `,` + row + `,` + row + `,` + row + `,` + row + `]`
	}
	for name, bad := range map[string]string{
		"a size too small":    `{"size":4,"colors":[],"rows":["00000000","00000000","00000000","00000000"]}`,
		"a row missing":       `{"size":8,"colors":[],"rows":[` + row + `]}`,
		"a short row":         `{"size":8,"colors":[],"rows":` + eight(`"00"`) + `}`,
		"a row not hex":       `{"size":8,"colors":[],"rows":` + eight(`"zz00000000000000"`) + `}`,
		"a colour not listed": `{"size":8,"colors":[` + ref + `],"rows":` + eight(`"0200000000000000"`) + `}`,
		"a repeated colour":   `{"size":8,"colors":[` + ref + `,` + ref + `],"rows":` + eight(row) + `}`,
		"clear in the table":  `{"size":8,"colors":[` + clear + `],"rows":` + eight(row) + `}`,
	} {
		if _, err := art.DecodeTile([]byte(bad)); err == nil {
			t.Fatalf("decoded a tile with %s", name)
		}
	}
	good := `{"size":8,"colors":[` + ref + `],"rows":` + eight(`"0100000000000000"`) + `}`
	if _, err := art.DecodeTile([]byte(good)); err != nil {
		t.Fatal(err)
	}
}

func TestTileBoundsAndColorLimit(t *testing.T) {
	t.Parallel()
	for _, bad := range []int{art.MinTileSize - 1, art.MaxTileSize + 1} {
		if _, err := art.NewTile(bad); !errors.Is(err, art.ErrSize) {
			t.Fatalf("tile size %d: %v", bad, err)
		}
	}
	tile, err := art.NewTile(16)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tile.Set(16, 0, art.ColorRef{}); !errors.Is(err, art.ErrBounds) {
		t.Fatalf("outside the tile: %v", err)
	}
	half := art.ColorRef{Palette: id(1), Ramp: ids.ULID{}, Shade: id(2)}
	if _, err := tile.Set(0, 0, half); err == nil {
		t.Fatal("a colour without its ramp was accepted")
	}
	colour := func(number int) art.ColorRef {
		return art.ColorRef{Palette: id(1), Ramp: id(2), Shade: id(1000 + number)}
	}
	for number := range art.MaxTileColors {
		set(t, tile, number%16, number/16, colour(number))
	}
	if _, err := tile.Set(15, 15, colour(art.MaxTileColors)); !errors.Is(err, art.ErrTooManyColors) {
		t.Fatalf("one colour too many: %v", err)
	}
	if _, err := tile.Set(0, 0, colour(art.MaxTileColors)); err != nil {
		t.Fatalf("replacing a colour's only use stays within the limit: %v", err)
	}
}

func TestTileImage(t *testing.T) {
	t.Parallel()
	palette, solid, glass := glassPalette(t)
	tile := newTile(t)
	set(t, tile, 0, 0, solid)
	set(t, tile, 1, 0, glass)
	set(t, tile, 2, 0, art.ColorRef{Palette: id(1), Ramp: id(10), Shade: id(99)})
	picture, missing := tile.Image(art.Palettes{id(1): palette})
	if missing != 1 {
		t.Fatalf("missing colours: %d", missing)
	}
	if got := picture.NRGBAAt(0, 0); got != (color.NRGBA{R: 0, G: 0, B: 255, A: 255}) {
		t.Fatalf("solid pixel %v", got)
	}
	if got := picture.NRGBAAt(1, 0); got != (color.NRGBA{R: 0, G: 0, B: 255, A: 102}) {
		t.Fatalf("glass pixel %v", got)
	}
	if picture.NRGBAAt(2, 0).A != 0 || picture.NRGBAAt(3, 3).A != 0 {
		t.Fatal("missing and clear pixels are transparent")
	}
	ground := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(ground, ground.Bounds(), image.NewUniform(color.RGBA{R: 255, G: 0, B: 0, A: 255}), image.Point{}, draw.Src)
	draw.Draw(ground, ground.Bounds(), picture, image.Point{}, draw.Over)
	through := ground.RGBAAt(1, 0)
	if near(through.R, 153) && through.G == 0 && near(through.B, 102) {
		return
	}
	t.Fatalf("40%% blue over red: %v", through)
}

func TestRenamedRampDrawsTheSame(t *testing.T) {
	t.Parallel()
	palette, solid, _ := glassPalette(t)
	palettes := art.Palettes{id(1): palette}
	tile := newTile(t)
	set(t, tile, 4, 4, solid)
	before, _ := tile.Image(palettes)
	if _, err := palette.RenameRamp(id(10), "sea"); err != nil {
		t.Fatal(err)
	}
	renamed, _ := tile.Image(palettes)
	if renamed.NRGBAAt(4, 4) != before.NRGBAAt(4, 4) {
		t.Fatal("renaming a ramp changed what is drawn")
	}
	if _, err := palette.SetShade(id(10), art.Shade{ID: id(11), Color: 0x00FF00, Opacity: 100}); err != nil {
		t.Fatal(err)
	}
	recoloured, _ := tile.Image(palettes)
	if recoloured.NRGBAAt(4, 4) != (color.NRGBA{R: 0, G: 255, B: 0, A: 255}) {
		t.Fatal("changing a shade must recolour every tile that uses it")
	}
}

func near(got, want uint8) bool {
	return got+1 >= want && got <= want+1
}

func TestLooseColors(t *testing.T) {
	t.Parallel()
	palette, solid, _ := glassPalette(t)
	tile := newTile(t)
	loose := art.Loose(0x5D5A63, 60)
	black := art.Loose(0x000000, 100)
	set(t, tile, 0, 0, loose)
	set(t, tile, 1, 0, black)
	set(t, tile, 2, 0, solid)
	if !loose.IsLoose() || loose.IsClear() || solid.IsLoose() || black.IsClear() {
		t.Fatal("a loose colour is neither clear nor named")
	}
	if got := tile.Count(art.ColorRef.IsLoose); got != 2 {
		t.Fatalf("loose pixels: %d", got)
	}
	picture, missing := tile.Image(art.Palettes{id(1): palette})
	if missing != 0 || picture.NRGBAAt(0, 0) != (color.NRGBA{R: 0x5D, G: 0x5A, B: 0x63, A: 153}) {
		t.Fatalf("a loose colour draws as its own value: %v, %d missing", picture.NRGBAAt(0, 0), missing)
	}
	if picture.NRGBAAt(1, 0) != (color.NRGBA{R: 0, G: 0, B: 0, A: 255}) {
		t.Fatal("loose black is black, not clear")
	}
	data, err := tile.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := art.DecodeTile(data)
	if err != nil || !decoded.Equal(tile) {
		t.Fatalf("loose colours round trip: %v %s", err, data)
	}
	for name, bad := range map[string]art.ColorRef{
		"an opacity over 100": art.Loose(0x101010, 101),
		"a value and a shade": {Palette: id(1), Ramp: id(10), Shade: id(11), Value: 1, Opacity: 100},
	} {
		if _, err := tile.Set(3, 0, bad); err == nil {
			t.Fatalf("painted a colour with %s", name)
		}
	}
}

func TestMovedColorStillDraws(t *testing.T) {
	t.Parallel()
	palette, solid, _ := glassPalette(t)
	if err := palette.InsertRamp(1, art.Ramp{ID: id(20), Name: "sea", Shades: nil}); err != nil {
		t.Fatal(err)
	}
	palettes := art.Palettes{id(1): palette}
	tile := newTile(t)
	set(t, tile, 0, 0, solid)
	if _, _, err := palette.MoveShade(solid.Shade, id(20), 0, nil); err != nil {
		t.Fatal(err)
	}
	picture, missing := tile.Image(palettes)
	if missing != 0 || picture.NRGBAAt(0, 0) != (color.NRGBA{R: 0, G: 0, B: 255, A: 255}) {
		t.Fatalf("a moved colour still draws: %v, %d missing", picture.NRGBAAt(0, 0), missing)
	}
	if !palettes.Moved(solid) || palettes.Moved(art.Named(id(1), id(20), solid.Shade)) {
		t.Fatal("only the old place is reported as moved")
	}
}
