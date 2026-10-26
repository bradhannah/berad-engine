package window

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// paneSurface is a pane's part of a shell's window. The shell gives it the
// frame's input before the pane runs, and turns what the pane asks of its
// window into tabs, focus and the window's title.
type paneSurface struct {
	input     ui.Input
	closing   closeRequest
	title     string
	attention bool
}

func newPaneSurface() *paneSurface {
	return &paneSurface{input: ui.NewInput(outside()), closing: closeNone, title: "", attention: false}
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

// SetFloating keeps the whole window above the others, or stops.
func (*paneSurface) SetFloating(on bool) {
	ebiten.SetWindowFloating(on)
}
