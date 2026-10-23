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
	wheelPan    = 24
)

// camera maps floor cells to screen positions: a zoom level and the screen
// offset of the floor's top-left corner within the grid area.
type camera struct {
	zoom   int
	offset image.Point
}

func newCamera() camera {
	return camera{zoom: defaultZoom, offset: image.Pt(gridMargin, gridMargin)}
}

func (c camera) cellSize() int { return zoomSizes()[c.zoom] }

// cellAt returns the cell under a screen point.
func (c camera) cellAt(point image.Point, grid image.Rectangle) (Cell, bool) {
	if !point.In(grid) {
		return image.Pt(0, 0), false
	}
	size := c.cellSize()
	localX := point.X - grid.Min.X - c.offset.X
	localY := point.Y - grid.Min.Y - c.offset.Y
	if localX < 0 || localY < 0 {
		return image.Pt(0, 0), false
	}

	return Cell{X: localX / size, Y: localY / size}, true
}

// cellRect returns a cell's screen rectangle.
func (c camera) cellRect(cell Cell, grid image.Rectangle) image.Rectangle {
	size := c.cellSize()
	x := grid.Min.X + c.offset.X + cell.X*size
	y := grid.Min.Y + c.offset.Y + cell.Y*size

	return image.Rect(x, y, x+size, y+size)
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

// reset returns to the default zoom with the floor's corner in view.
func (c *camera) reset() { *c = newCamera() }

// pan moves the view by a screen distance.
func (c *camera) pan(delta image.Point) { c.offset = c.offset.Add(delta) }
