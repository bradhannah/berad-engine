package window

import (
	"github.com/hajimehoshi/ebiten/v2"

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

// surface is what a host needs from the window it is shown in: the frame's
// input and close request, and the window's title, attention and On top. A
// host that runs alone uses its process's own Ebitengine window.
type surface interface {
	// Input returns this frame's input; focused is the view's focused text
	// field, for the input method.
	Input(focused *ui.TextField) ui.Input
	// Closing reports what the close button asked for this frame.
	Closing() closeRequest
	SetTitle(title string)
	// RequestAttention asks for attention; on macOS the Dock icon bounces.
	RequestAttention()
	// SetFloating keeps the window above the others, or stops.
	SetFloating(on bool)
}

// ownWindow is the process's own Ebitengine window.
type ownWindow struct {
	reader *inputReader
}

func newOwnWindow() *ownWindow {
	return &ownWindow{reader: newInputReader()}
}

// Input reads the frame's input from Ebitengine, running the input method for
// the focused field.
func (w *ownWindow) Input(focused *ui.TextField) ui.Input {
	return w.reader.read(focused)
}

// Closing reports Ebitengine's close request.
func (*ownWindow) Closing() closeRequest {
	if !ebiten.IsWindowBeingClosed() {
		return closeNone
	}
	if ebiten.IsKeyPressed(ebiten.KeyMeta) {
		return closeQuit
	}

	return closeWindow
}

// SetTitle sets the window's title.
func (*ownWindow) SetTitle(title string) {
	ebiten.SetWindowTitle(title)
}

// RequestAttention asks the operating system for attention.
func (*ownWindow) RequestAttention() {
	ebiten.RequestAttention()
}

// SetFloating keeps the window above the others, or stops.
func (*ownWindow) SetFloating(on bool) {
	ebiten.SetWindowFloating(on)
}
