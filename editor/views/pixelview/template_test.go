package pixelview_test

import (
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/pixelview"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

var (
	pickupID = frametest.ID(40)
	figureID = frametest.ID(41)
)

// pickup keeps the outermost ring of an 8-pixel tile clear, 28 pixels, and
// discourages the next ring, 20 pixels.
func pickup() artplugin.Template {
	return artplugin.Template{
		Rules: []artplugin.Rule{
			{Edge: artplugin.EdgeBorder, Width: 1, Mark: artplugin.MarkNotAllowed},
			{Edge: artplugin.EdgeBorder, Width: 2, Mark: artplugin.MarkDiscouraged},
		},
		Guides: []artplugin.Guide{{Name: "centre", Line: artplugin.LineDown, From: artplugin.FromMiddle, At: 0}},
	}
}

// withTemplates adds the templates Pickup and Figure to the fixture's
// project, and draws Lantern to one of them, or to none.
func withTemplates(t *testing.T, drawnTo ids.ULID) *fixture {
	t.Helper()
	f := newFixture(t)
	f.docs.View(project.IndexID(), func(document client.Document) {
		index := document.Model.(*project.Index)
		entries := append(index.Entries(),
			project.Entry{ID: pickupID, Name: "Pickup", Format: artplugin.TemplateFormatName},
			project.Entry{ID: figureID, Name: "Figure", Format: artplugin.TemplateFormatName})
		*index = *project.NewIndex(entries)
	})
	f.docs.Touch(project.IndexID())
	f.docs.Put(pickupID, "Pickup", &artplugin.TemplateDocument{Template: pickup()})
	f.docs.Put(figureID, "Figure", &artplugin.TemplateDocument{Template: artplugin.Template{Rules: []artplugin.Rule{
		{Edge: artplugin.EdgeSides, Width: 2, Mark: artplugin.MarkNotAllowed},
	}, Guides: nil}})
	f.docs.View(spriteID, func(document client.Document) {
		document.Model.(*artplugin.SpriteDocument).Sprite.Template = drawnTo
	})
	f.docs.Touch(spriteID)
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

func TestPencilSkipsWhatTheTemplateDoesNotAllow(t *testing.T) {
	t.Parallel()
	f := withTemplates(t, pickupID)
	if !f.has("template:Pickup") {
		t.Fatal("the sprite is drawn to Pickup")
	}
	f.click("swatch:gloomy / grey 3")
	f.drag("pixel:0,3", "pixel:7,3")
	pixels, gestures := f.paints()
	if len(pixels) != 6 || len(gestures) != 1 {
		t.Fatalf("the stroke paints the 6 pixels Pickup allows, as one step: %d %v", len(pixels), gestures)
	}
	for _, pixel := range pixels {
		if pixel.Col == 0 || pixel.Col == 7 {
			t.Fatalf("the pencil painted %+v, which Pickup does not allow", pixel)
		}
	}
	if status := f.status(); status != "2 pixels skipped: Pickup does not allow painting there" {
		t.Fatalf("status %q", status)
	}
	if !f.has("painted:2,0") || f.state.Problem(f.docs) != "" ||
		!strings.Contains(f.state.Note(f.docs), "2 pixels are where Pickup discourages painting") {
		t.Fatal("the two pixels of the second ring are counted as discouraged")
	}

	f.setPixels(grey(1), image.Pt(0, 0))
	f.click("ui:tool-eraser")
	f.drag("pixel:0,0")
	if f.pixel(0, 0) != (art.ColorRef{}) {
		t.Fatal("the eraser clears a pixel the template does not allow")
	}
}

func TestFillStopsAtWhatTheTemplateDoesNotAllow(t *testing.T) {
	t.Parallel()
	f := withTemplates(t, pickupID)
	f.click("swatch:gloomy / grey 4")
	f.keys("G")
	f.click("pixel:0,0")
	if len(f.requests.Of(artplugin.PaintEdit)) != 0 || f.status() != "Refused: Pickup does not allow painting there" {
		t.Fatalf("a fill started where it is not allowed: %q", f.status())
	}
	f.click("pixel:3,3")
	f.applyAll()
	pixels, _ := f.paints()
	if len(pixels) != 36 {
		t.Fatalf("the fill paints the 36 pixels inside the ring Pickup keeps clear, not %d", len(pixels))
	}
	if f.pixel(0, 4) != (art.ColorRef{}) || f.pixel(1, 4) != grey(4) || !f.has("painted:20,0") {
		t.Fatal("the fill stopped at the ring and filled the discouraged pixels")
	}
}

func TestChangingTheTemplateKeepsTheArt(t *testing.T) {
	t.Parallel()
	f := withTemplates(t, ids.ULID{})
	f.setPixels(grey(2), image.Pt(0, 2), image.Pt(1, 2), image.Pt(4, 4))
	if !f.has("template:") || f.state.Problem(f.docs) != "" {
		t.Fatal("a sprite drawn to no template has nothing to say about it")
	}
	f.click("ui:template-choice")
	if _, offered := f.state.Locate("menu:"+pixelview.EditTemplateItem(), f.docs); !offered {
		t.Fatal("the menu offers Edit template…")
	}
	f.click("menu:template:" + pickupID.String())
	f.applyAll()
	if !f.has("template:Pickup") || f.status() != "Drawn to Pickup" {
		t.Fatalf("drawn to Pickup: %q", f.status())
	}
	if f.pixel(0, 2) != grey(2) || !f.has("painted:1,1") {
		t.Fatal("changing the template keeps the art and counts it")
	}
	if problem := f.state.Problem(f.docs); problem != "1 pixel is where Pickup does not allow painting. "+
		"Clear them, or change the template." {
		t.Fatalf("problem %q", problem)
	}
	before := len(f.requests.Commands)
	f.click("ui:clear-them")
	sent := f.requests.Commands[before:]
	var cleared artplugin.PaintArgs
	if len(sent) != 1 || sent[0].Decode(&cleared) != nil || len(cleared.Pixels) != 1 ||
		cleared.Pixels[0] != (artplugin.Pixel{Col: 0, Row: 2, Color: art.ColorRef{}}) {
		t.Fatalf("Clear them clears the one pixel in one step: %+v", sent)
	}
	f.applyAll()
	if !f.has("painted:1,0") || f.state.Problem(f.docs) != "" {
		t.Fatal("cleared")
	}
	f.click("ui:template-choice")
	f.click("menu:template:")
	f.applyAll()
	if !f.has("template:") || f.status() != "Drawn to no template" {
		t.Fatalf("drawn to none: %q", f.status())
	}
}

func TestEditTemplateOpensTheTemplateEditor(t *testing.T) {
	t.Parallel()
	f := withTemplates(t, figureID)
	f.click("ui:template-choice")
	f.click("menu:" + pixelview.EditTemplateItem())
	if len(f.requests.Opened) != 1 || f.requests.Opened[0].View != pixelview.TemplateEditorView ||
		f.requests.Opened[0].Target != figureID {
		t.Fatalf("opened %+v", f.requests.Opened)
	}
}

func TestADeletedTemplateCountsAsNone(t *testing.T) {
	t.Parallel()
	f := withTemplates(t, frametest.ID(42))
	if !f.has("template:") {
		t.Fatal("a sprite drawn to a template the project no longer lists is drawn to none")
	}
	f.click("swatch:gloomy / grey 3")
	f.drag("pixel:0,0")
	if f.pixel(0, 0) != grey(3) {
		t.Fatal("the pencil paints anywhere")
	}
}
