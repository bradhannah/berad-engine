package art_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// id returns a fixed, valid ID for a number.
func id(number int) ids.ULID {
	return ids.MustParse(fmt.Sprintf("01J0000000000000000000%04d", number))
}

// sequence returns an ID source counting up from a number.
func sequence(start int) func() (ids.ULID, error) {
	next := start

	return func() (ids.ULID, error) {
		next++

		return id(next), nil
	}
}

func starter(t *testing.T) *art.Palette {
	t.Helper()
	palette, err := art.StarterPalette(sequence(100))
	if err != nil {
		t.Fatal(err)
	}

	return palette
}

func TestStarterPalette(t *testing.T) {
	t.Parallel()
	palette := starter(t)
	ramps := palette.Ramps()
	if len(ramps) != 1 || ramps[0].Name != art.StarterRampName || len(ramps[0].Shades) != 5 {
		t.Fatalf("starter palette: %+v", ramps)
	}
	for at, shade := range ramps[0].Shades {
		if shade.Opacity != art.MaxOpacity {
			t.Fatalf("starter shade %d is see-through", at+1)
		}
		if at > 0 && shade.Color <= ramps[0].Shades[at-1].Color {
			t.Fatal("starter shades must run dark to light")
		}
	}
	name, ok := palette.ShadeName(ramps[0].ID, ramps[0].Shades[2].ID)
	if !ok || name != "grey 3" {
		t.Fatalf("shade name %q", name)
	}
}

func TestRampsAndShades(t *testing.T) {
	t.Parallel()
	palette := starter(t)
	grey := palette.Ramps()[0]
	glass := art.Ramp{ID: id(1), Name: "glass", Shades: []art.Shade{{ID: id(2), Color: 0x8AB4D4, Opacity: 40}}}
	if err := palette.InsertRamp(1, glass); err != nil {
		t.Fatal(err)
	}
	for name, ramp := range map[string]art.Ramp{
		"a taken name":     {ID: id(3), Name: "glass", Shades: nil},
		"an untrimmed one": {ID: id(3), Name: " sand", Shades: nil},
		"no name":          {ID: id(3), Name: "", Shades: nil},
		"a taken ID":       {ID: grey.ID, Name: "sand", Shades: nil},
		"a taken shade ID": {ID: id(3), Name: "sand", Shades: []art.Shade{{ID: id(2), Color: 1, Opacity: 100}}},
	} {
		if err := palette.InsertRamp(0, ramp); err == nil {
			t.Fatalf("a ramp with %s was accepted", name)
		}
	}
	previous, err := palette.RenameRamp(grey.ID, "stone")
	if err != nil || previous != art.StarterRampName {
		t.Fatalf("rename: %q %v", previous, err)
	}
	if _, err := palette.RenameRamp(grey.ID, "glass"); !errors.Is(err, art.ErrName) {
		t.Fatalf("renaming to a taken name: %v", err)
	}
	if err := palette.InsertShade(glass.ID, 1, art.Shade{ID: id(4), Color: 0xC9E0F0, Opacity: 0}); err == nil {
		t.Fatal("a shade at 0% was accepted; clear is its own colour")
	}
	if err := palette.InsertShade(glass.ID, 1, art.Shade{ID: id(4), Color: 0xC9E0F0, Opacity: 60}); err != nil {
		t.Fatal(err)
	}
	if name, _ := palette.ShadeName(glass.ID, id(4)); name != "glass 2" {
		t.Fatalf("new shade's name %q", name)
	}
	was, err := palette.SetShade(glass.ID, art.Shade{ID: id(2), Color: 0x101010, Opacity: 55})
	if err != nil || was.Color != 0x8AB4D4 || was.Opacity != 40 {
		t.Fatalf("set shade: %+v %v", was, err)
	}
	removed, at, err := palette.RemoveShade(glass.ID, id(2))
	if err != nil || at != 0 || removed.Color != 0x101010 {
		t.Fatalf("remove shade: %+v %d %v", removed, at, err)
	}
	if name, _ := palette.ShadeName(glass.ID, id(4)); name != "glass 1" {
		t.Fatalf("a shade's number is its place: %q", name)
	}
	if _, _, err := palette.RemoveRamp(id(99)); !errors.Is(err, art.ErrNoRamp) {
		t.Fatalf("removing a missing ramp: %v", err)
	}
}

func TestPaletteRoundTrip(t *testing.T) {
	t.Parallel()
	palette := starter(t)
	glass := art.Ramp{ID: id(1), Name: "glass", Shades: []art.Shade{{ID: id(2), Color: 0x8AB4D4, Opacity: 40}}}
	if err := palette.InsertRamp(1, glass); err != nil {
		t.Fatal(err)
	}
	data, err := palette.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"color":"#8AB4D4"`) {
		t.Fatalf("colours are saved as #RRGGBB: %s", data)
	}
	decoded, err := art.DecodePalette(data)
	if err != nil || !decoded.Equal(palette) {
		t.Fatalf("round trip: %v", err)
	}
	for name, bad := range map[string]string{
		"not JSON":         `{`,
		"a bad colour":     `{"ramps":[{"id":"01J00000000000000000000001","name":"a","shades":[{"id":"01J00000000000000000000002","color":"#12","opacity":100}]}]}`,
		"a repeated shade": `{"ramps":[{"id":"01J00000000000000000000001","name":"a","shades":[{"id":"01J00000000000000000000002","color":"#000000","opacity":100},{"id":"01J00000000000000000000002","color":"#FFFFFF","opacity":100}]}]}`,
		"repeated names":   `{"ramps":[{"id":"01J00000000000000000000001","name":"a","shades":[]},{"id":"01J00000000000000000000003","name":"a","shades":[]}]}`,
	} {
		if _, err := art.DecodePalette([]byte(bad)); err == nil {
			t.Fatalf("decoded a palette with %s", name)
		}
	}
	empty, err := art.NewPalette().Encode()
	if err != nil || string(empty) != `{"ramps":[]}` {
		t.Fatalf("empty palette saved as %s", empty)
	}
}

func TestRGB(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]art.RGB{"#8a919c": 0x8A919C, "8A919C": 0x8A919C, "#000000": 0} {
		got, err := art.ParseRGB(text)
		if err != nil || got != want {
			t.Fatalf("parse %q: %v %v", text, got, err)
		}
	}
	for _, bad := range []string{"", "#FFF", "#GGGGGG", "#1234567"} {
		if _, err := art.ParseRGB(bad); err == nil {
			t.Fatalf("parsed %q", bad)
		}
	}
	if (art.RGB(0x8A919C)).String() != "#8A919C" {
		t.Fatal("colours are written upper case with #")
	}
	if alpha := art.RGB(0).NRGBA(40).A; alpha != 102 {
		t.Fatalf("40%% opacity is alpha %d", alpha)
	}
}

func TestFormatsRegister(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	for _, format := range []content.Format{
		art.PaletteFormat(), art.TileFormat(), art.SpriteFormat(), art.SettingsFormat(), art.StandInsFormat(),
	} {
		if err := registry.Register(format); err != nil {
			t.Fatal(err)
		}
	}
	if len(registry.Names()) != 5 {
		t.Fatalf("formats: %v", registry.Names())
	}
}
