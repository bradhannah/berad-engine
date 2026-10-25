package ui_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestWaveformDragsTheRegion picks up the region's start near it, drags it
// within the waveform, keeps it before the end, and reports the release.
func TestWaveformDragsTheRegion(t *testing.T) {
	t.Parallel()
	wave := ui.NewWaveform("wave")
	wave.Rect, wave.From, wave.To = image.Rect(0, 0, 1000, 100), 0, 1000
	wave.Start, wave.End, wave.HasRegion = 200, 800, true
	at, ok := wave.Locate("start")
	if !ok || at.X != 200 {
		t.Fatalf("the tab of the start: %v %v", at, ok)
	}
	press := ui.NewInput(image.Pt(203, 50))
	press.Pressed, press.Down = true, true
	wave.Update(&press)
	if !wave.Dragging() {
		t.Fatal("a press near the start picks it up")
	}
	drag := ui.NewInput(image.Pt(900, 50))
	drag.Down = true
	if changed, _ := wave.Update(&drag); !changed || wave.Start != 790 {
		t.Fatalf("the start stops before the end: %d", wave.Start)
	}
	released := ui.NewInput(image.Pt(900, 50))
	if _, done := wave.Update(&released); !done || wave.Dragging() {
		t.Fatal("releasing ends the drag")
	}
	away := ui.NewInput(image.Pt(500, 50))
	away.Pressed, away.Down = true, true
	if wave.Update(&away); wave.Dragging() {
		t.Fatal("a press away from both ends drags nothing")
	}
}
