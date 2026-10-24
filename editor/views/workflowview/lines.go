package workflowview

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Line drawing, in screen pixels.
const (
	selectedLine = 3
	// chevronRun is the shortest stretch of line that gets a chevron in its
	// middle, showing which way the line runs; chevronSize is its size.
	chevronRun  = 96
	chevronSize = 4
	// movingAlpha is how solid a line being moved is drawn where it was.
	movingAlpha = 0x66
	// A grip is gripLength by gripWidth either side of a line's middle.
	gripLength = 7
	gripWidth  = 3
)

// drawLines draws every line: the others first, then the lit ones over them.
func (s *State) drawLines(area *ebiten.Image) {
	links := lines(s.workflow, s.shapes)
	for _, lit := range []bool{false, true} {
		for _, link := range links {
			if s.lineLit(link.key) == lit {
				s.drawLine(area, link, lit)
			}
		}
	}
}

// lineLit reports whether a line is drawn blue: a line of the selected node,
// the line under the pointer, or the selected line.
func (s *State) lineLit(key routeKey) bool {
	to, linked := s.targetOf(key)
	if !linked {
		return false
	}

	return key.from == s.selected || to == s.selected || key == s.hoverLink || key == s.selectedLink
}

// drawLine draws one line along its route, with an arrowhead into its entry
// dot and a chevron along each long stretch. A loop, a line back to a node
// already on the way, is dashed; a time switch's "otherwise" line is grey;
// a line being moved is faint where it was.
func (s *State) drawLine(area *ebiten.Image, link line, lit bool) {
	corners := s.routes[link.key]
	if len(corners) < ui.Half {
		return
	}
	ink := s.lineInk(link, lit)
	width := float32(lineWidth)
	if link.key == s.selectedLink {
		width = selectedLine
	}
	for index := 1; index < len(corners); index++ {
		start, end := s.toScreen(corners[index-1]), s.toScreen(corners[index])
		if s.loops[link.key] {
			dashedLine(area, start, end, width, ink)
		} else {
			ui.Line(area, start, end, width, ink)
		}
		if index < len(corners)-1 && max(abs(end.X-start.X), abs(end.Y-start.Y)) >= chevronRun {
			drawChevron(area, start, end, ink)
		}
	}
	drawArrowhead(area, s.toScreen(corners[len(corners)-1]), ink)
	if link.key == s.hoverLink || link.key == s.selectedLink {
		s.drawGrip(area, corners)
	}
}

// drawGrip draws a small handle across the middle of a line's draggable
// part, showing it can be moved.
func (s *State) drawGrip(area *ebiten.Image, corners []image.Point) {
	start, end, ok := middleOf(corners)
	if !ok {
		return
	}
	centre := s.toScreen(image.Pt((start.X+end.X)/ui.Half, (start.Y+end.Y)/ui.Half))
	half := image.Pt(gripLength, gripWidth)
	if start.Y == end.Y {
		half = image.Pt(gripWidth, gripLength)
	}
	grip := image.Rectangle{Min: centre.Sub(half), Max: centre.Add(half)}
	ui.FillRound(area, grip, gripWidth, ui.ColorBackground())
	ui.StrokeRound(area, grip, gripWidth, ui.Border, ui.ColorSelect())
}

// lineInk is a line's colour: blue when lit, grey for a time switch's
// "otherwise", ink otherwise; faint where it was while it is being moved.
func (s *State) lineInk(link line, lit bool) color.Color {
	base := ui.ColorInk()
	if from, ok := s.workflow.Node(link.key.from); ok && from.Kind == flow.KindTimeSwitch &&
		link.key.output == flow.OutputOtherwise {
		base = ui.ColorMuted()
	}
	if lit {
		base = ui.ColorSelect()
	}
	if s.link != nil && s.link.from == link.key.from && s.link.output == link.key.output {
		return color.NRGBA{R: base.R, G: base.G, B: base.B, A: movingAlpha}
	}

	return base
}

// dashedLine draws a straight line in dashes.
func dashedLine(area *ebiten.Image, from, to image.Point, width float32, ink color.Color) {
	length := max(abs(to.X-from.X), abs(to.Y-from.Y))
	for start := 0; start < length; start += dashLength + dashGap {
		end := min(start+dashLength, length)
		ui.Line(area, lerp(from, to, start, length), lerp(from, to, end, length), width, ink)
	}
}

// drawChevron draws a small chevron in the middle of a level or upright
// stretch, pointing the way it runs.
func drawChevron(area *ebiten.Image, from, to image.Point, ink color.Color) {
	middle := image.Pt((from.X+to.X)/ui.Half, (from.Y+to.Y)/ui.Half)
	along := image.Pt(sign(to.X-from.X), sign(to.Y-from.Y))
	across := image.Pt(along.Y, along.X)
	back := middle.Sub(along.Mul(chevronSize))
	ui.Line(area, back.Add(across.Mul(chevronSize)), middle.Add(along.Mul(chevronSize/ui.Half)), lineWidth, ink)
	ui.Line(area, back.Sub(across.Mul(chevronSize)), middle.Add(along.Mul(chevronSize/ui.Half)), lineWidth, ink)
}

// drawArrowhead draws a line's arrowhead, its tip at the entry dot.
func drawArrowhead(area *ebiten.Image, tip image.Point, ink color.Color) {
	var arrow vector.Path
	arrow.MoveTo(float32(tip.X), float32(tip.Y))
	arrow.LineTo(float32(tip.X-arrowSize*ui.Half), float32(tip.Y-arrowSize))
	arrow.LineTo(float32(tip.X-arrowSize*ui.Half), float32(tip.Y+arrowSize))
	arrow.Close()
	fill := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	fill.ColorScale.ScaleWithColor(ink)
	vector.FillPath(area, &arrow, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, fill)
}

// drawMark draws a node's problem mark: a small warning circle with "!".
func (s *State) drawMark(area *ebiten.Image, drawn shape) {
	if _, problem := s.problems[drawn.node.ID]; !problem {
		return
	}
	centre := s.toScreen(markCentre(drawn))
	vector.FillCircle(area, float32(centre.X), float32(centre.Y), markRadius, ui.ColorWarn(), true)
	width := s.theme.SmallWidth("!")
	s.theme.DrawSmall(area, "!", centre.X-width/ui.Half, centre.Y-ui.LabelSize/ui.Half-ui.Half, ui.ColorBackground())
}

func sign(value int) int {
	switch {
	case value > 0:
		return 1
	case value < 0:
		return -1
	default:
		return 0
	}
}
