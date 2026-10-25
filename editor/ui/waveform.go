package ui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Waveform geometry, in screen pixels: how near a press must be to one of
// the region's ends to drag it, the shortest region, in milliseconds, and the
// size of the tab on each end.
const (
	waveReach     = 6
	waveMinRegion = 10
	waveTabWidth  = 8
	waveTabHeight = 10
	peakLevels    = 255
)

// waveDrag is which end of the region is being dragged.
type waveDrag int

const (
	dragNone waveDrag = iota
	dragStart
	dragEnd
)

// Waveform draws a recording's peaks between From and To, in its own
// milliseconds: each peak, 0 to 255, is the loudest moment of its share of
// the width. When HasRegion is set, the part from Start to End is marked and
// its ends can be dragged; while Playing, a line marks Playhead.
type Waveform struct {
	ID        string
	Rect      image.Rectangle
	Peaks     []byte
	From, To  int64
	Start     int64
	End       int64
	HasRegion bool
	Playhead  int64
	Playing   bool
	drag      waveDrag
}

// NewWaveform returns an empty waveform, placed later by setting Rect.
func NewWaveform(id string) *Waveform {
	return &Waveform{
		ID: id, Rect: Zero(), Peaks: nil, From: 0, To: 0, Start: 0, End: 0, HasRegion: false, Playhead: 0,
		Playing: false, drag: dragNone,
	}
}

// xOf is where a moment is across the waveform.
func (w *Waveform) xOf(ms int64) int {
	span := max(w.To-w.From, 1)

	return w.Rect.Min.X + int((ms-w.From)*int64(w.Rect.Dx())/span)
}

// msAt is the moment at a point across the waveform, kept within it.
func (w *Waveform) msAt(x int) int64 {
	width := max(w.Rect.Dx(), 1)
	ms := w.From + int64(x-w.Rect.Min.X)*(w.To-w.From)/int64(width)

	return min(max(ms, w.From), w.To)
}

// Dragging reports whether an end of the region is being dragged.
func (w *Waveform) Dragging() bool { return w.drag != dragNone }

// Update drags the region's ends: a press near one picks it up, and it
// follows the pointer until released. It reports whether the region changed
// and whether a drag ended this frame.
func (w *Waveform) Update(input *Input) (bool, bool) {
	if w.drag != dragNone {
		if !input.Down {
			w.drag = dragNone

			return false, true
		}
		at := w.msAt(input.Cursor.X)
		if w.drag == dragStart {
			w.Start = min(at, w.End-waveMinRegion)
		} else {
			w.End = max(at, w.Start+waveMinRegion)
		}
		input.ClearPointer()

		return true, false
	}
	if !w.HasRegion || !input.Pressed || !input.Cursor.In(w.Rect) {
		return false, false
	}
	startGap, endGap := abs(input.Cursor.X-w.xOf(w.Start)), abs(input.Cursor.X-w.xOf(w.End))
	switch {
	case startGap <= waveReach && startGap <= endGap:
		w.drag = dragStart
	case endGap <= waveReach:
		w.drag = dragEnd
	default:
		return false, false
	}
	input.ClearPointer()

	return false, false
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}

// Locate returns the tab of the region's "start" or "end", for input scripts.
func (w *Waveform) Locate(end string) (image.Point, bool) {
	if !w.HasRegion || w.Rect.Empty() {
		return image.Pt(0, 0), false
	}
	switch end {
	case "start":
		return image.Pt(w.xOf(w.Start), w.Rect.Min.Y+waveTabHeight/Half), true
	case "end":
		return image.Pt(w.xOf(w.End), w.Rect.Min.Y+waveTabHeight/Half), true
	}

	return image.Pt(0, 0), false
}

// Draw renders the waveform: the region washed in blue, the peaks mirrored
// about the middle, darker inside the region, its ends as blue lines with
// tabs, and the playhead in orange.
func (w *Waveform) Draw(screen *ebiten.Image) {
	if w.Rect.Empty() {
		return
	}
	FillRound(screen, w.Rect, Radius, ColorCanvas())
	left, right := w.xOf(w.Start), w.xOf(w.End)
	if w.HasRegion {
		FillRect(screen, image.Rect(max(left, w.Rect.Min.X), w.Rect.Min.Y+1, min(right, w.Rect.Max.X), w.Rect.Max.Y-1),
			ColorSelectWash())
	}
	middle, reach := w.Rect.Min.Y+w.Rect.Dy()/Half, w.Rect.Dy()/Half-Gap
	if len(w.Peaks) > 0 {
		for column := w.Rect.Min.X + 1; column < w.Rect.Max.X-1; column += Half {
			peak := int(w.Peaks[(column-w.Rect.Min.X)*len(w.Peaks)/w.Rect.Dx()])
			height := max(peak*reach/peakLevels, 1)
			ink := ColorSubdued()
			if !w.HasRegion || column >= left && column <= right {
				ink = ColorInk()
			}
			FillRect(screen, image.Rect(column, middle-height, column+1, middle+height), ink)
		}
	}
	if w.HasRegion {
		for _, x := range []int{left, right} {
			if x >= w.Rect.Min.X && x <= w.Rect.Max.X {
				Line(screen, image.Pt(x, w.Rect.Min.Y), image.Pt(x, w.Rect.Max.Y), Outline, ColorSelect())
				tab := image.Rect(x-waveTabWidth/Half, w.Rect.Min.Y, x+waveTabWidth/Half, w.Rect.Min.Y+waveTabHeight)
				FillRound(screen, tab, Half, ColorSelect())
			}
		}
	}
	if w.Playing && w.Playhead >= w.From && w.Playhead <= w.To {
		x := w.xOf(w.Playhead)
		Line(screen, image.Pt(x, w.Rect.Min.Y), image.Pt(x, w.Rect.Max.Y), Outline, ColorLive())
	}
	StrokeRound(screen, w.Rect, Radius, Border, ColorRule())
}
