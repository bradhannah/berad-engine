package browser

import (
	"fmt"
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/paletteview"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Palettes (increment 12): the Palettes section's [+] makes an empty palette
// and opens it in the Palette Editor; Default is marked and keeps its name;
// a zone's inspector lists the palettes it uses; and a sprite's inspector
// lists the palettes its tile is painted with, each of which can be removed
// from it, every pixel keeping its colour.
const (
	newPalette   = "new-palette"
	paletteView  = paletteview.ViewName
	defaultNote  = "default · read only"
	removePrefix = "remove-palette:"
)

// spritePalette is a palette a sprite's tile is painted with, and the button
// that removes it from the tile.
type spritePalette struct {
	palette ids.ULID
	name    string
	remove  *ui.Button
}

// createPalette makes an empty palette and opens it in the Palette Editor.
func (s *State) createPalette(ctx *frame.Context) {
	name := uniqueName("Palette", names(s.entries(ctx.Docs, art.PaletteFormatName)))
	data, err := art.NewPalette().Encode()
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	args := newDocument(art.PaletteFormatName, name, ids.ULID{}, data)
	args.Folder = s.targetFolder(ctx.Docs, art.PaletteFormatName)
	s.command(ctx, project.IndexID(), project.CreateEdit, "", args, func(result protocol.Result, _ error) {
		s.list.Selected = result.Created.String()
		ctx.Requests.Open(paletteView, result.Created, false)
		ctx.Frame.Status("Made “" + name + "”. Add ramps in the Palette Editor, or copy colours from Default.")
	}, nil)
}

// paletteDetail marks Default in the list.
func paletteDetail(entry project.Entry) string {
	if entry.ID == art.DefaultPaletteID() {
		return defaultNote
	}

	return ""
}

// layoutArt places what a zone's or a sprite's inspector adds below its
// actions: the zone's palettes, or the palettes the sprite's tile uses.
func (s *State) layoutArt(docs frame.Docs, entry project.Entry, left, width, top int) int {
	switch entry.Format {
	case zone.FormatName:
		return s.zonePalettes.Layout(docs, entry.ID, left, top+ui.ButtonH+ui.Inset, width, s.size)
	case art.SpriteFormatName:
		return s.layoutSpritePalettes(docs, entry.ID, left, top+ui.ButtonH+ui.Inset, width)
	}

	return top
}

// layoutSpritePalettes lists the palettes the sprite's tile is painted
// with, Default aside, each with Remove.
func (s *State) layoutSpritePalettes(docs frame.Docs, spriteID ids.ULID, left, top, width int) int {
	s.spritePalettes = nil
	tileID, ok := s.tileOf(docs, spriteID)
	if !ok {
		return top
	}
	var used []ids.ULID
	docs.View(tileID, func(document client.Document) {
		if tileDoc, isTile := document.Model.(*artplugin.TileDocument); isTile {
			for _, ref := range tileDoc.Tile.Colors() {
				if !ref.IsLoose() && ref.Palette != art.DefaultPaletteID() && !slices.Contains(used, ref.Palette) {
					used = append(used, ref.Palette)
				}
			}
		}
	})
	index := s.index(docs)
	top += ui.LineHeight
	for _, paletteID := range used {
		name := index.Name(paletteID, "a palette that is gone")
		remove := ui.NewButton(removePrefix+name, "Remove…")
		remove.Rect = image.Rect(left+width-removeWidth, top, left+width, top+ui.ButtonH)
		s.spritePalettes = append(s.spritePalettes, spritePalette{palette: paletteID, name: name, remove: remove})
		top += ui.ButtonH + ui.Gap
	}

	return top
}

// removeWidth is how wide a palette's Remove… is.
const removeWidth = 96

// syncArt holds what a selected sprite's inspector needs: its tile, the
// palettes the tile is painted with, and Default.
func (s *State) syncArt(ctx *frame.Context) {
	entry, ok := s.selected(ctx.Docs)
	if !ok || entry.Format != art.SpriteFormatName {
		return
	}
	tileID, found := s.tileOf(ctx.Docs, entry.ID)
	if !found {
		return
	}
	s.subscribe(tileID, ctx)
	s.subscribe(art.DefaultPaletteID(), ctx)
	for _, used := range s.spritePalettes {
		s.subscribe(used.palette, ctx)
	}
}

// updateArt handles the zone's palettes and the sprite's Remove…, and
// reports whether it handled the input.
func (s *State) updateArt(input *ui.Input, ctx *frame.Context) bool {
	if s.zonePalettes.Update(input, ctx) {
		return true
	}
	for _, used := range s.spritePalettes {
		if used.remove.Update(input) {
			s.removePalette(ctx, used)

			return true
		}
	}

	return false
}

// removePalette removes a palette from the selected sprite's tile, as one
// undo step: each pixel keeps its colour, as Default's exact match or else
// as a loose colour, which the Pixel editor warns about until it is
// repainted.
func (s *State) removePalette(ctx *frame.Context, used spritePalette) {
	entry, ok := s.selected(ctx.Docs)
	tileID, found := s.tileOf(ctx.Docs, entry.ID)
	palette, held := paletteOf(ctx.Docs, used.palette)
	defaults, has := paletteOf(ctx.Docs, art.DefaultPaletteID())
	if !ok || !found || !held || !has {
		ctx.Frame.Status("Still loading the sprite's colours; try again")

		return
	}
	var pixels []artplugin.Pixel
	matched, loose := 0, 0
	ctx.Docs.View(tileID, func(document client.Document) {
		tileDoc, isTile := document.Model.(*artplugin.TileDocument)
		if !isTile {
			return
		}
		changes := art.Dropped(tileDoc.Tile, used.palette, palette, defaults)
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
	if len(pixels) == 0 {
		return
	}
	status := fmt.Sprintf("Removed %s from %s: %d pixels are Default's colours, %d are loose", used.name, entry.Name,
		matched, loose)
	s.command(ctx, tileID, artplugin.PaintEdit, s.gesture(), artplugin.PaintArgs{Pixels: pixels},
		func(protocol.Result, error) { ctx.Frame.Status(status) }, nil)
}

// paletteOf returns a copy of a palette.
func paletteOf(docs frame.Docs, paletteID ids.ULID) (*art.Palette, bool) {
	var found *art.Palette
	docs.View(paletteID, func(document client.Document) {
		if paletteDoc, ok := document.Model.(*artplugin.PaletteDocument); ok {
			found = paletteDoc.Palette.Clone()
		}
	})

	return found, found != nil
}

// drawArt draws the zone's palettes, or the palettes the sprite's tile uses.
func (s *State) drawArt(screen *ebiten.Image, left int) {
	s.zonePalettes.Draw(screen)
	if len(s.spritePalettes) == 0 {
		return
	}
	top := s.spritePalettes[0].remove.Rect.Min.Y
	s.theme.Heading(screen, "PALETTES ITS TILE USES", left, top-ui.LineHeight)
	for _, used := range s.spritePalettes {
		s.theme.DrawText(screen, used.name, left, used.remove.Rect.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1,
			ui.ColorInk())
		used.remove.Draw(screen, s.theme)
	}
}

// locateArt finds the zone's palette buttons and a sprite's Remove….
func (s *State) locateArt(widget string) (image.Point, bool) {
	if point, found := s.zonePalettes.Locate("ui:" + widget); found {
		return point, true
	}
	for _, used := range s.spritePalettes {
		if used.remove.ID == widget {
			return ui.Center(used.remove.Rect), true
		}
	}

	return image.Pt(0, 0), false
}
