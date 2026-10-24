package ui

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Icon is a small drawing of a document type, shown before its name.
type Icon int

// Icons of the document types.
const (
	IconNone Icon = iota
	IconZone
	IconFloor
	IconWorkflow
	IconNPC
	IconText
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
	case IconNone:
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
		// A page with lines of writing.
		page := image.Rect(corner.X+iconPageGap, corner.Y, corner.X+IconSize-iconPageGap, corner.Y+IconSize)
		StrokeRound(screen, page, iconPageGap, iconStroke, ink)
		for line := range 3 {
			lineY := top + float32(iconLineGap*(line+1)+1)
			start, end := left+iconPageGap*Half, left+IconSize-iconPageGap*Half
			vector.StrokeLine(screen, start, lineY, end, lineY, Hairline, ink, true)
		}
	case IconNone:
	}
}

func fillPath(screen *ebiten.Image, path *vector.Path, fill color.Color) {
	options := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	options.ColorScale.ScaleWithColor(fill)
	vector.FillPath(screen, path, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, options)
}
