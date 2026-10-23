package orchestrator

import (
	"maps"
	"slices"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// MapView is the view name of Map windows, whose target is a zone.
const MapView = "map"

const (
	reasonNoPick      = "that pick is no longer active"
	reasonMapClosed   = "the map window closed"
	reasonAskerClosed = "the window that asked closed"
	reasonReplaced    = "another pick started in this map window"
	reasonNoMap       = "that map window is not open"
)

// pick is one pick in progress: the window that asked and the Map window
// answering it.
type pick struct {
	token string
	from  ids.ULID
	to    ids.ULID
	start protocol.PickStart
}

// picks routes pick-on-map messages (D13). The asking window and the Map
// window never talk directly; every message passes through the orchestrator.
type picks struct {
	active map[string]*pick
	// waiting holds picks for Map windows launched but not yet connected.
	waiting map[ids.ULID]protocol.PickStart
}

func newPicks() *picks {
	return &picks{active: map[string]*pick{}, waiting: map[ids.ULID]protocol.PickStart{}}
}

// handlePick routes one of the four pick messages.
func (o *Orchestrator) handlePick(from *window, message protocol.Message) {
	o.route(from, message, map[protocol.Kind]handler{
		protocol.KindPickAsk:    o.pickAsk,
		protocol.KindPickStart:  o.pickStart,
		protocol.KindPickAnswer: o.pickAnswer,
		protocol.KindPickCancel: o.pickCancel,
	})
}

// handler handles one kind of message from a window.
type handler func(*window, protocol.Message)

// route hands a message to the handler for its kind.
func (o *Orchestrator) route(from *window, message protocol.Message, handlers map[protocol.Kind]handler) {
	handle, ok := handlers[message.Kind]
	if !ok {
		o.reply(from, message.Request, reasonUnexpected, 0)

		return
	}
	handle(from, message)
}

// pickAsk replies with the open Map windows and whether each shows something
// that can be picked.
func (o *Orchestrator) pickAsk(from *window, message protocol.Message) {
	ask, err := protocol.DecodePayload[protocol.PickAsk](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	ask.Maps = []protocol.PickMap{}
	for _, id := range slices.SortedFunc(maps.Keys(o.windows), ids.ULID.Compare) {
		open := o.windows[id]
		if open.hello.View == MapView {
			ask.Maps = append(ask.Maps, protocol.PickMap{
				Window: open.id, Zone: open.hello.Target, Shows: o.zoneCanPick(open.hello.Target, ask.What),
			})
		}
	}
	o.send(from, mustMessage(protocol.KindPickAsk, message.Request, ask))
}

// zoneCanPick reports whether any part of a zone holds something to pick.
func (o *Orchestrator) zoneCanPick(zoneID ids.ULID, what string) bool {
	entry, ok := o.docs[zoneID]
	if !ok {
		return false
	}
	container, ok := entry.document.(model.Container)
	if !ok {
		return false
	}
	for _, part := range container.Parts() {
		if partEntry, found := o.docs[part]; found {
			if pickable, can := partEntry.document.(model.Pickable); can && pickable.CanPick(what) {
				return true
			}
		}
	}

	return false
}

// pickStart begins a pick in an open Map window, or opens one on a zone and
// begins the pick once it connects.
func (o *Orchestrator) pickStart(from *window, message protocol.Message) {
	start, err := protocol.DecodePayload[protocol.PickStart](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	start.From = from.id
	if start.Window.IsZero() {
		launched, err := o.launch(MapView, start.Zone)
		if err != nil {
			o.reply(from, message.Request, err.Error(), 0)

			return
		}
		start.Window = launched
		o.picks.waiting[launched] = start
		o.picks.active[start.Pick] = &pick{token: start.Pick, from: from.id, to: launched, start: start}
		o.reply(from, message.Request, "", 0)

		return
	}
	target, ok := o.windows[start.Window]
	if !ok || target.hello.View != MapView {
		o.reply(from, message.Request, reasonNoMap, 0)

		return
	}
	o.beginPick(target, start)
	o.reply(from, message.Request, "", 0)
}

// beginPick hands a pick to a connected Map window, cancelling any pick it
// was already answering.
func (o *Orchestrator) beginPick(target *window, start protocol.PickStart) {
	for token, active := range o.picks.active {
		if active.to == target.id && token != start.Pick {
			o.endPick(active, reasonReplaced, false, true)
		}
	}
	o.picks.active[start.Pick] = &pick{token: start.Pick, from: start.From, to: target.id, start: start}
	o.send(target, mustMessage(protocol.KindPickStart, 0, start))
	o.send(target, mustMessage(protocol.KindAttention, 0, nil))
}

// pickJoined starts a pick waiting for a newly connected Map window.
func (o *Orchestrator) pickJoined(joined *window) {
	start, ok := o.picks.waiting[joined.id]
	if !ok {
		return
	}
	delete(o.picks.waiting, joined.id)
	if _, active := o.picks.active[start.Pick]; active {
		o.beginPick(joined, start)
	}
}

// pickAnswer forwards a Map window's answer to the window that asked.
func (o *Orchestrator) pickAnswer(from *window, message protocol.Message) {
	answer, err := protocol.DecodePayload[protocol.PickAnswer](message)
	if err != nil {
		return
	}
	active, ok := o.picks.active[answer.Pick]
	if !ok || active.to != from.id {
		o.send(from, mustMessage(protocol.KindPickCancel, 0, protocol.PickCancel{Pick: answer.Pick, Reason: reasonNoPick}))

		return
	}
	delete(o.picks.active, answer.Pick)
	if asker, connected := o.windows[active.from]; connected {
		o.send(asker, mustMessage(protocol.KindPickAnswer, 0, answer))
	}
}

// pickCancel ends a pick from either side and tells the other side.
func (o *Orchestrator) pickCancel(from *window, message protocol.Message) {
	cancel, err := protocol.DecodePayload[protocol.PickCancel](message)
	if err != nil {
		return
	}
	active, ok := o.picks.active[cancel.Pick]
	if !ok || (active.from != from.id && active.to != from.id) {
		return
	}
	o.endPick(active, cancel.Reason, active.from == from.id, active.to == from.id)
}

// endPick forgets a pick and tells whichever side did not end it.
func (o *Orchestrator) endPick(active *pick, reason string, tellMap, tellAsker bool) {
	delete(o.picks.active, active.token)
	delete(o.picks.waiting, active.to)
	notice := mustMessage(protocol.KindPickCancel, 0, protocol.PickCancel{Pick: active.token, Reason: reason})
	if target, ok := o.windows[active.to]; ok && tellMap {
		o.send(target, notice)
	}
	if asker, ok := o.windows[active.from]; ok && tellAsker {
		o.send(asker, notice)
	}
}

// endPicksWith cancels every pick involving a window that left.
func (o *Orchestrator) endPicksWith(left ids.ULID) {
	for _, active := range slices.Collect(maps.Values(o.picks.active)) {
		switch left {
		case active.from:
			o.endPick(active, reasonAskerClosed, true, false)
		case active.to:
			o.endPick(active, reasonMapClosed, false, true)
		}
	}
}

// query answers questions that need the whole project.
func (o *Orchestrator) query(from *window, message protocol.Message) {
	question, err := protocol.DecodePayload[protocol.Query](message)
	if err != nil || question.Question != protocol.QuestionWhereUsed {
		o.reply(from, message.Request, "unknown question", 0)

		return
	}
	o.send(from, mustMessage(protocol.KindAnswer, message.Request, protocol.Answer{Uses: o.whereUsed(question)}))
}

// whereUsed lists the documents that refer to a document, or to one item in
// it, ordered by ID.
func (o *Orchestrator) whereUsed(question protocol.Query) []protocol.Use {
	uses := []protocol.Use{}
	for _, documentID := range slices.SortedFunc(maps.Keys(o.docs), ids.ULID.Compare) {
		entry := o.docs[documentID]
		referencer, ok := entry.document.(model.Referencer)
		if !ok || documentID == question.Document {
			continue
		}
		for _, reference := range referencer.References() {
			if reference.Document != question.Document {
				continue
			}
			if question.Item != "" && reference.Item != question.Item {
				continue
			}
			uses = append(uses, protocol.Use{
				Document: documentID, Name: entry.info.Name, Format: entry.info.Format, Label: reference.Label,
			})
		}
	}

	return uses
}
