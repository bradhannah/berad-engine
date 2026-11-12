package npcrun

import (
	"fmt"
	"strconv"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// What a node read or did, in words, for the change it sends the NPC on
// by: a time of day, a span of the day, a length of time and a number of
// tiles.

// minutesPerHour and secondsPerMinute turn minutes and seconds into the
// parts of a time.
const (
	minutesPerHour   = 60
	secondsPerMinute = 60
)

// ClockText is a minute of the day as a time, as "9:05".
func ClockText(minute int) string {
	return fmt.Sprintf("%d:%02d", minute/minutesPerHour, minute%minutesPerHour)
}

// RangeText is a span of the day, as "9:00 to 12:00".
func RangeText(span flow.TimeRange) string {
	return ClockText(span.From) + " to " + ClockText(span.To)
}

// secondsText is a length of time, in seconds or whole minutes, as "30s" or
// "5 min".
func secondsText(seconds float64) string {
	whole := int(seconds)
	if whole >= secondsPerMinute && whole%secondsPerMinute == 0 {
		return strconv.Itoa(whole/secondsPerMinute) + " min"
	}

	return strconv.FormatFloat(seconds, 'f', -1, 64) + "s"
}

// tilesText is a number of tiles, as "1 tile" or "3 tiles".
func tilesText(tiles int) string {
	if tiles == 1 {
		return "1 tile"
	}

	return strconv.Itoa(tiles) + " tiles"
}
