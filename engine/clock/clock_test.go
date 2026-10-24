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
