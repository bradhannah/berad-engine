package ui

import (
	"bytes"
	_ "embed" // The editor's fonts are embedded.
	"fmt"
	"image"
	"image/color"
	"strings"

	berad "github.com/bradhannah/berad-sdk/modules/berad-text"
	"github.com/hajimehoshi/ebiten/v2"
	etext "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Sizes, in screen pixels.
const (
	TextSize  = 13
	LabelSize = 11
	MonoSize  = 11
	TitleSize = 16
	// DisplaySize is the size of large figures, such as the Clock window's
	// time.
	DisplaySize = 36
	LineHeight  = 20
	Inset       = 14
	RowHeight   = 24
	ButtonH     = 26
	Hairline    = 1
	Outline     = 2
	captionDrop = 6
	// Border is the width of the outlines round boxes, buttons and panels,
	// and Radius the rounding of their corners.
	Border float32 = 1.5
	Radius float32 = 6
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
	// letterSpacing spreads the letters of small uppercase labels.
	letterSpacing = 0.7
)

// The plain white look of the editor wireframes: dark ink outlines, blue for
// selection and orange for anything live.
const (
	hexBackground = 0xffffff
	hexInk        = 0x1f2328
	hexMuted      = 0x5a6270
	hexSubdued    = 0x8a919c
	hexSelect     = 0x1f5fbf
	hexSelectWash = 0xe8eef8
	hexPanel      = 0xf7f7f5
	hexCanvas     = 0xfafaf8
	hexOutside    = 0xebece8
	hexRule       = 0xc9cbc5
	hexRuleSoft   = 0xe4e5e1
	hexWarn       = 0xb3261e
	hexHover      = 0xf0f0ec
	hexLive       = 0xb4540a
	hexLiveWash   = 0xfbf1e8
	hexGroup      = 0xe3f0ec
	shadeAlpha    = 0x66
)

// Muted colours that tell document types apart, always beside an icon, never
// alone. None is blue (selection), orange (live) or red (warnings).
const (
	hexZone     = 0x3d7a4b
	hexWorkflow = 0x6a4f9e
	hexNPC      = 0x4c6377
	hexText     = 0x7a6248
	hexSound    = 0x8a4b72
	hexArt      = 0x267378
)

// Embedded IBM Plex fonts (SIL Open Font License; see fonts/OFL.txt).
var (
	//go:embed fonts/IBMPlexSans-Regular.ttf
	sansRegular []byte
	//go:embed fonts/IBMPlexSans-SemiBold.ttf
	sansSemiBold []byte
	//go:embed fonts/IBMPlexMono-Regular.ttf
	monoRegular []byte
	//go:embed fonts/IBMPlexMono-SemiBold.ttf
	monoSemiBold []byte
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

// ColorInk is the colour of text and of outlines.
func ColorInk() color.RGBA { return Hex(hexInk) }

// ColorMuted is the colour of labels and secondary text.
func ColorMuted() color.RGBA { return Hex(hexMuted) }

// ColorSubdued is the colour of IDs and placeholders (D4: read-only and
// subdued).
func ColorSubdued() color.RGBA { return Hex(hexSubdued) }

// ColorSelect is the colour of selection, focus and links.
func ColorSelect() color.RGBA { return Hex(hexSelect) }

// ColorSelectWash is the background of selected text and rows.
func ColorSelectWash() color.RGBA { return Hex(hexSelectWash) }

// ColorPanel is the background of the status bar and quiet panels.
func ColorPanel() color.RGBA { return Hex(hexPanel) }

// ColorCanvas is the background of a map or workflow canvas.
func ColorCanvas() color.RGBA { return Hex(hexCanvas) }

// ColorOutside is the shade of a map beyond the floor bounds.
func ColorOutside() color.RGBA { return Hex(hexOutside) }

// ColorRule is the colour of light dividing lines and canvas dots.
func ColorRule() color.RGBA { return Hex(hexRule) }

// ColorRuleSoft is the colour of the faintest lines, such as a canvas grid.
func ColorRuleSoft() color.RGBA { return Hex(hexRuleSoft) }

// ColorWarn is the colour of warnings and unsaved marks.
func ColorWarn() color.RGBA { return Hex(hexWarn) }

// ColorHover is the background of a hovered button or row.
func ColorHover() color.RGBA { return Hex(hexHover) }

// ColorZone tints the icons of zones and floors.
func ColorZone() color.RGBA { return Hex(hexZone) }

// ColorWorkflow tints workflow icons.
func ColorWorkflow() color.RGBA { return Hex(hexWorkflow) }

// ColorNPC tints NPC icons.
func ColorNPC() color.RGBA { return Hex(hexNPC) }

// ColorText tints text document icons.
func ColorText() color.RGBA { return Hex(hexText) }

// ColorSound tints the icons of cues and recordings.
func ColorSound() color.RGBA { return Hex(hexSound) }

// ColorArt tints the icons of sprites and palettes.
func ColorArt() color.RGBA { return Hex(hexArt) }

// ColorLive is the colour of anything happening now in a preview.
func ColorLive() color.RGBA { return Hex(hexLive) }

// ColorLiveWash is the background behind live information.
func ColorLiveWash() color.RGBA { return Hex(hexLiveWash) }

// ColorGroup is the background of a group's heading, such as a container.
func ColorGroup() color.RGBA { return Hex(hexGroup) }

// ColorShade dims a window behind a dialog.
func ColorShade() color.RGBA { return HexAlpha(0, shadeAlpha) }

// Theme holds the fonts every widget draws with.
type Theme struct {
	// Face draws text, Strong its semi-bold form, Small the small uppercase
	// labels, Title headings, Mono and MonoStrong numbers, times and IDs,
	// and Display large figures.
	Face       *etext.GoTextFace
	Strong     *etext.GoTextFace
	Small      *etext.GoTextFace
	Title      *etext.GoTextFace
	Mono       *etext.GoTextFace
	MonoStrong *etext.GoTextFace
	Display    *etext.GoTextFace
	// Font lays out and draws editable text.
	Font *berad.Font
	// Text draws laid-out editable text.
	Text *berad.Renderer
}

// NewTheme loads the editor fonts. Text rendering needs a running game loop,
// so drawing starts only once Ebitengine runs; layout works without one.
func NewTheme() (*Theme, error) {
	sources := map[string]*etext.GoTextFaceSource{}
	for name, data := range map[string][]byte{
		"sans": sansRegular, "sans-bold": sansSemiBold, "mono": monoRegular, "mono-bold": monoSemiBold,
	} {
		source, err := etext.NewGoTextFaceSource(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("load font %s: %w", name, err)
		}
		sources[name] = source
	}
	face := func(name string, size float64) *etext.GoTextFace {
		var made etext.GoTextFace
		made.Source = sources[name]
		made.Size = size

		return &made
	}
	parsed, err := berad.ParseFont(sansRegular)
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

	return &Theme{
		Face: face("sans", TextSize), Strong: face("sans-bold", TextSize), Small: face("sans-bold", LabelSize),
		Title: face("sans-bold", TitleSize), Mono: face("mono", MonoSize), MonoStrong: face("mono-bold", MonoSize),
		Display: face("mono-bold", DisplaySize), Font: font, Text: renderer,
	}, nil
}

// Close releases the text renderer.
func (t *Theme) Close() { t.Text.Close() }

// TextWidth measures text in the ordinary face.
func (t *Theme) TextWidth(value string) int { return measure(value, t.Face) }

// FitText shortens text in the ordinary face to fit a width, ending it with
// an ellipsis when it had to be cut.
func (t *Theme) FitText(value string, width int) string {
	if t.TextWidth(value) <= width {
		return value
	}
	runes := []rune(value)
	for cut := len(runes) - 1; cut > 0; cut-- {
		if shortened := strings.TrimRight(string(runes[:cut]), " ") + "\u2026"; t.TextWidth(shortened) <= width {
			return shortened
		}
	}

	return ""
}

// StrongWidth measures text in the semi-bold face.
func (t *Theme) StrongWidth(value string) int { return measure(value, t.Strong) }

// MonoWidth measures text in the monospaced face.
func (t *Theme) MonoWidth(value string) int { return measure(value, t.Mono) }

func measure(value string, face *etext.GoTextFace) int {
	width, _ := etext.Measure(value, face, LineHeight)

	return int(width)
}

// DrawText draws text with its top-left corner at a point.
func (t *Theme) DrawText(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	draw(screen, value, t.Face, float64(x), float64(y), ink)
}

// DrawStrong draws semi-bold text with its top-left corner at a point.
func (t *Theme) DrawStrong(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	draw(screen, value, t.Strong, float64(x), float64(y), ink)
}

// DrawTitle draws a heading with its top-left corner at a point.
func (t *Theme) DrawTitle(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	draw(screen, value, t.Title, float64(x), float64(y), ink)
}

// DrawSmall draws small semi-bold text, such as a name beside a mark on a
// map, with its top-left corner at a point.
func (t *Theme) DrawSmall(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	draw(screen, value, t.Small, float64(x), float64(y), ink)
}

// SmallWidth measures text drawn with DrawSmall.
func (t *Theme) SmallWidth(value string) int { return measure(value, t.Small) }

// DrawMono draws monospaced text with its top-left corner at a point.
func (t *Theme) DrawMono(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	draw(screen, value, t.Mono, float64(x), float64(y), ink)
}

// DrawMonoStrong draws semi-bold monospaced text.
func (t *Theme) DrawMonoStrong(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	draw(screen, value, t.MonoStrong, float64(x), float64(y), ink)
}

// DrawDisplay draws large semi-bold monospaced figures with their top-left
// corner at a point.
func (t *Theme) DrawDisplay(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	draw(screen, value, t.Display, float64(x), float64(y), ink)
}

// DrawLabel draws a small uppercase label with spread letters, the
// wireframes' style for headings and field names.
func (t *Theme) DrawLabel(screen *ebiten.Image, value string, x, y int, ink color.Color) {
	left := float64(x)
	for _, letter := range strings.ToUpper(value) {
		draw(screen, string(letter), t.Small, left, float64(y), ink)
		width, _ := etext.Measure(string(letter), t.Small, LineHeight)
		left += width + letterSpacing
	}
}

// LabelWidth measures a label drawn with DrawLabel.
func (t *Theme) LabelWidth(value string) int {
	width := 0.0
	for _, letter := range strings.ToUpper(value) {
		advance, _ := etext.Measure(string(letter), t.Small, LineHeight)
		width += advance + letterSpacing
	}

	return int(width)
}

func draw(screen *ebiten.Image, value string, face *etext.GoTextFace, x, y float64, ink color.Color) {
	options := &etext.DrawOptions{}
	options.GeoM.Translate(x, y)
	options.ColorScale.ScaleWithColor(ink)
	etext.Draw(screen, value, face, options)
}

// DrawTextIn draws text vertically centred in a rectangle, inset from its
// left edge, clipped to it.
func (t *Theme) DrawTextIn(screen *ebiten.Image, value string, rect image.Rectangle, ink color.Color) {
	clipped, ok := screen.SubImage(rect).(*ebiten.Image)
	if !ok {
		return
	}
	t.DrawText(clipped, value, rect.Min.X+captionDrop+Half, rect.Min.Y+(rect.Dy()-TextSize)/Half-1, ink)
}

// Heading draws a small muted panel heading.
func (t *Theme) Heading(screen *ebiten.Image, value string, x, y int) {
	t.DrawLabel(screen, value, x, y, ColorMuted())
}

// DrawSection draws a section heading: uppercase semi-bold ink with spread
// letters, larger than a panel heading so a list's groups stand out.
func (t *Theme) DrawSection(screen *ebiten.Image, value string, x, y int) {
	left := float64(x)
	for _, letter := range strings.ToUpper(value) {
		draw(screen, string(letter), t.Strong, left, float64(y), ColorInk())
		width, _ := etext.Measure(string(letter), t.Strong, LineHeight)
		left += width + letterSpacing
	}
}

// SectionWidth measures a heading drawn with DrawSection.
func (t *Theme) SectionWidth(value string) int {
	width := 0.0
	for _, letter := range strings.ToUpper(value) {
		advance, _ := etext.Measure(string(letter), t.Strong, LineHeight)
		width += advance + letterSpacing
	}

	return int(width)
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

// FillRound fills a rectangle with rounded corners.
func FillRound(screen *ebiten.Image, rect image.Rectangle, radius float32, fill color.Color) {
	if rect.Empty() {
		return
	}
	options := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	options.ColorScale.ScaleWithColor(fill)
	vector.FillPath(screen, rounded(rect, radius, 0), &vector.FillOptions{FillRule: vector.FillRuleNonZero}, options)
}

// StrokeRound outlines a rectangle with rounded corners, keeping the line
// inside the rectangle.
func StrokeRound(screen *ebiten.Image, rect image.Rectangle, radius, width float32, stroke color.Color) {
	if rect.Empty() {
		return
	}
	options := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	options.ColorScale.ScaleWithColor(stroke)
	var line vector.StrokeOptions
	line.Width = width
	vector.StrokePath(screen, rounded(rect, radius, width/Half), &line, options)
}

// Box draws the wireframes' basic box: a rounded fill with an ink outline.
func Box(screen *ebiten.Image, rect image.Rectangle, fill color.Color) {
	FillRound(screen, rect, Radius, fill)
	StrokeRound(screen, rect, Radius, Border, ColorInk())
}

// rounded returns the path of a rounded rectangle, inset by inset pixels.
func rounded(rect image.Rectangle, radius, inset float32) *vector.Path {
	left, top := float32(rect.Min.X)+inset, float32(rect.Min.Y)+inset
	right, bottom := float32(rect.Max.X)-inset, float32(rect.Max.Y)-inset
	radius = max(min(radius, (right-left)/Half, (bottom-top)/Half), 0)
	var path vector.Path
	path.MoveTo(left+radius, top)
	path.LineTo(right-radius, top)
	path.ArcTo(right, top, right, top+radius, radius)
	path.LineTo(right, bottom-radius)
	path.ArcTo(right, bottom, right-radius, bottom, radius)
	path.LineTo(left+radius, bottom)
	path.ArcTo(left, bottom, left, bottom-radius, radius)
	path.LineTo(left, top+radius)
	path.ArcTo(left, top, left+radius, top, radius)
	path.Close()

	return &path
}

// Divider draws the ink line between a window's panes.
func Divider(screen *ebiten.Image, from, to image.Point) {
	vector.StrokeLine(screen, float32(from.X)+Border/Half, float32(from.Y)+Border/Half, float32(to.X)+Border/Half,
		float32(to.Y)+Border/Half, Border, ColorInk(), true)
}

// Line draws a straight line.
func Line(screen *ebiten.Image, from, to image.Point, width float32, ink color.Color) {
	vector.StrokeLine(screen, float32(from.X), float32(from.Y), float32(to.X), float32(to.Y), width, ink, false)
}
