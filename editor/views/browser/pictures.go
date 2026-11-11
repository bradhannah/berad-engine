package browser

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/tileart"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
)

// Pictures (increment 32): art found by looking. Each sprite shows as the
// map draws it, each palette as its swatches, each template as its marks,
// and each folder with up to four of the sprites in it, at the size the
// place keeps.

// Sizes of Pictures' tiles, in pixels, and how many pictures it keeps.
const (
	pictureLabels = 40
	pictureTop    = 6
	tileMinWidth  = 108
	tileTextTop   = 4
	folderMinis   = 4
	pictureRadius = 4
	picturesKept  = 200
)

// pictureTile is the size of a tile in Pictures.
func (s *State) pictureTile() image.Point {
	size := s.pictureSize()

	return image.Pt(max(size+gridPad, tileMinWidth), size+pictureTop+pictureLabels)
}

// spritesIn returns up to some sprites directly in a folder.
func spritesIn(index *project.Index, path string, most int) []ids.ULID {
	var found []ids.ULID
	for _, entry := range index.OfFormat(art.SpriteFormatName) {
		if entry.Folder == path && len(found) < most {
			found = append(found, entry.ID)
		}
	}

	return found
}

// syncPictures asks for what the pictures shown are drawn from: each
// sprite's tile and palettes, and each palette and template; and lets go
// of the pictures no longer shown once it keeps many.
func (s *State) syncPictures(ctx *frame.Context) {
	if !s.gridShown() {
		return
	}
	index := s.index(ctx.Docs)
	shown := map[ids.ULID]bool{}
	for _, cell := range s.grid.cells {
		for _, spriteID := range s.cellSprites(ctx.Docs, index, cell.row.ID) {
			shown[spriteID] = true
			s.subscribe(spriteID, ctx)
			if tileID, ok := s.tileOf(ctx.Docs, spriteID); ok {
				s.subscribe(tileID, ctx)
				for _, paletteID := range tileart.Palettes(ctx.Docs, tileID) {
					s.subscribe(paletteID, ctx)
				}
			}
		}
		if documentID, err := ids.Parse(cell.row.ID); err == nil {
			s.subscribe(documentID, ctx)
		}
	}
	if len(s.pictures) > picturesKept {
		for spriteID, picture := range s.pictures {
			if !shown[spriteID] {
				picture.Deallocate()
				delete(s.pictures, spriteID)
			}
		}
	}
}

// cellSprites returns the sprites a tile shows: a sprite's own, an NPC's,
// or a sprite folder's first few.
func (s *State) cellSprites(docs frame.Docs, index *project.Index, rowID string) []ids.ULID {
	if folder, isFolder := parseFolderRow(rowID); isFolder && folder.Format == art.SpriteFormatName {
		return spritesIn(index, folder.Path, folderMinis)
	}
	documentID, err := ids.Parse(rowID)
	if err != nil {
		return nil
	}
	entry, found := index.Entry(documentID)
	switch {
	case !found:
		return nil
	case entry.Format == art.SpriteFormatName:
		return []ids.ULID{documentID}
	case entry.Format == npc.FormatName:
		if person, loaded := s.npcOf(docs, documentID); loaded && !person.Sprite.IsZero() {
			return []ids.ULID{person.Sprite}
		}
	}

	return nil
}

// spritePicture returns a sprite's picture, drawn again when it changes.
func (s *State) spritePicture(docs frame.Docs, spriteID ids.ULID) (*ebiten.Image, bool) {
	tileID, ok := s.tileOf(docs, spriteID)
	if !ok {
		return nil, false
	}
	picture := s.pictures[spriteID]
	if picture == nil {
		picture = tileart.NewPicture()
		s.pictures[spriteID] = picture
	}
	if !picture.Refresh(docs, tileID) {
		return nil, false
	}

	return picture.Image(), true
}

// drawScaled draws a picture as large as fits a square, whole pixels when
// it grows, smoothed when it shrinks.
func drawScaled(screen, picture *ebiten.Image, box image.Rectangle) {
	side := max(picture.Bounds().Dx(), picture.Bounds().Dy(), 1)
	scale := float64(min(box.Dx(), box.Dy())) / float64(side)
	if scale >= 1 {
		scale = float64(int(scale))
	}
	var place ebiten.DrawImageOptions
	place.GeoM.Scale(scale, scale)
	width, height := float64(picture.Bounds().Dx())*scale, float64(picture.Bounds().Dy())*scale
	place.GeoM.Translate(float64(box.Min.X)+(float64(box.Dx())-width)/ui.Half,
		float64(box.Min.Y)+(float64(box.Dy())-height)/ui.Half)
	if scale < 1 {
		place.Filter = ebiten.FilterLinear
	}
	screen.DrawImage(picture, &place)
}

// drawPictures draws Pictures' tiles and its groups' headings.
func (s *State) drawPictures(screen *ebiten.Image, docs frame.Docs) {
	area, ok := screen.SubImage(s.grid.rect).(*ebiten.Image)
	if !ok {
		return
	}
	index := s.index(docs)
	for _, cell := range s.grid.cells {
		rect := s.grid.onScreen(cell)
		if !rect.Overlaps(s.grid.rect) {
			continue
		}
		if cell.row.Heading {
			s.theme.DrawLabel(area, cell.row.Label+"  "+cell.row.Detail, rect.Min.X, rect.Min.Y+ui.Gap, ui.ColorMuted())

			continue
		}
		s.drawTileFrame(area, cell.row, rect)
		size := s.pictureSize()
		box := image.Rect(rect.Min.X+(rect.Dx()-size)/ui.Half, rect.Min.Y+pictureTop,
			rect.Min.X+(rect.Dx()+size)/ui.Half, rect.Min.Y+pictureTop+size)
		s.drawPicture(area, docs, index, cell.row, box)
		s.drawTileText(area, cell.row, image.Rect(rect.Min.X, box.Max.Y, rect.Max.X, rect.Max.Y))
	}
}

// drawTileFrame draws a tile's ground, washed and outlined when selected.
func (s *State) drawTileFrame(screen *ebiten.Image, row ui.Row, rect image.Rectangle) {
	if row.ID == s.list.Selected {
		ui.FillRound(screen, rect, ui.Radius, ui.ColorSelectWash())
		ui.StrokeRound(screen, rect, ui.Radius, ui.Outline, ui.ColorSelect())
	}
}

// drawTileText draws a tile's name and, under it, its detail.
func (s *State) drawTileText(screen *ebiten.Image, row ui.Row, rect image.Rectangle) {
	width := rect.Dx() - ui.Inset
	name := s.theme.FitText(row.Label, width)
	s.theme.DrawText(screen, name, rect.Min.X+(rect.Dx()-s.theme.TextWidth(name))/ui.Half, rect.Min.Y+tileTextTop,
		ui.ColorInk())
	if row.Detail == "" {
		return
	}
	detail := s.theme.FitText(row.Detail, width)
	s.theme.DrawLabel(screen, detail, rect.Min.X+(rect.Dx()-s.theme.TextWidth(detail))/ui.Half,
		rect.Min.Y+tileTextTop+ui.TextSize+ui.Gap, ui.ColorMuted())
}

// drawPicture draws what a tile shows in its box: a sprite, a palette's
// swatches, a template's marks, a folder's first sprites, or an icon.
func (s *State) drawPicture(
	screen *ebiten.Image, docs frame.Docs, index *project.Index, row ui.Row, box image.Rectangle,
) {
	ui.FillRound(screen, box, pictureRadius, ui.ColorPage())
	if folder, isFolder := parseFolderRow(row.ID); isFolder {
		s.drawFolderPicture(screen, docs, spritesIn(index, folder.Path, folderMinis), box)

		return
	}
	documentID, err := ids.Parse(row.ID)
	entry, found := index.Entry(documentID)
	switch {
	case err != nil || !found:
		ui.DrawIcon(screen, row.Icon, ui.Center(box).Sub(image.Pt(ui.IconSize/ui.Half, ui.IconSize/ui.Half)))
	case entry.Format == art.SpriteFormatName:
		if picture, ok := s.spritePicture(docs, documentID); ok {
			drawScaled(screen, picture, box)
		}
	case entry.Format == art.PaletteFormatName:
		if palette, ok := paletteOf(docs, documentID); ok {
			drawSwatches(screen, palette.Ramps(), box)
		}
	case entry.Format == artplugin.TemplateFormatName:
		s.drawTemplate(screen, docs, documentID, box)
	default:
		ui.DrawIcon(screen, row.Icon, ui.Center(box).Sub(image.Pt(ui.IconSize/ui.Half, ui.IconSize/ui.Half)))
	}
}

// drawFolderPicture draws a folder's first sprites in a square of four, or
// a folder's icon when it holds none.
func (s *State) drawFolderPicture(screen *ebiten.Image, docs frame.Docs, sprites []ids.ULID, box image.Rectangle) {
	if len(sprites) == 0 {
		ui.DrawIcon(screen, ui.IconFolder, ui.Center(box).Sub(image.Pt(ui.IconSize/ui.Half, ui.IconSize/ui.Half)))

		return
	}
	half := box.Dx() / ui.Half
	for position, spriteID := range sprites {
		corner := box.Min.Add(image.Pt(position%ui.Half*half, position/ui.Half*half))
		if picture, ok := s.spritePicture(docs, spriteID); ok {
			drawScaled(screen, picture, image.Rect(corner.X, corner.Y, corner.X+half, corner.Y+half).Inset(ui.Gap))
		}
	}
}

// drawSwatches draws a palette's ramps as rows of its shades.
func drawSwatches(screen *ebiten.Image, ramps []art.Ramp, box image.Rectangle) {
	widest := 1
	for _, ramp := range ramps {
		widest = max(widest, len(ramp.Shades))
	}
	if len(ramps) == 0 {
		return
	}
	side := max(min(box.Dx()/widest, box.Dy()/len(ramps)), 1)
	top := box.Min.Y + (box.Dy()-side*len(ramps))/ui.Half
	for line, ramp := range ramps {
		for position, shade := range ramp.Shades {
			corner := image.Pt(box.Min.X+(box.Dx()-side*widest)/ui.Half+position*side, top+line*side)
			ui.FillRect(screen, image.Rect(corner.X, corner.Y, corner.X+side, corner.Y+side),
				shade.Color.NRGBA(shade.Opacity))
		}
	}
}

// drawTemplate draws a template's marks over a tile of the project's size.
func (s *State) drawTemplate(screen *ebiten.Image, docs frame.Docs, templateID ids.ULID, box image.Rectangle) {
	size, known := s.projectTileSize(docs)
	if !known {
		return
	}
	docs.View(templateID, func(document client.Document) {
		templateDoc, ok := document.Model.(*artplugin.TemplateDocument)
		if !ok {
			return
		}
		zoom := max(box.Dx()/size, 1)
		corner := box.Min.Add(image.Pt((box.Dx()-size*zoom)/ui.Half, (box.Dy()-size*zoom)/ui.Half))
		tileart.DrawMarks(screen, templateDoc.Template, image.Rect(corner.X, corner.Y, corner.X+size*zoom,
			corner.Y+size*zoom), size, zoom)
	})
}
