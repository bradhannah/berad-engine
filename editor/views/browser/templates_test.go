package browser_test

import (
	"image"
	"slices"
	"testing"

	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/art"
)

var pickupID = frametest.ID(62)

// withTemplate adds the template Pickup to the fixture's art.
func withTemplate(f *fixture) {
	f.t.Helper()
	withArt(f)
	docs := f.ctx.Docs.(*frametest.Docs)
	putArtIndex(docs, project.Entry{ID: pickupID, Name: "Pickup", Format: artplugin.TemplateFormatName})
	docs.Put(pickupID, "Pickup", &artplugin.TemplateDocument{Template: artplugin.Template{Rules: []artplugin.Rule{
		{Edge: artplugin.EdgeBorder, Width: 1, Mark: artplugin.MarkNotAllowed},
	}, Guides: nil}})
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

func TestTemplatesSectionMakesAndOpensTemplates(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withTemplate(f)
	var section ui.Row
	for _, row := range f.state.Places(f.ctx.Docs) {
		if row.ID == "section:templates" {
			section = row
		}
	}
	if section.Label != "Templates" || section.Detail != "1" || !section.Add {
		t.Fatalf("the Templates section has a count and a [+]: %+v", section)
	}
	if got := f.rowsIn("section:templates"); !slices.Equal(got, []string{"Pickup/0"}) {
		t.Fatalf("templates %v", got)
	}
	if pickup, _ := f.row(pickupID.String()); pickup.Icon != ui.IconTemplate ||
		ui.IconColor(ui.IconTemplate) != ui.ColorArt() {
		t.Fatalf("a template has its own icon in the art colour: %+v", pickup)
	}
	f.click("row:Pickup", true)
	if opened := f.requests.Opened; len(opened) != 1 || opened[0].View != "template" || opened[0].Target != pickupID {
		t.Fatalf("double-clicking opens the Template editor: %+v", opened)
	}
	f.click("ui:new-template", false)
	creates := f.requests.Of(project.CreateEdit)
	var args project.DocumentArgs
	if len(creates) != 1 || creates[0].Decode(&args) != nil || args.Format != artplugin.TemplateFormatName ||
		args.Name != "Template 1" || string(args.Data) != `{"rules":[],"guides":[]}` {
		t.Fatalf("a template with no rules: %+v", args)
	}
	made := frametest.ID(73)
	creates[0].Succeed(made)
	if opened := f.requests.Opened; len(opened) != 2 || opened[1].View != "template" || opened[1].Target != made {
		t.Fatalf("it opens in the Template editor: %+v", opened)
	}
}

func TestNewOffersASpriteDrawnToEachTemplate(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withTemplate(f)
	f.click("ui:new", false)
	f.click("menu:new-sprite-drawn-to:"+pickupID.String(), false)
	creates := f.requests.Of(project.CreateEdit)
	if len(creates) != 1 {
		t.Fatalf("creates %d", len(creates))
	}
	newTile := frametest.ID(74)
	creates[0].Succeed(newTile)
	creates = f.requests.Of(project.CreateEdit)
	var spriteArgs project.DocumentArgs
	if len(creates) != 2 || creates[1].Decode(&spriteArgs) != nil {
		t.Fatalf("creates %d", len(creates))
	}
	sprite, err := art.DecodeSprite(spriteArgs.Data)
	if err != nil || sprite.Tile != newTile || sprite.Template != pickupID {
		t.Fatalf("the sprite is drawn to Pickup: %+v %v", sprite, err)
	}
	creates[1].Succeed(frametest.ID(75))
	if f.status() != "Created “Sprite 1”, drawn to Pickup, with one tile, 24 × 24 pixels" {
		t.Fatalf("status %q", f.status())
	}
}
