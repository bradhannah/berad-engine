package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	berad "github.com/bradhannah/berad-sdk/modules/berad-text"
	"github.com/hajimehoshi/ebiten/v2"
	etext "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"
)

// Sizes, in screen pixels.
const (
	TextSize    = 13
	LineHeight  = 20
	Inset       = 14
	RowHeight   = 24
	ButtonH     = 26
	Hairline    = 1
	Outline     = 2
	captionDrop = 6
	// Half and Gap are for centring and small spacing.
	Half        = 2
	Gap         = 4
	third       = 3
	toggleInset = 3
	labelGap    = 8
	swatchTop   = 5
	swatchSize  = 14
	opaque      = 0xff
	byteMask    = 0xff
	redShift    = 16
	greenShift  = 8
)

// The plain white look agreed for the editor, with blue for selection.
const (
	hexBackground = 0xffffff
	hexInk        = 0x1f2328
	hexMuted      = 0x5a6270
	hexSubdued    = 0x9aa1ab
	hexSelect     = 0x1f5fbf
	hexSelectWash = 0xdbe7fb
	hexPanel      = 0xf7f7f5
	hexRule       = 0xd0d3d8
	hexWarn       = 0xb3261e
	hexHover      = 0xeef1f5
	shadeAlpha    = 0x66
)

// Hex returns an opaque colour from a 0xRRGGBB value.
func Hex(rgb uint32) color.RGBA {
	return HexAlpha(rgb, opaque)
}

// HexAlpha returns a colour from a 0xRRGGBB value and an alpha. Go colours
// carry premultiplied alpha, so each channel is scaled by it.
func HexAlpha(rgb uint32, alpha uint8) color.RGBA {
	scale := func(channel uint32) uint8 {
		return uint8(channel & byteMask * uint32(alpha) / opaque) //nolint:gosec // At most 255 by construction.
	}

	return color.RGBA{R: scale(rgb >> redShift), G: scale(rgb >> greenShift), B: scale(rgb), A: alpha}
}

// ColorBackground is the background of windows and fields.
func ColorBackground() color.RGBA { return Hex(hexBackground) }

// ColorInk is the colour of text.
func ColorInk() color.RGBA { return Hex(hexInk) }

// ColorMuted is the colour of headings and secondary text.
func ColorMuted() color.RGBA { return Hex(hexMuted) }

// ColorSubdued is the colour of IDs and placeholders (D4: read-only and
// subdued).
func ColorSubdued() color.RGBA { return Hex(hexSubdued) }

// ColorSelect is the colour of selection and focus.
func ColorSelect() color.RGBA { return Hex(hexSelect) }

// ColorSelectWash is the background of selected text and rows.
func ColorSelectWash() color.RGBA { return Hex(hexSelectWash) }

// ColorPanel is the background of side panels and the status bar.
func ColorPanel() color.RGBA { return Hex(hexPanel) }

// ColorRule is the colour of dividing lines.
func ColorRule() color.RGBA { return Hex(hexRule) }

// ColorWarn is the colour of warnings and unsaved marks.
func ColorWarn() color.RGBA { return Hex(hexWarn) }

// ColorHover is the background of a hovered button or row.
func ColorHover() color.RGBA { return Hex(hexHover) }

// ColorShade dims a window behind a dialog.
func ColorShade() color.RGBA { return HexAlpha(0, shadeAlpha) }

// Theme holds the fonts every widget draws with.
type Theme struct {
	// Face draws labels.
	Face *etext.GoTextFace
	// Font lays out and draws editable text.
	Font *berad.Font
	// Text draws laid-out editable text.
	Text *berad.Renderer
}

// NewTheme loads the editor font. Text rendering needs a running game loop,
// so drawing starts only once Ebitengine runs; layout works without one.
func NewTheme() (*Theme, error) {
	source, err := etext.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}
	var face etext.GoTextFace
	face.Source = source
	face.Size = TextSize
	parsed, err := berad.ParseFont(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("load text font: %w", err)
	}
	var options berad.FontOptions
	options.Size = TextSize
	font, err := parsed.NewFont(options)
	if err != nil {
		return nil, fmt.Errorf("size text font: %w", err)
	}
	renderer, err := berad.NewRenderer(berad.DefaultRendererOptions())
	if err != nil {
		return nil, fmt.Errorf("text renderer: %w", err)
	}

	return &Theme{Face: &face, Font: font, Text: renderer}, nil
}

// Close releases the text renderer.
func (t *Theme) Close() { t.Text.Close() }

// TextWidth measures a label.
func (t *Theme) TextWidth(value string) int {
	width, _ := etext.Measure(value, t.Face, LineHeight)

	return int(width)
}

// DrawText draws a label with its top-left corner at a point.
func (t *Theme) DrawText(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	options := &etext.DrawOptions{}
	options.GeoM.Translate(float64(x), float64(y))
	options.ColorScale.ScaleWithColor(ink)
	etext.Draw(screen, value, t.Face, options)
}

// DrawTextIn draws a label vertically centred in a rectangle, inset from its
// left edge, clipped to it.
func (t *Theme) DrawTextIn(screen *ebiten.Image, value string, rect image.Rectangle, ink color.Color) {
	clipped, ok := screen.SubImage(rect).(*ebiten.Image)
	if !ok {
		return
	}
	t.DrawText(clipped, value, rect.Min.X+captionDrop+Half, rect.Min.Y+(rect.Dy()-TextSize)/Half-1, ink)
}

// FillRect fills a rectangle.
func FillRect(screen *ebiten.Image, rect image.Rectangle, fill color.Color) {
	minX, minY := float32(rect.Min.X), float32(rect.Min.Y)
	vector.FillRect(screen, minX, minY, float32(rect.Dx()), float32(rect.Dy()), fill, false)
}

// StrokeRect outlines a rectangle.
func StrokeRect(screen *ebiten.Image, rect image.Rectangle, width float32, stroke color.Color) {
	minX, minY := float32(rect.Min.X), float32(rect.Min.Y)
	vector.StrokeRect(screen, minX, minY, float32(rect.Dx()), float32(rect.Dy()), width, stroke, false)
}

// Line draws a straight line.
func Line(screen *ebiten.Image, from, to image.Point, width float32, ink color.Color) {
	vector.StrokeLine(screen, float32(from.X), float32(from.Y), float32(to.X), float32(to.Y), width, ink, false)
}

// Heading draws a small muted panel heading.
func (t *Theme) Heading(screen *ebiten.Image, value string, x, y int) {
	t.DrawText(screen, value, x, y, ColorMuted())
}
