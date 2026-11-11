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
	IconFolder
	IconSound
	IconProject
	IconClock
	IconWatch
	IconSprite
	IconPalette
	IconSettings
	// IconPlay marks the Play tab and Launch.
	IconPlay
	// IconTemplate is a template: a frame round a clear middle.
	IconTemplate
	// IconPlace is a place: the ring a map draws for an anchor.
	IconPlace
	// IconZoneLoop is a zone loop, the kind of workflow a zone runs: an
	// arrow going round.
	IconZoneLoop
	// IconSignal is the signals workflows send and answer: a dot with
	// waves going out from it.
	IconSignal
	// IconHome is the Project window's Home: a house.
	IconHome
	// IconConstant is a constant: a diamond.
	IconConstant
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
	// A template's frame has a fainter frame this far inside it.
	iconFrame = 4
	frameFade = 0x99
)

// IconColor returns the muted colour of an icon's type.
func IconColor(icon Icon) color.RGBA {
	switch icon {
	case IconZone, IconFloor:
		return ColorZone()
	case IconWorkflow, IconZoneLoop, IconSignal:
		return ColorWorkflow()
	case IconNPC:
		return ColorNPC()
	case IconSound:
		return ColorSound()
	case IconFolder:
		return ColorMuted()
	case IconWatch, IconPlay:
		return ColorLive()
	case IconSprite, IconPalette, IconTemplate:
		return ColorArt()
	case IconPlace, IconHome:
		return ColorSelect()
	case IconConstant:
		return ColorConstant()
	case IconNone, IconProject, IconClock, IconSettings:
	}

	return ColorInk()
}

// ChipPad is how far a document's chip reaches beyond its icon on each
// side.
const ChipPad = 2

// chipRadius rounds a chip's corners.
const chipRadius = 4

// Chipped reports whether an icon is a document's, drawn white on a filled
// chip of its type's muted colour, so its type is found at a glance: zones
// and floors, workflows and zone loops, NPCs, sounds, sprites, palettes and
// templates. Folders, places, Home, the clock, Watch, Play, settings and the
// project keep a plain mark.
func Chipped(icon Icon) bool {
	switch icon {
	case IconZone, IconFloor, IconWorkflow, IconZoneLoop, IconNPC, IconSound, IconSprite, IconPalette, IconTemplate:
		return true
	case IconNone, IconFolder, IconProject, IconClock, IconWatch, IconSettings, IconPlay, IconPlace, IconSignal,
		IconHome, IconConstant:
	}

	return false
}

// DrawIcon draws an icon with its top-left corner at a point: a document's
// white on a chip of its type's colour, any other in its own colour.
func DrawIcon(screen *ebiten.Image, icon Icon, corner image.Point) {
	if !Chipped(icon) {
		DrawIconInk(screen, icon, corner, IconColor(icon))

		return
	}
	chip := image.Rect(corner.X-ChipPad, corner.Y-ChipPad, corner.X+IconSize+ChipPad, corner.Y+IconSize+ChipPad)
	FillRound(screen, chip, chipRadius, IconColor(icon))
	DrawIconInk(screen, icon, corner, ColorReverse())
}

// DrawIconInk draws an icon in an ink of its own, as on a filled button.
func DrawIconInk(screen *ebiten.Image, icon Icon, corner image.Point, ink color.RGBA) {
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
	case IconFolder:
		drawFolder(screen, corner, ink)
	case IconSound:
		drawSpeaker(screen, left, top, ink)
	case IconProject, IconClock, IconWatch, IconHome:
		drawWindowIcon(screen, icon, left, top, ink)
	case IconSprite, IconPalette, IconSettings, IconTemplate:
		drawArtIcon(screen, icon, left, top, ink)
	case IconPlay, IconPlace, IconZoneLoop, IconSignal:
		drawMark(screen, icon, corner, ink)
	case IconConstant:
		drawDiamond(screen, left, top, ink)
	case IconNone:
	}
}

// drawDiamond draws a constant's mark: a square on its corner, filled.
func drawDiamond(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	var diamond vector.Path
	diamond.MoveTo(left+IconSize/Half, top+1)
	diamond.LineTo(left+IconSize-1, top+IconSize/Half)
	diamond.LineTo(left+IconSize/Half, top+IconSize-1)
	diamond.LineTo(left+1, top+IconSize/Half)
	diamond.Close()
	fillPath(screen, &diamond, ink)
}

// drawMark draws Play's triangle, a place's ring, a zone loop's arrow or
// a signal's waves.
func drawMark(screen *ebiten.Image, icon Icon, corner image.Point, ink color.RGBA) {
	if icon == IconSignal {
		drawWaves(screen, float32(corner.X), float32(corner.Y), ink)

		return
	}
	if icon == IconPlace {
		DrawRing(screen, image.Pt(corner.X+IconSize/Half, corner.Y+IconSize/Half), ink, false)

		return
	}
	if icon == IconZoneLoop {
		drawLoop(screen, float32(corner.X), float32(corner.Y), ink)

		return
	}
	drawPlay(screen, float32(corner.X), float32(corner.Y), ink)
}

// drawLoop draws a zone loop: three quarters of a circle, with an arrowhead
// at its end.
func drawLoop(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	centreX, centreY := left+IconSize/Half, top+IconSize/Half
	radius := float32(IconSize)/Half - iconStroke
	var loop vector.Path
	loop.Arc(centreX, centreY, radius, -quarterTurn*Half, math.Pi, vector.Clockwise)
	strokePath(screen, &loop, ink)
	tipX, tipY := centreX-radius, centreY
	var head vector.Path
	head.MoveTo(tipX-iconHead+1, tipY-iconHead+1)
	head.LineTo(tipX, tipY+1)
	head.LineTo(tipX+iconHead-1, tipY-iconHead+1)
	strokePath(screen, &head, ink)
}

// drawWaves draws a signal: a dot at the left with two waves going out
// from it to the right.
func drawWaves(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	centreX, centreY := left+iconDot, top+IconSize/Half
	vector.FillCircle(screen, centreX, centreY, iconDot, ink, true)
	for _, radius := range []float32{IconSize/Half - 1, IconSize/Half + iconDot} {
		var wave vector.Path
		wave.Arc(centreX, centreY, radius, -quarterTurn, quarterTurn, vector.Clockwise)
		strokePath(screen, &wave, ink)
	}
}

// drawPlay draws Play: a triangle pointing right.
func drawPlay(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	var triangle vector.Path
	triangle.MoveTo(left+iconPageGap, top+1)
	triangle.LineTo(left+IconSize-1, top+IconSize/Half)
	triangle.LineTo(left+iconPageGap, top+IconSize-1)
	triangle.Close()
	fillPath(screen, &triangle, ink)
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
	case IconHome:
		drawHouse(screen, left, top, ink)
	case IconNone, IconZone, IconFloor, IconWorkflow, IconNPC, IconFolder, IconSound, IconSprite,
		IconPalette, IconSettings, IconPlay, IconTemplate, IconPlace, IconZoneLoop, IconSignal, IconConstant:
	}
}

// drawHouse draws a house: a roof over a body.
func drawHouse(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	middle, eaves := left+IconSize/Half, top+IconSize/Half
	vector.StrokeLine(screen, left+1, eaves, middle, top+1, iconStroke, ink, true)
	vector.StrokeLine(screen, middle, top+1, left+IconSize-1, eaves, iconStroke, ink, true)
	vector.StrokeRect(screen, left+iconDot+1, eaves-1, IconSize-Half*(iconDot+1), IconSize/Half, iconStroke, ink, true)
}

// drawArtIcon draws the icon of a sprite, a palette, the settings or a
// template.
func drawArtIcon(screen *ebiten.Image, icon Icon, left, top float32, ink color.RGBA) {
	switch icon {
	case IconSprite:
		drawHeart(screen, left, top, ink)
	case IconPalette:
		drawRamp(screen, left, top, ink)
	case IconSettings:
		drawSliders(screen, left, top, ink)
	case IconTemplate:
		drawFrame(screen, left, top, ink)
	case IconNone, IconZone, IconFloor, IconWorkflow, IconNPC, IconFolder, IconSound, IconProject,
		IconClock, IconWatch, IconPlay, IconPlace, IconZoneLoop, IconSignal, IconHome, IconConstant:
	}
}

// ringRadius is a place's ring, drawn as the map draws an anchor's.
const (
	ringRadius = 4.5
	ringStroke = 2
)

// DrawRing draws a place's ring round a centre, filled when it is the place
// chosen now.
func DrawRing(screen *ebiten.Image, centre image.Point, ink color.RGBA, filled bool) {
	x, y := float32(centre.X), float32(centre.Y)
	if filled {
		vector.FillCircle(screen, x, y, ringRadius, ink, true)
	}
	vector.StrokeCircle(screen, x, y, ringRadius, ringStroke, ink, true)
}

// drawFrame draws a template: a square frame round a clear middle, with a
// fainter frame inside it.
func drawFrame(screen *ebiten.Image, left, top float32, ink color.RGBA) {
	vector.StrokeRect(screen, left+1, top+1, IconSize-Half, IconSize-Half, iconStroke, ink, false)
	inner := float32(iconFrame)
	vector.StrokeRect(screen, left+inner, top+inner, IconSize-Half*inner, IconSize-Half*inner, 1, fade(ink, frameFade),
		false)
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
