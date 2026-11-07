// Package show keeps what workflows show the player while a game or a
// preview runs: each speaker's lines, said one after another over its head;
// the message boxes, shown one at a time at the foot of the screen; and a
// shake of the screen. Everything is counted in ticks of real time, so it
// repeats exactly however fast the game clock runs, and a host only draws
// it.
package show

import (
	"math"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/clock"
)

// Reading time: a line or a box shows for readBase seconds, and a second
// more for every readChars characters, at most readMost seconds.
const (
	readBase  = 2
	readChars = 12
	readMost  = 10
)

// ReadingTicks is how many ticks a text takes to read, counting only what
// shows, not its markup.
func ReadingTicks(text string) int {
	seconds := min(readBase+utf8.RuneCountInString(Plain(text))/readChars, readMost)

	return seconds * clock.TicksPerSecond
}

// TooLongToRead reports whether a text needs longer to read than a line or
// a box shows for, readMost seconds, so one that goes when read would go
// before it was read.
func TooLongToRead(text string) bool {
	return readBase+utf8.RuneCountInString(Plain(text))/readChars > readMost
}

// SecondsTicks is how many ticks a number of seconds lasts.
func SecondsTicks(seconds int) int { return seconds * clock.TicksPerSecond }

// Line is one line a speaker says: its number, its text, and its ticks in
// all and left.
type Line struct {
	ID    uint64
	Text  string
	Ticks int
	Left  int
}

// Speech is one speaker's lines: the one said now, first, then those
// waiting their turn.
type Speech struct {
	lines []Line
	asked uint64
}

// Say queues a line for a number of ticks and returns its number.
func (s *Speech) Say(text string, ticks int) uint64 {
	s.asked++
	s.lines = append(s.lines, Line{ID: s.asked, Text: text, Ticks: max(ticks, 1), Left: max(ticks, 1)})

	return s.asked
}

// Tick counts down the line said now, going on to the next when it ends.
func (s *Speech) Tick() {
	if len(s.lines) == 0 {
		return
	}
	s.lines[0].Left--
	if s.lines[0].Left <= 0 {
		s.lines = s.lines[1:]
	}
}

// Said reports whether a line has been said and gone.
func (s *Speech) Said(id uint64) bool {
	for _, line := range s.lines {
		if line.ID == id {
			return false
		}
	}

	return true
}

// Now returns the line said now, if any.
func (s *Speech) Now() (Line, bool) {
	if len(s.lines) == 0 {
		return Line{ID: 0, Text: "", Ticks: 0, Left: 0}, false
	}

	return s.lines[0], true
}

// DropWaiting drops the lines not yet said, leaving the one said now.
func (s *Speech) DropWaiting() {
	if len(s.lines) > 1 {
		s.lines = s.lines[:1]
	}
}

// Box is one message box: its number, its text, the name on it, its ticks
// in all and left, and whether it waits for the player to close it instead.
type Box struct {
	ID      uint64
	Text    string
	Name    string
	Ticks   int
	Left    int
	Closing bool
}

// Shake strengths, in pixels of the game's scale.
const (
	GentlePixels   = 2
	ModeratePixels = 4
	StrongPixels   = 8
)

// Stage is what every workflow of a game shares: the boxes, shown one at a
// time in the order asked for, and the shake.
type Stage struct {
	boxes []Box
	asked uint64
	// pixels is how far the screen shakes at most, left and total its
	// ticks, and ticks the ticks counted, which set its direction.
	pixels      float64
	left, total int
	ticks       uint64
}

// NewStage returns a stage with nothing on it.
func NewStage() *Stage {
	return &Stage{boxes: nil, asked: 0, pixels: 0, left: 0, total: 0, ticks: 0}
}

// Ask queues a box and returns its number. A box that waits for the player
// to close it has no ticks of its own.
func (s *Stage) Ask(text, name string, ticks int, closing bool) uint64 {
	s.asked++
	s.boxes = append(s.boxes, Box{ID: s.asked, Text: text, Name: name, Ticks: max(ticks, 1), Left: max(ticks, 1),
		Closing: closing})

	return s.asked
}

// Tick counts down the box shown now, unless it waits to be closed, and the
// shake.
func (s *Stage) Tick() {
	s.ticks++
	if s.left > 0 {
		s.left--
	}
	if len(s.boxes) == 0 || s.boxes[0].Closing {
		return
	}
	s.boxes[0].Left--
	if s.boxes[0].Left <= 0 {
		s.boxes = s.boxes[1:]
	}
}

// Gone reports whether a box has been shown and gone.
func (s *Stage) Gone(id uint64) bool {
	for _, box := range s.boxes {
		if box.ID == id {
			return false
		}
	}

	return true
}

// Front returns the box shown now and how many boxes are queued in all.
func (s *Stage) Front() (Box, int, bool) {
	if len(s.boxes) == 0 {
		return Box{ID: 0, Text: "", Name: "", Ticks: 0, Left: 0, Closing: false}, 0, false
	}

	return s.boxes[0], len(s.boxes), true
}

// Holding reports whether the box shown now waits for the player to close
// it, which holds the player until then.
func (s *Stage) Holding() bool { return len(s.boxes) > 0 && s.boxes[0].Closing }

// Close closes the box shown now when it waits for the player.
func (s *Stage) Close() {
	if s.Holding() {
		s.boxes = s.boxes[1:]
	}
}

// Shake shakes the screen at a strength, in pixels, for a number of ticks;
// two shakes at once take the stronger and the later end.
func (s *Stage) Shake(pixels float64, ticks int) {
	if s.left > 0 {
		pixels = math.Max(pixels, s.pixels*float64(s.left)/float64(s.total))
		ticks = max(ticks, s.left)
	}
	s.pixels, s.left, s.total = pixels, ticks, ticks
}

// Shaking reports whether the screen is shaking.
func (s *Stage) Shaking() bool { return s.left > 0 }

// Offset is how far the world is moved this tick, in pixels of the game's
// scale: back and forth, settling to still as the shake ends.
func (s *Stage) Offset() (float64, float64) {
	if s.left <= 0 || s.total <= 0 {
		return 0, 0
	}
	reach := s.pixels * float64(s.left) / float64(s.total)
	turn := float64(s.ticks)

	return reach * math.Sin(turn*shakeTurnX), reach * math.Cos(turn*shakeTurnY)
}

// How fast a shake turns across and down, in radians a tick, different so
// the screen does not move in a line.
const (
	shakeTurnX = 1.7
	shakeTurnY = 2.3
)
