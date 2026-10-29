package paletteview

import (
	"fmt"
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Delete panel geometry, in screen pixels.
const (
	deleteWidth  = 620
	useHeight    = 52
	removeWidth  = 168
	deleteHeader = 72
)

// deletePanel asks before the palette is deleted. It lists every zone that
// lists the palette and every tile painted with it, each with a way to
// remove the reference; the palette is deleted only once none is left.
type deletePanel struct {
	uses    []protocol.Use
	known   bool
	removes []*ui.Button
	all     *ui.Button
	cancel  *ui.Button
	confirm *ui.Button
	rect    image.Rectangle
}

// openDeleting asks what refers to the palette and opens the panel.
func (s *State) openDeleting(ctx *frame.Context) {
	if s.ReadOnly() {
		return
	}
	s.gradient.open, s.copying.open = false, false
	s.deleting = &deletePanel{
		uses: nil, known: false, removes: nil, all: ui.NewButton("remove-all", "Remove all"),
		cancel: ui.NewButton("cancel-delete", "Cancel"), confirm: ui.NewButton("confirm-delete", "Delete palette"),
		rect: ui.Zero(),
	}
	s.deleting.confirm.Danger = true
	s.askDeleting(ctx)
}

// askDeleting asks again what refers to the palette, and holds the tiles
// and sprites a removal needs.
func (s *State) askDeleting(ctx *frame.Context) {
	panel := s.deleting
	ctx.Requests.WhereUsed(s.target, "", func(uses []protocol.Use, err error) {
		if err != nil || s.deleting != panel {
			return
		}
		panel.uses, panel.known = distinct(uses), true
		for _, use := range panel.uses {
			s.subscribe(use.Document, ctx)
		}
		for _, sprite := range spriteEntries(ctx.Docs) {
			s.subscribe(sprite, ctx)
		}
	})
}

// distinct keeps each document once.
func distinct(uses []protocol.Use) []protocol.Use {
	var kept []protocol.Use
	seen := map[ids.ULID]bool{}
	for _, use := range uses {
		if !seen[use.Document] {
			seen[use.Document] = true
			kept = append(kept, use)
		}
	}

	return kept
}

// spriteEntries lists the project's sprites, to name a tile by its sprite.
func spriteEntries(docs frame.Docs) []ids.ULID {
	var sprites []ids.ULID
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			for _, entry := range index.OfFormat(art.SpriteFormatName) {
				sprites = append(sprites, entry.ID)
			}
		}
	})

	return sprites
}

// useName names a reference as the author knows it: a zone by its name, a
// tile by its sprite's.
func useName(docs frame.Docs, use protocol.Use) string {
	if use.Format != art.TileFormatName {
		return use.Name
	}
	for _, sprite := range spriteEntries(docs) {
		shows := false
		docs.View(sprite, func(document client.Document) {
			spriteDoc, ok := document.Model.(*artplugin.SpriteDocument)
			shows = ok && spriteDoc.Sprite.Tile == use.Document
		})
		if shows {
			return documentName(docs, sprite, use.Name)
		}
	}

	return use.Name
}

// layoutDeleting places the delete panel in the middle of the window.
func (s *State) layoutDeleting() {
	panel := s.deleting
	if panel == nil {
		return
	}
	height := deleteHeader + max(1, len(panel.uses))*useHeight + ui.ButtonH + ui.Half*ui.Inset
	width := min(deleteWidth, s.size.X-ui.Half*ui.Inset)
	corner := image.Pt((s.size.X-width)/ui.Half, max(ui.Inset, (s.size.Y-height)/ui.Half))
	panel.rect = image.Rectangle{Min: corner, Max: corner.Add(image.Pt(width, height))}
	for len(panel.removes) < len(panel.uses) {
		panel.removes = append(panel.removes, ui.NewButton("remove-use-"+strconv.Itoa(len(panel.removes)+1), ""))
	}
	panel.removes = panel.removes[:len(panel.uses)]
	for at, use := range panel.uses {
		top := panel.rect.Min.Y + deleteHeader + at*useHeight
		button := panel.removes[at]
		button.Label = "Remove from tile"
		if use.Format == zone.FormatName {
			button.Label = "Remove from zone"
		}
		button.Rect = image.Rect(panel.rect.Max.X-ui.Inset-removeWidth, top, panel.rect.Max.X-ui.Inset, top+ui.ButtonH)
	}
	bottom := panel.rect.Max.Y - ui.Inset - ui.ButtonH
	s.place(panel.all, panel.rect.Min.X+ui.Inset, bottom)
	panel.all.Disabled = len(panel.uses) == 0
	right := panel.rect.Max.X - ui.Inset - s.buttonWidth(panel.confirm)
	s.place(panel.confirm, right, bottom)
	s.place(panel.cancel, right-ui.Gap-s.buttonWidth(panel.cancel), bottom)
	panel.confirm.Disabled = !panel.known || len(panel.uses) > 0
}

// updateDeleting handles the delete panel, which takes all input.
func (s *State) updateDeleting(input *ui.Input, ctx *frame.Context) {
	panel := s.deleting
	switch {
	case panel.cancel.Update(input) || input.Consume(ui.KeyEscape, ui.Plain()):
		s.deleting = nil
	case panel.confirm.Update(input):
		s.deletePalette(ctx)
	case panel.all.Update(input):
		gesture := s.gesture()
		for _, use := range panel.uses {
			s.removeUse(ctx, use, gesture)
		}
	default:
		for at, button := range panel.removes {
			if button.Update(input) {
				s.removeUse(ctx, panel.uses[at], s.gesture())
			}
		}
	}
	input.ClearAll()
}

// removeUse takes the palette off a zone's list, or out of a tile: each of
// the tile's pixels keeps its colour, as Default's exact match or else as a
// loose colour, which the tile warns about until it is repainted.
func (s *State) removeUse(ctx *frame.Context, use protocol.Use, gesture string) {
	asked := func(protocol.Result) {
		if s.deleting != nil {
			s.askDeleting(ctx)
		}
	}
	switch use.Format {
	case zone.FormatName:
		args := zoneplugin.PaletteArgs{Palette: s.target, Index: 0}
		s.command(ctx, use.Document, zoneplugin.RemovePaletteEdit, gesture, args, asked, nil)
	case art.TileFormatName:
		pixels, matched, loose := s.dropped(ctx.Docs, use.Document)
		if len(pixels) == 0 {
			return
		}
		args := artplugin.PaintArgs{Pixels: pixels}
		name := useName(ctx.Docs, use)
		s.command(ctx, use.Document, artplugin.PaintEdit, gesture, args, func(result protocol.Result) {
			ctx.Frame.Status(fmt.Sprintf("Removed %s from %s: %d pixels are Default's colours, %d are loose",
				s.paletteName(ctx.Docs), name, matched, loose))
			asked(result)
		}, nil)
	}
}

// dropped returns the pixels that change when the palette is removed from a
// tile, and how many become Default's colours and how many loose ones.
func (s *State) dropped(docs frame.Docs, tileID ids.ULID) ([]artplugin.Pixel, int, int) {
	palette, ok := s.palette(docs)
	defaults, found := paletteOf(docs, art.DefaultPaletteID())
	if !ok || !found {
		return nil, 0, 0
	}
	var pixels []artplugin.Pixel
	matched, loose := 0, 0
	docs.View(tileID, func(document client.Document) {
		tileDoc, isTile := document.Model.(*artplugin.TileDocument)
		if !isTile {
			return
		}
		changes := art.Dropped(tileDoc.Tile, s.target, palette, defaults)
		size := tileDoc.Tile.Size()
		for row := range size {
			for col := range size {
				ref, err := tileDoc.Tile.At(col, row)
				replacement, hit := changes[ref]
				if err != nil || !hit {
					continue
				}
				pixels = append(pixels, artplugin.Pixel{Col: col, Row: row, Color: replacement})
				if replacement.IsLoose() {
					loose++
				} else if !replacement.IsClear() {
					matched++
				}
			}
		}
	})

	return pixels, matched, loose
}

// deletePalette deletes the palette once nothing refers to it.
func (s *State) deletePalette(ctx *frame.Context) {
	name := s.paletteName(ctx.Docs)
	args := project.DocumentArgs{
		ID: s.target, Format: "", Name: "", Version: 0, Revision: 0, Scope: ids.ULID{}, Folder: "", Tags: nil,
		Data: nil, Layout: nil,
	}
	s.command(ctx, project.IndexID(), project.DeleteEdit, "", args, func(protocol.Result) {
		s.deleting = nil
		ctx.Frame.Status("Deleted the palette “" + name + "”. Undo brings it back.")
	}, nil)
}

// drawDeleting draws the delete panel over the window.
func (s *State) drawDeleting(screen *ebiten.Image, docs frame.Docs) {
	panel := s.deleting
	if panel == nil {
		return
	}
	ui.FillRect(screen, image.Rectangle{Min: image.Pt(0, 0), Max: s.size}, ui.ColorShade())
	ui.Box(screen, panel.rect, ui.ColorBackground())
	left, top := panel.rect.Min.X+ui.Inset, panel.rect.Min.Y+ui.Inset
	s.theme.DrawTitle(screen, "Delete "+s.paletteName(docs)+"?", left, top, ui.ColorInk())
	lead := "Each zone and tile that refers to it is removed first, and every removal undoes."
	switch {
	case !panel.known:
		lead = "Looking for what refers to it…"
	case len(panel.uses) == 0:
		lead = "Nothing refers to it any more, so it can be deleted."
	}
	s.theme.DrawText(screen, s.theme.FitText(lead, panel.rect.Dx()-ui.Half*ui.Inset), left, top+ui.LineHeight+ui.Gap,
		ui.ColorMuted())
	for at, use := range panel.uses {
		row := panel.rect.Min.Y + deleteHeader + at*useHeight
		s.theme.DrawStrong(screen, useName(docs, use), left, row, ui.ColorInk())
		s.theme.DrawText(screen, s.useDetail(docs, use), left, row+ui.LineHeight, ui.ColorMuted())
		panel.removes[at].Draw(screen, s.theme)
	}
	for _, button := range []*ui.Button{panel.all, panel.cancel, panel.confirm} {
		button.Draw(screen, s.theme)
	}
}

// useDetail says what removing a reference does.
func (s *State) useDetail(docs frame.Docs, use protocol.Use) string {
	if use.Format == zone.FormatName {
		return "lists it, so its art offers it first"
	}
	pixels, matched, loose := s.dropped(docs, use.Document)

	return fmt.Sprintf("%d pixels painted with it: %d match Default exactly, %d would be loose", len(pixels), matched,
		loose)
}
