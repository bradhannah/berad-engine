package ui_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

func press(wave *ui.Waveform, x int) {
	input := ui.NewInput(image.Pt(x, 50))
	input.Pressed, input.Down = true, true
	wave.Update(&input)
}

func release(wave *ui.Waveform, x int) {
	input := ui.NewInput(image.Pt(x, 50))
	wave.Update(&input)
}

func move(wave *ui.Waveform, x int) bool {
	input := ui.NewInput(image.Pt(x, 50))
	input.Down = true
	changed, _ := wave.Update(&input)

	return changed
}

// TestWaveformDragsTheRegion picks up the region's start near it, drags it
// within the waveform, keeps it before the end, and reports the release.
func TestWaveformDragsTheRegion(t *testing.T) {
	t.Parallel()
	wave := ui.NewWaveform("wave")
	wave.Rect, wave.From, wave.To = image.Rect(0, 0, 1000, 100), 0, 1000
	wave.Start, wave.End, wave.HasRegion = 200, 800, true
	at, ok := wave.Locate("start")
	if !ok || at.X != 200 {
		t.Fatalf("the flag of the start: %v %v", at, ok)
	}
	press(wave, 207)
	if !wave.Dragging() {
		t.Fatal("a press near the start picks it up")
	}
	if move(wave, 207) || wave.Start != 200 {
		t.Fatal("holding still moves nothing")
	}
	if !move(wave, 307) || wave.Start != 300 {
		t.Fatalf("the start moves as far as the pointer, not to it: %d", wave.Start)
	}
	if !move(wave, 1500) || wave.Start != 790 {
		t.Fatalf("the start stops before the end: %d", wave.Start)
	}
	released := ui.NewInput(image.Pt(1500, 50))
	if _, done := wave.Update(&released); !done || wave.Dragging() {
		t.Fatal("releasing ends the drag")
	}
	press(wave, 500)
	if wave.Dragging() {
		t.Fatal("a press away from both ends drags nothing")
	}
}

// TestWaveformEndsAtTheEdges picks up an end that sits on the waveform's
// border, from inside it or just outside, and the nearer of two close ends.
func TestWaveformEndsAtTheEdges(t *testing.T) {
	t.Parallel()
	wave := ui.NewWaveform("wave")
	wave.Rect, wave.From, wave.To = image.Rect(100, 0, 1100, 100), 0, 1000
	wave.Start, wave.End, wave.HasRegion = 0, 1000, true
	press(wave, 94)
	if !wave.Dragging() || !move(wave, 194) || wave.Start != 100 {
		t.Fatalf("the start on the left border, pressed just outside it: %d", wave.Start)
	}
	move(wave, 94)
	release(wave, 94)
	press(wave, 1105)
	if !wave.Dragging() || !move(wave, 1005) || wave.End != 900 {
		t.Fatalf("the end on the right border, pressed just outside it: %d", wave.End)
	}
	release(wave, 1005)

	wave.Start, wave.End = 500, 504
	press(wave, 607)
	if !wave.Dragging() || !move(wave, 617) || wave.End != 514 {
		t.Fatalf("right of two close ends picks up the end: %d %d", wave.Start, wave.End)
	}
}
