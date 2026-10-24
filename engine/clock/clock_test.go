package clock_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/engine/clock"
)

func TestClockRunsStopsWrapsAndJumps(t *testing.T) {
	t.Parallel()
	game := clock.New(8*3600, clock.MinutePerSecond)
	for range clock.TicksPerSecond {
		game.Advance()
	}
	if game.MinuteOfDay() != 8*60+1 || clock.Format(game.Seconds()) != "08:01" {
		t.Fatalf("a second at 1 min/s is a game minute: %s", clock.Format(game.Seconds()))
	}
	game.Stop(true)
	if passed := game.Advance(); passed != 0 || game.MinuteOfDay() != 8*60+1 {
		t.Fatal("a stopped clock does not move")
	}
	game.Stop(false)
	game.SetRate(clock.TenMinutesPerSecond)
	if passed := game.Advance(); passed != 10 {
		t.Fatalf("a tick at 10 min/s is 10 game seconds, not %v", passed)
	}
	game.Jump(23*3600 + 59*60 + 59)
	game.SetRate(clock.Real)
	for range clock.TicksPerSecond {
		game.Advance()
	}
	if game.MinuteOfDay() != 0 || game.Jumps() != 1 {
		t.Fatalf("midnight wraps to 00:00 after one jump: %s, %d jumps", clock.Format(game.Seconds()), game.Jumps())
	}
	if clock.Format(-60) != "23:59" || clock.RateLabel(clock.MinutePerSecond) != "1 min/s" {
		t.Fatal("formatting")
	}
}

// TestParse reads times of day as typed, and refuses what is not one.
func TestParse(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]float64{"8": 8 * clock.Hour, "8:30": 30600, " 08:30 ": 30600, "0830": 30600,
		"23:59": 86340, "0": 0} {
		if got, ok := clock.Parse(value); !ok || got != want {
			t.Fatalf("%q: %v %v, want %v", value, got, ok, want)
		}
	}
	for _, value := range []string{"", "24:00", "8:60", "-1", "noon", "8:3x"} {
		if _, ok := clock.Parse(value); ok {
			t.Fatalf("%q is not a time of day", value)
		}
	}
}
