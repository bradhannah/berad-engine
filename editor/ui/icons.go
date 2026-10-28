package ui

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Icon is a small drawing of a document type, shown before its name.
type Icon int

// Icons of the document types, of a folder, which holds documents of one
// type, and of a sound: a cue or a recording of the audio library. The
// project, the clock and Watch mark the windows that show no one document,
// and Settings the project's settings.
const (
	IconNone Icon = iota
	IconZone
	IconFloor
	IconWorkflow
	IconNPC
	IconText
	IconFolder
	IconSound
	IconProject
	IconClock
	IconWatch
	IconSprite
	IconPalette
	IconSettings
)

// Icon geometry, in screen pixels: icons are IconSize square.
const (
	IconSize    = 14
	iconStroke  = 1.5
	iconRadius  = 3
	iconDot     = 2.5
	iconHead    = 3
	iconPageGap = 2
	iconLineGap = 3
	iconSlab    = 4
	// A folder's tab is this wide and its body starts this far down.
	iconTab    = 6
	iconTabTop = 3
	// A speaker's box is this wide and as tall as its cone's narrow end;
	// its wave is this far out, a quarter turn long.
	iconSpeaker = 4
	iconWave    = 4
	quarterTurn = math.Pi / 4
	// The project is a list of rows; a clock's hands stop this far short of
	// its rim; an eye's lids curve towards points this far from its middle.
	iconRows   = 3
	iconRowGap = IconSize / iconRows
	iconHand   = 4
	iconLid    = 8
	// A sprite is drawn in pixels this big; a palette is a ramp of swatches
	// this wide, each lighter than the last by fadeStep; a setting's slider
	// has its knob this far from one end.
	iconPixel  = 2
	iconSwatch = 4
	fadeStep   = 0x55
	iconKnob   = 4
)

// IconColor returns the muted colour of an icon's type.
func IconColor(icon Icon) color.RGBA {
	switch icon {
	case IconZone, IconFloor:
		return ColorZone()
	case IconWorkflow:
		return ColorWorkflow()
	case IconNPC:
		return ColorNPC()
	case IconText:
		return ColorText()
	case IconSound:
		return ColorSound()
	case IconFolder:
		return ColorMuted()
	case IconWatch:
		return ColorLive()
	case IconSprite, IconPalette:
		return ColorArt()
	case IconNone, IconProject, IconClock, IconSettings:
	}

	return ColorInk()
}

// DrawIcon draws an icon in its type's colour with its top-left corner at a
// point.
func DrawIcon(screen *ebiten.Image, icon Icon, corner image.Point) {
	ink := IconColor(icon)
	box := image.Rect(corner.X, corner.Y, corner.X+IconSize, corner.Y+IconSize)
	left, top := float32(corner.X), float32(corner.Y)
	switch icon {
	case IconZone:
		// A square in four: the cells of a map.
		StrokeRound(screen, box, iconRadius, iconStroke, ink)
		vector.StrokeLine(screen, left+IconSize/Half, top+1, left+IconSize/Half, top+IconSize-1, iconStroke, ink, true)
		vector.StrokeLine(screen, left+1, top+IconSize/Half, left+IconSize-1, top+IconSize/Half, iconStroke, ink, true)
	case IconFloor:
		// A flat slab seen from above at an angle.
		var slab vector.Path
		slab.MoveTo(left+iconSlab, top+iconSlab)
		slab.LineTo(left+IconSize, top+iconSlab)
		slab.LineTo(left+IconSize-iconSlab, top+IconSize-iconSlab)
		slab.LineTo(left, top+IconSize-iconSlab)
		slab.Close()
		fillPath(screen, &slab, ink)
	case IconWorkflow:
		// One step leading to two.
		start, end := left+iconDot, left+IconSize-iconDot
		vector.StrokeLine(screen, start, top+IconSize/Half, end, top+iconDot, iconStroke, ink, true)
		vector.StrokeLine(screen, start, top+IconSize/Half, end, top+IconSize-iconDot, iconStroke, ink, true)
		for _, dot := range [][2]float32{{iconDot, IconSize / Half}, {IconSize - iconDot, iconDot}, {
			IconSize - iconDot, IconSize - iconDot,
		}} {
			vector.FillCircle(screen, left+dot[0], top+dot[1], iconDot, ink, true)
		}
	case IconNPC:
		// A head and shoulders.
		vector.FillCircle(screen, left+IconSize/Half, top+iconHead+1, iconHead, ink, true)
		shoulders := image.Rect(corner.X+1, corner.Y+IconSize/Half+1, corner.X+IconSize-1, corner.Y+IconSize)
		FillRound(screen, shoulders, IconSize/Half, ink)
	case IconText:
		drawPage(screen, corner, ink)
	case IconFolder:
		drawFolder(screen, corner, ink)
	case IconSound:
		drawSpeaker(screen, left, top, ink)
	case IconProject, IconClock, IconWatch:
		drawWindowIcon(screen, icon, left, top, ink)
	case IconSprite, IconPalette, IconSettings:
		drawArtIcon(screen, icon, left, top, ink)
	case IconNone:
	}
}

// drawPage draws a text document: a page with lines of writing.
func drawPage(screen *ebiten.Image, corner image.Point, ink color.RGBA) {
	left, top := float32(corner.X), float32(corner.Y)
	page := image.Rect(corner.X+iconPageGap, corner.Y, corner.X+IconSize-iconPageGap, corner.Y+IconSize)
	StrokeRound(screen, page, iconPageGap, iconStroke, ink)
	for line := range 3 {
		lineY := top + float32(iconLineGap*(line+1)+1)
		start, end := left+iconPageGap*Half, left+IconSize-iconPageGap*Half
		vector.StrokeLine(screen, start, lineY, end, lineY, Hairline, ink, true)
	}
}

// drawWindowIcon draws the icon of a window that shows no one document.
func drawWindowIcon(screen *ebiten.Image, icon Icon, left, top float32, ink color.RGBA) {
	switch icon {
	case IconProject:
		drawList(screen, left, top, ink)
	case IconClock:
		drawClock(screen, left, top, ink)
	case IconWatch:
		drawEye(screen, left, top, ink)
	case IconNone, IconZone, IconFloor, IconWorkflow, IconNPC, IconText, IconFolder, IconSound, IconSprite,
		IconPalette, IconSettings:
	}
}

// drawArtIcon draws the icon of a sprite, a palette or the settings.
func drawArtIcon(screen *ebiten.Image, icon Icon, left, top float32, ink color.RGBA) {
	switch icon {
	case IconSprite:
		drawHeart(screen, left, top, ink)
	case IconPalette:
		drawRamp(screen, left, top, ink)
	case IconSettings:
		drawSliders(screen, left, top, ink)
	case IconNone, IconZone, IconFloor, IconWorkflow, IconNPC, IconText, IconFolder, IconSound, IconProject,
		IconClock, IconWatch:
	}
}

// heartRows is a sprite's icon, a heart in pixels, one string per row.
func heartRows() []string {
	return []string{".XX.XX.", "XXXXXXX", "XXXXXXX", ".XXXXX.", "..XXX..", "...X..."}
}

// drawHeart draws a sprite: a heart drawn in big pixels.
func drawHeart(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	rows := heartRows()
	top += float32(IconSize-len(rows)*iconPixel) / Half
	for row, pixels := range rows {
		for col := range len(pixels) {
			if pixels[col] == 'X' {
				pixelX, pixelY := left+float32(col*iconPixel), top+float32(row*iconPixel)
				vector.FillRect(screen, pixelX, pixelY, iconPixel, iconPixel, ink, false)
			}
		}
	}
}

// drawRamp draws a palette: a ramp of three swatches, dark to light.
func drawRamp(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	for swatch := range 3 {
		alpha := uint8(opaque - swatch*fadeStep) //nolint:gosec // At most 255 by construction.
		swatchLeft := left + float32(swatch*(iconSwatch+1))
		vector.FillRect(screen, swatchLeft, top+iconPixel+1, iconSwatch, IconSize-iconPixel*Half-Half, fade(ink, alpha),
			false)
	}
}

// fade returns a premultiplied colour with its alpha scaled.
func fade(ink color.RGBA, alpha uint8) color.RGBA {
	scale := func(channel uint8) uint8 {
		return uint8(uint32(channel) * uint32(alpha) / opaque) //nolint:gosec // At most 255 by construction.
	}

	return color.RGBA{R: scale(ink.R), G: scale(ink.G), B: scale(ink.B), A: scale(ink.A)}
}

// drawSliders draws the settings: two sliders, their knobs set apart.
func drawSliders(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	for slider, knob := range []float32{iconKnob, IconSize - iconKnob} {
		lineY := top + float32(iconKnob+slider*(IconSize-iconKnob*Half))
		vector.StrokeLine(screen, left+1, lineY, left+IconSize-1, lineY, iconStroke, ink, true)
		vector.FillCircle(screen, left+knob, lineY, iconDot, ink, true)
	}
}

// drawList draws the project: rows, each a dot and a line.
func drawList(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	for row := range iconRows {
		rowY := top + float32(row*iconRowGap) + iconRowGap/Half
		vector.FillCircle(screen, left+iconDot/Half+1, rowY, iconDot/Half, ink, true)
		vector.StrokeLine(screen, left+iconDot+iconLineGap, rowY, left+IconSize-1, rowY, iconStroke, ink, true)
	}
}

// drawClock draws a clock face with its hands at three o'clock.
func drawClock(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	centreX, centreY := left+IconSize/Half, top+IconSize/Half
	vector.StrokeCircle(screen, centreX, centreY, IconSize/Half-1, iconStroke, ink, true)
	vector.StrokeLine(screen, centreX, centreY, centreX, top+iconHand, iconStroke, ink, true)
	vector.StrokeLine(screen, centreX, centreY, left+IconSize-iconHand, centreY, iconStroke, ink, true)
}

// drawEye draws an eye: two lids around its pupil.
func drawEye(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	middle := top + IconSize/Half
	var lids vector.Path
	lids.MoveTo(left, middle)
	lids.QuadTo(left+IconSize/Half, middle-iconLid, left+IconSize, middle)
	lids.QuadTo(left+IconSize/Half, middle+iconLid, left, middle)
	lids.Close()
	strokePath(screen, &lids, ink)
	vector.FillCircle(screen, left+IconSize/Half, middle, iconDot, ink, true)
}

// drawFolder draws a folder: a tab on the back of its body.
func drawFolder(screen *ebiten.Image, corner image.Point, ink color.RGBA) {
	tab := image.Rect(corner.X, corner.Y+1, corner.X+iconTab, corner.Y+iconTabTop+1)
	FillRound(screen, tab, 1, ink)
	body := image.Rect(corner.X, corner.Y+iconTabTop, corner.X+IconSize, corner.Y+IconSize-1)
	StrokeRound(screen, body, iconPageGap, iconStroke, ink)
}

// drawSpeaker draws a speaker with one wave coming from it.
func drawSpeaker(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	middle := top + IconSize/Half
	var cone vector.Path
	cone.MoveTo(left+1, middle-iconSpeaker/Half)
	cone.LineTo(left+1+iconSpeaker, middle-iconSpeaker/Half)
	cone.LineTo(left+iconSpeaker*Half+1, top+1)
	cone.LineTo(left+iconSpeaker*Half+1, top+IconSize-1)
	cone.LineTo(left+1+iconSpeaker, middle+iconSpeaker/Half)
	cone.LineTo(left+1, middle+iconSpeaker/Half)
	cone.Close()
	fillPath(screen, &cone, ink)
	var wave vector.Path
	centre := left + iconSpeaker*Half + 1
	wave.Arc(centre, middle, iconWave, -quarterTurn, quarterTurn, vector.Clockwise)
	strokePath(screen, &wave, ink)
}

func strokePath(screen *ebiten.Image, path *vector.Path, ink color.Color) {
	options := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	options.ColorScale.ScaleWithColor(ink)
	vector.StrokePath(screen, path, &vector.StrokeOptions{
		Width: iconStroke, LineCap: vector.LineCapRound, LineJoin: vector.LineJoinRound, MiterLimit: 0,
	}, options)
}

func fillPath(screen *ebiten.Image, path *vector.Path, fill color.Color) {
	options := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	options.ColorScale.ScaleWithColor(fill)
	vector.FillPath(screen, path, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, options)
}
