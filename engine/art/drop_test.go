package art_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/engine/art"
)

func TestDroppedPaletteKeepsColors(t *testing.T) {
	t.Parallel()
	gloomy := art.NewPalette()
	if err := gloomy.InsertRamp(0, art.Ramp{ID: id(10), Name: "night", Shades: []art.Shade{
		{ID: id(11), Color: 0x2B3449, Opacity: 100},
		{ID: id(12), Color: 0x5D5A63, Opacity: 100},
		{ID: id(13), Color: 0x888888, Opacity: 40},
	}}); err != nil {
		t.Fatal(err)
	}
	tile := newTile(t)
	night := art.Named(id(1), id(10), id(11))
	smoke := art.Named(id(1), id(10), id(12))
	glass := art.Named(id(1), id(10), id(13))
	gone := art.Named(id(1), id(10), id(99))
	other := art.Named(id(2), id(30), id(31))
	for col, ref := range []art.ColorRef{night, smoke, glass, gone, other} {
		set(t, tile, col, 0, ref)
	}
	changes := art.Dropped(tile, id(1), gloomy, defaults(t))
	if len(changes) != 4 {
		t.Fatalf("changes for every colour of the palette, and no other: %v", changes)
	}
	if want := art.Named(art.DefaultPaletteID(), art.DefaultRampID(17), art.DefaultShadeID(17, 4)); changes[night] != want {
		t.Fatalf("an exact match becomes Default's slate 4: %+v", changes[night])
	}
	if changes[smoke] != art.Loose(0x5D5A63, 100) {
		t.Fatalf("no match leaves a loose colour: %+v", changes[smoke])
	}
	if changes[glass] != art.Loose(0x888888, 40) {
		t.Fatal("a match must have the same opacity too")
	}
	if !changes[gone].IsClear() {
		t.Fatal("a colour the palette no longer holds becomes clear")
	}
}
