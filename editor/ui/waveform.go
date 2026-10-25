package ui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Waveform geometry, in screen pixels: how near a press must be to one of
// the region's ends to drag it, which is also the width of the flag on each
// end, the flag's height, and the shortest region, in milliseconds.
const (
	waveReach      = 10
	waveFlagHeight = 18
	waveGrip       = 6
	waveMinRegion  = 10
	peakLevels     = 255
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
	// hover is the end the pointer is over, drawn heavier. A dragged end
	// moves as far as the pointer has from pressX, where it was at origin.
	hover  waveDrag
	pressX int
	origin int64
}

// NewWaveform returns an empty waveform, placed later by setting Rect.
func NewWaveform(id string) *Waveform {
	return &Waveform{
		ID: id, Rect: Zero(), Peaks: nil, From: 0, To: 0, Start: 0, End: 0, HasRegion: false, Playhead: 0,
		Playing: false, drag: dragNone, hover: dragNone, pressX: 0, origin: 0,
	}
}

// xOf is where a moment is across the waveform.
func (w *Waveform) xOf(ms int64) int {
	span := max(w.To-w.From, 1)

	return w.Rect.Min.X + int((ms-w.From)*int64(w.Rect.Dx())/span)
}

// dragged is where the dragged end has moved to, kept within what is shown.
func (w *Waveform) dragged(x int) int64 {
	width := max(w.Rect.Dx(), 1)
	ms := w.origin + int64(x-w.pressX)*(w.To-w.From)/int64(width)

	return min(max(ms, w.From), w.To)
}

// endX is where an end of the region is drawn and picked up: kept just
// inside the waveform's border, so an end at either edge can be seen.
func (w *Waveform) endX(ms int64) int {
	return min(max(w.xOf(ms), w.Rect.Min.X+Outline), w.Rect.Max.X-Outline)
}

// shown reports whether a moment is within the part the waveform shows.
func (w *Waveform) shown(ms int64) bool { return ms >= w.From && ms <= w.To }

// endAt returns the end of the region a press at a point picks up: the
// nearer of those within reach across, anywhere from the waveform's top to
// its bottom; for ends drawn in the same place, the one on the side pressed.
func (w *Waveform) endAt(point image.Point) waveDrag {
	if !w.HasRegion || point.Y < w.Rect.Min.Y || point.Y >= w.Rect.Max.Y {
		return dragNone
	}
	startX, endX := w.endX(w.Start), w.endX(w.End)
	startGap, endGap := abs(point.X-startX), abs(point.X-endX)
	startNear, endNear := w.shown(w.Start) && startGap <= waveReach, w.shown(w.End) && endGap <= waveReach
	switch {
	case startNear && (!endNear || startGap < endGap || startGap == endGap && point.X < startX):
		return dragStart
	case endNear:
		return dragEnd
	}

	return dragNone
}

// Dragging reports whether an end of the region is being dragged.
func (w *Waveform) Dragging() bool { return w.drag != dragNone }

// Update drags the region's ends: a press near one picks it up, and it
// moves as the pointer does until released. It reports whether the region changed
// and whether a drag ended this frame.
func (w *Waveform) Update(input *Input) (bool, bool) {
	if w.drag != dragNone {
		if !input.Down {
			w.drag = dragNone

			return false, true
		}
		start, end := w.Start, w.End
		if w.drag == dragStart {
			w.Start = min(w.dragged(input.Cursor.X), w.End-waveMinRegion)
		} else {
			w.End = max(w.dragged(input.Cursor.X), w.Start+waveMinRegion)
		}
		input.ClearPointer()

		return w.Start != start || w.End != end, false
	}
	w.hover = w.endAt(input.Cursor)
	if w.hover == dragNone || !input.Pressed {
		return false, false
	}
	w.drag, w.pressX, w.origin = w.hover, input.Cursor.X, w.Start
	if w.drag == dragEnd {
		w.origin = w.End
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

// Locate returns the flag of the region's "start" or "end", for input
// scripts.
func (w *Waveform) Locate(end string) (image.Point, bool) {
	if !w.HasRegion || w.Rect.Empty() {
		return image.Pt(0, 0), false
	}
	switch end {
	case "start":
		return image.Pt(w.endX(w.Start), w.Rect.Min.Y+waveFlagHeight/Half), true
	case "end":
		return image.Pt(w.endX(w.End), w.Rect.Min.Y+waveFlagHeight/Half), true
	}

	return image.Pt(0, 0), false
}

// Draw renders the waveform: the region washed in blue, the peaks mirrored
// about the middle, darker inside the region, its ends as blue lines with
// flags pointing into it, heavier under the pointer, and the playhead in
// orange.
func (w *Waveform) Draw(screen *ebiten.Image) {
	if w.Rect.Empty() {
		return
	}
	FillRound(screen, w.Rect, Radius, ColorCanvas())
	left, right := w.endX(w.Start), w.endX(w.End)
	if w.HasRegion {
		FillRect(screen, image.Rect(left, w.Rect.Min.Y+1, right, w.Rect.Max.Y-1), ColorSelectWash())
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
		w.drawEnd(screen, dragStart, w.Start, 1)
		w.drawEnd(screen, dragEnd, w.End, -1)
	}
	if w.Playing && w.shown(w.Playhead) {
		x := w.xOf(w.Playhead)
		Line(screen, image.Pt(x, w.Rect.Min.Y), image.Pt(x, w.Rect.Max.Y), Outline, ColorLive())
	}
	StrokeRound(screen, w.Rect, Radius, Border, ColorRule())
}

// drawEnd draws one end of the region: a line from top to bottom and a flag
// with a grip, pointing into the region, which way says.
func (w *Waveform) drawEnd(screen *ebiten.Image, end waveDrag, ms int64, which int) {
	if !w.shown(ms) {
		return
	}
	x, width := w.endX(ms), float32(Outline)
	if w.drag == end || w.drag == dragNone && w.hover == end {
		width = Outline * Half
	}
	Line(screen, image.Pt(x, w.Rect.Min.Y), image.Pt(x, w.Rect.Max.Y), width, ColorSelect())
	flag := image.Rect(x, w.Rect.Min.Y, x+which*waveReach, w.Rect.Min.Y+waveFlagHeight).Canon()
	FillRect(screen, flag, ColorSelect())
	grip := flag.Min.X + (flag.Dx()-Half)/Half
	top := flag.Min.Y + (flag.Dy()-waveGrip)/Half
	FillRect(screen, image.Rect(grip, top, grip+Hairline, top+waveGrip), ColorBackground())
	FillRect(screen, image.Rect(grip+Half, top, grip+Half+Hairline, top+waveGrip), ColorBackground())
}
