package tileart

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/ui"
)

// A template's marks and guides are drawn the same way in the Pixel editor
// and the Template editor (increment 16): pixels it does not allow are tinted
// dark and hatched, pixels it discourages are tinted amber and dotted, and
// each guide is a dashed line reaching past the tile, with its name.

// Template geometry and inks. From PatternZoom, marks show a pattern as well
// as a tint.
const (
	PatternZoom     = 6
	dotDivisor      = 6
	minDot          = 2
	dashLength      = 6
	dashGap         = 4
	guideWidth      = 2
	guideReach      = 10
	hexNotAllowed   = 0x1f2328
	hexDiscouraged  = 0xa16207
	notAllowedTint  = 0x40
	notAllowedHatch = 0x80
	discouragedTint = 0x38
	discouragedDot  = 0xc0
)

// NotAllowedInk is the ink of what a template does not allow.
func NotAllowedInk() color.RGBA { return ui.HexAlpha(hexNotAllowed, notAllowedHatch) }

// DiscouragedInk is the ink of what a template discourages.
func DiscouragedInk() color.RGBA { return ui.HexAlpha(hexDiscouraged, discouragedDot) }

// DrawMarks tints and patterns each pixel a template marks, on a tile of a
// size shown in rect at a whole magnification.
func DrawMarks(screen *ebiten.Image, template artplugin.Template, rect image.Rectangle, size, zoom int) {
	zoom = max(zoom, 1)
	pattern := zoom >= PatternZoom
	for row := range size {
		for col := range size {
			corner := rect.Min.Add(image.Pt(col*zoom, row*zoom))
			pixel := image.Rectangle{Min: corner, Max: corner.Add(image.Pt(zoom, zoom))}
			switch template.Mark(col, row, size, size) {
			case artplugin.MarkNotAllowed:
				ui.FillRect(screen, pixel, ui.HexAlpha(hexNotAllowed, notAllowedTint))
				if pattern {
					ui.Line(screen, image.Pt(pixel.Min.X, pixel.Max.Y), image.Pt(pixel.Max.X, pixel.Min.Y), ui.Hairline,
						NotAllowedInk())
				}
			case artplugin.MarkDiscouraged:
				ui.FillRect(screen, pixel, ui.HexAlpha(hexDiscouraged, discouragedTint))
				if pattern {
					dot := max(minDot, zoom/dotDivisor)
					start := ui.Center(pixel).Sub(image.Pt(dot/ui.Half, dot/ui.Half))
					ui.FillRect(screen, image.Rectangle{Min: start, Max: start.Add(image.Pt(dot, dot))}, DiscouragedInk())
				}
			case artplugin.MarkOpen:
			}
		}
	}
}

// DrawGuides draws a template's guides over a tile of a size shown in rect
// at a whole magnification: across guides named at their left, down guides
// above the tile.
func DrawGuides(screen *ebiten.Image, theme *ui.Theme, template artplugin.Template, rect image.Rectangle, size,
	zoom int,
) {
	zoom = max(zoom, 1)
	ink := ui.ColorSelect()
	for _, guide := range template.Guides {
		offset := guide.Position(size, size) * zoom
		if guide.Line == artplugin.LineDown {
			x := rect.Min.X + offset
			dashed(screen, image.Pt(x, rect.Min.Y-guideReach), image.Pt(x, rect.Max.Y+guideReach), ink)
			theme.DrawSmall(screen, guide.Name, x-theme.SmallWidth(guide.Name)/ui.Half,
				rect.Min.Y-guideReach-ui.LineHeight, ink)

			continue
		}
		y := rect.Min.Y + offset
		dashed(screen, image.Pt(rect.Min.X-guideReach, y), image.Pt(rect.Max.X+guideReach, y), ink)
		theme.DrawSmall(screen, guide.Name, rect.Min.X-guideReach-ui.Gap-theme.SmallWidth(guide.Name),
			y-ui.LineHeight/ui.Half, ink)
	}
}

// dashed draws a dashed line across or down.
func dashed(screen *ebiten.Image, from, to image.Point, ink color.Color) {
	length := max(abs(to.X-from.X), abs(to.Y-from.Y))
	step := image.Pt(sign(to.X-from.X), sign(to.Y-from.Y))
	for start := 0; start < length; start += dashLength + dashGap {
		end := min(length, start+dashLength)
		ui.Line(screen, from.Add(step.Mul(start)), from.Add(step.Mul(end)), guideWidth, ink)
	}
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
