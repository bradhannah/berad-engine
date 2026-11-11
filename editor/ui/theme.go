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
	// Waiting is the heavier outline of a field whose draft waits for Save.
	Waiting float32 = 2.5
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

// The desk and the stage: four tones, each with one job. White is a field,
// where the author types or chooses; the page is what is read; the desk is
// the window's own frame; the stage is behind the game. Regions meet at a
// hairline, and the dark outline is kept for what is pressed or typed in.
const (
	hexField = 0xffffff
	hexPage  = 0xf4f2ec
	hexDesk  = 0xe7e4da
	hexStage = 0x35373b
	// The stage's own ink and quieter ink, and the alpha of white for the
	// grid drawn over it.
	hexStageInk    = 0xeceae3
	hexStageMuted  = 0xb4b6ba
	stageGridAlpha = 0x18
)

// One job for each colour: blue is where the author is, orange is running
// now, red is stop. Text is at least 4.5 to 1 against every ground it sits
// on; see TestTextReadsOnEveryGround.
const (
	hexInk        = 0x1f2328
	hexMuted      = 0x4f555e
	hexSubdued    = 0x5f646d
	hexSelect     = 0x1f5fbf
	hexSelectWash = 0xdbe6f8
	hexEdge       = 0xb7b3a6
	hexRule       = 0xc7c3b7
	hexRuleSoft   = 0xdedbd0
	hexWarn       = 0xb3261e
	hexWarnWash   = 0xfbe9e7
	hexCaution    = 0x8a5a00
	hexHover      = 0xe9e6dc
	hexLive       = 0xa54c07
	hexLiveWash   = 0xf8e6d5
	hexGroup      = 0xdcebe0
	shadeAlpha    = 0x66
	// floatAlpha is the strength of the hard shadow under what floats.
	floatAlpha = 0x59
)

// Muted colours that tell document types apart, always beside an icon, never
// alone. None is blue (selection), orange (live) or red (warnings).
const (
	hexZone     = 0x3d7a4b
	hexWorkflow = 0x6a4f9e
	hexNPC      = 0x4c6377
	hexSound    = 0x8a4b72
	hexArt      = 0x267378
)

// Where constants and values meet, a constant's path is amber and a value's
// slate, each with a wash to sit on.
const (
	hexConstant     = 0x8a5a1c
	hexConstantWash = 0xf5e6cc
	hexValue        = 0x3d5f8a
	hexValueWash    = 0xdfe7f2
)

// Embedded IBM Plex fonts (SIL Open Font License; see fonts/OFL.txt).
var (
	//go:embed fonts/IBMPlexSans-Regular.ttf
	sansRegular []byte
	//go:embed fonts/IBMPlexSans-SemiBold.ttf
	sansSemiBold []byte
	//go:embed fonts/IBMPlexSans-Italic.ttf
	sansItalic []byte
	//go:embed fonts/IBMPlexSans-SemiBoldItalic.ttf
	sansSemiBoldItalic []byte
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

// ColorField is white: the ground of what the author types in or chooses
// from, such as a field, a button, a workflow's node or a menu. It is the
// only white on screen.
func ColorField() color.RGBA { return Hex(hexField) }

// ColorPage is the ground of what is read: a view's panels and lists, a
// dialog, and a workflow's canvas.
func ColorPage() color.RGBA { return Hex(hexPage) }

// ColorDesk is the window's own frame: the window bar, the status bar and a
// split's divider.
func ColorDesk() color.RGBA { return Hex(hexDesk) }

// ColorStage is the dark, neutral ground behind the game: a Map's canvas,
// the Pixel editor's and Play's.
func ColorStage() color.RGBA { return Hex(hexStage) }

// ColorStageInk is text and marks written straight on the stage.
func ColorStageInk() color.RGBA { return Hex(hexStageInk) }

// ColorStageMuted is quieter text and marks on the stage.
func ColorStageMuted() color.RGBA { return Hex(hexStageMuted) }

// ColorStageGrid is the faint grid drawn over the stage.
func ColorStageGrid() color.RGBA { return HexAlpha(hexField, stageGridAlpha) }

// ColorReverse is text and marks drawn on a filled colour, such as the
// label of a primary button: white.
func ColorReverse() color.RGBA { return Hex(hexField) }

// ColorInk is the colour of text and of outlines.
func ColorInk() color.RGBA { return Hex(hexInk) }

// ColorMuted is the colour of labels and secondary text.
func ColorMuted() color.RGBA { return Hex(hexMuted) }

// ColorSubdued is the colour of IDs, hints and placeholders (D4: read-only
// and subdued). It still reads at 4.5 to 1 on the desk.
func ColorSubdued() color.RGBA { return Hex(hexSubdued) }

// ColorSelect is the colour of where the author is: the selection, the field
// with the keyboard, the tool in hand and a panel's main button.
func ColorSelect() color.RGBA { return Hex(hexSelect) }

// ColorSelectWash is the background of selected text and rows.
func ColorSelectWash() color.RGBA { return Hex(hexSelectWash) }

// ColorEdge is the hairline where two regions meet, a step darker than the
// desk.
func ColorEdge() color.RGBA { return Hex(hexEdge) }

// ColorRule is the colour of light dividing lines and canvas dots.
func ColorRule() color.RGBA { return Hex(hexRule) }

// ColorRuleSoft is the colour of the faintest lines, such as a canvas grid.
func ColorRuleSoft() color.RGBA { return Hex(hexRuleSoft) }

// ColorWarn is the colour of stop: refused, never runs, will delete.
func ColorWarn() color.RGBA { return Hex(hexWarn) }

// ColorWarnWash is the background of a refusal's message.
func ColorWarnWash() color.RGBA { return Hex(hexWarnWash) }

// ColorCaution is the colour of what deserves a look but is not wrong, such
// as a receiver nothing sends to: amber, beside problems' red.
func ColorCaution() color.RGBA { return Hex(hexCaution) }

// ColorHover is the background of a hovered button or row.
func ColorHover() color.RGBA { return Hex(hexHover) }

// ColorZone tints the icons of zones and floors.
func ColorZone() color.RGBA { return Hex(hexZone) }

// ColorWorkflow tints workflow icons.
func ColorWorkflow() color.RGBA { return Hex(hexWorkflow) }

// ColorNPC tints NPC icons.
func ColorNPC() color.RGBA { return Hex(hexNPC) }

// ColorSound tints the icons of cues and recordings.
func ColorSound() color.RGBA { return Hex(hexSound) }

// ColorArt tints the icons of sprites and palettes.
func ColorArt() color.RGBA { return Hex(hexArt) }

// ColorConstant is the colour of a constant's path among values'.
func ColorConstant() color.RGBA { return Hex(hexConstant) }

// ColorConstantWash is the background of a constant's chip.
func ColorConstantWash() color.RGBA { return Hex(hexConstantWash) }

// ColorValue is the colour of a value's path among constants'.
func ColorValue() color.RGBA { return Hex(hexValue) }

// ColorValueWash is the background of a value's chip.
func ColorValueWash() color.RGBA { return Hex(hexValueWash) }

// ColorLive is the colour of anything happening now in a preview.
func ColorLive() color.RGBA { return Hex(hexLive) }

// ColorLiveWash is the background behind live information.
func ColorLiveWash() color.RGBA { return Hex(hexLiveWash) }

// ColorGroup is the background of a group's heading, such as a container.
func ColorGroup() color.RGBA { return Hex(hexGroup) }

// ColorShade dims a window behind a dialog.
func ColorShade() color.RGBA { return HexAlpha(0, shadeAlpha) }

// ColorFloat is the hard shadow under what floats.
func ColorFloat() color.RGBA { return HexAlpha(0, floatAlpha) }

// Theme holds the fonts every widget draws with.
type Theme struct {
	// Face draws text, Strong its semi-bold form, Italic and StrongItalic
	// their italics, for a message's markup, Small the small uppercase
	// labels, Title headings, Mono and MonoStrong numbers, times and IDs,
	// and Display large figures.
	Face         *etext.GoTextFace
	Strong       *etext.GoTextFace
	Italic       *etext.GoTextFace
	StrongItalic *etext.GoTextFace
	Small        *etext.GoTextFace
	Title        *etext.GoTextFace
	Mono         *etext.GoTextFace
	MonoStrong   *etext.GoTextFace
	Display      *etext.GoTextFace
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
		"sans-italic": sansItalic, "sans-bold-italic": sansSemiBoldItalic,
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
		Face: face("sans", TextSize), Strong: face("sans-bold", TextSize), Italic: face("sans-italic", TextSize),
		StrongItalic: face("sans-bold-italic", TextSize), Small: face("sans-bold", LabelSize),
		Title: face("sans-bold", TitleSize), Mono: face("mono", MonoSize), MonoStrong: face("mono-bold", MonoSize),
		Display: face("mono-bold", DisplaySize), Font: font, Text: renderer,
	}, nil
}

// Close releases the text renderer.
func (t *Theme) Close() { t.Text.Close() }

// TextWidth measures text in the ordinary face.
func (t *Theme) TextWidth(value string) int { return measure(value, t.Face) }

// Wrap breaks text into lines that fit a width, keeping its own line
// breaks.
func (t *Theme) Wrap(text string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			candidate := strings.TrimSpace(line + " " + word)
			if line != "" && t.TextWidth(candidate) > width {
				lines = append(lines, line)
				candidate = word
			}
			line = candidate
		}
		lines = append(lines, line)
	}

	return lines
}

// FitText shortens text in the ordinary face to fit a width, ending it with
// an ellipsis when it had to be cut.
func (t *Theme) FitText(value string, width int) string { return fit(value, width, t.TextWidth) }

// FitStrong shortens text in the semi-bold face to fit a width, as FitText
// does in the ordinary face.
func (t *Theme) FitStrong(value string, width int) string { return fit(value, width, t.StrongWidth) }

// fit shortens text to fit a width as a face measures it, ending it with an
// ellipsis when it had to be cut.
func fit(value string, width int, measured func(string) int) string {
	if measured(value) <= width {
		return value
	}
	runes := []rune(value)
	for cut := len(runes) - 1; cut > 0; cut-- {
		if shortened := strings.TrimRight(string(runes[:cut]), " ") + "\u2026"; measured(shortened) <= width {
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

// FloatDrop is how far the hard shadow under something that floats falls,
// straight down.
const FloatDrop = 2

// Float draws the hard shadow under a box that floats over the window, such
// as a menu, a dialog or a picker: its own shape, FloatDrop lower, never
// blurred. Draw it before the box.
func Float(screen *ebiten.Image, rect image.Rectangle, radius float32) {
	FillRound(screen, rect.Add(image.Pt(0, FloatDrop)), radius, ColorFloat())
}

// FloatBox draws a box that floats: its hard shadow, then the box.
func FloatBox(screen *ebiten.Image, rect image.Rectangle, fill color.Color) {
	Float(screen, rect, Radius)
	Box(screen, rect, fill)
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

// Divider draws the hairline where two regions meet, such as a window's
// panes or a panel and a canvas.
func Divider(screen *ebiten.Image, from, to image.Point) {
	const middle = float32(Hairline) / Half
	vector.StrokeLine(screen, float32(from.X)+middle, float32(from.Y)+middle, float32(to.X)+middle,
		float32(to.Y)+middle, Hairline, ColorEdge(), false)
}

// Line draws a straight line.
func Line(screen *ebiten.Image, from, to image.Point, width float32, ink color.Color) {
	vector.StrokeLine(screen, float32(from.X), float32(from.Y), float32(to.X), float32(to.Y), width, ink, false)
}
