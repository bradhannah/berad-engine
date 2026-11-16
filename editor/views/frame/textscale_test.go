package frame_test

import (
	"image"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// TestTheTextButtonOffersTheSteps opens the status bar's Text button and
// chooses 125%, which the frame asks the window for (increment 56).
func TestTheTextButtonOffersTheSteps(t *testing.T) {
	t.Parallel()
	shown := frame.NewFrame(time.Now)
	size := image.Pt(1200, 800)
	click := func(name string) {
		t.Helper()
		shown.UpdateStatus(&ui.Input{}, size)
		at, ok := shown.LocateStatus(name)
		if !ok {
			t.Fatalf("cannot locate %s", name)
		}
		shown.UpdateStatus(&ui.Input{Cursor: at, Pressed: true, Down: true}, size)
		shown.UpdateStatus(&ui.Input{Cursor: at, Released: true}, size)
	}
	if _, ok := shown.LocateStatus("state:text-scale=100"); !ok {
		t.Fatal("text and controls start at 100%")
	}
	click("ui:text-scale")
	click("ui:text-125")
	if asked, ok := shown.TakeTextScale(); !ok || asked != 1.25 {
		t.Fatalf("asked %v %v", asked, ok)
	}
	if _, ok := shown.TakeTextScale(); ok {
		t.Fatal("a scale asked for is taken once")
	}
}

// TestStepsStopAtTheEnds steps the scale up and down its steps, from 75% to
// 200%, and no further.
func TestStepsStopAtTheEnds(t *testing.T) {
	t.Parallel()
	shown := frame.NewFrame(time.Now)
	for _, step := range []struct {
		from  float64
		steps int
		want  float64
	}{{1, 1, 1.1}, {1, -1, 0.9}, {2, 1, 2}, {0.75, -1, 0.75}, {1.2, 1, 1.25}, {1.2, -1, 1.1}, {1.5, 2, 2}} {
		shown.SetTextScale(step.from)
		shown.StepTextScale(step.steps)
		if asked, _ := shown.TakeTextScale(); asked != step.want {
			t.Fatalf("from %v by %d gives %v, want %v", step.from, step.steps, asked, step.want)
		}
	}
}
