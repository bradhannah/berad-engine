package window

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// fakeSurface stands in for the window a host is shown in, recording
// what it is asked to float.
type fakeSurface struct {
	closing closeRequest
	floated []bool
}

func (*fakeSurface) Input(*ui.TextField) ui.Input { return ui.NewInput(image.Pt(0, 0)) }
func (s *fakeSurface) Closing() closeRequest      { return s.closing }
func (*fakeSurface) SetTitle(string)              {}
func (*fakeSurface) RequestAttention()            {}
func (s *fakeSurface) SetFloating(on bool)        { s.floated = append(s.floated, on) }

// TestOnTopAsksTheWindowToFloat asks the window to float once when the
// toggle turns on, and to stop when it turns off.
func TestOnTopAsksTheWindowToFloat(t *testing.T) {
	t.Parallel()
	shown := &fakeSurface{}
	h := &host{frame: frame.NewFrame(time.Now), surface: shown}
	h.applyOnTop()
	h.frame.OnTop.On = true
	h.applyOnTop()
	h.applyOnTop()
	h.frame.OnTop.On = false
	h.applyOnTop()
	if !slices.Equal(shown.floated, []bool{true, false}) {
		t.Fatalf("asked %v", shown.floated)
	}
}

// TestCloseButtonAsksTheViewToClose turns the window's close button into a
// request to close, which the view answers, and leaves a frame without one
// alone.
func TestCloseButtonAsksTheViewToClose(t *testing.T) {
	t.Parallel()
	shown := &fakeSurface{}
	h := &host{frame: frame.NewFrame(time.Now), surface: shown}
	input := ui.NewInput(image.Pt(0, 0))
	h.windowKeys(&input)
	if h.frame.Closing {
		t.Fatal("closing without a close request")
	}
	shown.closing = closeWindow
	h.windowKeys(&input)
	if !h.frame.Closing {
		t.Fatal("the close button did not ask to close")
	}
}
