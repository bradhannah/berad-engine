package main_test

import (
	"path/filepath"
	"testing"

	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/engine/art"
)

// checkSampleTemplates checks the sample's templates (increment 16): the
// lantern is drawn to Pickup and painted only where it allows, and both
// figures are drawn to Character.
func checkSampleTemplates(t *testing.T, project string, lantern art.Sprite, tile *art.Tile) {
	t.Helper()
	pickupFile := readEnvelope(t, project, "templates/pickup--*.json")
	characterFile := readEnvelope(t, project, "templates/character--*.json")
	pickup, err := artplugin.DecodeTemplateData(pickupFile.Data)
	if err != nil || lantern.Template != pickupFile.ID || len(pickup.Rules) != 2 || len(pickup.Guides) != 2 {
		t.Fatalf("the lantern is drawn to Pickup: %+v %v", pickup, err)
	}
	size := tile.Size()
	for row := range size {
		for col := range size {
			if ref, _ := tile.At(col, row); !ref.IsClear() &&
				pickup.Mark(col, row, size, size) == artplugin.MarkNotAllowed {
				t.Fatalf("the lantern is painted at %d, %d, where Pickup does not allow it", col, row)
			}
		}
	}
	character, err := artplugin.DecodeTemplateData(characterFile.Data)
	if err != nil || len(character.Rules) != 3 || len(character.Guides) != 3 {
		t.Fatalf("Character: %+v %v", character, err)
	}
	for _, name := range []string{"hero", "clerk"} {
		figure, err := art.DecodeSprite(readEnvelope(t, project, "sprites/"+name+"--*.json").Data)
		if err != nil || figure.Template != characterFile.ID {
			t.Fatalf("%s is drawn to Character: %+v %v", name, figure, err)
		}
	}
}

// TestPixelEditorDrawsToATemplate opens the sample's lantern, drawn to
// Pickup, in a Pixel editor: a stroke from the tile's edge paints only the
// pixels Pickup allows, the first being kept clear. Its Template menu opens
// Pickup in a Template editor, which adds a third rule.
func TestPixelEditorDrawsToATemplate(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	spriteShot, templateShot := shotPath(t, "template-lantern.png"), shotPath(t, "template-editor.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("row:Lantern"), wait(30), click("ui:open"), quitWhenAlone(),
	)
	spriteScript := script(
		must("state:loaded", 600), must("state:template:Pickup", 600), wait(5),
		click("swatch:gloomy / iron 3"), must("state:color:gloomy / iron 3", 60),
		drag("pixel:0,12", "pixel:2,12", "pixel:4,12"), must("state:pixel:4,12:gloomy / iron 3", 120),
		must("state:pixel:0,12:clear", 60), must("state:painted:17,0", 120), shoot(spriteShot),
		click("ui:template-choice"), must("menu:edit-template", 60), click("menu:edit-template"), wait(60), quit(),
	)
	templateScript := script(
		must("state:loaded", 600), must("state:rules:2", 60), wait(5),
		click("ui:add-rule"), must("state:rules:3", 120), click("ui:rule-3-edge"), wait(30), shoot(templateShot),
		wait(60), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "sprite": {spriteScript}, "template": {templateScript},
	})
	noCrashes(t, project)

	tile, err := art.DecodeTile(readEnvelope(t, project, "tiles/lantern--*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	for col := range 5 {
		if ref, _ := tile.At(col, 12); ref.IsClear() != (col == 0) {
			t.Fatalf("pixel %d, 12: the stroke skips the edge Pickup keeps clear: %+v", col, ref)
		}
	}
	pickup, err := artplugin.DecodeTemplateData(readEnvelope(t, project, "templates/pickup--*.json").Data)
	if err != nil || len(pickup.Rules) != 3 || pickup.Rules[2].Edge != artplugin.EdgeSides {
		t.Fatalf("the Template editor added a rule of the sides: %+v %v", pickup, err)
	}
	t.Logf("screenshots: %s %s", spriteShot, templateShot)
}
