package mapview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/art"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// TestInspectorListsTheZonesPalettes lists gloomy for Town from the Map's
// inspector while nothing is selected, then takes it off again.
func TestInspectorListsTheZonesPalettes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	gloomyID := frametest.ID(30)
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"}, {ID: groundID, Name: "Ground", Format: floor.FormatName},
		{ID: upperID, Name: "Upper", Format: floor.FormatName},
		{ID: art.DefaultPaletteID(), Name: art.DefaultPaletteName, Format: art.PaletteFormatName},
		{ID: gloomyID, Name: "gloomy", Format: art.PaletteFormatName},
	}))
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.click("ui:add-zone-palette")
	if _, offered := f.state.Locate("menu:"+art.DefaultPaletteID().String(), f.ctx.Docs); offered {
		t.Fatal("Default is not offered")
	}
	f.click("menu:" + gloomyID.String())
	inserts := f.requests.Of(zoneplugin.InsertPaletteEdit)
	var args zoneplugin.PaletteArgs
	if len(inserts) != 1 || inserts[0].Decode(&args) != nil || args.Palette != gloomyID ||
		inserts[0].Command.Document != zoneID {
		t.Fatalf("Town lists gloomy: %+v", args)
	}
	f.docs.View(zoneID, func(document client.Document) {
		if err := document.Model.(*zoneplugin.Document).Zone.InsertPalette(0, gloomyID); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.Touch(zoneID)
	f.draw()
	f.click("ui:remove-zone-palette:gloomy")
	if removes := f.requests.Of(zoneplugin.RemovePaletteEdit); len(removes) != 1 || removes[0].Decode(&args) != nil ||
		args.Palette != gloomyID {
		t.Fatalf("× takes gloomy off: %+v", args)
	}
}
