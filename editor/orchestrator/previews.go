package orchestrator

import (
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const (
	reasonPreviewRunning = "a preview is already running; stop it first"
	reasonPreviewMap     = "only a Map window showing the zone can preview it"
	reasonPreviewClosed  = "the window running the preview closed"
)

// preview is the one preview running (D15): the Map window whose process
// runs it, its zone, and the latest state that window sent.
type preview struct {
	run    ids.ULID
	owner  ids.ULID
	zone   ids.ULID
	latest *protocol.Message
}

// handlePreview routes one of the three preview messages.
func (o *Orchestrator) handlePreview(from *window, message protocol.Message) {
	o.route(from, message, map[protocol.Kind]handler{
		protocol.KindPreviewStart: o.previewStart,
		protocol.KindPreviewState: o.previewState,
		protocol.KindPreviewStop:  o.previewStop,
	})
}

// previewStart starts the project's preview for a Map window and tells the
// zone's other Map windows.
func (o *Orchestrator) previewStart(from *window, message protocol.Message) {
	start, err := protocol.DecodePayload[protocol.PreviewStart](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	if from.hello.View != MapView || from.hello.Target != start.Zone {
		o.reply(from, message.Request, reasonPreviewMap, 0)

		return
	}
	if o.preview != nil {
		o.reply(from, message.Request, reasonPreviewRunning, 0)

		return
	}
	run, err := o.ids.New()
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	o.preview = &preview{run: run, owner: from.id, zone: start.Zone, latest: nil}
	o.send(from, mustMessage(protocol.KindResult, message.Request, protocol.Result{
		OK: true, Reason: "", Revision: 0, Created: run,
	}))
	start.Run, start.Window = run, from.id
	o.toZoneViews(mustMessage(protocol.KindPreviewStart, 0, start), false)
}

// previewState keeps the running preview's latest state and passes it on.
// States from any other window or run are ignored.
func (o *Orchestrator) previewState(from *window, message protocol.Message) {
	state, err := protocol.DecodePayload[protocol.PreviewState](message)
	if err != nil || o.preview == nil || o.preview.owner != from.id || o.preview.run != state.Run {
		return
	}
	message.Request = 0
	o.preview.latest = &message
	o.toZoneViews(message, true)
}

// previewStop ends the running preview at the request of its window.
func (o *Orchestrator) previewStop(from *window, message protocol.Message) {
	stop, err := protocol.DecodePayload[protocol.PreviewStop](message)
	if err != nil || o.preview == nil || o.preview.owner != from.id || o.preview.run != stop.Run {
		return
	}
	o.endPreview(stop.Reason)
}

// endPreview forgets the running preview and tells the zone's other Map
// windows.
func (o *Orchestrator) endPreview(reason string) {
	stop := protocol.PreviewStop{Run: o.preview.run, Reason: reason}
	o.toZoneViews(mustMessage(protocol.KindPreviewStop, 0, stop), false)
	o.preview = nil
}

// previewLeft ends the preview of a window that closed.
func (o *Orchestrator) previewLeft(left ids.ULID) {
	if o.preview != nil && o.preview.owner == left {
		o.endPreview(reasonPreviewClosed)
	}
}

// previewJoined shows a Map window that opens on the zone being previewed
// the preview's latest state.
func (o *Orchestrator) previewJoined(joined *window) {
	if o.preview == nil || !o.showsPreview(joined) {
		return
	}
	start := protocol.PreviewStart{Zone: o.preview.zone, Run: o.preview.run, Window: o.preview.owner}
	o.send(joined, mustMessage(protocol.KindPreviewStart, 0, start))
	if o.preview.latest != nil {
		o.send(joined, *o.preview.latest)
	}
}

// toZoneViews sends a preview message to every Map window of the preview's
// zone except the one running it. A state is skipped for a window that is
// behind: the next one carries a newer value, so it never builds a backlog.
func (o *Orchestrator) toZoneViews(message protocol.Message, latestWins bool) {
	for _, open := range o.windows {
		if !o.showsPreview(open) {
			continue
		}
		if latestWins && len(open.out) > cap(open.out)/2 {
			continue
		}
		o.send(open, message)
	}
}

func (o *Orchestrator) showsPreview(open *window) bool {
	return open.id != o.preview.owner && open.hello.View == MapView && open.hello.Target == o.preview.zone
}
