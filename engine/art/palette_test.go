package art_test

import (
	"encoding/json"
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

// greys returns a palette of one ramp, "grey", of five shades, id(21) to
// id(25), dark to light.
func greys(t *testing.T) *art.Palette {
	t.Helper()
	ramp := art.Ramp{ID: id(20), Name: "grey", Shades: nil}
	for number, value := range []art.RGB{0x202020, 0x4A4A4A, 0x7A7A7A, 0xABABAB, 0xDCDCDC} {
		ramp.Shades = append(ramp.Shades, art.Shade{ID: id(21 + number), Color: value, Opacity: art.MaxOpacity})
	}
	palette := art.NewPalette()
	if err := palette.InsertRamp(0, ramp); err != nil {
		t.Fatal(err)
	}

	return palette
}

func defaults(t *testing.T) *art.Palette {
	t.Helper()
	palette, err := art.DefaultPalette()
	if err != nil {
		t.Fatal(err)
	}

	return palette
}

func TestDefaultPalette(t *testing.T) {
	t.Parallel()
	palette := defaults(t)
	ramps := palette.Ramps()
	names := make([]string, 0, len(ramps))
	count := 0
	for place, ramp := range ramps {
		names = append(names, ramp.Name)
		if ramp.ID != art.DefaultRampID(place+1) {
			t.Fatalf("ramp %s has ID %s", ramp.Name, ramp.ID)
		}
		for number, shade := range ramp.Shades {
			count++
			if shade.ID != art.DefaultShadeID(place+1, number+1) || shade.Opacity != art.MaxOpacity || shade.Label != "" {
				t.Fatalf("%s %d: %+v", ramp.Name, number+1, shade)
			}
			if number > 0 && luminance(shade.Color) <= luminance(ramp.Shades[number-1].Color) {
				t.Fatalf("%s %d is not lighter than the one before", ramp.Name, number+1)
			}
		}
	}
	want := "grey red orange amber yellow lime green moss teal cyan sky blue violet magenta skin earth slate"
	if strings.Join(names, " ") != want || count != 256 || len(ramps[0].Shades) != 16 {
		t.Fatalf("default ramps %v, %d colours", names, count)
	}
	if name, _ := palette.ShadeName(art.DefaultRampID(1), art.DefaultShadeID(1, 3)); name != "grey 3" {
		t.Fatalf("shade name %q", name)
	}
	if again := defaults(t); !again.Equal(palette) {
		t.Fatal("Default must be the same every time")
	}
	for _, fixed := range []ids.ULID{
		art.DefaultPaletteID(), art.SettingsID(), art.StandInsID(), art.DefaultRampID(17), art.DefaultShadeID(17, 15),
	} {
		if fixed.IsZero() || !fixed.Time().Equal(ids.ULID{}.Time()) {
			t.Fatalf("fixed ID %s must have a zero time part", fixed)
		}
	}
}

// luminance is a colour's rough lightness, enough to check a ramp's order.
func luminance(value art.RGB) int {
	return 299*int(value>>16&0xFF) + 587*int(value>>8&0xFF) + 114*int(value&0xFF)
}

func TestLabels(t *testing.T) {
	t.Parallel()
	palette := greys(t)
	if _, err := palette.SetShade(id(20), art.Shade{ID: id(21), Color: 0x202020, Opacity: 100, Label: "mortar"}); err != nil {
		t.Fatal(err)
	}
	if ramp, shade, ok := palette.Labelled("mortar"); !ok || ramp != id(20) || shade.ID != id(21) {
		t.Fatalf("labelled: %v %+v", ramp, shade)
	}
	for name, label := range map[string]string{
		"another colour's":   "mortar",
		"sixteen letters'":   "abcdefghijklmnop",
		"an untrimmed one's": " soot",
	} {
		_, err := palette.SetShade(id(20), art.Shade{ID: id(22), Color: 0x4A4A4A, Opacity: 100, Label: label})
		if !errors.Is(err, art.ErrLabel) {
			t.Fatalf("a colour took %s label: %v", name, err)
		}
	}
	if _, err := palette.SetShade(id(20), art.Shade{ID: id(22), Color: 0x4A4A4A, Opacity: 100, Label: "fifteen letters"}); err != nil {
		t.Fatalf("fifteen characters: %v", err)
	}
	if _, err := palette.SetShade(id(20), art.Shade{ID: id(22), Color: 0x4A4A4A, Opacity: 100, Label: "épée épée épée!"}); err != nil {
		t.Fatalf("fifteen characters are counted as characters, not bytes: %v", err)
	}
	clash := art.Ramp{ID: id(30), Name: "brick", Shades: []art.Shade{{ID: id(31), Color: 1, Opacity: 100, Label: "mortar"}}}
	if err := palette.InsertRamp(1, clash); !errors.Is(err, art.ErrLabel) {
		t.Fatalf("a ramp brought a taken label: %v", err)
	}
	if err := palette.InsertShade(id(20), 0, art.Shade{ID: id(32), Color: 1, Opacity: 100, Label: "mortar"}); !errors.Is(err, art.ErrLabel) {
		t.Fatalf("a shade brought a taken label: %v", err)
	}
	data, err := palette.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := art.DecodePalette(data)
	if err != nil || !decoded.Equal(palette) {
		t.Fatalf("labels round trip: %v", err)
	}
}

func TestMoveRamp(t *testing.T) {
	t.Parallel()
	palette := greys(t)
	for at, name := range []string{"brick", "moss"} {
		if err := palette.InsertRamp(at+1, art.Ramp{ID: id(40 + at), Name: name, Shades: nil}); err != nil {
			t.Fatal(err)
		}
	}
	from, err := palette.MoveRamp(id(41), 0)
	if err != nil || from != 2 {
		t.Fatalf("move ramp: %d %v", from, err)
	}
	order := ""
	for _, ramp := range palette.Ramps() {
		order += ramp.Name + " "
	}
	if order != "moss grey brick " {
		t.Fatalf("order after a move: %q", order)
	}
	if _, err := palette.MoveRamp(id(41), 3); err == nil {
		t.Fatal("moved a ramp past the end")
	}
}

func TestMovedShadeKeepsItsOldPlace(t *testing.T) {
	t.Parallel()
	palette := greys(t)
	if err := palette.InsertRamp(1, art.Ramp{ID: id(40), Name: "murk", Shades: []art.Shade{
		{ID: id(41), Color: 0x1D2620, Opacity: 100},
	}}); err != nil {
		t.Fatal(err)
	}
	from, index, err := palette.MoveShade(id(21), id(40), 1, nil)
	if err != nil || from != id(20) || index != 0 {
		t.Fatalf("move shade: %v %d %v", from, index, err)
	}
	if name, _ := palette.ShadeName(id(40), id(21)); name != "murk 2" {
		t.Fatalf("moved shade is named %q", name)
	}
	if name, _ := palette.ShadeName(id(20), id(22)); name != "grey 1" {
		t.Fatalf("the shades after it renumber: %q", name)
	}
	shade, moved, ok := palette.Resolve(id(20), id(21))
	if !ok || !moved || shade.Color != 0x202020 {
		t.Fatalf("a tile naming grey 1 still finds it: %+v %v %v", shade, moved, ok)
	}
	if _, moved, ok := palette.Resolve(id(40), id(21)); !ok || moved {
		t.Fatal("the shade's new place is not a move")
	}
	record, ok := palette.MovedFrom(id(20), id(21))
	if !ok || record.Name != "grey 1" || len(palette.MovedShades()) != 1 {
		t.Fatalf("shadow record: %+v %v", record, ok)
	}
	if _, _, err := palette.MoveShade(id(21), id(20), 2, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := palette.MovedFrom(id(20), id(21)); ok {
		t.Fatal("coming back to a ramp drops its shadow record")
	}
	if _, ok := palette.MovedFrom(id(40), id(21)); !ok {
		t.Fatal("leaving murk keeps a record of murk")
	}
	before := palette.Clone()
	from, index, err = palette.MoveShade(id(22), id(40), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := palette.MoveShade(id(22), from, index, before.MovedShades()); err != nil {
		t.Fatal(err)
	}
	if !palette.Equal(before) {
		t.Fatal("moving back with the records as they were restores the palette exactly")
	}
	data, err := palette.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := art.DecodePalette(data)
	if err != nil || !decoded.Equal(palette) {
		t.Fatalf("shadow records round trip: %v", err)
	}
	for _, bad := range []struct {
		shade, ramp ids.ULID
		index       int
	}{{id(99), id(40), 0}, {id(22), id(99), 0}, {id(22), id(40), 5}, {id(22), id(20), 5}} {
		if _, _, err := palette.MoveShade(bad.shade, bad.ramp, bad.index, nil); err == nil {
			t.Fatalf("moved %+v", bad)
		}
	}
}

func TestRampsAndShades(t *testing.T) {
	t.Parallel()
	palette := greys(t)
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
	if err != nil || previous != "grey" {
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
	palette := greys(t)
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
	data := json.RawMessage(`{"ramps":[]}`)
	for _, format := range []string{art.PaletteFormatName, art.TileFormatName} {
		envelope := content.Envelope{Format: format, Version: 1, ID: id(1), Name: "old", Revision: 1, Data: data}
		upgraded, changed, err := registry.Upgrade(envelope)
		if err != nil || !changed || upgraded.Version != 2 || string(upgraded.Data) != string(data) {
			t.Fatalf("%s version 1 reads unchanged as version 2: %+v %v", format, upgraded, err)
		}
	}
}
