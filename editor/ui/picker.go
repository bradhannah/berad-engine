package ui

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Colour picker geometry, in screen pixels: the height of the hue and
// opacity strips, the radius of the square's ring, how far a strip's mark
// reaches past it, and the size of the checkerboard behind opacity.
const (
	pickerStrip  = 14
	pickerRing   = 5
	pickerReach  = 2
	pickerCheck  = 4
	hexChecker   = 0xe4e5e1
	hueSectors   = 6
	greenSector  = 2
	blueSector   = 4
	channelScale = 255
	percent      = 100
)

// pickerPart is a part of the picker a press can land on.
type pickerPart int

const (
	partNone pickerPart = iota
	partSquare
	partHue
	partOpacity
)

// ColorPicker chooses a colour by eye. It is drawn by the editor itself, so
// it looks and works the same on every system: a square of saturation, left
// to right, and brightness, bottom to top, for the hue chosen in the strip
// below it, then a strip of opacity. A press in a part sets it, and dragging
// keeps setting it until the button is released.
type ColorPicker struct {
	ID   string
	Rect image.Rectangle
	// hue, saturation, bright and opacity run from 0 to 1. The hue is kept
	// while a colour is grey or black, so it is not lost on the way through.
	hue, saturation, bright, opacity float64
	drag                             pickerPart
	// The square and the strips are painted once, and again only when what
	// they show or their size changes.
	square, hues, opacities *ebiten.Image
	paintedHue              float64
	paintedColor            color.RGBA
	paintedSize             image.Point
}

// NewColorPicker returns a picker showing black, placed later by setting
// Rect.
func NewColorPicker(id string) *ColorPicker {
	return &ColorPicker{
		ID: id, Rect: Zero(), hue: 0, saturation: 0, bright: 0, opacity: 1, drag: partNone,
		square: nil, hues: nil, opacities: nil, paintedHue: -1, paintedColor: color.RGBA{R: 0, G: 0, B: 0, A: 0},
		paintedSize: image.Pt(0, 0),
	}
}

// SetColor shows a colour and an opacity, from 0 to 1. While a part is being
// dragged the picker keeps its own, so the drag is not pulled back.
func (p *ColorPicker) SetColor(rgb color.RGBA, opacity float64) {
	if p.drag != partNone {
		return
	}
	p.opacity = clampUnit(opacity)
	rgb.A = channelScale
	if p.RGB() == rgb {
		return
	}
	hue, saturation, bright := rgbToHSV(rgb)
	if saturation > 0 {
		p.hue = hue
	}
	if bright > 0 {
		p.saturation = saturation
	}
	p.bright = bright
}

// RGB is the chosen colour, opaque.
func (p *ColorPicker) RGB() color.RGBA { return hsvToRGB(p.hue, p.saturation, p.bright) }

// Opacity is the chosen opacity, from 0 to 1.
func (p *ColorPicker) Opacity() float64 { return p.opacity }

// Dragging reports whether a part is being dragged.
func (p *ColorPicker) Dragging() bool { return p.drag != partNone }

// squareRect, hueRect and opacityRect are where the picker's parts are.
func (p *ColorPicker) squareRect() image.Rectangle {
	return image.Rect(p.Rect.Min.X, p.Rect.Min.Y, p.Rect.Max.X, p.Rect.Max.Y-Half*(pickerStrip+Gap))
}

func (p *ColorPicker) hueRect() image.Rectangle {
	top := p.squareRect().Max.Y + Gap

	return image.Rect(p.Rect.Min.X, top, p.Rect.Max.X, top+pickerStrip)
}

func (p *ColorPicker) opacityRect() image.Rectangle {
	top := p.hueRect().Max.Y + Gap

	return image.Rect(p.Rect.Min.X, top, p.Rect.Max.X, top+pickerStrip)
}

// partAt is the part a point is on.
func (p *ColorPicker) partAt(point image.Point) pickerPart {
	switch {
	case point.In(p.squareRect()):
		return partSquare
	case point.In(p.hueRect()):
		return partHue
	case point.In(p.opacityRect()):
		return partOpacity
	}

	return partNone
}

// Update handles one frame of input and reports whether the colour or the
// opacity changed.
func (p *ColorPicker) Update(input *Input) bool {
	if p.Rect.Empty() {
		p.drag = partNone

		return false
	}
	if input.Pressed {
		p.drag = p.partAt(input.Cursor)
	}
	changed := false
	if p.drag != partNone && (input.Down || input.Pressed) {
		changed = p.setAt(input.Cursor)
	}
	if !input.Down {
		p.drag = partNone
	}

	return changed
}

// setAt sets the dragged part from a point, kept within the part.
func (p *ColorPicker) setAt(point image.Point) bool {
	before := [4]float64{p.hue, p.saturation, p.bright, p.opacity}
	switch p.drag {
	case partSquare:
		square := p.squareRect()
		p.saturation = fraction(point.X, square.Min.X, square.Dx())
		p.bright = 1 - fraction(point.Y, square.Min.Y, square.Dy())
	case partHue:
		strip := p.hueRect()
		p.hue = fraction(point.X, strip.Min.X, strip.Dx())
	case partOpacity:
		strip := p.opacityRect()
		p.opacity = fraction(point.X, strip.Min.X, strip.Dx())
	case partNone:
	}

	return before != [4]float64{p.hue, p.saturation, p.bright, p.opacity}
}

// fraction is how far a coordinate is along a length from its start, from 0
// at the first pixel to 1 at the last.
func fraction(at, start, length int) float64 {
	return clampUnit(float64(at-start) / float64(max(length-1, 1)))
}

// atFraction is the pixel a fraction of a length falls on, the inverse of
// fraction.
func atFraction(share float64, start, length int) int {
	return start + int(math.Round(clampUnit(share)*float64(max(length-1, 1))))
}

func clampUnit(value float64) float64 { return min(max(value, 0), 1) }

// Locate finds a point of the picker for input scripts, each as percentages:
// "square <saturation> <brightness>", "hue <hue>" and "opacity <opacity>".
func (p *ColorPicker) Locate(value string) (image.Point, bool) {
	words := strings.Fields(value)
	if p.Rect.Empty() || len(words) < Half {
		return image.Pt(0, 0), false
	}
	shares := make([]float64, 0, len(words)-1)
	for _, word := range words[1:] {
		number, err := strconv.Atoi(word)
		if err != nil {
			return image.Pt(0, 0), false
		}
		shares = append(shares, float64(number)/percent)
	}
	switch {
	case words[0] == "square" && len(shares) == Half:
		square := p.squareRect()

		return image.Pt(atFraction(shares[0], square.Min.X, square.Dx()),
			atFraction(1-shares[1], square.Min.Y, square.Dy())), true
	case words[0] == "hue" && len(shares) == 1:
		strip := p.hueRect()

		return image.Pt(atFraction(shares[0], strip.Min.X, strip.Dx()), strip.Min.Y+strip.Dy()/Half), true
	case words[0] == "opacity" && len(shares) == 1:
		strip := p.opacityRect()

		return image.Pt(atFraction(shares[0], strip.Min.X, strip.Dx()), strip.Min.Y+strip.Dy()/Half), true
	}

	return image.Pt(0, 0), false
}

// Draw draws the square, the strips and where the colour is on each.
func (p *ColorPicker) Draw(screen *ebiten.Image) {
	if p.Rect.Empty() {
		return
	}
	p.paint()
	square, hues, opacities := p.squareRect(), p.hueRect(), p.opacityRect()
	for _, part := range []struct {
		image *ebiten.Image
		rect  image.Rectangle
	}{{p.square, square}, {p.hues, hues}, {p.opacities, opacities}} {
		options := &ebiten.DrawImageOptions{}
		options.GeoM.Translate(float64(part.rect.Min.X), float64(part.rect.Min.Y))
		screen.DrawImage(part.image, options)
		StrokeRect(screen, part.rect, Hairline, ColorRule())
	}
	ringX := float32(atFraction(p.saturation, square.Min.X, square.Dx()))
	ringY := float32(atFraction(1-p.bright, square.Min.Y, square.Dy()))
	vector.StrokeCircle(screen, ringX, ringY, pickerRing+1, Hairline, ColorInk(), true)
	vector.StrokeCircle(screen, ringX, ringY, pickerRing, Hairline+1, ColorBackground(), true)
	p.drawMark(screen, hues, p.hue)
	p.drawMark(screen, opacities, p.opacity)
}

// drawMark marks a share of a strip with a bar reaching past its edges.
func (p *ColorPicker) drawMark(screen *ebiten.Image, strip image.Rectangle, share float64) {
	x := atFraction(share, strip.Min.X, strip.Dx())
	mark := image.Rect(x-pickerReach, strip.Min.Y-pickerReach, x+pickerReach+1, strip.Max.Y+pickerReach)
	FillRect(screen, mark, ColorBackground())
	StrokeRect(screen, mark, Hairline, ColorInk())
}

// paint repaints the square, the strips or both when what they show or the
// picker's size has changed.
func (p *ColorPicker) paint() {
	size := p.squareRect().Size()
	resized := size != p.paintedSize || p.square == nil
	if resized {
		p.square = ebiten.NewImage(max(size.X, 1), max(size.Y, 1))
		p.hues = ebiten.NewImage(max(size.X, 1), pickerStrip)
		p.opacities = ebiten.NewImage(max(size.X, 1), pickerStrip)
		p.paintedSize = size
		p.paintStrip(p.hues, func(share float64, _ bool) color.RGBA { return hsvToRGB(share, 1, 1) })
	}
	if resized || p.hue != p.paintedHue {
		p.paintSquare()
		p.paintedHue = p.hue
	}
	if rgb := p.RGB(); resized || rgb != p.paintedColor {
		p.paintStrip(p.opacities, func(share float64, dark bool) color.RGBA { return overChecker(rgb, share, dark) })
		p.paintedColor = rgb
	}
}

// paintSquare paints every saturation and brightness of the hue.
func (p *ColorPicker) paintSquare() {
	width, height := p.square.Bounds().Dx(), p.square.Bounds().Dy()
	pixels := make([]byte, 0, width*height*bytesPerPixel)
	for y := range height {
		for x := range width {
			rgb := hsvToRGB(p.hue, fraction(x, 0, width), 1-fraction(y, 0, height))
			pixels = append(pixels, rgb.R, rgb.G, rgb.B, rgb.A)
		}
	}
	p.square.WritePixels(pixels)
}

// paintStrip paints a strip column by column, each from its share of the
// width; dark marks the checkerboard's darker squares.
func (p *ColorPicker) paintStrip(strip *ebiten.Image, at func(share float64, dark bool) color.RGBA) {
	width, height := strip.Bounds().Dx(), strip.Bounds().Dy()
	pixels := make([]byte, 0, width*height*bytesPerPixel)
	for y := range height {
		for x := range width {
			rgb := at(fraction(x, 0, width), (x/pickerCheck+y/pickerCheck)%Half == 1)
			pixels = append(pixels, rgb.R, rgb.G, rgb.B, rgb.A)
		}
	}
	strip.WritePixels(pixels)
}

// bytesPerPixel is the size of one pixel written to an image.
const bytesPerPixel = 4

// overChecker is a colour at an opacity over a square of the checkerboard.
func overChecker(rgb color.RGBA, opacity float64, dark bool) color.RGBA {
	under := ColorBackground()
	if dark {
		under = Hex(hexChecker)
	}
	blend := func(over, back uint8) uint8 {
		return uint8(math.Round(float64(over)*opacity + float64(back)*(1-opacity)))
	}

	return color.RGBA{R: blend(rgb.R, under.R), G: blend(rgb.G, under.G), B: blend(rgb.B, under.B), A: channelScale}
}

// The four levels a channel takes within one sixth of the hues.
const (
	levelTop = iota
	levelRising
	levelFalling
	levelBottom
)

// hsvToRGB is the opaque colour of a hue, saturation and brightness, each
// from 0 to 1.
func hsvToRGB(hue, saturation, bright float64) color.RGBA {
	sector := math.Mod(hue, 1) * hueSectors
	whole := math.Floor(sector)
	part := sector - whole
	levels := [...]float64{
		levelTop:     bright,
		levelRising:  bright * (1 - saturation*(1-part)),
		levelFalling: bright * (1 - saturation*part),
		levelBottom:  bright * (1 - saturation),
	}
	order := [hueSectors][3]int{
		{levelTop, levelRising, levelBottom}, {levelFalling, levelTop, levelBottom},
		{levelBottom, levelTop, levelRising}, {levelBottom, levelFalling, levelTop},
		{levelRising, levelBottom, levelTop}, {levelTop, levelBottom, levelFalling},
	}[int(whole)%hueSectors]

	return color.RGBA{
		R: toChannel(levels[order[0]]), G: toChannel(levels[order[1]]), B: toChannel(levels[order[2]]), A: channelScale,
	}
}

func toChannel(level float64) uint8 { return uint8(math.Round(clampUnit(level) * channelScale)) }

// rgbToHSV is a colour's hue, saturation and brightness, each from 0 to 1.
// A grey has hue 0.
func rgbToHSV(rgb color.RGBA) (float64, float64, float64) {
	highest, lowest := max(rgb.R, rgb.G, rgb.B), min(rgb.R, rgb.G, rgb.B)
	bright := float64(highest) / channelScale
	if highest == 0 {
		return 0, 0, 0
	}
	spread := float64(highest - lowest)
	saturation := spread / float64(highest)
	if highest == lowest {
		return 0, saturation, bright
	}
	red, green, blue := float64(rgb.R), float64(rgb.G), float64(rgb.B)
	var sector float64
	switch highest {
	case rgb.R:
		sector = (green - blue) / spread
	case rgb.G:
		sector = (blue-red)/spread + greenSector
	default:
		sector = (red-green)/spread + blueSector
	}

	return math.Mod(sector/hueSectors+1, 1), saturation, bright
}
