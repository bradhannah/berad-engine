package pixelview

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
)

// Tool is what pressing on the tile does.
type Tool string

// Tools.
const (
	ToolPencil     Tool = "pencil"
	ToolEraser     Tool = "eraser"
	ToolFill       Tool = "fill"
	ToolEyedropper Tool = "eyedropper"
)

type toolInfo struct {
	tool  Tool
	label string
	key   ui.Key
	help  string
}

// tools lists the tools in toolbar order, each with the letter that chooses
// it.
func tools() []toolInfo {
	return []toolInfo{
		{ToolPencil, "Pencil", "B", "Drag to paint pixels in the chosen colour."},
		{ToolEraser, "Eraser", "E", "Drag to clear pixels."},
		{ToolFill, "Fill", "G", "Click to fill the pixels of one colour that touch, side by side."},
		{ToolEyedropper, "Eyedropper", "I", "Click a pixel to paint with its colour."},
	}
}

// toolHelp says what a tool does.
func toolHelp(tool Tool) string {
	for _, info := range tools() {
		if info.tool == tool {
			return info.help
		}
	}

	return ""
}

// paintColor is the colour the current tool paints: clear for the eraser.
func (s *State) paintColor() art.ColorRef {
	if s.tool == ToolEraser {
		return clearColor()
	}

	return s.color
}

// paintable reports whether the current tool can paint: not with a ramp
// chosen without one of its shades.
func (s *State) paintable(ctx *frame.Context) bool {
	if color := s.paintColor(); color.IsClear() || !color.Shade.IsZero() {
		return true
	}
	ctx.Frame.Status("Choose a shade to paint with")

	return false
}

// updateStroke paints with the pencil or the eraser: a press paints a pixel,
// and dragging paints the pixels on a line to each new one. Every command of
// a stroke shares its gesture, so the stroke undoes as one step.
func (s *State) updateStroke(input *ui.Input, ctx *frame.Context, pixel image.Point, onTile bool) {
	switch {
	case input.Pressed && onTile && !s.stroke.active:
		if !s.paintable(ctx) {
			return
		}
		s.stroke = stroke{active: true, count: s.stroke.count + 1, last: pixel}
		s.paint(ctx, []image.Point{pixel})
	case s.stroke.active && input.Down && pixel != s.stroke.last:
		s.paint(ctx, linePixels(s.stroke.last, pixel)[1:])
		s.stroke.last = pixel
	}
}

// paint sends one command for the pixels on the tile whose colour would
// change.
func (s *State) paint(ctx *frame.Context, pixels []image.Point) {
	tileID, ok := s.tileID(ctx.Docs)
	if !ok {
		return
	}
	color := s.paintColor()
	changed := make([]artplugin.Pixel, 0, len(pixels))
	ctx.Docs.View(tileID, func(document client.Document) {
		tileDoc, isTile := document.Model.(*artplugin.TileDocument)
		if !isTile {
			return
		}
		for _, pixel := range pixels {
			if current, err := tileDoc.Tile.At(pixel.X, pixel.Y); err == nil && current != color {
				changed = append(changed, artplugin.Pixel{Col: pixel.X, Row: pixel.Y, Color: color})
			}
		}
	})
	if len(changed) == 0 {
		return
	}
	s.command(ctx, tileID, artplugin.PaintEdit, s.gesture(), artplugin.PaintArgs{Pixels: changed}, nil)
}

// fill paints a pixel and every pixel joined to it, side by side, in its
// colour, as one command and one undo step.
func (s *State) fill(ctx *frame.Context, start image.Point) {
	tileID, ok := s.tileID(ctx.Docs)
	if !ok || !s.paintable(ctx) {
		return
	}
	color := s.paintColor()
	var region []image.Point
	ctx.Docs.View(tileID, func(document client.Document) {
		if tileDoc, isTile := document.Model.(*artplugin.TileDocument); isTile {
			region = fillRegion(tileDoc.Tile, start, color)
		}
	})
	if len(region) == 0 {
		return
	}
	pixels := make([]artplugin.Pixel, 0, len(region))
	for _, pixel := range region {
		pixels = append(pixels, artplugin.Pixel{Col: pixel.X, Row: pixel.Y, Color: color})
	}
	s.stroke.count++
	s.command(ctx, tileID, artplugin.PaintEdit, s.gesture(), artplugin.PaintArgs{Pixels: pixels}, nil)
}

// fillRegion returns the pixels a fill from start paints: start, and every
// pixel of its colour joined to it side by side, within the tile. It returns
// none when start is already the fill's colour.
func fillRegion(tile *art.Tile, start image.Point, color art.ColorRef) []image.Point {
	target, err := tile.At(start.X, start.Y)
	if err != nil || target == color {
		return nil
	}
	size := tile.Size()
	seen := make([]bool, size*size)
	seen[start.Y*size+start.X] = true
	queue := []image.Point{start}
	var region []image.Point
	for len(queue) > 0 {
		pixel := queue[0]
		queue = queue[1:]
		region = append(region, pixel)
		for _, side := range []image.Point{image.Pt(1, 0), image.Pt(-1, 0), image.Pt(0, 1), image.Pt(0, -1)} {
			next := pixel.Add(side)
			if next.X < 0 || next.Y < 0 || next.X >= size || next.Y >= size || seen[next.Y*size+next.X] {
				continue
			}
			if ref, atErr := tile.At(next.X, next.Y); atErr == nil && ref == target {
				seen[next.Y*size+next.X] = true
				queue = append(queue, next)
			}
		}
	}

	return region
}

// pick chooses the colour of a pixel to paint with: a colour that has moved
// at its new place. A colour that cannot be found is not chosen, and nor is a
// loose colour, which names no palette and is not painted with.
func (s *State) pick(ctx *frame.Context, pixel image.Point) {
	ref, ok := s.pixelColor(ctx.Docs, pixel)
	if !ok {
		return
	}
	if ref.IsLoose() {
		ctx.Frame.Status("A loose colour, " + ref.Value.String() + ", names no palette: repaint it with a palette colour")

		return
	}
	name, named := s.colorName(ctx.Docs, ref)
	if !named {
		ctx.Frame.Status("That pixel's colour cannot be found in its palette")

		return
	}
	if found, has := palette(ctx.Docs, ref.Palette); has && !ref.IsClear() {
		if ramp, placed := found.Place(ref.Shade); placed {
			ref = art.Named(ref.Palette, ramp, ref.Shade)
		}
	}
	s.color = ref
	ctx.Frame.Status("Painting with " + name)
}

// linePixels returns the pixels on a line from one pixel to another, both
// included, so a fast drag leaves no gaps.
func linePixels(from, to image.Point) []image.Point {
	deltaX, deltaY := abs(to.X-from.X), -abs(to.Y-from.Y)
	stepX, stepY := sign(to.X-from.X), sign(to.Y-from.Y)
	balance := deltaX + deltaY
	pixels := []image.Point{from}
	for point := from; point != to; {
		doubled := ui.Half * balance
		if doubled >= deltaY {
			balance += deltaY
			point.X += stepX
		}
		if doubled <= deltaX {
			balance += deltaX
			point.Y += stepY
		}
		pixels = append(pixels, point)
	}

	return pixels
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}

func sign(value int) int {
	switch {
	case value > 0:
		return 1
	case value < 0:
		return -1
	}

	return 0
}
