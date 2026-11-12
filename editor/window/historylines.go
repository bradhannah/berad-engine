package window

import (
	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// A line in History selects what it names (increment 45): the view finds,
// among what a line's change touched, the thing it can still select, and a
// click on the line asks it to show that thing, as Back and Forward do. A
// pane whose changes wait for Save keeps its selection, and says so.

// keptForSave is what the status bar says when a pane keeps its selection
// because changes in it wait for Save.
const keptForSave = "Kept what is selected in this tab: it has changes waiting for Save"

// updateHistory runs the open History list, and shows the thing a clicked
// line names.
func (h *host) updateHistory(input *ui.Input) {
	h.frame.UpdateHistory(input, h.visible, h.named())
	item, name, ok := h.frame.TakeChosen()
	if !ok {
		return
	}
	if h.reported.Count > 0 {
		h.frame.Status(keptForSave)

		return
	}
	var update client.Update
	update.Attention, update.Item = true, item
	h.view.Changed(update, &h.ctx)
	h.frame.Status("Selected " + name)
}

// named finds, through the view, what a change touched that it can still
// select; nil when the view cannot say.
func (h *host) named() func(protocol.Touch) (string, string, bool) {
	namer, ok := h.view.(frame.Namer)
	if !ok {
		return nil
	}

	return func(touched protocol.Touch) (string, string, bool) {
		return namer.Named(touched, h.ctx.Docs)
	}
}
