package workflowview

import (
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// crossingAlpha is how strongly a line is drawn over a node it passes:
// faint enough to read the node through it, strong enough to show the
// line carries on past, joined to nothing there.
const crossingAlpha = 0x66

// The tag under a node nothing links to.
const (
	looseTag    = "Not linked: never runs"
	looseTagPad = 6
	looseTagGap = 4
)

// drawCrossings draws again, faintly over the nodes, each stretch of line
// that passes under a node it neither leaves nor enters, dots included, so
// a node placed over a line does not look joined to it.
func (s *State) drawCrossings(area *ebiten.Image) {
	for _, link := range lines(s.workflow, s.shapes) {
		corners := s.routes[link.key]
		ink := faint(s.lineInk(link, s.lineLit(link.key)))
		for _, drawn := range s.shapes {
			if !runsBehindLines(drawn.node.Kind) || drawn.node.ID == link.key.from || drawn.node.ID == link.to {
				continue
			}
			for _, stretch := range crossing(corners, drawn.rect.Inset(-portRadius)) {
				start, end := s.toScreen(stretch[0]), s.toScreen(stretch[1])
				if s.loops[link.key] {
					dashedLine(area, start, end, lineWidth, ink)
				} else {
					ui.Line(area, start, end, lineWidth, ink)
				}
			}
		}
	}
}

// faint returns a colour no stronger than crossingAlpha.
func faint(ink color.Color) color.Color {
	plain, ok := color.NRGBAModel.Convert(ink).(color.NRGBA)
	if !ok {
		return ink
	}
	plain.A = min(plain.A, crossingAlpha)

	return plain
}

// crossing returns the stretches of a route, in canvas pixels, that run
// inside a rectangle. A route runs across and down, never on a slant.
func crossing(corners []image.Point, rect image.Rectangle) [][2]image.Point {
	var stretches [][2]image.Point
	for index := 1; index < len(corners); index++ {
		start, end := corners[index-1], corners[index]
		switch {
		case start.Y == end.Y && start.Y >= rect.Min.Y && start.Y < rect.Max.Y:
			low, high := max(min(start.X, end.X), rect.Min.X), min(max(start.X, end.X), rect.Max.X)
			if low < high {
				stretches = append(stretches, [2]image.Point{image.Pt(low, start.Y), image.Pt(high, start.Y)})
			}
		case start.X == end.X && start.X >= rect.Min.X && start.X < rect.Max.X:
			low, high := max(min(start.Y, end.Y), rect.Min.Y), min(max(start.Y, end.Y), rect.Max.Y)
			if low < high {
				stretches = append(stretches, [2]image.Point{image.Pt(start.X, low), image.Pt(start.X, high)})
			}
		}
	}

	return stretches
}

// loose reports whether nothing links into a node, so it never runs.
func (s *State) loose(drawn shape) bool {
	return s.problems[drawn.node.ID] == problemUnreached
}

// drawLooseTag says under a node nothing links to that it never runs.
func (s *State) drawLooseTag(area *ebiten.Image, drawn shape) {
	if !s.loose(drawn) {
		return
	}
	rect := s.screenRect(drawn.rect)
	width := s.theme.SmallWidth(looseTag) + ui.Half*looseTagPad
	top := rect.Max.Y + looseTagGap
	tag := image.Rect(rect.Min.X, top, rect.Min.X+width, top+ui.LabelSize+looseTagPad)
	ui.FillRound(area, tag, float32(tag.Dy())/ui.Half, ui.ColorField())
	ui.StrokeRound(area, tag, float32(tag.Dy())/ui.Half, 1, ui.ColorWarn())
	s.theme.DrawSmall(area, looseTag, tag.Min.X+looseTagPad, tag.Min.Y+looseTagPad/ui.Half-1, ui.ColorWarn())
}

// hasCrossingState answers "loose:<kind>:<n>", true when nothing links into
// the node, and "crossed:<kind>:<n>", true when a line it neither leaves
// nor enters runs under it.
func (s *State) hasCrossingState(state string) (bool, bool) {
	if which, asked := strings.CutPrefix(state, "loose:"); asked {
		drawn, ok := s.nthNode(which)

		return ok && s.loose(drawn), true
	}
	which, asked := strings.CutPrefix(state, "crossed:")
	if !asked {
		return false, false
	}
	drawn, ok := s.nthNode(which)
	if !ok || s.workflow == nil {
		return false, true
	}
	for _, link := range lines(s.workflow, s.shapes) {
		if link.key.from != drawn.node.ID && link.to != drawn.node.ID &&
			len(crossing(s.routes[link.key], drawn.rect.Inset(-portRadius))) > 0 {
			return true, true
		}
	}

	return false, true
}
