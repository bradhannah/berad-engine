// Package pen draws in points at the editor window's scale (increment 56).
//
// Views lay out in points, and the pen turns points into the screen's own
// pixels, so text and lines are drawn at the screen's resolution rather than
// drawn small and enlarged. A point takes the screen's pixels to a point (its
// device scale) times the author's scale for text and controls.
//
// A canvas's content, such as a Map's cells or a Workflow's nodes, is drawn
// without the author's scale, so its zoom does not change: it is laid out in
// canvas points, the window's points times the author's scale, and drawn at
// the screen's pixels to a point.
//
// The pen's shapes take the same arguments as Ebitengine's vector package,
// in points.
package pen

import (
	"image"
	"image/color"
	"math"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// scaling is the screen's pixels to a point, the author's scale, and whether
// a canvas's content is being drawn.
type scaling struct {
	device float64
	author float64
	canvas bool
}

// current is the window's scaling, set once a frame and read by every view.
//
//nolint:gochecknoglobals // One window draws at one scale; every drawing helper reads it.
var current atomic.Pointer[scaling]

// loadScaling returns the window's scaling: one pixel to a point and 100%
// until the window sets one.
func loadScaling() scaling {
	if now := current.Load(); now != nil {
		return *now
	}

	return scaling{device: 1, author: 1, canvas: false}
}

// SetScale sets the screen's pixels to a point and the author's scale, once a
// frame.
func SetScale(device, author float64) {
	now := loadScaling()
	if now.device == device && now.author == author && !now.canvas {
		return
	}
	current.Store(&scaling{device: device, author: author, canvas: false})
}

// Scale is how many of the screen's pixels a point drawn now takes: the
// device scale times the author's, or the device scale alone on a canvas.
func Scale() float64 {
	now := loadScaling()
	if now.canvas {
		return now.device
	}

	return now.device * now.author
}

// Device is the screen's pixels to a point.
func Device() float64 { return loadScaling().device }

// Author is the author's scale for text and controls, 1 for 100%.
func Author() float64 { return loadScaling().author }

// OnCanvas runs draw for a canvas's content, drawn without the author's
// scale, in canvas points.
func OnCanvas(draw func()) {
	before := loadScaling()
	if before.canvas || before.author == 1 {
		draw()

		return
	}
	onCanvas := before
	onCanvas.canvas = true
	current.Store(&onCanvas)
	defer current.Store(&before)
	draw()
}

// CanvasPoint is a window point in a canvas's points.
func CanvasPoint(point image.Point) image.Point {
	author := Author()
	if author == 1 {
		return point
	}

	return image.Pt(int(math.Floor(float64(point.X)*author)), int(math.Floor(float64(point.Y)*author)))
}

// CanvasRect is a window rectangle in a canvas's points.
func CanvasRect(rect image.Rectangle) image.Rectangle { return scaleRect(rect, Author()) }

// WindowRect is a canvas's rectangle in the window's points.
func WindowRect(rect image.Rectangle) image.Rectangle { return scaleRect(rect, 1/Author()) }

// PixelRect is a rectangle in points as the screen's pixels it covers.
func PixelRect(rect image.Rectangle) image.Rectangle { return scaleRect(rect, Scale()) }

// scaleRect scales a rectangle's edges, rounding each to a whole number, so
// rectangles that meet still meet.
func scaleRect(rect image.Rectangle, scale float64) image.Rectangle {
	if scale == 1 {
		return rect
	}
	edge := func(value int) int { return int(math.Round(float64(value) * scale)) }

	return image.Rect(edge(rect.Min.X), edge(rect.Min.Y), edge(rect.Max.X), edge(rect.Max.Y))
}

// Bounds is an image's bounds in points.
func Bounds(screen *ebiten.Image) image.Rectangle { return scaleRect(screen.Bounds(), 1/Scale()) }

// Clip returns the part of an image under a rectangle in points, to draw
// into it alone.
func Clip(screen *ebiten.Image, rect image.Rectangle) (*ebiten.Image, bool) {
	area, ok := screen.SubImage(PixelRect(rect)).(*ebiten.Image)

	return area, ok
}

// DrawImage draws a picture placed in points by its options.
func DrawImage(screen, picture *ebiten.Image, options *ebiten.DrawImageOptions) {
	if scale := Scale(); scale != 1 {
		options.GeoM.Scale(scale, scale)
	}
	screen.DrawImage(picture, options)
}

// Px is a length or position in points in the screen's pixels.
func Px(value float32) float32 { return value * float32(Scale()) }

// Edge is the edge of a box in points in the screen's pixels, a whole pixel
// so that boxes that meet still meet.
func Edge(value int) float32 {
	scale := Scale()
	if scale == 1 {
		return float32(value)
	}

	return float32(math.Round(float64(value) * scale))
}

// Line is the width of a line drawn without smoothing in the screen's
// pixels: as wide as asked at one pixel to a point, and otherwise a whole
// number of pixels, at least one, so lines of one width look alike.
func Line(width float32) float32 {
	scale := Scale()
	if scale == 1 {
		return width
	}

	return max(1, float32(math.Round(float64(width)*scale)))
}

// width is a line's width in the screen's pixels: smoothed lines exactly,
// others a whole number of pixels.
func width(points float32, antialias bool) float32 {
	if antialias {
		return Px(points)
	}

	return Line(points)
}

// StrokeLine draws a line between two points.
func StrokeLine(dst *ebiten.Image, x0, y0, x1, y1, strokeWidth float32, clr color.Color, antialias bool) {
	vector.StrokeLine(dst, Px(x0), Px(y0), Px(x1), Px(y1), width(strokeWidth, antialias), clr, antialias)
}

// FillRect fills a rectangle.
func FillRect(dst *ebiten.Image, x, y, w, h float32, clr color.Color, antialias bool) {
	vector.FillRect(dst, Px(x), Px(y), Px(w), Px(h), clr, antialias)
}

// StrokeRect outlines a rectangle.
func StrokeRect(dst *ebiten.Image, x, y, w, h, strokeWidth float32, clr color.Color, antialias bool) {
	vector.StrokeRect(dst, Px(x), Px(y), Px(w), Px(h), width(strokeWidth, antialias), clr, antialias)
}

// FillCircle fills a circle.
func FillCircle(dst *ebiten.Image, cx, cy, r float32, clr color.Color, antialias bool) {
	vector.FillCircle(dst, Px(cx), Px(cy), Px(r), clr, antialias)
}

// StrokeCircle outlines a circle.
func StrokeCircle(dst *ebiten.Image, cx, cy, r, strokeWidth float32, clr color.Color, antialias bool) {
	vector.StrokeCircle(dst, Px(cx), Px(cy), Px(r), width(strokeWidth, antialias), clr, antialias)
}

// FillPath fills a path drawn in points.
func FillPath(dst *ebiten.Image, path *vector.Path, fill *vector.FillOptions, options *vector.DrawPathOptions) {
	vector.FillPath(dst, Path(path), fill, options)
}

// StrokePath outlines a path drawn in points.
func StrokePath(dst *ebiten.Image, path *vector.Path, stroke *vector.StrokeOptions, options *vector.DrawPathOptions) {
	scaled := *stroke
	scaled.Width = Px(stroke.Width)
	vector.StrokePath(dst, Path(path), &scaled, options)
}

// Path is a path drawn in points as the screen's pixels it covers.
func Path(path *vector.Path) *vector.Path {
	scale := Scale()
	if scale == 1 {
		return path
	}
	var scaled vector.Path
	var options vector.AddPathOptions
	options.GeoM.Scale(scale, scale)
	scaled.AddPath(path, &options)

	return &scaled
}

// Layer returns an image to draw a part of the window into, the screen's
// pixels under a rectangle in points, keeping the image given when it is
// already that size. What is drawn into it is drawn from its own top-left
// corner.
func Layer(layer *ebiten.Image, rect image.Rectangle) *ebiten.Image {
	size := PixelRect(rect).Size()
	size = image.Pt(max(size.X, 1), max(size.Y, 1))
	if layer != nil && layer.Bounds().Size() == size {
		return layer
	}
	if layer != nil {
		layer.Deallocate()
	}

	return ebiten.NewImage(size.X, size.Y)
}

// DrawLayer draws a layer's image, or a part of it, at the top-left corner
// of the rectangle in points it was made for.
func DrawLayer(screen, layer *ebiten.Image, rect image.Rectangle) {
	var options ebiten.DrawImageOptions
	at := PixelRect(rect).Min
	options.GeoM.Translate(float64(at.X), float64(at.Y))
	screen.DrawImage(layer, &options)
}

// Cursor is a pointer position in the screen's pixels in points.
func Cursor(across, down int) image.Point {
	scale := Scale()
	if scale == 1 {
		return image.Pt(across, down)
	}

	return image.Pt(int(math.Floor(float64(across)/scale)), int(math.Floor(float64(down)/scale)))
}

// CanvasDelta is a distance moved in the window's points in a canvas's
// points.
func CanvasDelta(delta image.Point) image.Point {
	author := Author()
	if author == 1 {
		return delta
	}

	return image.Pt(int(math.Round(float64(delta.X)*author)), int(math.Round(float64(delta.Y)*author)))
}

// WindowPoint is a canvas's point in the window's points.
func WindowPoint(point image.Point) image.Point {
	return WindowRect(image.Rectangle{Min: point, Max: point}).Min
}
