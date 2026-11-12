package window

import (
	"context"
	"image"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/playview"
	"github.com/bradhannah/berad-engine/engine/game"
)

// Quit dialog button IDs, for input scripts.
const (
	quitNow     = "quit"
	quitSave    = "quit-save"
	quitDiscard = "quit-discard"
)

// windowKeys handles what every window does the same way, before its view
// sees the input: Cmd+Q asks to quit the editor, Cmd+S saves, Cmd+R launches
// the game in the Play tab, the keys that move between tabs move, and the
// close button asks to close the window.
func (h *host) windowKeys(input *ui.Input) {
	switch h.surface.Closing() {
	case closeQuit:
		h.askQuit()
	case closeWindow:
		h.frame.Closing = true
	case closeNone:
	}
	switch {
	case input.Consume(ui.KeyQ, ui.Cmd()):
		h.askQuit()
	case input.Consume(ui.KeyS, ui.Cmd()):
		h.save()
	case input.Consume(ui.KeyR, ui.Cmd()):
		h.launch()
	case input.ConsumeExact(ui.KeyI, ui.Cmd()):
		h.frame.ShowInspector.On = !h.frame.ShowInspector.On
	case input.ConsumeExact(ui.KeyI, ui.CmdShift()):
		h.frame.ShowLeft.On = !h.frame.ShowLeft.On
	}
	if h.tabKeys != nil {
		h.tabKeys(input, h.frame.Status)
	}
}

// launch opens the Play tab, or launches the open one again (increment 14).
func (h *host) launch() {
	h.OpenItem(playview.ViewName, game.SettingsID(), playview.LaunchItem)
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

// quitDialog asks whether the author is done: every tab will close. With
// unsaved changes anywhere it names them and offers Save and quit, Don't save
// and Cancel; with none, Quit and Cancel.
func quitDialog(ask protocol.QuitAsk) *ui.Dialog {
	const closes = "Every tab will close."
	if len(ask.Unsaved) == 0 {
		return ui.NewDialog("Quit the editor?", []string{closes, "Everything is saved."},
			ui.DialogButton(quitNow, "Quit"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	}
	lines := []string{closes, "Unsaved changes:"}
	for _, window := range ask.Unsaved {
		name := window.Name
		if name == "" {
			name = "a " + window.View + " tab"
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

// shows is the host's part of the window, where the quit dialog is drawn.
// It is not the size the view lays out for, which is wider while the
// inspector is folded.
func (h *host) shows() image.Rectangle {
	return image.Rectangle{Min: image.Pt(0, 0), Max: h.visible}
}

// updateQuitDialog handles the quit dialog while it shows; it takes all of
// the frame's input.
func (h *host) updateQuitDialog(input *ui.Input) {
	if h.quitDialog == nil {
		return
	}
	switch h.quitDialog.Update(input, h.shows()) {
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
		h.frame.Status("This tab could not save its changes, so it stays open. Fix them, then quit again.")
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
		h.frame.Status("Some changes could not be saved, so this tab stays open.")

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
// dialog's buttons while it shows, how many tabs the editor window has
// ("state:tabs:1"), a tab's tint ("state:tab-tint:<title>=<tint>"), whether
// a tab holds changes that wait for Save ("state:tab-unsaved:<title>"),
// whether a tab is in front ("state:front:<title>"), the
// folded inspector's strip ("ui:folded-inspector") and whether it is folded
// ("state:inspector-folded"), the status bar's toggles, or the view's parts.
func (h *host) Locate(name string, docs frame.Docs) (image.Point, bool) {
	if h.quitDialog != nil {
		_, value, _ := strings.Cut(name, ":")

		return h.quitDialog.Locate(value, h.shows())
	}
	if found, handled := h.tabState(name); handled {
		return image.Pt(0, 0), found
	}
	switch name {
	case "ui:folded-inspector":
		return ui.Center(frame.Folded(h.visible)), h.folded()
	case "state:inspector-folded":
		return image.Pt(0, 0), h.folded()
	case "state:left-folded":
		return image.Pt(0, 0), !h.frame.ShowLeft.On
	}
	if point, found := h.frame.LocateStatus(name); found || name == "state:on-top" || name == "state:inspector" {
		return point, found
	}

	return h.view.Locate(name, docs)
}

// tabState answers the input scripts' questions about the editor window's
// tabs, and reports whether the name was one: how many there are, a tab's
// tint, whether it holds changes that wait for Save, and whether it is in
// front.
func (h *host) tabState(name string) (bool, bool) {
	if count, ok := strings.CutPrefix(name, "state:tabs:"); ok && h.tabs != nil {
		return strconv.Itoa(h.tabs()) == count, true
	}
	if tinted, ok := strings.CutPrefix(name, "state:tab-tint:"); ok && h.tabTint != nil {
		title, tint, _ := strings.Cut(tinted, "=")

		return h.tabTint(title) == tint, true
	}
	if title, ok := strings.CutPrefix(name, "state:tab-unsaved:"); ok && h.tabUnsaved != nil {
		return h.tabUnsaved(title), true
	}
	if title, ok := strings.CutPrefix(name, "state:front:"); ok && h.tabFront != nil {
		return h.tabFront(title), true
	}

	return false, false
}
