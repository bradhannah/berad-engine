package browser

import (
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/templateview"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Templates (increment 16): the Templates section's [+] makes a template
// with no rules and opens it in the Template editor, and New ▾ offers a
// sprite drawn to each template, such as "Sprite: Pickup".
const (
	sectionTemplates = sectionPrefix + "templates"
	newTemplate      = "new-template"
	templateView     = templateview.ViewName
	// newDrawnPrefix prefixes a template's ID in New ▾'s items that make a
	// sprite drawn to it.
	newDrawnPrefix = "new-sprite-drawn-to:"
)

// createTemplate makes a template with no rules and opens it in the Template
// editor.
func (s *State) createTemplate(ctx *frame.Context) {
	name := uniqueName("Template", names(s.entries(ctx.Docs, artplugin.TemplateFormatName)))
	data, err := artplugin.Template{Rules: nil, Guides: nil}.Encode()
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	args := newDocument(artplugin.TemplateFormatName, name, ids.ULID{}, data)
	args.Folder = s.targetFolder(ctx.Docs, artplugin.TemplateFormatName)
	s.command(ctx, project.IndexID(), project.CreateEdit, "", args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
		ctx.Requests.Open(templateView, result.Created, false)
		ctx.Frame.Status("Made “" + name + "”. Add rules and guides in the Template editor.")
	}, nil)
}

// createDrawn makes a sprite drawn to the template a New ▾ item names.
func (s *State) createDrawn(ctx *frame.Context, value string) {
	template, err := ids.Parse(value)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	s.createSpriteDrawnTo(ctx, template)
}

// newItems lists what New ▾ makes: menuItems, with a sprite drawn to each of
// the project's templates after Sprite.
func (s *State) newItems(docs frame.Docs) []ui.MenuItem {
	items := menuItems()
	var templates []project.Entry
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			templates = index.OfFormat(artplugin.TemplateFormatName)
		}
	})
	drawn := make([]ui.MenuItem, 0, len(templates))
	for _, entry := range templates {
		drawn = append(drawn, ui.MenuItem{
			ID: newDrawnPrefix + entry.ID.String(), Label: "Sprite: " + entry.Name, Icon: ui.IconSprite,
			Disabled: false, Danger: false, Rule: false,
		})
	}
	after := slices.IndexFunc(items, func(item ui.MenuItem) bool { return item.ID == newSprite }) + 1

	return slices.Insert(items, after, drawn...)
}

// drawnItem reports whether a New ▾ item makes a sprite drawn to a template,
// and which.
func drawnItem(item string) (string, bool) {
	return strings.CutPrefix(item, newDrawnPrefix)
}
