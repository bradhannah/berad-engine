package orchestrator

import (
	"encoding/json"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const (
	reasonNothingToUndo = "nothing to undo"
	reasonNothingToRedo = "nothing to redo"
	reasonClash         = "something in this step was changed in another window afterwards"
	reasonUnexpected    = "unexpected message"
)

func (o *Orchestrator) join(joined *window) {
	o.windows[joined.id] = joined
	o.history.join(joined.group)
	if _, ok := o.recent[joined.id]; !ok {
		o.recent[joined.id] = newRing(o.cfg.RecentMessages)
	}
	o.send(joined, mustMessage(protocol.KindWelcome, 0, protocol.Welcome{Window: joined.id}))
	for topic, value := range o.session {
		o.send(joined, mustMessage(protocol.KindSession, 0, protocol.Session{Topic: topic, Value: value}))
	}
}

// leave forgets a window. Its undo list ends unless another view shares it.
func (o *Orchestrator) leave(left *window) {
	if _, ok := o.windows[left.id]; !ok {
		return
	}
	delete(o.windows, left.id)
	for _, subscribers := range o.subscribed {
		delete(subscribers, left.id)
	}
	o.history.leave(left.group)
	close(left.out)
}

func (o *Orchestrator) handle(message inbound) {
	if _, ok := o.windows[message.from.id]; !ok {
		return
	}
	o.record(message.from, directionIn, message.message)
	request := message.message.Request
	switch message.message.Kind {
	case protocol.KindSubscribe:
		o.subscribe(message.from, message.message)
	case protocol.KindCommand:
		o.command(message.from, message.message)
	case protocol.KindUndo:
		o.undo(message.from, request)
	case protocol.KindRedo:
		o.redo(message.from, request)
	case protocol.KindSession:
		o.sessionUpdate(message.from, message.message)
	case protocol.KindOpen:
		o.openWindow(message.from, message.message)
	case protocol.KindHello, protocol.KindWelcome, protocol.KindSnapshot, protocol.KindResult,
		protocol.KindEvent, protocol.KindShutdown:
		o.reply(message.from, request, reasonUnexpected, 0)
	default:
		o.reply(message.from, request, reasonUnexpected, 0)
	}
}

func (o *Orchestrator) subscribe(from *window, message protocol.Message) {
	request, err := protocol.DecodePayload[protocol.Subscribe](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	entry, found := o.docs[request.Document]
	if !found {
		o.reply(from, message.Request, ErrUnknownDocument.Error(), 0)

		return
	}
	data, err := entry.document.Encode()
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	subscribers, exists := o.subscribed[request.Document]
	if !exists {
		subscribers = map[ids.ULID]*window{}
		o.subscribed[request.Document] = subscribers
	}
	subscribers[from.id] = from
	o.send(from, mustMessage(protocol.KindSnapshot, message.Request, protocol.Snapshot{
		Document: entry.info.ID,
		Format:   entry.info.Format,
		Name:     entry.info.Name,
		Revision: entry.info.Revision,
		Data:     data,
	}))
}

func (o *Orchestrator) command(from *window, message protocol.Message) {
	command, err := protocol.DecodePayload[protocol.Command](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	applied, revision, err := o.applyEdit(from, command.Document, command.Edit, command.Args, message.Request)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	o.history.record(from.group, command.Gesture, applied)
	o.reply(from, message.Request, "", revision)
}

// applyEdit applies, journals and broadcasts one edit. If the journal cannot
// record it, the edit is reverted and reported as not saved.
func (o *Orchestrator) applyEdit(
	origin *window, documentID ids.ULID, editName string, args json.RawMessage, request uint64,
) (action, uint64, error) {
	entry, ok := o.docs[documentID]
	if !ok {
		return action{}, 0, fmt.Errorf("%w: %s", ErrUnknownDocument, documentID)
	}
	kind, ok := o.cfg.Registry.Edit(editName)
	if !ok {
		return action{}, 0, fmt.Errorf("%w: %s", ErrUnknownEdit, editName)
	}
	applied, err := kind.Apply(entry.document, args)
	if err != nil {
		return action{}, 0, fmt.Errorf("%s refused: %w", editName, err)
	}
	o.seq++
	revision := entry.info.Revision + 1
	journalErr := o.journal.append(journalEntry{
		Version: journalVersion, Seq: o.seq, Document: documentID, Revision: revision, Edit: editName, Args: args,
	})
	if journalErr != nil {
		if _, revertErr := kind.Apply(entry.document, applied.Inverse); revertErr != nil {
			o.cfg.Logger.Error("could not revert an unsaved edit", "document", documentID, "error", revertErr)
		}

		return action{}, 0, fmt.Errorf("%w: %w", ErrNotSaved, journalErr)
	}
	entry.info.Revision = revision
	entry.dirty = true
	o.history.noteWrite(origin.group, documentID, applied.Touched, o.seq)
	o.broadcast(documentID, protocol.Event{
		Document: documentID, Revision: revision, Edit: editName, Args: args, Origin: origin.id, Request: request,
	})

	return action{
		document: documentID, edit: editName, forward: args, back: applied.Inverse, touched: applied.Touched, seq: o.seq,
	}, revision, nil
}

// undo reverts the window's latest step unless another window has changed any
// of its items since.
func (o *Orchestrator) undo(from *window, request uint64) {
	candidate, ok := o.history.popUndo(from.group)
	if !ok {
		o.reply(from, request, reasonNothingToUndo, 0)

		return
	}
	if o.history.clash(from.group, candidate) {
		o.history.pushUndo(from.group, candidate)
		o.reply(from, request, reasonClash, 0)

		return
	}
	redone := step{gesture: candidate.gesture, actions: make([]action, len(candidate.actions))}
	var revision uint64
	for index := len(candidate.actions) - 1; index >= 0; index-- {
		reverted := candidate.actions[index]
		applied, rev, err := o.applyEdit(from, reverted.document, reverted.edit, reverted.back, 0)
		if err != nil {
			o.reply(from, request, err.Error(), 0)

			return
		}
		revision = rev
		redone.actions[index] = invert(applied)
	}
	o.history.pushRedo(from.group, redone)
	o.reply(from, request, "", revision)
}

func (o *Orchestrator) redo(from *window, request uint64) {
	candidate, ok := o.history.popRedo(from.group)
	if !ok {
		o.reply(from, request, reasonNothingToRedo, 0)

		return
	}
	if o.history.clash(from.group, candidate) {
		o.history.pushRedo(from.group, candidate)
		o.reply(from, request, reasonClash, 0)

		return
	}
	again := step{gesture: candidate.gesture, actions: make([]action, 0, len(candidate.actions))}
	var revision uint64
	for _, reapplied := range candidate.actions {
		applied, rev, err := o.applyEdit(from, reapplied.document, reapplied.edit, reapplied.forward, 0)
		if err != nil {
			o.reply(from, request, err.Error(), 0)

			return
		}
		revision = rev
		again.actions = append(again.actions, applied)
	}
	o.history.pushUndo(from.group, again)
	o.reply(from, request, "", revision)
}

// invert turns the result of applying an undo into the action that redoes it.
func invert(applied action) action {
	applied.forward, applied.back = applied.back, applied.forward

	return applied
}

func (o *Orchestrator) sessionUpdate(from *window, message protocol.Message) {
	update, err := protocol.DecodePayload[protocol.Session](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	o.session[update.Topic] = update.Value
	for _, other := range o.windows {
		if other.id != from.id {
			o.send(other, mustMessage(protocol.KindSession, 0, update))
		}
	}
}

func (o *Orchestrator) publishNotice(notice protocol.Notice) {
	value, err := json.Marshal(notice)
	if err != nil {
		o.cfg.Logger.Error("encode notice", "error", err)

		return
	}
	update := protocol.Session{Topic: protocol.TopicNotice, Value: value}
	o.session[update.Topic] = value
	for _, other := range o.windows {
		o.send(other, mustMessage(protocol.KindSession, 0, update))
	}
}

func (o *Orchestrator) openWindow(from *window, message protocol.Message) {
	request, err := protocol.DecodePayload[protocol.Open](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	if _, err := o.launch(request.View, request.Target); err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	o.reply(from, message.Request, "", 0)
}

// reply answers a request; an empty reason means success.
func (o *Orchestrator) reply(target *window, request uint64, reason string, revision uint64) {
	result := protocol.Result{OK: reason == "", Reason: reason, Revision: revision}
	o.send(target, mustMessage(protocol.KindResult, request, result))
}

func (o *Orchestrator) broadcast(documentID ids.ULID, event protocol.Event) {
	message := mustMessage(protocol.KindEvent, 0, event)
	for _, subscriber := range o.subscribed[documentID] {
		o.send(subscriber, message)
	}
}

// send queues a message for a window. A window whose queue is full has fallen
// too far behind and is disconnected; it can reconnect and resubscribe.
func (o *Orchestrator) send(target *window, message protocol.Message) {
	if _, ok := o.windows[target.id]; !ok {
		return
	}
	o.record(target, directionOut, message)
	select {
	case target.out <- message:
	default:
		o.cfg.Logger.Warn("disconnecting a window that fell behind", "window", target.id)
		o.leave(target)
	}
}

func (o *Orchestrator) record(target *window, direction string, message protocol.Message) {
	item := Recorded{At: o.cfg.Clock().UTC(), Window: target.id, Direction: direction, Message: message}
	if recent, ok := o.recent[target.id]; ok {
		recent.add(item)
	}
	if o.cfg.Tap != nil {
		if err := writeTap(o.cfg.Tap, item); err != nil {
			o.cfg.Logger.Warn("wire tap failed", "error", err)
		}
	}
}
