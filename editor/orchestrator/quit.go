package orchestrator

import (
	"maps"
	"slices"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// handleQuit routes the messages about windows closing: a window's unsaved
// changes, what quitting would close, quitting itself, and a pane's failure
// as it closes.
func (o *Orchestrator) handleQuit(from *window, message protocol.Message) {
	o.route(from, message, map[protocol.Kind]handler{
		protocol.KindUnsaved: o.unsaved,
		protocol.KindQuitAsk: func(from *window, message protocol.Message) {
			o.send(from, mustMessage(protocol.KindQuitAsk, message.Request, o.quitAsk()))
		},
		protocol.KindQuit:    o.quitEditor,
		protocol.KindCrashed: o.crashed,
	})
}

// unsaved records what a window last said of its unsaved changes.
func (o *Orchestrator) unsaved(from *window, message protocol.Message) {
	if unsaved, err := protocol.DecodePayload[protocol.Unsaved](message); err == nil {
		from.unsaved = unsaved
	}
}

// quitEditor tells every window to close, saving first if asked; the
// orchestrator stops when the last one has gone.
func (o *Orchestrator) quitEditor(from *window, message protocol.Message) {
	quit, err := protocol.DecodePayload[protocol.Quit](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	o.reply(from, message.Request, "", 0)
	o.quitting = true
	for _, open := range o.windows {
		o.send(open, mustMessage(protocol.KindShutdown, 0, protocol.Shutdown{
			Reason: "the editor is quitting", Save: quit.Save,
		}))
	}
}

// quitAsk lists the open windows' unsaved changes, in a steady order.
func (o *Orchestrator) quitAsk() protocol.QuitAsk {
	ask := protocol.QuitAsk{Windows: len(o.windows), Unsaved: []protocol.UnsavedWindow{}}
	for _, id := range slices.SortedFunc(maps.Keys(o.windows), ids.ULID.Compare) {
		open := o.windows[id]
		if open.unsaved.Count > 0 {
			ask.Unsaved = append(ask.Unsaved, protocol.UnsavedWindow{
				Window: open.id, View: open.hello.View, Name: open.unsaved.Name, Count: open.unsaved.Count,
			})
		}
	}

	return ask
}
