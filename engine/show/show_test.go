package show_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/show"
)

// TestReadingTime gives a short line 2 seconds, a longer one a second more
// for every 12 characters, and a long one at most 10.
func TestReadingTime(t *testing.T) {
	t.Parallel()
	for text, seconds := range map[string]int{
		"Hi": 2, strings.Repeat("a", 24): 4, strings.Repeat("a", 500): 10,
	} {
		if got := show.ReadingTicks(text); got != seconds*clock.TicksPerSecond {
			t.Fatalf("%d characters read in %d seconds, not %d ticks", len(text), seconds, got)
		}
	}
}

// TestSpeechQueue says lines in turn, and drops those waiting when asked,
// leaving the one said now.
func TestSpeechQueue(t *testing.T) {
	t.Parallel()
	var speech show.Speech
	first, second := speech.Say("one", 2), speech.Say("two", 2)
	speech.Say("three", 2)
	speech.Tick()
	if line, _ := speech.Now(); line.ID != first || speech.Said(first) {
		t.Fatalf("the first line is said first: %+v", line)
	}
	speech.Tick()
	if line, _ := speech.Now(); line.ID != second || !speech.Said(first) {
		t.Fatalf("then the second: %+v", line)
	}
	speech.DropWaiting()
	speech.Tick()
	speech.Tick()
	if _, saying := speech.Now(); saying {
		t.Fatal("the third was dropped while waiting")
	}
}
