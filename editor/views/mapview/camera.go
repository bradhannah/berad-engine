package mapview

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// Cell sizes on screen, one per zoom level. Integer sizes keep pixel art
// scaled uniformly once real tiles replace stand-ins.
func zoomSizes() []int { return []int{6, 12, 18, 24, 36, 48, 72, 96} }

const (
	defaultZoom = 3
	gridMargin  = 16
)

// camera maps zone cells to screen positions: a zoom level and the screen
// offset of the zone origin (the top-left corner of cell 0,0) within the grid
// area. Every floor of a zone shares the origin, so switching floors keeps
// the view still.
type camera struct {
	zoom   int
	offset image.Point
}

func newCamera() camera {
	return camera{zoom: defaultZoom, offset: image.Pt(gridMargin, gridMargin)}
}

func (c camera) cellSize() int { return zoomSizes()[c.zoom] }

// cellAt returns the cell under a screen point, and whether the point is in
// the grid area. Cells left of and above the origin have negative numbers.
func (c camera) cellAt(point image.Point, grid image.Rectangle) (Cell, bool) {
	size := c.cellSize()
	local := point.Sub(grid.Min).Sub(c.offset)

	return Cell{X: floorDiv(local.X, size), Y: floorDiv(local.Y, size)}, point.In(grid)
}

// boundary returns the cell boundary nearest a screen point: the column and
// row whose left and top edges lie closest to it.
func (c camera) boundary(point image.Point, grid image.Rectangle) image.Point {
	size := c.cellSize()
	local := point.Sub(grid.Min).Sub(c.offset)

	return image.Pt(floorDiv(local.X+size/ui.Half, size), floorDiv(local.Y+size/ui.Half, size))
}

// cellRect returns a cell's screen rectangle.
func (c camera) cellRect(cell Cell, grid image.Rectangle) image.Rectangle {
	return c.rectOf(image.Rectangle{Min: cell, Max: cell.Add(image.Pt(1, 1))}, grid)
}

// rectOf returns the screen rectangle covering a rectangle of cells.
func (c camera) rectOf(cells, grid image.Rectangle) image.Rectangle {
	size := c.cellSize()
	origin := grid.Min.Add(c.offset)

	return image.Rect(
		origin.X+cells.Min.X*size, origin.Y+cells.Min.Y*size, origin.X+cells.Max.X*size, origin.Y+cells.Max.Y*size,
	)
}

// visible returns the cells of bounds that show in the grid area.
func (c camera) visible(bounds, grid image.Rectangle) image.Rectangle {
	return bounds.Intersect(c.inView(grid))
}

// inView returns every cell that shows, at least in part, in the grid area.
func (c camera) inView(grid image.Rectangle) image.Rectangle {
	first, _ := c.cellAt(grid.Min, grid)
	last, _ := c.cellAt(grid.Max.Sub(image.Pt(1, 1)), grid)

	return image.Rectangle{Min: first, Max: last.Add(image.Pt(1, 1))}
}

// zoomBy changes the zoom level, keeping the point under focus still.
func (c *camera) zoomBy(steps int, focus image.Point, grid image.Rectangle) {
	next := min(max(c.zoom+steps, 0), len(zoomSizes())-1)
	if next == c.zoom {
		return
	}
	before, after := c.cellSize(), zoomSizes()[next]
	local := focus.Sub(grid.Min)
	if !focus.In(grid) {
		local = image.Pt(grid.Dx()/ui.Half, grid.Dy()/ui.Half)
	}
	c.offset = image.Pt(
		local.X-(local.X-c.offset.X)*after/before,
		local.Y-(local.Y-c.offset.Y)*after/before,
	)
	c.zoom = next
}

// frame returns to the default zoom with the top-left corner of bounds in
// view.
func (c *camera) frame(bounds image.Rectangle) {
	*c = newCamera()
	size := c.cellSize()
	c.offset = image.Pt(gridMargin-bounds.Min.X*size, gridMargin-bounds.Min.Y*size)
}

// fit shows the whole of bounds as large as the grid allows, centred.
func (c *camera) fit(bounds, grid image.Rectangle) {
	room := grid.Inset(gridMargin)
	c.zoom = 0
	for level, size := range zoomSizes() {
		if bounds.Dx()*size <= room.Dx() && bounds.Dy()*size <= room.Dy() {
			c.zoom = level
		}
	}
	size := c.cellSize()
	middle := image.Pt(grid.Dx()/ui.Half, grid.Dy()/ui.Half)
	c.offset = middle.Sub(image.Pt((bounds.Min.X*2+bounds.Dx())*size/ui.Half, (bounds.Min.Y*2+bounds.Dy())*size/ui.Half))
}

// pan moves the view by a screen distance.
func (c *camera) pan(delta image.Point) { c.offset = c.offset.Add(delta) }

// floorDiv divides, rounding towards negative infinity, so the cell just left
// of the origin is -1 rather than 0.
func floorDiv(value, divisor int) int {
	quotient := value / divisor
	if value%divisor != 0 && value < 0 {
		quotient--
	}

	return quotient
}
