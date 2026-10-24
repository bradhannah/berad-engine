package window

import (
	"context"
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Quit dialog button IDs, for input scripts.
const (
	quitNow     = "quit"
	quitSave    = "quit-save"
	quitDiscard = "quit-discard"
)

// windowKeys handles what every window does the same way, before its view
// sees the input: Cmd+Q asks to quit the editor, Cmd+S saves, and the close
// button asks to close the window. On macOS, Cmd+Q reaches a window as a
// close request while Cmd is held, so that is how it is told apart from the
// close button.
func (h *host) windowKeys(input *ui.Input) {
	if ebiten.IsWindowBeingClosed() {
		if ebiten.IsKeyPressed(ebiten.KeyMeta) {
			h.askQuit()
		} else {
			h.frame.Closing = true
		}
	}
	switch {
	case input.Consume(ui.KeyQ, ui.Cmd()):
		h.askQuit()
	case input.Consume(ui.KeyS, ui.Cmd()):
		h.save()
	}
}

// askQuit asks the orchestrator what quitting would close, then asks the
// author whether they are done.
func (h *host) askQuit() {
	if h.quitDialog != nil || h.quitAsking {
		return
	}
	h.quitAsking = true
	h.background(func(ctx context.Context) func() {
		ask, err := h.conn.QuitAsk(ctx)

		return func() {
			h.quitAsking = false
			if err != nil {
				h.frame.Status(err.Error())

				return
			}
			h.quitDialog = quitDialog(ask)
		}
	})
}

// quitDialog asks whether the author is done: every window will close. With
// unsaved changes anywhere it names them and offers Save and quit, Don't save
// and Cancel; with none, Quit and Cancel.
func quitDialog(ask protocol.QuitAsk) *ui.Dialog {
	closes := "This window will close."
	if ask.Windows > 1 {
		closes = fmt.Sprintf("All %d windows will close.", ask.Windows)
	}
	if len(ask.Unsaved) == 0 {
		return ui.NewDialog("Quit the editor?", []string{closes, "Everything is saved."},
			ui.DialogButton(quitNow, "Quit"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	}
	lines := []string{closes, "Unsaved changes:"}
	for _, window := range ask.Unsaved {
		name := window.Name
		if name == "" {
			name = "a " + window.View + " window"
		}
		changes := "1 change"
		if window.Count > 1 {
			changes = strconv.Itoa(window.Count) + " changes"
		}
		lines = append(lines, "  "+name+" · "+changes)
	}

	return ui.NewDialog("Quit the editor?", lines, ui.DialogButton(quitSave, "Save and quit"),
		ui.DialogButton(quitDiscard, "Don't save"), ui.DialogButton(ui.DialogCancel, "Cancel"))
}

// updateQuitDialog handles the quit dialog while it shows; it takes all of
// the frame's input.
func (h *host) updateQuitDialog(input *ui.Input) {
	if h.quitDialog == nil {
		return
	}
	switch h.quitDialog.Update(input, image.Rectangle{Min: image.Pt(0, 0), Max: h.ctx.Size}) {
	case "":
		return
	case quitNow, quitDiscard:
		h.quit(false)
	case quitSave:
		h.quit(true)
	}
	h.quitDialog = nil
}

// quit asks the orchestrator to quit; it tells every window to close.
func (h *host) quit(save bool) {
	h.background(func(ctx context.Context) func() {
		result, err := h.conn.Quit(ctx, save)

		return func() {
			if err != nil || !result.OK {
				h.frame.Status(strings.TrimSpace("Could not quit: " + result.Reason))
			}
		}
	})
}

// saveForQuit saves the window's unsaved changes before it closes for a
// quit, and reports whether it must stay open to finish, or because they
// could not be saved.
func (h *host) saveForQuit() bool {
	saver, ok := h.view.(frame.Saver)
	if !ok || saver.Unsaved(h.conn) == 0 {
		return false
	}
	if saver.Save(&h.ctx) {
		h.quitSaving = true
	} else {
		h.frame.Status("This window could not save its changes, so it stays open. Fix them, then quit again.")
	}

	return true
}

// savedForQuit reports whether a window saving for a quit has heard back
// about every command; if a save was refused, it stays open.
func (h *host) savedForQuit() bool {
	if len(h.outbox) > 0 || len(h.pending) > 0 {
		return false
	}
	h.quitSaving = false
	if saver, ok := h.view.(frame.Saver); ok && saver.Unsaved(h.conn) > 0 {
		h.frame.Status("Some changes could not be saved, so this window stays open.")

		return false
	}

	return true
}

// reportUnsaved tells the orchestrator when the window's unsaved changes
// change, so a quit can name them.
func (h *host) reportUnsaved() {
	saver, ok := h.view.(frame.Saver)
	if !ok {
		return
	}
	unsaved := protocol.Unsaved{Count: saver.Unsaved(h.conn), Name: ""}
	if unsaved.Count > 0 {
		unsaved.Name = saver.UnsavedName(h.conn)
	}
	if unsaved == h.reported {
		return
	}
	h.reported = unsaved
	if err := h.conn.Unsaved(unsaved); err != nil {
		h.frame.Status(err.Error())
	}
}

// Locate finds a named part of the window for input scripts: the quit
// dialog's buttons while it shows, or the view's parts.
func (h *host) Locate(name string, docs frame.Docs) (image.Point, bool) {
	if h.quitDialog != nil {
		_, value, _ := strings.Cut(name, ":")

		return h.quitDialog.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: h.ctx.Size})
	}

	return h.view.Locate(name, docs)
}
