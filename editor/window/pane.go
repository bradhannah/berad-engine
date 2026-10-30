package window

import (
	"image"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// closeRequest is what the window's close button asked for this frame.
type closeRequest int

const (
	closeNone closeRequest = iota
	// closeWindow is the close button.
	closeWindow
	// closeQuit is Cmd+Q: on macOS it reaches a window as a close request
	// while Cmd is held, which is how it is told apart from the close button.
	closeQuit
)

// paneSurface is a pane's part of the editor window: what a host needs from
// the window it is shown in. The shell gives it the frame's input before the
// pane runs, and turns what the pane asks of its window into tabs, focus and
// the window's title.
type paneSurface struct {
	input     ui.Input
	closing   closeRequest
	title     string
	attention bool
	// front is an input script asking for the pane's tab to come to the
	// front, without asking the operating system for attention.
	front bool
}

func newPaneSurface() *paneSurface {
	return &paneSurface{input: ui.NewInput(outside()), closing: closeNone, title: "", attention: false, front: false}
}

// outside is a cursor position in no pane, for a pane given no mouse.
func outside() image.Point {
	return image.Pt(-1, -1)
}

// Input returns the input the shell gave the pane this frame.
func (p *paneSurface) Input(*ui.TextField) ui.Input {
	return p.input
}

// Closing reports, once, a close request the shell passed on.
func (p *paneSurface) Closing() closeRequest {
	request := p.closing
	p.closing = closeNone

	return request
}

// SetTitle keeps the pane's title for its tab and the window.
func (p *paneSurface) SetTitle(title string) {
	p.title = title
}

// RequestAttention asks the shell to bring the pane forward.
func (p *paneSurface) RequestAttention() {
	p.attention = true
}
