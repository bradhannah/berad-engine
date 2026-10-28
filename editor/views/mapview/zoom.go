package mapview

import (
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// The floating zoom control's geometry, in screen pixels: a card inset from
// the grid's bottom-right corner holding −, the zoom, + and Fit.
const (
	zoomInset   = 12
	zoomPad     = 4
	zoomStep    = 30
	zoomLabel   = 56
	zoomFit     = 44
	zoomPercent = 100
	zoomShadow  = 3
	zoomShade   = 0x22
)

// zoomControl is the map's floating zoom control.
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

// layout places the control in the grid's bottom-right corner and labels the
// zoom as a percentage of the tile size, so 100% shows sprites at their
// actual size.
func (z *zoomControl) layout(grid image.Rectangle, view camera) {
	widths := []int{zoomStep, zoomLabel, zoomStep, zoomFit}
	total := zoomPad
	for _, width := range widths {
		total += width + zoomPad
	}
	right, bottom := grid.Max.X-zoomInset, grid.Max.Y-zoomInset
	z.card.Rect = image.Rect(right-total, bottom-ui.ButtonH-ui.Half*zoomPad, right, bottom)
	left := z.card.Rect.Min.X + zoomPad
	for index, button := range z.buttons() {
		button.Rect = image.Rect(left, z.card.Rect.Min.Y+zoomPad, left+widths[index], z.card.Rect.Max.Y-zoomPad)
		left += widths[index] + zoomPad
	}
	z.reset.Label = strconv.Itoa(view.cellSize()*zoomPercent/view.tile) + "%"
	z.out.Disabled, z.in.Disabled = view.zoom == 0, view.zoom == len(zoomSizes(view.tile))-1
}

// update handles the control's buttons, and keeps any press on the card from
// reaching the grid underneath.
func (z *zoomControl) update(input *ui.Input, view *camera, grid, bounds image.Rectangle) {
	centre := ui.Center(grid)
	switch {
	case z.out.Update(input):
		view.zoomBy(-1, centre, grid)
	case z.in.Update(input):
		view.zoomBy(1, centre, grid)
	case z.reset.Update(input):
		view.zoomBy(defaultZoom-view.zoom, centre, grid)
	case z.fit.Update(input):
		view.fit(bounds, grid)
	case input.Pressed && z.contains(input.Cursor):
		input.ClearPointer()
	}
}

// contains reports whether a point is on the control's card.
func (z *zoomControl) contains(point image.Point) bool { return point.In(z.card.Rect) }

// locate finds a button of the control, for input scripts.
func (z *zoomControl) locate(id string) (image.Point, bool) {
	for _, button := range z.buttons() {
		if button.ID == id {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// draw renders the control as a card floating over the map.
func (z *zoomControl) draw(screen *ebiten.Image, theme *ui.Theme) {
	ui.FillRound(screen, z.card.Rect.Add(image.Pt(0, zoomShadow)), ui.Radius, ui.HexAlpha(0, zoomShade))
	ui.Box(screen, z.card.Rect, ui.ColorBackground())
	for _, button := range z.buttons() {
		button.Draw(screen, theme)
	}
}
