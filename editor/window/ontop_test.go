package window

import (
	"image"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// onTopPane is a pane with a host whose status bar has an On top toggle.
func onTopPane(view string) *pane {
	shown := titled(view, view)
	shown.host = &host{frame: frame.NewFrame(time.Now), surface: shown.place}

	return shown
}

// TestOnTopIsTheWholeWindows turns On top on from one pane and off from
// another: it keeps the whole window above other applications' windows, so
// every pane's toggle shows it.
func TestOnTopIsTheWholeWindows(t *testing.T) {
	t.Parallel()
	browser, text := onTopPane("browser"), onTopPane("text")
	s := &shell{tabs: []*tab{{panes: []*pane{browser}}, {panes: []*pane{text}}}}
	text.host.frame.OnTop.On = true
	s.followOnTop()
	if !s.onTop || !browser.host.frame.OnTop.On {
		t.Fatalf("on top %v, the browser's toggle %v", s.onTop, browser.host.frame.OnTop.On)
	}
	browser.host.frame.OnTop.On = false
	s.followOnTop()
	if s.onTop || text.host.frame.OnTop.On {
		t.Fatalf("on top %v, the text's toggle %v", s.onTop, text.host.frame.OnTop.On)
	}
}

// TestCloseButtonAsksTheViewToClose turns the window's close button into a
// request to close, which the view answers, and leaves a frame without one
// alone.
func TestCloseButtonAsksTheViewToClose(t *testing.T) {
	t.Parallel()
	shown := newPaneSurface()
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
