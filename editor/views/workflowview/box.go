package workflowview

import (
	"image"
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// A box's wash is the selection's blue at boxWash of opaque.
const (
	boxWash = 24
	opaque  = 255
)

// selectBox is a box being dragged on empty canvas, in canvas pixels: where
// it started and where the pointer is, and what was selected before it,
// which a box dragged with Cmd adds to.
type selectBox struct {
	from, to image.Point
	before   []ids.ULID
}

// rect is the box's area.
func (b *selectBox) rect() image.Rectangle { return image.Rectangle{Min: b.from, Max: b.to}.Canon() }

// startBox starts a box on empty canvas, adding to what is selected with
// Cmd held and starting afresh without.
func (s *State) startBox(point image.Point, adding bool) {
	var before []ids.ULID
	if adding {
		before = slices.Clone(s.selection())
	}
	s.box = &selectBox{from: point, to: point, before: before}
	s.selectNodes(before)
}

// updateBox selects what the box holds as it is dragged, and ends it when
// the button is let go.
func (s *State) updateBox(input *ui.Input, point image.Point) {
	if !input.Down {
		s.box = nil

		return
	}
	s.box.to = point
	s.selectNodes(append(slices.Clone(s.box.before), s.boxed(s.box.rect())...))
}

// boxed lists the nodes a box selects: those it touches, and a container
// only when it holds the container whole.
func (s *State) boxed(area image.Rectangle) []ids.ULID {
	var chosen []ids.ULID
	for _, id := range s.drawOrder() {
		drawn := s.shapes[id]
		if flow.IsContainer(drawn.node.Kind) && drawn.rect.In(area) ||
			!flow.IsContainer(drawn.node.Kind) && drawn.rect.Overlaps(area) {
			chosen = append(chosen, id)
		}
	}

	return chosen
}

// drawBox draws the box being dragged: a light wash of the selection's
// blue, edged in it.
func (s *State) drawBox(area *ebiten.Image) {
	if s.box == nil {
		return
	}
	rect := s.screenRect(s.box.rect())
	blue := ui.ColorSelect()
	vector.FillRect(area, float32(rect.Min.X), float32(rect.Min.Y), float32(rect.Dx()), float32(rect.Dy()),
		washOf(blue), false)
	vector.StrokeRect(area, float32(rect.Min.X), float32(rect.Min.Y), float32(rect.Dx()), float32(rect.Dy()), 1, blue,
		false)
}

// washOf is a colour at boxWash of opaque, premultiplied as ebiten draws.
func washOf(full color.RGBA) color.RGBA {
	scale := func(channel uint8) uint8 {
		return uint8(int(channel) * boxWash / opaque) //nolint:gosec // At most boxWash, under 256.
	}

	return color.RGBA{R: scale(full.R), G: scale(full.G), B: scale(full.B), A: boxWash}
}
