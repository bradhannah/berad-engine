package pixelview_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/pixelview"
	"github.com/bradhannah/berad-engine/engine/art"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// headings returns the palettes' headings from top to bottom, folded ones
// marked with "+".
func (f *fixture) headings() []string {
	f.t.Helper()
	var found []string
	for _, name := range []string{"aardvark", "Default", "gloomy", "sunny"} {
		point, ok := f.state.Locate("palette:"+name, f.docs)
		if !ok {
			continue
		}
		found = append(found, name)
		_ = point
	}
	slices.SortFunc(found, func(left, right string) int { return f.at("palette:"+left).Y - f.at("palette:"+right).Y })
	for at, name := range found {
		if _, open := f.state.Locate("ramp:"+name+" / "+firstRamp(name), f.docs); !open {
			found[at] = "+" + name
		}
	}

	return found
}

func firstRamp(palette string) string {
	switch palette {
	case "Default":
		return "grey"
	case "gloomy":
		return "grey"
	}

	return "sand"
}

// addPalettes adds two more palettes, sunny and aardvark, each with one ramp
// "sand", and a zone with one floor that lists sunny.
func (f *fixture) addPalettes(placed bool) {
	f.t.Helper()
	sunnyID, aardvarkID, zoneID, floorID := frametest.ID(40), frametest.ID(41), frametest.ID(42), frametest.ID(43)
	entries := []project.Entry{
		{ID: spriteID, Name: "Lantern", Format: art.SpriteFormatName},
		{ID: art.DefaultPaletteID(), Name: art.DefaultPaletteName, Format: art.PaletteFormatName},
		{ID: gloomyID, Name: "gloomy", Format: art.PaletteFormatName},
		{ID: sunnyID, Name: "sunny", Format: art.PaletteFormatName},
		{ID: aardvarkID, Name: "aardvark", Format: art.PaletteFormatName},
		{ID: zoneID, Name: "Test zone", Format: "zone"},
	}
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(entries))
	for at, id := range []byte{40, 41} {
		palette := art.NewPalette()
		ramp := art.Ramp{ID: frametest.ID(50 + id), Name: "sand", Shades: []art.Shade{
			{ID: frametest.ID(60 + id), Color: art.RGB(0xC2AE6A + at), Opacity: 100},
		}}
		if err := palette.InsertRamp(0, ramp); err != nil {
			f.t.Fatal(err)
		}
		f.docs.Put(frametest.ID(id), entries[3+at].Name, &artplugin.PaletteDocument{Palette: palette})
	}
	zoneDoc := zoneplugin.NewDocument()
	if err := zoneDoc.Zone.Insert(0, floorID); err != nil {
		f.t.Fatal(err)
	}
	if err := zoneDoc.Zone.InsertPalette(0, sunnyID); err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(zoneID, "Test zone", zoneDoc)
	grid, err := floor.New(4, 4, "grass")
	if err != nil {
		f.t.Fatal(err)
	}
	if placed {
		bounds := grid.Bounds()
		if _, err := grid.SetTop(bounds.Min.X+1, bounds.Min.Y+1, "pickup.small"); err != nil {
			f.t.Fatal(err)
		}
		f.docs.View(art.StandInsID(), func(document client.Document) {
			if _, err := document.Model.(*artplugin.StandInsDocument).Sprites.Set("pickup.small", spriteID); err != nil {
				f.t.Fatal(err)
			}
		})
	}
	f.docs.Put(floorID, "Floor 1", &tilefloor.Document{Floor: grid})
	f.state.Changed(client.Update{Event: nil, Snapshot: nil}, f.ctx)
	f.state.Start(f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestPanelOffersPalettesInOrder offers the palettes of the zones the sprite
// is placed in first, A to Z, then Default, then the rest A to Z, folded; a
// sprite placed in no zone shows Default first. A heading folds and opens its
// palette.
func TestPanelOffersPalettesInOrder(t *testing.T) {
	t.Parallel()
	unplaced := newFixture(t)
	unplaced.addPalettes(false)
	if got := strings.Join(unplaced.headings(), " "); got != "Default +aardvark gloomy +sunny" {
		t.Fatalf("placed nowhere: %s", got)
	}
	placed := newFixture(t)
	placed.addPalettes(true)
	if got := strings.Join(placed.headings(), " "); got != "sunny Default +aardvark gloomy" {
		t.Fatalf("placed in Test zone, which lists sunny: %s", got)
	}
	placed.click("palette:sunny")
	placed.click("palette:aardvark")
	if got := strings.Join(placed.headings(), " "); got != "+sunny Default aardvark gloomy" {
		t.Fatalf("headings fold and open: %s", got)
	}
}

// TestEditPaletteOpensThePaletteEditor opens the chosen colour's palette in
// the Palette Editor, at that colour; with clear chosen it is disabled.
func TestEditPaletteOpensThePaletteEditor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:edit-palette")
	if len(f.requests.Opened) != 0 {
		t.Fatal("Edit palette… does nothing with clear chosen")
	}
	f.click("swatch:gloomy / grey 3")
	f.click("ui:edit-palette")
	if len(f.requests.Opened) != 1 {
		t.Fatalf("opened %+v", f.requests.Opened)
	}
	opened := f.requests.Opened[0]
	if opened.View != pixelview.PaletteEditorView || opened.Target != gloomyID || opened.Item != grey(3).Shade.String() {
		t.Fatalf("opens gloomy at grey 3: %+v", opened)
	}
	f.click("swatch:Default / slate 4")
	if !f.has("color:Default / slate 4") {
		t.Fatal("Default's colours are offered")
	}
}

// TestWarnsAboutMovedAndLooseColours says how many pixels name a colour that
// has moved, or are loose colours, until they are repainted; Show them marks
// them. The eyedropper picks a moved colour at its new place and refuses a
// loose one.
func TestWarnsAboutMovedAndLooseColours(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.setPixels(grey(3), image.Pt(0, 0), image.Pt(1, 0))
	f.docs.View(gloomyID, func(document client.Document) {
		if _, _, err := document.Model.(*artplugin.PaletteDocument).Palette.MoveShade(grey(3).Shade, glassRampID, 1, nil); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.Touch(gloomyID)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if moved, loose := f.state.Warned(f.docs); moved != 2 || loose != 0 {
		t.Fatalf("2 pixels name a moved colour: %d %d", moved, loose)
	}
	if !strings.HasPrefix(f.state.Problem(f.docs), "2 pixels name a colour that has moved") {
		t.Fatalf("problem %q", f.state.Problem(f.docs))
	}
	f.click("ui:show-them")
	f.keys("I")
	f.click("pixel:0,0")
	if f.state.Color() != art.Named(gloomyID, glassRampID, grey(3).Shade) || !f.has("color:gloomy / glass 2") {
		t.Fatalf("the eyedropper picks the moved colour at its new place: %+v", f.state.Color())
	}
	f.setPixels(art.Loose(0x5D5A63, 100), image.Pt(0, 0), image.Pt(1, 0), image.Pt(2, 0))
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !strings.HasPrefix(f.state.Problem(f.docs), "3 pixels are loose colours") {
		t.Fatalf("problem %q", f.state.Problem(f.docs))
	}
	f.click("pixel:2,0")
	if f.state.Color() == art.Loose(0x5D5A63, 100) || !strings.Contains(f.status(), "names no palette") {
		t.Fatalf("a loose colour is not picked: %q", f.status())
	}
	if !f.has("pixel:2,0:loose #5D5A63") {
		t.Fatal("a loose pixel is named by its value")
	}
}
