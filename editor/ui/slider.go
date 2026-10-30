package ui

import (
	"image"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Slider geometry, in screen pixels: the track's thickness and the handle's
// radius.
const (
	sliderTrack  = 4
	sliderHandle = 7
)

// Slider sets a whole number from Min to Max by pressing or dragging along a
// track. The track is inset by the handle's radius at each end, so the
// handle is whole at Min and at Max.
type Slider struct {
	ID       string
	Rect     image.Rectangle
	Min, Max int
	Value    int
	dragging bool
}

// NewSlider returns a slider from low to high, placed later by setting Rect.
func NewSlider(id string, low, high int) *Slider {
	return &Slider{ID: id, Rect: Zero(), Min: low, Max: high, Value: low, dragging: false}
}

// Dragging reports whether the handle is being dragged.
func (s *Slider) Dragging() bool { return s.dragging }

// track is where the handle's centre can be, left to right.
func (s *Slider) track() (int, int) {
	return s.Rect.Min.X + sliderHandle, s.Rect.Max.X - sliderHandle
}

// Update handles one frame of input and reports whether the value changed.
func (s *Slider) Update(input *Input) bool {
	if s.Rect.Empty() {
		s.dragging = false

		return false
	}
	if input.Pressed && input.Cursor.In(s.Rect) {
		s.dragging = true
	}
	changed := false
	if s.dragging && (input.Down || input.Pressed) {
		low, high := s.track()
		share := fraction(input.Cursor.X, low, high-low+1)
		value := s.Min + int(math.Round(share*float64(s.Max-s.Min)))
		changed = value != s.Value
		s.Value = value
	}
	if !input.Down {
		s.dragging = false
	}

	return changed
}

// Locate finds the point on the track for a value, for input scripts.
func (s *Slider) Locate(value string) (image.Point, bool) {
	number, err := strconv.Atoi(value)
	if err != nil || s.Rect.Empty() || number < s.Min || number > s.Max {
		return image.Pt(0, 0), false
	}

	return image.Pt(s.xOf(number), s.Rect.Min.Y+s.Rect.Dy()/Half), true
}

// xOf is where a value's handle is.
func (s *Slider) xOf(value int) int {
	low, high := s.track()

	return atFraction(float64(value-s.Min)/float64(max(s.Max-s.Min, 1)), low, high-low+1)
}

// Draw draws the track, filled up to the value, and the handle.
func (s *Slider) Draw(screen *ebiten.Image) {
	if s.Rect.Empty() {
		return
	}
	low, high := s.track()
	middle := s.Rect.Min.Y + s.Rect.Dy()/Half
	handle := s.xOf(min(max(s.Value, s.Min), s.Max))
	FillRound(screen, image.Rect(low, middle-sliderTrack/Half, high, middle+sliderTrack/Half), sliderTrack/Half,
		ColorRule())
	FillRound(screen, image.Rect(low, middle-sliderTrack/Half, handle, middle+sliderTrack/Half), sliderTrack/Half,
		ColorSelect())
	vector.FillCircle(screen, float32(handle), float32(middle), sliderHandle, ColorBackground(), true)
	vector.StrokeCircle(screen, float32(handle), float32(middle), sliderHandle, Hairline, ColorInk(), true)
}
