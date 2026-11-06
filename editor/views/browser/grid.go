package browser

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// The grid (increment 32): the main pane's things as tiles, for Pictures
// and Cards. It lays out the same rows the list holds, one tile each, with
// a group's heading across the whole width. A zone's floors follow it as
// tiles of their own; a folder's contents are not opened in place.

// Sizes of the grid, in pixels.
const (
	gridPad     = 12
	gridGap     = 12
	gridHeading = 28
)

// gridCell is a row's place in the grid, from the grid's top-left before
// scrolling.
type gridCell struct {
	row  ui.Row
	rect image.Rectangle
}

// tileGrid lays out tiles and follows what is clicked.
type tileGrid struct {
	rect    image.Rectangle
	scroll  int
	content int
	cells   []gridCell
}

func newTileGrid() *tileGrid {
	return &tileGrid{rect: ui.Zero(), scroll: 0, content: 0, cells: nil}
}

// layout places a tile of a size for each row at the top level, in rows
// as wide as the grid is, a heading starting a line of its own.
func (g *tileGrid) layout(rows []ui.Row, area image.Rectangle, tile image.Point) {
	g.rect, g.cells = area, nil
	left, top, lineBottom := gridPad, gridPad, gridPad
	for _, row := range rows {
		switch {
		case row.Indent > 0 && row.Icon != ui.IconFloor:
			continue
		case row.Heading:
			if left > gridPad {
				top = lineBottom + gridGap
			}
			g.cells = append(g.cells, gridCell{row: row, rect: image.Rect(gridPad, top, area.Dx()-gridPad, top+gridHeading)})
			left, top, lineBottom = gridPad, top+gridHeading, top+gridHeading
		default:
			if left > gridPad && left+tile.X > area.Dx()-gridPad {
				left, top = gridPad, lineBottom+gridGap
			}
			rect := image.Rect(left, top, left+tile.X, top+tile.Y)
			g.cells = append(g.cells, gridCell{row: row, rect: rect})
			left, lineBottom = rect.Max.X+gridGap, rect.Max.Y
		}
	}
	g.content = lineBottom + gridPad
	g.scroll = min(max(g.scroll, 0), max(g.content-area.Dy(), 0))
}

// onScreen returns where a cell shows on the screen.
func (g *tileGrid) onScreen(cell gridCell) image.Rectangle {
	return cell.rect.Add(g.rect.Min).Sub(image.Pt(0, g.scroll))
}

// at returns the tile at a point, if any.
func (g *tileGrid) at(point image.Point) (gridCell, bool) {
	if !point.In(g.rect) {
		return gridCell{row: ui.NewRow("", ""), rect: ui.Zero()}, false
	}
	for _, cell := range g.cells {
		if !cell.row.Heading && point.In(g.onScreen(cell)) {
			return cell, true
		}
	}

	return gridCell{row: ui.NewRow("", ""), rect: ui.Zero()}, false
}

// update scrolls with the wheel and follows a click, a double-click and a
// right-click, as the list does, selecting a tile in selected.
func (g *tileGrid) update(input *ui.Input, selected *string) ui.ListResult {
	result := ui.ListResult{
		Changed: false, Activated: false, Toggled: "", Added: "", Dragged: "", Dropped: "", Rested: "",
		Context: false, ContextRow: "",
	}
	if !input.Cursor.In(g.rect) {
		return result
	}
	if input.WheelY != 0 {
		g.scroll -= int(input.WheelY * ui.RowHeight)
		g.scroll = min(max(g.scroll, 0), max(g.content-g.rect.Dy(), 0))
	}
	cell, onTile := g.at(input.Cursor)
	switch {
	case input.Secondary:
		result.Context, result.ContextRow = true, cell.row.ID
		input.ClearPointer()
	case input.Pressed && onTile:
		result.Changed = *selected != cell.row.ID
		*selected = cell.row.ID
		result.Activated = input.DoubleClick
		input.ClearPointer()
	case input.Pressed:
		input.ClearPointer()
	}

	return result
}

// reveal scrolls as little as it takes to show a tile.
func (g *tileGrid) reveal(id string) {
	for _, cell := range g.cells {
		if cell.row.ID != id {
			continue
		}
		switch {
		case cell.rect.Min.Y < g.scroll:
			g.scroll = cell.rect.Min.Y - gridPad
		case cell.rect.Max.Y > g.scroll+g.rect.Dy():
			g.scroll = cell.rect.Max.Y - g.rect.Dy() + gridPad
		}
		g.scroll = max(g.scroll, 0)
	}
}

// locate finds a tile's centre, for input scripts.
func (g *tileGrid) locate(id string) (image.Point, bool) {
	g.reveal(id)
	for _, cell := range g.cells {
		if cell.row.ID == id && !cell.row.Heading {
			return ui.Center(g.onScreen(cell)), true
		}
	}

	return image.Pt(0, 0), false
}
