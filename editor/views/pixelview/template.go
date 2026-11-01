package pixelview

import (
	"fmt"
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/templateview"
	"github.com/bradhannah/berad-engine/editor/views/tileart"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// A sprite drawn to a template (increment 16) shows the template's marks over
// its tile: pixels it does not allow are hatched, pixels it discourages are
// dotted, and its guides are dashed lines with their names. The pencil and
// the fill never paint a pixel the template does not allow; the eraser still
// clears one. Changing a template never erases art: the readout counts what is
// painted where the template discourages or does not allow it, Show them
// marks those pixels, and Clear them clears the ones not allowed.

// TemplateEditorView is the view Edit template… opens.
const TemplateEditorView = templateview.ViewName

// templateItem prefixes a template's ID in the Template menu; the item for
// none is the prefix alone.
const (
	templateItem     = "template:"
	editTemplateItem = "edit-template"
)

// Template widgets' widths, in screen pixels.
const (
	clearWidth    = 96
	templateWidth = 74
)

// templateID returns the template the sprite is drawn to.
func (s *State) templateID(docs frame.Docs) (ids.ULID, bool) {
	var template ids.ULID
	docs.View(s.sprite, func(document client.Document) {
		if sprite, ok := document.Model.(*artplugin.SpriteDocument); ok {
			template = sprite.Sprite.Template
		}
	})

	return template, !template.IsZero()
}

// template returns the template the sprite is drawn to, once it is loaded.
func (s *State) template(docs frame.Docs) (artplugin.Template, bool) {
	var found artplugin.Template
	loaded := false
	if templateID, ok := s.templateID(docs); ok {
		docs.View(templateID, func(document client.Document) {
			if templateDoc, isTemplate := document.Model.(*artplugin.TemplateDocument); isTemplate {
				found, loaded = templateDoc.Template.Clone(), true
			}
		})
	}

	return found, loaded
}

// templateName names the template the sprite is drawn to, or "" for none.
func (s *State) templateName(docs frame.Docs) string {
	templateID, ok := s.templateID(docs)
	if !ok {
		return ""
	}

	return documentName(docs, templateID, "its template")
}

// documentName names a document from the project index.
func documentName(docs frame.Docs, document ids.ULID, fallback string) string {
	name := fallback
	docs.View(project.IndexID(), func(found client.Document) {
		if index, ok := found.Model.(*project.Index); ok {
			name = index.Name(document, fallback)
		}
	})

	return name
}

// templates lists the project's templates, A–Z, as the index orders them.
func templates(docs frame.Docs) []project.Entry {
	var entries []project.Entry
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = index.OfFormat(artplugin.TemplateFormatName)
		}
	})

	return entries
}

// barrier says whether the current tool may paint a pixel: it may when it
// erases, when the sprite is drawn to no template, or when the template
// allows the pixel. waiting is true while the sprite's template is still
// loading, when nothing is painted.
type barrier struct {
	template artplugin.Template
	drawn    bool
	waiting  bool
	erasing  bool
	size     int
}

func (s *State) barrier(docs frame.Docs, size int) barrier {
	template, loaded := s.template(docs)
	_, drawn := s.templateID(docs)

	return barrier{template: template, drawn: drawn, waiting: drawn && !loaded, erasing: s.paintColor().IsClear(),
		size: size}
}

// blocks reports whether the tool may not paint a pixel.
func (b barrier) blocks(pixel image.Point) bool {
	if b.erasing || !b.drawn {
		return false
	}

	return b.waiting || b.template.Mark(pixel.X, pixel.Y, b.size, b.size) == artplugin.MarkNotAllowed
}

// skipped tells the author how many pixels of the stroke the template did
// not allow.
func (s *State) skipped(ctx *frame.Context, count int) {
	if count == 0 {
		return
	}
	s.stroke.skipped += count
	ctx.Frame.Status(fmt.Sprintf("%s skipped: %s does not allow painting there", pixels(s.stroke.skipped),
		s.templateName(ctx.Docs)))
}

// templatePixels returns the tile's painted pixels where the sprite's
// template discourages them and where it does not allow them.
func (s *State) templatePixels(docs frame.Docs) ([]image.Point, []image.Point) {
	template, ok := s.template(docs)
	tileID, tiled := s.tileID(docs)
	if !ok || !tiled {
		return nil, nil
	}
	var discouraged, notAllowed []image.Point
	docs.View(tileID, func(document client.Document) {
		tileDoc, isTile := document.Model.(*artplugin.TileDocument)
		if !isTile {
			return
		}
		size := tileDoc.Tile.Size()
		for row := range size {
			for col := range size {
				if ref, err := tileDoc.Tile.At(col, row); err != nil || ref.IsClear() {
					continue
				}
				switch template.Mark(col, row, size, size) {
				case artplugin.MarkDiscouraged:
					discouraged = append(discouraged, image.Pt(col, row))
				case artplugin.MarkNotAllowed:
					notAllowed = append(notAllowed, image.Pt(col, row))
				case artplugin.MarkOpen:
				}
			}
		}
	})

	return discouraged, notAllowed
}

// clearNotAllowed clears every pixel painted where the template does not
// allow it, as one undo step.
func (s *State) clearNotAllowed(ctx *frame.Context) {
	_, notAllowed := s.templatePixels(ctx.Docs)
	tileID, ok := s.tileID(ctx.Docs)
	if !ok || len(notAllowed) == 0 {
		return
	}
	cleared := make([]artplugin.Pixel, 0, len(notAllowed))
	for _, pixel := range notAllowed {
		cleared = append(cleared, artplugin.Pixel{Col: pixel.X, Row: pixel.Y, Color: clearColor()})
	}
	s.stroke.count++
	status := pixels(len(cleared)) + " cleared where " + s.templateName(ctx.Docs) + " does not allow them"
	s.command(ctx, tileID, artplugin.PaintEdit, s.gesture(), artplugin.PaintArgs{Pixels: cleared},
		func() { ctx.Frame.Status(status) })
}

// layoutTemplateChoice places the Template heading and its button above
// Shows.
func (s *State) layoutTemplateChoice(docs frame.Docs, left, width, bottom int) {
	top := bottom - ui.ButtonH
	s.templateChoice.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	s.panel.templateTop = top - ui.LineHeight
	s.templateChoice.Label = "None"
	if name := s.templateName(docs); name != "" {
		s.templateChoice.Label = name
	}
}

// openTemplates offers no template, each of the project's templates, and
// Edit template… for the sprite's own.
func (s *State) openTemplates(docs frame.Docs) {
	entries := templates(docs)
	items := make([]ui.MenuItem, 0, len(entries)+ui.Half)
	items = append(items, ui.MenuItem{ID: templateItem, Label: "None", Icon: ui.IconNone, Disabled: false,
		Danger: false, Rule: false})
	for index, entry := range entries {
		items = append(items, ui.MenuItem{
			ID: templateItem + entry.ID.String(), Label: entry.Name, Icon: ui.IconTemplate, Disabled: false,
			Danger: false, Rule: index == 0,
		})
	}
	_, drawn := s.templateID(docs)
	items = append(items, ui.MenuItem{ID: editTemplateItem, Label: "Edit template…", Icon: ui.IconNone,
		Disabled: !drawn, Danger: false, Rule: true})
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	s.templateMenu.Items = items
	s.templateMenu.Width = min(ui.MenuWidth(items, s.theme), window.Dx()-ui.Half*ui.Inset)
	s.templateMenu.OpenAt(image.Pt(s.templateChoice.Rect.Min.X, s.templateChoice.Rect.Max.Y+ui.Gap), window)
}

// updateTemplates handles the open Template menu, which takes all input.
func (s *State) updateTemplates(input *ui.Input, ctx *frame.Context) {
	chosen := s.templateMenu.Update(input)
	input.ClearAll()
	switch {
	case chosen == editTemplateItem:
		if templateID, ok := s.templateID(ctx.Docs); ok {
			ctx.Requests.Open(TemplateEditorView, templateID, false)
		}
	case strings.HasPrefix(chosen, templateItem):
		s.chooseTemplate(ctx, strings.TrimPrefix(chosen, templateItem))
	}
}

// chooseTemplate draws the sprite to a template, or to none, as one undo
// step.
func (s *State) chooseTemplate(ctx *frame.Context, value string) {
	var chosen ids.ULID
	if value != "" {
		parsed, err := ids.Parse(value)
		if err != nil {
			ctx.Frame.Status(err.Error())

			return
		}
		chosen = parsed
	}
	if current, _ := s.templateID(ctx.Docs); chosen == current {
		return
	}
	status := "Drawn to no template"
	if !chosen.IsZero() {
		status = "Drawn to " + documentName(ctx.Docs, chosen, "the template")
	}
	s.stroke.count++
	s.command(ctx, s.sprite, artplugin.UseTemplateEdit, s.gesture(), artplugin.UseTemplateArgs{Template: chosen},
		func() { ctx.Frame.Status(status) })
}

// drawTemplateChoice draws the Template heading and its button.
func (s *State) drawTemplateChoice(screen *ebiten.Image, left int) {
	if s.templateChoice.Rect.Empty() {
		return
	}
	s.theme.Heading(screen, "TEMPLATE", left, s.panel.templateTop)
	s.templateChoice.Draw(screen, s.theme)
}

// drawTemplate draws the template's marks over the tile, and its guides.
func (s *State) drawTemplate(screen *ebiten.Image, docs frame.Docs, size int) {
	template, ok := s.template(docs)
	if !ok || !s.overlay.On {
		return
	}
	rect := s.tileRect(size)
	tileart.DrawMarks(screen, template, rect, size, s.zoom)
	tileart.DrawGuides(screen, s.theme, template, rect, size, s.zoom)
}

// drawTemplateMarks outlines the pixels painted where the template
// discourages them, and those where it does not allow them.
func (s *State) drawTemplateMarks(screen *ebiten.Image, docs frame.Docs, size int) {
	discouraged, notAllowed := s.templatePixels(docs)
	for _, pixel := range discouraged {
		ui.StrokeRect(screen, s.pixelRect(pixel, size).Inset(1), hoverWidth, tileart.DiscouragedInk())
	}
	for _, pixel := range notAllowed {
		ui.StrokeRect(screen, s.pixelRect(pixel, size).Inset(1), hoverWidth, ui.ColorWarn())
	}
}

// templateProblem says what is painted where the template does not allow it,
// and templateNote what is painted where it discourages it.
func (s *State) templateProblem(docs frame.Docs) string {
	if _, notAllowed := s.templatePixels(docs); len(notAllowed) > 0 {
		return pixels(len(notAllowed)) + " " + isAre(len(notAllowed)) + " where " + s.templateName(docs) +
			" does not allow painting. Clear them, or change the template."
	}

	return ""
}

func (s *State) templateNote(docs frame.Docs) string {
	if discouraged, _ := s.templatePixels(docs); len(discouraged) > 0 {
		return pixels(len(discouraged)) + " " + isAre(len(discouraged)) + " where " + s.templateName(docs) +
			" discourages painting."
	}

	return ""
}

// isAre agrees with a count.
func isAre(count int) string {
	if count == 1 {
		return "is"
	}

	return "are"
}
