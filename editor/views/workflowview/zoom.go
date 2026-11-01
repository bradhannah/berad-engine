package workflowview

import (
	"image"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Zooming draws the canvas at its own size into an image as large as the
// canvas shows at the current zoom, then scales that image onto the screen.
// The drawing code works in unscaled pixels throughout; only turning screen
// points into canvas points, and back for input scripts, knows the zoom.

// zoomLevels returns the canvas's zoom levels, as percentages.
func zoomLevels() []int { return []int{25, 33, 50, 75, 100, 150, 200} }

const (
	// defaultZoom is 100%, nodes at their drawn size.
	defaultZoom = 4
	zoomPercent = 100
	// maxDrawn is the widest or tallest image a zoomed-out canvas is drawn
	// into: the smallest largest image Ebiten allows. A level that needs a
	// larger one is not offered.
	maxDrawn = 8192
)

// The floating zoom control's geometry, in screen pixels, as in the Map: a
// card inset from the canvas's bottom-right corner holding −, the zoom, +
// and Fit.
const (
	zoomInset  = 12
	zoomPad    = 4
	zoomStep   = 30
	zoomLabel  = 56
	zoomFit    = 44
	zoomShadow = 3
	zoomShade  = 0x22
)

// zoomControl is the canvas's floating zoom control.
type zoomControl struct {
	card                *ui.Button
	out, reset, in, fit *ui.Button
}

func newZoomControl() *zoomControl {
	return &zoomControl{
		card: ui.NewButton("zoom", ""), out: ui.NewButton("zoom-out", "−"), reset: ui.NewButton("zoom-reset", "100%"),
		in: ui.NewButton("zoom-in", "+"), fit: ui.NewButton("zoom-fit", "Fit"),
	}
}

// buttons lists the control's buttons from left to right.
func (z *zoomControl) buttons() []*ui.Button { return []*ui.Button{z.out, z.reset, z.in, z.fit} }

// layout places the control in the canvas's bottom-right corner.
func (z *zoomControl) layout(canvas image.Rectangle, level, lowest int) {
	widths := []int{zoomStep, zoomLabel, zoomStep, zoomFit}
	total := zoomPad
	for _, width := range widths {
		total += width + zoomPad
	}
	right, bottom := canvas.Max.X-zoomInset, canvas.Max.Y-zoomInset
	z.card.Rect = image.Rect(right-total, bottom-ui.ButtonH-ui.Half*zoomPad, right, bottom)
	left := z.card.Rect.Min.X + zoomPad
	for index, button := range z.buttons() {
		button.Rect = image.Rect(left, z.card.Rect.Min.Y+zoomPad, left+widths[index], z.card.Rect.Max.Y-zoomPad)
		left += widths[index] + zoomPad
	}
	z.reset.Label = strconv.Itoa(zoomLevels()[level]) + "%"
	z.out.Disabled, z.in.Disabled = level <= lowest, level == len(zoomLevels())-1
}

// contains reports whether a point is on the control's card.
func (z *zoomControl) contains(point image.Point) bool { return point.In(z.card.Rect) }

// draw renders the control as a card floating over the canvas.
func (z *zoomControl) draw(screen *ebiten.Image, theme *ui.Theme) {
	ui.FillRound(screen, z.card.Rect.Add(image.Pt(0, zoomShadow)), ui.Radius, ui.HexAlpha(0, zoomShade))
	ui.Box(screen, z.card.Rect, ui.ColorBackground())
	for _, button := range z.buttons() {
		button.Draw(screen, theme)
	}
}

// scale is how many screen pixels one canvas pixel takes. A live view in
// the Watch window is never zoomed.
func (s *State) scale() float64 {
	if s.live != nil {
		return 1
	}

	return float64(zoomLevels()[s.zoom]) / zoomPercent
}

// drawnArea is the part of the canvas on screen, in unscaled drawing
// pixels: the canvas itself at 100%, larger zoomed out, smaller zoomed in.
func (s *State) drawnArea() image.Rectangle {
	canvas := s.Canvas()
	size := scaledSize(canvas.Size(), s.scale())

	return image.Rectangle{Min: canvas.Min, Max: canvas.Min.Add(size)}
}

// scaledSize is how many canvas pixels a screen area of the given size
// shows at a scale.
func scaledSize(size image.Point, scale float64) image.Point {
	return image.Pt(int(math.Ceil(float64(size.X)/scale)), int(math.Ceil(float64(size.Y)/scale)))
}

// lowestZoom is the furthest out the canvas can be zoomed at its size.
func (s *State) lowestZoom() int {
	size := s.Canvas().Size()
	for level := range zoomLevels() {
		drawn := scaledSize(size, float64(zoomLevels()[level])/zoomPercent)
		if drawn.X <= maxDrawn && drawn.Y <= maxDrawn {
			return level
		}
	}

	return defaultZoom
}

// onScreen returns where a canvas point is on screen.
func (s *State) onScreen(point image.Point) image.Point {
	canvas, scale := s.Canvas(), s.scale()
	drawn := s.toScreen(point).Sub(canvas.Min)

	return canvas.Min.Add(image.Pt(int(math.Round(float64(drawn.X)*scale)), int(math.Round(float64(drawn.Y)*scale))))
}

// panBy moves the canvas by a distance in screen pixels, keeping the part
// of a canvas pixel that a slow drag at a high zoom moves.
func (s *State) panBy(delta image.Point) {
	scale := s.scale()
	x, y := s.panRest[0]+float64(delta.X)/scale, s.panRest[1]+float64(delta.Y)/scale
	whole := image.Pt(int(x), int(y))
	s.offset = s.offset.Add(whole)
	s.panRest = [2]float64{x - float64(whole.X), y - float64(whole.Y)}
}

// zoomTo changes the zoom level, keeping the canvas point under focus, a
// screen point, where it is.
func (s *State) zoomTo(level int, focus image.Point) {
	level = min(max(level, s.lowestZoom()), len(zoomLevels())-1)
	if level == s.zoom {
		return
	}
	canvas, before := s.Canvas(), s.scale()
	from := focus.Sub(canvas.Min)
	underX := float64(from.X)/before - float64(s.offset.X)
	underY := float64(from.Y)/before - float64(s.offset.Y)
	s.zoom = level
	after := s.scale()
	s.offset = image.Pt(int(math.Round(float64(from.X)/after-underX)), int(math.Round(float64(from.Y)/after-underY)))
	s.panRest = [2]float64{0, 0}
}

// zoomFocus is where keys zoom about: the pointer when it is on the canvas,
// otherwise the canvas's centre.
func (s *State) zoomFocus(cursor image.Point) image.Point {
	if canvas := s.Canvas(); cursor.In(canvas) {
		return cursor
	}

	return ui.Center(s.Canvas())
}

// wheelZoom zooms by a Cmd+scroll, one level for each whole step of the
// wheel, about the pointer.
func (s *State) wheelZoom(input *ui.Input) {
	s.wheel += input.WheelY
	for s.wheel >= 1 {
		s.zoomTo(s.zoom+1, input.Cursor)
		s.wheel--
	}
	for s.wheel <= -1 {
		s.zoomTo(s.zoom-1, input.Cursor)
		s.wheel++
	}
}

// updateZoom handles the zoom control's buttons, and keeps any press on the
// card from reaching the canvas underneath.
func (s *State) updateZoom(input *ui.Input) {
	centre, card := ui.Center(s.Canvas()), s.zoomCard
	switch {
	case card.out.Update(input):
		s.zoomTo(s.zoom-1, centre)
	case card.in.Update(input):
		s.zoomTo(s.zoom+1, centre)
	case card.reset.Update(input):
		s.zoomTo(defaultZoom, centre)
	case card.fit.Update(input):
		s.fitView()
	case input.Pressed && card.contains(input.Cursor):
		input.ClearPointer()
	}
}

// fitView zooms out as far as needed to show the whole workflow, never in
// past 100%, and centres it.
func (s *State) fitView() {
	bounds, ok := s.workflowBounds()
	if !ok {
		return
	}
	wanted := bounds.Size().Add(image.Pt(ui.Half*ui.Inset, ui.Half*ui.Inset))
	room := s.Canvas().Size()
	level := s.lowestZoom()
	for candidate := defaultZoom; candidate > level; candidate-- {
		shown := scaledSize(room, float64(zoomLevels()[candidate])/zoomPercent)
		if wanted.X <= shown.X && wanted.Y <= shown.Y {
			level = candidate

			break
		}
	}
	s.zoom, s.panRest = level, [2]float64{0, 0}
	s.centreOn(bounds)
}

// drawZoomed draws the canvas onto the screen at the current zoom.
func (s *State) drawZoomed(screen *ebiten.Image, docs frame.Docs) {
	canvas := s.Canvas()
	target, ok := screen.SubImage(canvas).(*ebiten.Image)
	if !ok {
		return
	}
	scale := s.scale()
	if scale == 1 {
		s.drawCanvas(target, docs)

		return
	}
	area := s.drawnArea()
	if s.drawn == nil || s.drawn.Bounds() != area {
		if s.drawn != nil {
			s.drawn.Deallocate()
		}
		s.drawn = ebiten.NewImageWithOptions(area, nil)
	}
	s.drawn.Clear()
	s.drawCanvas(s.drawn, docs)
	// Ebiten draws an image from its own top-left corner, wherever its
	// bounds start, onto the screen's own coordinates.
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(float64(canvas.Min.X), float64(canvas.Min.Y))
	options.Filter = ebiten.FilterLinear
	if zoomLevels()[s.zoom]%zoomPercent == 0 {
		options.Filter = ebiten.FilterNearest
	}
	target.DrawImage(s.drawn, options)
}
