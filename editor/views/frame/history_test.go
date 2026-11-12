package frame_test

import (
	"errors"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TestARefusalStaysAndHistoryKeepsWhatWasSaid keeps every message in the
// log; a refusal stays in the status bar after other messages fade, until
// Dismiss; History opens the list, which Esc closes; and Undo says what it
// took back (increment 39).
func TestARefusalStaysAndHistoryKeepsWhatWasSaid(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 11, 16, 0, 0, 0, time.UTC)
	status := frame.NewFrame(func() time.Time { return now })
	size := image.Pt(1200, 800)
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	press := func(name string) {
		point, found := status.LocateStatus(name)
		if !found {
			t.Fatalf("no %s", name)
		}
		input := ui.NewInput(point)
		input.Pressed, input.Down = true, true
		status.UpdateHistory(&input, size, nil)
		status.UpdateStatus(&input, size)
	}
	status.UpdateStatus(&ui.Input{}, size)
	status.ReportStep("Undid")(protocol.Result{OK: true, Said: "Painted Ground floor"}, nil)
	if text, _ := status.Message(); text != "Undid: Painted Ground floor" {
		t.Fatalf("undo says what it took back: %q", text)
	}
	status.Report("")(protocol.Result{OK: false, Reason: "The floor was deleted"}, errors.New("refused"))
	status.Status("Saved")
	now = now.Add(10 * time.Second)
	if text, _ := status.Message(); text != "Refused: The floor was deleted" || status.Refusal() == "" {
		t.Fatalf("the refusal stays once other messages fade: %q", text)
	}
	status.UpdateStatus(&ui.Input{}, size)
	status.DrawStatus(ebiten.NewImage(size.X, size.Y), theme)
	press("ui:dismiss")
	if text, _ := status.Message(); strings.HasPrefix(text, "Refused") || status.Refusal() != "" {
		t.Fatalf("Dismiss takes the refusal away: %q", text)
	}
	log := status.Log()
	if len(log) != 3 || log[0].Text != "Undid: Painted Ground floor" || !log[1].Refusal || log[2].Text != "Saved" {
		t.Fatalf("the log keeps what was said, the refusal marked: %+v", log)
	}
	if _, said := status.LocateStatus("state:said:Ground floor"); !said {
		t.Fatal("scripts find what was said")
	}
	press("ui:history")
	if !status.HistoryOpen() {
		t.Fatal("History opens the list")
	}
	status.DrawHistory(ebiten.NewImage(size.X, size.Y), theme)
	escape := ui.NewInput(image.Pt(0, 0))
	escape.Presses = []ui.Press{ui.ParsePress("Escape")}
	status.UpdateHistory(&escape, size, nil)
	if status.HistoryOpen() || len(escape.Presses) != 0 {
		t.Fatal("Esc closes the list and goes nowhere else")
	}
}

// TestALineInHistorySelectsWhatItNames chooses, on a click, the thing a
// line's change touched that the view can still select, and closes History;
// a line that names nothing does nothing (increment 45).
func TestALineInHistorySelectsWhatItNames(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 11, 16, 0, 0, 0, time.UTC)
	status := frame.NewFrame(func() time.Time { return now })
	size := image.Pt(1200, 800)
	workflow := ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")
	status.StatusAbout("Added nodes to Clerk", protocol.Touching(workflow, []string{"box:x", "node:wait"}))
	status.Status("Saved")
	named := func(touched protocol.Touch) (string, string, bool) {
		for _, item := range touched.Items {
			if touched.Document == workflow && item == "node:wait" {
				return item, "Wait", true
			}
		}

		return "", "", false
	}
	click := func(name string) {
		status.UpdateHistory(&ui.Input{}, size, named)
		point, found := status.LocateStatus(name)
		if !found {
			t.Fatalf("no %s", name)
		}
		input := ui.NewInput(point)
		input.Pressed, input.Down = true, true
		status.UpdateHistory(&input, size, named)
		status.UpdateStatus(&input, size)
	}
	status.UpdateStatus(&ui.Input{}, size)
	click("ui:history")
	click("history:Saved")
	if _, _, chosen := status.TakeChosen(); chosen || !status.HistoryOpen() {
		t.Fatal("a line that names nothing does nothing")
	}
	click("history:Added nodes")
	item, name, chosen := status.TakeChosen()
	if !chosen || item != "node:wait" || name != "Wait" || status.HistoryOpen() {
		t.Fatalf("a line chooses what it names and closes History: %q %q %v", item, name, chosen)
	}
	if _, _, again := status.TakeChosen(); again {
		t.Fatal("a choice is taken once")
	}
}

// TestAStrokeKeepsOneLine says each command of a stroke again in the line
// it began, so History holds one line for the stroke (increment 51).
func TestAStrokeKeepsOneLine(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 11, 16, 0, 0, 0, time.UTC)
	status := frame.NewFrame(func() time.Time { return now })
	status.StatusAbout("Painted 1 cell of Road on Ground floor", protocol.Untouched())
	status.StatusAgain("Painted 4 cells of Road on Ground floor", protocol.Untouched())
	log := status.Log()
	if len(log) != 1 || log[0].Text != "Painted 4 cells of Road on Ground floor" {
		t.Fatalf("the stroke keeps one line, saying it whole: %+v", log)
	}
	if text, _ := status.Message(); text != "Painted 4 cells of Road on Ground floor" {
		t.Fatalf("and the status bar says it: %q", text)
	}
}
