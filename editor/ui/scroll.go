package ui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui/pen"
)

// Scroll is how far a panel's body is scrolled: the part between its
// heading and its foot, where an inspector's fields are, when they need more
// height than the body has. What scrolls moves up under the heading, and
// what went past the foot comes into view; the heading and the foot, such as
// a save bar and Delete, stay where they are (increment 61).
//
// A view lays the body out from its top less Offset, gives Fit where it
// ended, draws it onto Clip and lets only presses inside Body reach it.
type Scroll struct {
	// Body is where what scrolls shows, as last laid out.
	Body image.Rectangle
	// Offset is how far it is scrolled, and Most the furthest it goes, as
	// last laid out.
	Offset, Most int
	// of is what the body shows: another thing starts at the top.
	of string
}

// NewScroll returns a body scrolled to its top.
func NewScroll() Scroll { return Scroll{Body: Zero(), Offset: 0, Most: 0, of: ""} }

// For starts at the top when the body shows something else than it did.
func (s *Scroll) For(shown string) {
	if s.of != shown {
		s.Offset, s.Most, s.of = 0, 0, shown
	}
}

// Fit takes where what scrolls ends, as laid out at Offset, and keeps Offset
// within it. It reports whether Offset changed, so the body is laid out
// again.
func (s *Scroll) Fit(bottom int) bool {
	s.Most = max(bottom+s.Offset-s.Body.Max.Y, 0)
	if s.Offset > s.Most {
		s.Offset = s.Most

		return true
	}

	return false
}

// Wheel scrolls with the wheel over a panel, taking the wheel from the
// input. It reports whether Offset changed.
func (s *Scroll) Wheel(input *Input, over image.Rectangle) bool {
	if s.Most == 0 || input.WheelY == 0 || !input.Cursor.In(over) {
		return false
	}
	before := s.Offset
	s.Offset = min(max(s.Offset-int(input.WheelY*RowHeight), 0), s.Most)
	input.WheelY = 0

	return s.Offset != before
}

// Hidden reports whether a point in the body's column lies above or below
// the body, where what scrolled away is: it must not take a press.
func (s *Scroll) Hidden(point image.Point) bool {
	return point.X >= s.Body.Min.X && point.X < s.Body.Max.X && !point.In(s.Body)
}

// Reveal scrolls a point of the body's column that is hidden into the
// body's middle, as an author scrolls to a field, and reports whether it
// did, so the body is laid out again and the point found anew.
func (s *Scroll) Reveal(point image.Point) bool {
	if s.Most == 0 || !s.Hidden(point) {
		return false
	}
	before := s.Offset
	s.Offset = min(max(s.Offset+point.Y-(s.Body.Min.Y+s.Body.Max.Y)/Half, 0), s.Most)

	return s.Offset != before
}

// RevealField scrolls a field of the body's column that is not wholly
// inside the body into its middle, as Tab moving to it does (increments 59
// and 61); the next frame lays it out there. A field elsewhere in the
// window is left alone. It suits Focus.SetReveal.
func (s *Scroll) RevealField(field *TextField) {
	rect := field.Rect
	if s.Most == 0 || rect.Empty() || rect.In(s.Body) || rect.Min.X < s.Body.Min.X || rect.Max.X > s.Body.Max.X {
		return
	}
	middle := (rect.Min.Y + rect.Max.Y) / Half
	s.Offset = min(max(s.Offset+middle-(s.Body.Min.Y+s.Body.Max.Y)/Half, 0), s.Most)
}

// Clip is the screen clipped to the body, for drawing what scrolls, or
// false when the body has no room to show anything.
func (s *Scroll) Clip(screen *ebiten.Image) (*ebiten.Image, bool) { return pen.Clip(screen, s.Body) }

// DrawThumb draws, while the body scrolls, a thumb at its right edge.
func (s *Scroll) DrawThumb(screen *ebiten.Image) { DrawScrollThumb(screen, s.Body, s.Offset, s.Most) }
