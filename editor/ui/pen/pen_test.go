package pen_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui/pen"
)

// scaled runs a test at a scale and puts the window's scale back after it.
// Tests that set the scale run one at a time, before the parallel ones.
func scaled(t *testing.T, device, author float64) {
	t.Helper()
	pen.SetScale(device, author)
	t.Cleanup(func() { pen.SetScale(1, 1) })
}

// At one pixel to a point and 100%, nothing changes.
func TestOnePixelToAPointChangesNothing(t *testing.T) {
	t.Parallel()
	rect := image.Rect(3, 5, 41, 27)
	if pen.Scale() != 1 || pen.PixelRect(rect) != rect || pen.CanvasRect(rect) != rect {
		t.Fatalf("scale %v, pixels %v, canvas %v", pen.Scale(), pen.PixelRect(rect), pen.CanvasRect(rect))
	}
	if pen.Cursor(17, 9) != image.Pt(17, 9) || pen.Edge(7) != 7 || pen.Line(1.5) != 1.5 || pen.Px(2.5) != 2.5 {
		t.Fatalf("cursor %v, edge %v, line %v", pen.Cursor(17, 9), pen.Edge(7), pen.Line(1.5))
	}
}

// A point takes the screen's pixels to a point times the author's scale.
//
//nolint:paralleltest // It sets the window's scale.
func TestAPointTakesTheDeviceAndAuthorScale(t *testing.T) {
	scaled(t, 2, 1.5)
	if pen.Scale() != 3 || pen.Device() != 2 || pen.Author() != 1.5 {
		t.Fatalf("scale %v, device %v, author %v", pen.Scale(), pen.Device(), pen.Author())
	}
	if got := pen.PixelRect(image.Rect(1, 2, 11, 12)); got != image.Rect(3, 6, 33, 36) {
		t.Fatalf("pixels %v", got)
	}
	// Pixels under the pointer arrive as the point they are in.
	if got := pen.Cursor(32, 7); got != image.Pt(10, 2) {
		t.Fatalf("cursor %v", got)
	}
}

// Rectangles that meet in points still meet in pixels at a scale that is not
// a whole number.
//
//nolint:paralleltest // It sets the window's scale.
func TestRectanglesThatMeetStillMeet(t *testing.T) {
	scaled(t, 1, 1.25)
	left, right := pen.PixelRect(image.Rect(0, 0, 13, 10)), pen.PixelRect(image.Rect(13, 0, 30, 10))
	if left.Max.X != right.Min.X {
		t.Fatalf("left %v, right %v", left, right)
	}
}

// A line drawn without smoothing is a whole number of pixels, at least one.
//
//nolint:paralleltest // It sets the window's scale.
func TestLinesAreWholePixels(t *testing.T) {
	scaled(t, 1, 0.75)
	if pen.Line(1) != 1 || pen.Line(2) != 2 || pen.Line(4) != 3 {
		t.Fatalf("lines %v %v %v", pen.Line(1), pen.Line(2), pen.Line(4))
	}
	pen.SetScale(1, 1.5)
	if pen.Line(1) != 2 || pen.Line(2) != 3 {
		t.Fatalf("lines %v %v", pen.Line(1), pen.Line(2))
	}
}

// A canvas draws without the author's scale, in its own points, and the
// window's scale comes back after.
//
//nolint:paralleltest // It sets the window's scale.
func TestACanvasDrawsWithoutTheAuthorsScale(t *testing.T) {
	scaled(t, 2, 1.5)
	window := image.Rect(10, 20, 110, 220)
	canvas := pen.CanvasRect(window)
	if canvas != image.Rect(15, 30, 165, 330) || pen.WindowRect(canvas) != window {
		t.Fatalf("canvas %v, back %v", canvas, pen.WindowRect(canvas))
	}
	if got := pen.CanvasPoint(image.Pt(11, 21)); got != image.Pt(16, 31) {
		t.Fatalf("canvas point %v", got)
	}
	var drawn float64
	pen.OnCanvas(func() { drawn = pen.Scale() })
	if drawn != 2 || pen.Scale() != 3 {
		t.Fatalf("on the canvas %v, after %v", drawn, pen.Scale())
	}
	// The canvas's pixels are the window's: a canvas point takes the device
	// scale, a window point the device and the author's.
	pen.OnCanvas(func() { drawn = float64(pen.PixelRect(canvas).Min.X) })
	if want := float64(pen.PixelRect(window).Min.X); drawn != want {
		t.Fatalf("canvas starts at pixel %v, window at %v", drawn, want)
	}
}

// A layer is the screen's pixels under its rectangle.
//
//nolint:paralleltest // It sets the window's scale.
func TestALayerIsThePixelsUnderIt(t *testing.T) {
	scaled(t, 2, 1)
	layer := pen.Layer(nil, image.Rect(5, 5, 105, 55))
	if layer.Bounds().Size() != image.Pt(200, 100) {
		t.Fatalf("layer %v", layer.Bounds())
	}
	if pen.Bounds(layer) != image.Rect(0, 0, 100, 50) {
		t.Fatalf("bounds in points %v", pen.Bounds(layer))
	}
	if again := pen.Layer(layer, image.Rect(0, 0, 100, 50)); again != layer {
		t.Fatal("a layer of the same size was made again")
	}
}
