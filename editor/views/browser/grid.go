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

// tileGrid lays out tiles and follows what is clicked. A strip lays them
// out in one line that scrolls across, as Preview's does.
type tileGrid struct {
	rect    image.Rectangle
	scroll  int
	content int
	cells   []gridCell
	strip   bool
}

func newTileGrid() *tileGrid {
	return &tileGrid{rect: ui.Zero(), scroll: 0, content: 0, cells: nil, strip: false}
}

// layout places a tile of a size for each row at the top level, in rows
// as wide as the grid is, a heading starting a line of its own.
func (g *tileGrid) layout(rows []ui.Row, area image.Rectangle, tile image.Point) {
	g.rect, g.cells, g.strip = area, nil, false
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

// layoutStrip places a tile for each thing in one line across, skipping
// headings and what is listed under a thing other than a zone's floors.
func (g *tileGrid) layoutStrip(rows []ui.Row, area image.Rectangle, tile image.Point) {
	g.rect, g.cells, g.strip = area, nil, true
	left := gridPad
	for _, row := range rows {
		if row.Heading || row.Indent > 0 && row.Icon != ui.IconFloor {
			continue
		}
		g.cells = append(g.cells, gridCell{row: row, rect: image.Rect(left, gridGap, left+tile.X, gridGap+tile.Y)})
		left += tile.X + gridGap
	}
	g.content = left + gridPad
	g.scroll = min(max(g.scroll, 0), max(g.content-area.Dx(), 0))
}

// onScreen returns where a cell shows on the screen.
func (g *tileGrid) onScreen(cell gridCell) image.Rectangle {
	if g.strip {
		return cell.rect.Add(g.rect.Min).Sub(image.Pt(g.scroll, 0))
	}

	return cell.rect.Add(g.rect.Min).Sub(image.Pt(0, g.scroll))
}

// span is how far the grid scrolls: down a grid, or across a strip.
func (g *tileGrid) span() int {
	if g.strip {
		return g.rect.Dx()
	}

	return g.rect.Dy()
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
	if wheel := input.WheelY + input.WheelX; wheel != 0 {
		g.scroll -= int(wheel * ui.RowHeight)
		g.scroll = min(max(g.scroll, 0), max(g.content-g.span(), 0))
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
		start, end := cell.rect.Min.Y, cell.rect.Max.Y
		if g.strip {
			start, end = cell.rect.Min.X, cell.rect.Max.X
		}
		switch {
		case start < g.scroll:
			g.scroll = start - gridPad
		case end > g.scroll+g.span():
			g.scroll = end - g.span() + gridPad
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
