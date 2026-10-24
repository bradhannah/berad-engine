// Package clock is the game clock a preview or a game runs on: a time of
// day that advances by a rate each tick of the game loop, can be stopped,
// and can jump to another time. Jumps are counted, so what runs on the
// clock can start again from the new time rather than live through the gap.
package clock

import (
	"fmt"
	"math"
)

// unitsPerSecond is how finely the clock counts: in sixtieths of a game
// second, so a tick at any rate adds a whole number and time never drifts.
const unitsPerSecond = TicksPerSecond

// Rate is how many game seconds pass in one real second.
type Rate int

// Rates a preview offers. Each is named for what it does, not called normal:
// how fast game time runs in the finished game is still open.
const (
	Real                Rate = 1
	MinutePerSecond     Rate = 60
	TenMinutesPerSecond Rate = 600
)

// Time limits, in seconds.
const (
	// TicksPerSecond is how many ticks of the game loop make a real second.
	TicksPerSecond = 60
	// Day is the length of a game day.
	Day              = 24 * 60 * secondsPerMinute
	secondsPerMinute = 60
	minutesPerHour   = 60
)

// Clock is a game clock. Its zero value stands at midnight and does not
// move until it has a rate.
type Clock struct {
	units   int64
	rate    Rate
	stopped bool
	jumps   int
}

// New returns a running clock at a time of day, in seconds, with a rate.
func New(seconds float64, rate Rate) *Clock {
	return &Clock{units: toUnits(seconds), rate: rate, stopped: false, jumps: 0}
}

// Advance moves the clock on by one tick, unless it is stopped, and returns
// how many game seconds passed.
func (c *Clock) Advance() float64 {
	if c.stopped {
		return 0
	}
	c.units = (c.units + int64(c.rate)) % (Day * unitsPerSecond)

	return float64(c.rate) / unitsPerSecond
}

// Jump sets the time of day, in seconds. Whatever runs on the clock sees the
// jump through Jumps.
func (c *Clock) Jump(seconds float64) {
	c.units = toUnits(seconds)
	c.jumps++
}

// Jumps counts the clock's jumps.
func (c *Clock) Jumps() int { return c.jumps }

// SetRate changes how fast the clock runs.
func (c *Clock) SetRate(rate Rate) { c.rate = rate }

// Rate returns how fast the clock runs.
func (c *Clock) Rate() Rate { return c.rate }

// Stop stops or starts the clock.
func (c *Clock) Stop(stopped bool) { c.stopped = stopped }

// Stopped reports whether the clock is stopped.
func (c *Clock) Stopped() bool { return c.stopped }

// Seconds returns the time of day in seconds.
func (c *Clock) Seconds() float64 { return float64(c.units) / unitsPerSecond }

// MinuteOfDay returns the time of day in whole minutes from midnight.
func (c *Clock) MinuteOfDay() int { return int(c.units / unitsPerSecond / secondsPerMinute) }

// Format writes a time of day in seconds as "09:42".
func Format(seconds float64) string {
	minute := int(wrap(seconds)) / secondsPerMinute

	return fmt.Sprintf("%02d:%02d", minute/minutesPerHour, minute%minutesPerHour)
}

// RateLabel names a rate as the preview's buttons do.
func RateLabel(rate Rate) string {
	switch rate {
	case Real:
		return "real time"
	case MinutePerSecond:
		return "1 min/s"
	case TenMinutesPerSecond:
		return "10 min/s"
	default:
		return fmt.Sprintf("%d s/s", rate)
	}
}

func toUnits(seconds float64) int64 {
	return int64(math.Round(wrap(seconds)*unitsPerSecond)) % (Day * unitsPerSecond)
}

func wrap(seconds float64) float64 {
	wrapped := math.Mod(seconds, Day)
	if wrapped < 0 {
		wrapped += Day
	}

	return wrapped
}
