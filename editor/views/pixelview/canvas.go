package pixelview

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/tileart"
	"github.com/bradhannah/berad-engine/engine/art"
)

// Toolbar and canvas geometry, in screen pixels.
const (
	toolWidth   = 88
	zoomStep    = 30
	zoomLabel   = 56
	zoomFitW    = 44
	gridWidth   = 60
	marksWidth  = 96
	zoomPercent = 100
	// canvasMargin is kept clear round a fitted tile, and readoutSpace below
	// the canvas holds the readout's two lines.
	canvasMargin = 24
	readoutSpace = 2*ui.LineHeight + ui.Gap
	// The checkerboard under clear pixels follows the tile's pixels, each
	// split into checksPerPixel by checksPerPixel squares once magnified to
	// minSplitZoom; the pixel grid shows from minGridZoom.
	checksPerPixel = 2
	minSplitZoom   = 8
	minGridZoom    = 4
	maxZoom        = 64
	hoverWidth     = 2
	gridAlpha      = 0x30
	hexChecker     = 0xe4e5e1
)

// zoomSteps are the magnifications + and − step through; Fit can choose any
// whole number between.
func zoomSteps() []int { return []int{1, 2, 3, 4, 6, 8, 10, 12, 16, 20, 24, 32, 40, 48, 64} }

// toolbarTop is the top of the toolbar; the canvas starts below it.
func toolbarTop() int { return ui.Inset }

// layoutToolbar places the tool buttons at the left of the toolbar, and the
// zoom and the grid toggle at its right.
func (s *State) layoutToolbar() {
	top := toolbarTop()
	left := ui.Inset
	for index, button := range s.tools {
		button.Rect = image.Rect(left, top, left+toolWidth, top+ui.ButtonH)
		button.On = tools()[index].tool == s.tool
		left += toolWidth + ui.Gap
	}
	right := frame.Content(s.size).Max.X - ui.Inset
	s.grid.Rect = image.Rect(right-gridWidth, top, right, top+ui.ButtonH)
	right = s.grid.Rect.Min.X - ui.Inset
	s.zoomFit.Rect = image.Rect(right-zoomFitW, top, right, top+ui.ButtonH)
	right = s.zoomFit.Rect.Min.X - ui.Gap
	s.zoomIn.Rect = image.Rect(right-zoomStep, top, right, top+ui.ButtonH)
	right = s.zoomIn.Rect.Min.X - zoomLabel
	s.zoomOut.Rect = image.Rect(right-zoomStep, top, right, top+ui.ButtonH)
	s.zoomOut.Disabled = s.zoom <= 1
	s.zoomIn.Disabled = s.zoom >= maxZoom
}

// canvas is the area that shows the tile, between the toolbar and the
// readout.
func (s *State) canvas() image.Rectangle {
	content := frame.Content(s.size)
	top := toolbarTop() + ui.ButtonH + ui.Inset

	return image.Rect(0, top, content.Max.X, max(top, content.Max.Y-readoutSpace))
}

// tileRect is where the tile shows: centred in the canvas, then panned.
func (s *State) tileRect(size int) image.Rectangle {
	side := size * max(s.zoom, 1)
	centre := ui.Center(s.canvas()).Add(s.pan)
	corner := centre.Sub(image.Pt(side/ui.Half, side/ui.Half))

	return image.Rectangle{Min: corner, Max: corner.Add(image.Pt(side, side))}
}

// pixelAt returns the tile pixel under a screen point, and whether the point
// is on the tile within the canvas. Points off the tile give pixels outside
// it, so a stroke dragged off and back draws up to the edge.
func (s *State) pixelAt(point image.Point, size int) (image.Point, bool) {
	rect := s.tileRect(size)
	zoom := max(s.zoom, 1)
	local := point.Sub(rect.Min)
	pixel := image.Pt(floorDiv(local.X, zoom), floorDiv(local.Y, zoom))

	return pixel, point.In(rect) && point.In(s.canvas())
}

// pixelRect returns a tile pixel's screen rectangle.
func (s *State) pixelRect(pixel image.Point, size int) image.Rectangle {
	zoom := max(s.zoom, 1)
	corner := s.tileRect(size).Min.Add(pixel.Mul(zoom))

	return image.Rectangle{Min: corner, Max: corner.Add(image.Pt(zoom, zoom))}
}

func floorDiv(value, divisor int) int {
	quotient := value / divisor
	if value%divisor != 0 && value < 0 {
		quotient--
	}

	return quotient
}

// fitZoom is the largest whole magnification at which a tile fits the
// canvas.
func fitZoom(size int, canvas image.Rectangle) int {
	room := min(canvas.Dx(), canvas.Dy()) - ui.Half*canvasMargin

	return max(1, min(maxZoom, room/size))
}

// fit magnifies the tile as much as fits, and centres it.
func (s *State) fit(docs frame.Docs) {
	if size, ok := s.tileSize(docs); ok {
		s.zoom = fitZoom(size, s.canvas())
		s.pan = image.Pt(0, 0)
	}
}

// zoomBy steps the magnification in or out, keeping the canvas's centre
// still.
func (s *State) zoomBy(steps int) {
	previous := max(s.zoom, 1)
	next := previous
	for _, step := range zoomSteps() {
		if steps > 0 && step > previous {
			next = step

			break
		}
		if steps < 0 && step < previous {
			next = step
		}
	}
	s.zoom = next
	s.pan = image.Pt(s.pan.X*next/previous, s.pan.Y*next/previous)
}

// updateCanvas fits a newly loaded tile, pans and zooms, and runs the current
// tool on the tile.
func (s *State) updateCanvas(input *ui.Input, ctx *frame.Context) {
	size, ok := s.tileSize(ctx.Docs)
	if !ok {
		s.hovering = false

		return
	}
	canvas := s.canvas()
	if s.zoom == 0 {
		s.zoom = fitZoom(size, canvas)
	}
	s.panAndZoom(input, canvas)
	pixel, onTile := s.pixelAt(input.Cursor, size)
	s.hover, s.hovering = pixel, onTile
	switch s.tool {
	case ToolPencil, ToolEraser:
		s.updateStroke(input, ctx, pixel, onTile)
	case ToolFill:
		if input.Pressed && onTile {
			s.fill(ctx, pixel)
		}
	case ToolEyedropper:
		if input.Pressed && onTile {
			s.pick(ctx, pixel)
		}
	}
	if !input.Down {
		s.stroke.active = false
	}
}

// panAndZoom pans the tile with the wheel or a trackpad, and zooms with the
// wheel while Cmd is held.
func (s *State) panAndZoom(input *ui.Input, canvas image.Rectangle) {
	if !input.Cursor.In(canvas) || (input.WheelX == 0 && input.WheelY == 0) {
		return
	}
	if !input.Held.Shortcut {
		s.pan = s.pan.Add(input.WheelPan())

		return
	}
	s.wheel += input.WheelY
	for s.wheel >= 1 {
		s.zoomBy(1)
		s.wheel--
	}
	for s.wheel <= -1 {
		s.zoomBy(-1)
		s.wheel++
	}
}

// picture is the tile drawn at one image pixel per tile pixel, kept until
// the tile or a palette changes, and the checkerboard drawn under it.
type picture struct {
	tile        *tileart.Picture
	checker     *ebiten.Image
	checkerZoom int
}

func newPicture() picture {
	return picture{tile: tileart.NewPicture(), checker: nil, checkerZoom: 0}
}

// refresh redraws the picture when the tile or a palette changed. It
// reports whether there is a tile to show.
func (s *State) refresh(docs frame.Docs) bool {
	tileID, ok := s.tileID(docs)

	return ok && s.picture.tile.Refresh(docs, tileID)
}

// checkerboard returns the checkerboard for a tile shown at a size, drawn
// again only when the size or the zoom changes. Its squares line up with
// the tile's pixels, so the grid never cuts one.
func (s *State) checkerboard(size image.Point) *ebiten.Image {
	zoom := max(s.zoom, 1)
	if s.picture.checker != nil && s.picture.checker.Bounds().Size() == size && s.picture.checkerZoom == zoom {
		return s.picture.checker
	}
	split := 1
	if zoom >= minSplitZoom {
		split = checksPerPixel
	}
	board := image.NewNRGBA(image.Rectangle{Min: image.Pt(0, 0), Max: size})
	light, dark := color.NRGBA(ui.ColorBackground()), color.NRGBA(ui.Hex(hexChecker))
	for row := range size.Y {
		for col := range size.X {
			ink := light
			if (col*split/zoom+row*split/zoom)%ui.Half == 1 {
				ink = dark
			}
			board.SetNRGBA(col, row, ink)
		}
	}
	s.picture.checkerZoom = zoom
	if s.picture.checker != nil {
		s.picture.checker.Deallocate()
	}
	s.picture.checker = ebiten.NewImageFromImage(board)

	return s.picture.checker
}

// drawCanvas draws the tile over its checkerboard, magnified, with the pixel
// grid and the pixel under the cursor outlined.
func (s *State) drawCanvas(screen *ebiten.Image, docs frame.Docs) {
	canvas := s.canvas()
	ui.FillRect(screen, canvas, ui.ColorCanvas())
	ui.Divider(screen, canvas.Min, image.Pt(canvas.Max.X, canvas.Min.Y))
	size, ok := s.tileSize(docs)
	area, clipped := screen.SubImage(canvas).(*ebiten.Image)
	if !ok || !clipped || !s.refresh(docs) {
		s.theme.DrawText(screen, "Loading the sprite…", canvas.Min.X+ui.Inset, canvas.Min.Y+ui.Inset, ui.ColorMuted())

		return
	}
	rect := s.tileRect(size)
	var place ebiten.DrawImageOptions
	place.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	area.DrawImage(s.checkerboard(rect.Size()), &place)
	var magnify ebiten.DrawImageOptions
	magnify.GeoM.Scale(float64(s.zoom), float64(s.zoom))
	magnify.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	area.DrawImage(s.picture.tile.Image(), &magnify)
	if s.grid.On && s.zoom >= minGridZoom {
		ink := ui.HexAlpha(0, gridAlpha)
		for line := 1; line < size; line++ {
			x, y := rect.Min.X+line*s.zoom, rect.Min.Y+line*s.zoom
			ui.Line(area, image.Pt(x, rect.Min.Y), image.Pt(x, rect.Max.Y), ui.Hairline, ink)
			ui.Line(area, image.Pt(rect.Min.X, y), image.Pt(rect.Max.X, y), ui.Hairline, ink)
		}
	}
	ui.StrokeRect(area, rect, ui.Hairline, ui.ColorRule())
	if s.marks.On {
		s.drawMarks(area, docs, size)
	}
	if s.hovering {
		ui.StrokeRect(area, s.pixelRect(s.hover, size), hoverWidth, ui.ColorSelect())
	}
}

// drawMarks outlines the pixels the readout warns about: those painted with
// a colour that has moved, and loose colours.
func (s *State) drawMarks(screen *ebiten.Image, docs frame.Docs, size int) {
	colors := s.colors(docs)
	tileID, ok := s.tileID(docs)
	if !ok {
		return
	}
	docs.View(tileID, func(document client.Document) {
		tileDoc, isTile := document.Model.(*artplugin.TileDocument)
		if !isTile {
			return
		}
		for row := range size {
			for col := range size {
				ref, err := tileDoc.Tile.At(col, row)
				if err == nil && (ref.IsLoose() || colors.Moved(ref)) {
					ui.StrokeRect(screen, s.pixelRect(image.Pt(col, row), size).Inset(1), hoverWidth, ui.ColorWarn())
				}
			}
		}
	})
}

// drawToolbar draws the tool buttons, the zoom and the grid toggle.
func (s *State) drawToolbar(screen *ebiten.Image) {
	for _, button := range s.tools {
		button.Draw(screen, s.theme)
	}
	s.zoomOut.Draw(screen, s.theme)
	s.zoomIn.Draw(screen, s.theme)
	s.zoomFit.Draw(screen, s.theme)
	label := fmt.Sprintf("%d%%", max(s.zoom, 1)*zoomPercent)
	between := image.Rect(s.zoomOut.Rect.Max.X, s.zoomOut.Rect.Min.Y, s.zoomIn.Rect.Min.X, s.zoomOut.Rect.Max.Y)
	s.theme.DrawMono(screen, label, between.Min.X+(between.Dx()-s.theme.MonoWidth(label))/ui.Half,
		between.Min.Y+(between.Dy()-ui.MonoSize)/ui.Half-1, ui.ColorInk())
	s.grid.Draw(screen, s.theme)
}

// drawReadout draws the two lines below the canvas: the pixel under the
// cursor and its colour, or the tool's help, with the tile's name and size
// at the right; and below, anything wrong with the tile.
func (s *State) drawReadout(screen *ebiten.Image, docs frame.Docs) {
	canvas := s.canvas()
	top := canvas.Max.Y + ui.Gap
	size, ok := s.tileSize(docs)
	if !ok {
		return
	}
	line := toolHelp(s.tool)
	if s.hovering {
		line = fmt.Sprintf("Pixel %d, %d", s.hover.X, s.hover.Y)
		if ref, found := s.pixelColor(docs, s.hover); found {
			if name, named := s.colorName(docs, ref); named {
				line += " · " + name
			}
		}
	}
	s.theme.DrawText(screen, line, ui.Inset, top, ui.ColorInk())
	measure := fmt.Sprintf("%d × %d pixels", size, size)
	s.theme.DrawText(screen, measure, canvas.Max.X-ui.Inset-s.theme.TextWidth(measure), top, ui.ColorMuted())
	if problem := s.problem(docs, size); problem != "" {
		room := canvas.Dx() - ui.Half*ui.Inset
		if !s.marks.Rect.Empty() {
			room = s.marks.Rect.Min.X - ui.Gap - ui.Inset
		}
		s.theme.DrawText(screen, s.theme.FitText(problem, room), ui.Inset, top+ui.LineHeight, ui.ColorWarn())
	}
	s.marks.Draw(screen, s.theme)
}

// layoutReadout places Show them at the right of the readout's second line,
// while there are pixels to mark.
func (s *State) layoutReadout(docs frame.Docs) {
	s.marks.Rect = ui.Zero()
	if moved, loose := s.warned(docs); moved+loose > 0 {
		canvas := s.canvas()
		top := canvas.Max.Y + ui.Gap + ui.LineHeight - ui.Gap
		right := canvas.Max.X - ui.Inset
		s.marks.Rect = image.Rect(right-marksWidth, top, right, top+ui.ButtonH)
	}
}

// warned counts the tile's pixels painted with a colour that has moved, and
// its loose colours.
func (s *State) warned(docs frame.Docs) (int, int) {
	colors := s.colors(docs)
	moved, loose := 0, 0
	if tileID, ok := s.tileID(docs); ok {
		docs.View(tileID, func(document client.Document) {
			if tileDoc, isTile := document.Model.(*artplugin.TileDocument); isTile {
				moved = tileDoc.Tile.Count(colors.Moved)
				loose = tileDoc.Tile.Count(art.ColorRef.IsLoose)
			}
		})
	}

	return moved, loose
}

// problem says what is wrong with the tile, if anything: a size other than
// the project's tile size, pixels whose colour cannot be found, pixels
// painted with a colour that has moved, or loose colours. The last two stay
// until the pixels are repainted.
func (s *State) problem(docs frame.Docs, size int) string {
	if project, ok := projectTileSize(docs); ok && project != size {
		return fmt.Sprintf("This tile is %d × %d, but the project's tiles are %d × %d.", size, size, project, project)
	}
	switch missing := s.picture.tile.Missing(); {
	case missing == 1:
		return "1 pixel's colour cannot be found in its palette, so it shows clear."
	case missing > 1:
		return fmt.Sprintf("%d pixels' colours cannot be found in their palettes, so they show clear.", missing)
	}
	moved, loose := s.warned(docs)
	switch {
	case moved > 0:
		return pixels(moved) + " name a colour that has moved to another ramp. Repaint them to clear this."
	case loose > 0:
		return pixels(loose) + " are loose colours, outside every palette. Repaint them to clear this."
	}

	return ""
}

// pixels counts pixels in words.
func pixels(count int) string {
	if count == 1 {
		return "1 pixel"
	}

	return fmt.Sprintf("%d pixels", count)
}
