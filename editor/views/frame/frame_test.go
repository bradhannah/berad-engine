package frame_test

import (
	"image"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// TestOnTopToggle flips the status bar's On top toggle with a click, and
// reports it to input scripts; a narrow status bar still draws.
func TestOnTopToggle(t *testing.T) {
	t.Parallel()
	status := frame.NewFrame(time.Now)
	size := image.Pt(1200, 800)
	status.UpdateStatus(&ui.Input{Cursor: image.Pt(0, 0)}, size)
	at, found := status.LocateStatus("ui:on-top")
	if !found || at.Y < size.Y-frame.StatusHeight || at.X < size.X/2 {
		t.Fatalf("the toggle is at the right of the status bar: %v %v", at, found)
	}
	if _, on := status.LocateStatus("state:on-top"); on {
		t.Fatal("windows start not on top")
	}
	click := &ui.Input{Cursor: at, Pressed: true, Down: true}
	status.UpdateStatus(click, size)
	if _, on := status.LocateStatus("state:on-top"); !on || click.Pressed {
		t.Fatalf("a click turns it on and is used up: %v", click.Pressed)
	}
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	status.DrawStatus(ebiten.NewImage(1200, 800), theme)
	status.DrawStatus(ebiten.NewImage(320, 300), theme)
}

// TestCrashKeyNeedsItsSwitch presses Cmd+Shift+K, which crashes a window on
// purpose to test failure handling. Without the frame's switch, which only
// tests turn on, it does nothing and is left for the view; with it, the
// frame asks to crash.
func TestCrashKeyNeedsItsSwitch(t *testing.T) {
	t.Parallel()
	ctx := frametest.Context(frametest.NewDocs(), &frametest.Requests{}, image.Pt(1200, 800))
	press := func() (*ui.Input, bool) {
		input := ui.NewInput(image.Pt(0, 0))
		input.Presses = []ui.Press{ui.ParsePress("Cmd+Shift+K")}

		return &input, frame.CommonKeys(&input, ctx)
	}
	if input, used := press(); used || ctx.Frame.Crash || len(input.Presses) != 1 {
		t.Fatalf("the crash key without its switch: used %v, crash %v", used, ctx.Frame.Crash)
	}
	ctx.Frame.CrashKey = true
	if input, used := press(); !used || !ctx.Frame.Crash || len(input.Presses) != 0 {
		t.Fatalf("the crash key with its switch: used %v, crash %v", used, ctx.Frame.Crash)
	}
}
