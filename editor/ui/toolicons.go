package ui

import (
	"image"
	"image/color"

	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Geometry of the Map's tool icons (increment 66), in screen pixels inside
// an icon's square.
const (
	toolEdge   = 2
	toolMiddle = IconSize / Half
	toolFar    = IconSize - toolEdge
	toolDot    = 1.5
	toolKnob   = 9
	toolCorner = 4
)

// drawToolIcon draws one of the Map's tool icons, the grip of something
// that is dragged, or a cross.
func drawToolIcon(screen *ebiten.Image, icon Icon, left, top float32, ink color.RGBA) {
	stroke := toolStroke{screen: screen, left: left, top: top, ink: ink}
	switch icon {
	case IconSelect:
		stroke.pointer()
	case IconGround:
		stroke.tile()
	case IconWalls:
		stroke.bricks()
	case IconDoor:
		stroke.door()
	case IconErase:
		stroke.rubber()
	case IconResize:
		stroke.corners()
	case IconGrip:
		stroke.grip()
	case IconCross:
		stroke.line(toolCorner, toolCorner, toolFar-toolEdge, toolFar-toolEdge)
		stroke.line(toolFar-toolEdge, toolCorner, toolCorner, toolFar-toolEdge)
	case IconNone, IconZone, IconFloor, IconWorkflow, IconNPC, IconFolder, IconSound, IconProject, IconClock,
		IconWatch, IconSprite, IconPalette, IconSettings, IconPlay, IconTemplate, IconPlace, IconZoneLoop, IconSignal,
		IconHome, IconConstant:
	}
}

// toolStroke draws a tool icon's lines in its ink, from the icon's corner.
type toolStroke struct {
	screen    *ebiten.Image
	left, top float32
	ink       color.RGBA
}

// line draws a line between two points of the icon's square.
func (s toolStroke) line(fromX, fromY, toX, toY float32) {
	pen.StrokeLine(s.screen, s.left+fromX, s.top+fromY, s.left+toX, s.top+toY, iconStroke, s.ink, true)
}

// box is the icon's square, a little inside its edge.
func (s toolStroke) box() image.Rectangle {
	left, top := int(s.left), int(s.top)

	return image.Rect(left+toolEdge, top+toolEdge, left+toolFar, top+toolFar)
}

// pointer is Select: an arrow from the top left.
func (s toolStroke) pointer() {
	var arrow vector.Path
	arrow.MoveTo(s.left+toolEdge, s.top+1)
	arrow.LineTo(s.left+toolFar+1, s.top+toolMiddle-1)
	arrow.LineTo(s.left+toolMiddle+1, s.top+toolMiddle+1)
	arrow.LineTo(s.left+toolMiddle-1, s.top+toolFar+1)
	arrow.Close()
	fillPath(s.screen, &arrow, s.ink)
}

// tile is Ground: a square, its lower half laid.
func (s toolStroke) tile() {
	box := s.box()
	StrokeRound(s.screen, box, 1, iconStroke, s.ink)
	FillRect(s.screen, image.Rect(box.Min.X, box.Min.Y+box.Dy()/Half, box.Max.X, box.Max.Y), s.ink)
}

// bricks is Walls: two courses of bricks.
func (s toolStroke) bricks() {
	box := s.box()
	StrokeRound(s.screen, image.Rect(box.Min.X-1, box.Min.Y+1, box.Max.X+1, box.Max.Y-1), 1, iconStroke, s.ink)
	s.line(toolEdge-1, toolMiddle, toolFar+1, toolMiddle)
	s.line(toolMiddle-toolEdge, toolEdge+1, toolMiddle-toolEdge, toolMiddle)
	s.line(toolMiddle+toolEdge, toolMiddle, toolMiddle+toolEdge, toolFar-1)
}

// door is doors, stairs and pickups: a door in its frame, with a knob,
// standing on the floor.
func (s toolStroke) door() {
	s.line(toolEdge+1, toolFar+1, toolEdge+1, toolEdge-1)
	s.line(toolEdge+1, toolEdge-1, toolFar-1, toolEdge-1)
	s.line(toolFar-1, toolEdge-1, toolFar-1, toolFar+1)
	s.line(1, toolFar+1, IconSize-1, toolFar+1)
	pen.FillCircle(s.screen, s.left+toolKnob, s.top+toolMiddle+1, 1, s.ink, true)
}

// rubber is Erase: a rubber, tilted, with the band round it.
func (s toolStroke) rubber() {
	var rubber vector.Path
	rubber.MoveTo(s.left+toolEdge, s.top+toolKnob)
	rubber.LineTo(s.left+toolMiddle+1, s.top+toolEdge)
	rubber.LineTo(s.left+toolFar+1, s.top+toolMiddle)
	rubber.LineTo(s.left+toolMiddle+1, s.top+toolFar)
	rubber.LineTo(s.left+toolCorner, s.top+toolFar)
	rubber.Close()
	strokePath(s.screen, &rubber, s.ink)
	s.line(toolCorner+1, toolMiddle-1, toolKnob, toolKnob)
}

// corners is Resize floor: two corners of a floor pulled apart.
func (s toolStroke) corners() {
	s.line(toolEdge, toolMiddle-1, toolEdge, toolEdge)
	s.line(toolEdge, toolEdge, toolMiddle-1, toolEdge)
	s.line(toolFar, toolMiddle+1, toolFar, toolFar)
	s.line(toolFar, toolFar, toolMiddle+1, toolFar)
	s.line(toolCorner, toolCorner, toolFar-toolEdge, toolFar-toolEdge)
}

// grip is six dots in two columns: something to drag.
func (s toolStroke) grip() {
	for _, column := range []float32{toolMiddle - toolEdge, toolMiddle + toolEdge} {
		for _, row := range []float32{toolEdge + 1, toolMiddle, toolFar - 1} {
			pen.FillCircle(s.screen, s.left+column, s.top+row, toolDot, s.ink, true)
		}
	}
}

// DrawJoined draws buttons that stand side by side as one control
// (increment 66): one outline round them all, a hairline between each, and
// the one that is On filled with ink. Quiet buttons make a quiet control: the
// page's tone inside a hairline. The buttons keep their own places and take
// their own presses; only their boxes are drawn as one.
func DrawJoined(screen *ebiten.Image, theme *Theme, buttons []*Button) {
	if len(buttons) == 0 || buttons[0].Rect.Empty() {
		return
	}
	whole := buttons[0].Rect
	for _, button := range buttons[1:] {
		whole = whole.Union(button.Rect)
	}
	ground, edge, width := ColorField(), ColorInk(), float32(Border)
	if buttons[0].Quiet {
		ground, edge, width = ColorPage(), ColorRule(), Hairline
	}
	FillRound(screen, whole, Radius, ground)
	for index, button := range buttons {
		fill, ink := ground, ColorInk()
		switch {
		case button.Disabled:
			ink = ColorSubdued()
		case button.On:
			fill, ink = ColorInk(), ColorReverse()
		case button.hover:
			fill = ColorHover()
		}
		if fill != ground {
			FillRound(screen, button.Rect, Radius, fill)
		}
		if index > 0 && !button.On && !buttons[index-1].On {
			Line(screen, button.Rect.Min, image.Pt(button.Rect.Min.X, button.Rect.Max.Y), Hairline, ColorRule())
		}
		theme.DrawText(screen, button.Label, button.Rect.Min.X+(button.Rect.Dx()-theme.TextWidth(button.Label))/Half,
			button.Rect.Min.Y+(button.Rect.Dy()-TextSize)/Half-1, ink)
	}
	StrokeRound(screen, whole, Radius, width, edge)
}
