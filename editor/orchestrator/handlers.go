package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
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
	o.hostJoined(joined)
	o.pickJoined(joined)
	o.previewJoined(joined)
	if item, ok := o.attentions[joined.id]; ok {
		delete(o.attentions, joined.id)
		o.send(joined, mustMessage(protocol.KindAttention, 0, protocol.Attention{Item: item}))
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
	o.endPicksWith(left.id)
	o.previewLeft(left.id)
	o.hostLeft(left)
}

func (o *Orchestrator) handle(ctx context.Context, message inbound) {
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
	case protocol.KindQuery:
		o.query(message.from, message.message)
	case protocol.KindPickAsk, protocol.KindPickStart, protocol.KindPickAnswer, protocol.KindPickCancel:
		o.handlePick(message.from, message.message)
	case protocol.KindPreviewStart, protocol.KindPreviewState, protocol.KindPreviewStop,
		protocol.KindPreviewControl:
		o.handlePreview(message.from, message.message)
	case protocol.KindUnsaved, protocol.KindQuitAsk, protocol.KindQuit:
		o.handleQuit(message.from, message.message)
	case protocol.KindAudioLibrary, protocol.KindRecording, protocol.KindAudioEdit, protocol.KindAudioAnalysis,
		protocol.KindAudioCheck, protocol.KindAudioLoudness:
		o.handleAudio(ctx, message.from, message.message)
	case protocol.KindHello, protocol.KindWelcome, protocol.KindSnapshot, protocol.KindResult,
		protocol.KindEvent, protocol.KindShutdown, protocol.KindAnswer, protocol.KindAttention,
		protocol.KindAudioChanged, protocol.KindHost:
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
	layout, err := model.LayoutOf(entry.document)
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
		Layout:   layout,
	}))
}

func (o *Orchestrator) command(from *window, message protocol.Message) {
	command, err := protocol.DecodePayload[protocol.Command](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	applied, err := o.applyEdit(from, command.Document, command.Edit, command.Args, message.Request)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	o.history.record(from.group, command.Gesture, applied.action)
	o.send(from, mustMessage(protocol.KindResult, message.Request, protocol.Result{
		OK: true, Reason: "", Revision: applied.revision, Created: applied.created, Item: "",
	}))
}

// appliedEdit is the outcome of applyEdit.
type appliedEdit struct {
	action   action
	revision uint64
	created  ids.ULID
}

// applyEdit prepares, applies, journals and broadcasts one edit. If the
// journal cannot record it, the edit is reverted and reported as not saved.
func (o *Orchestrator) applyEdit(
	origin *window, documentID ids.ULID, editName string, args json.RawMessage, request uint64,
) (appliedEdit, error) {
	entry, ok := o.docs[documentID]
	if !ok {
		return appliedEdit{}, fmt.Errorf("%w: %s", ErrUnknownDocument, documentID)
	}
	kind, ok := o.cfg.Registry.Edit(editName)
	if !ok {
		return appliedEdit{}, fmt.Errorf("%w: %s", ErrUnknownEdit, editName)
	}
	if err := o.refuseReadOnly(documentID); err != nil {
		return appliedEdit{}, fmt.Errorf("%s refused: %w", editName, err)
	}
	args, created, err := o.prepare(entry, kind, args)
	if err != nil {
		return appliedEdit{}, fmt.Errorf("%s refused: %w", editName, err)
	}
	if remover, ok := kind.(model.Remover); ok {
		if err := o.checkRemoval(documentID, entry.document, remover, args); err != nil {
			return appliedEdit{}, fmt.Errorf("%s refused: %w", editName, err)
		}
	}
	applied, err := kind.Apply(entry.document, args)
	if err != nil {
		return appliedEdit{}, fmt.Errorf("%s refused: %w", editName, err)
	}
	o.seq++
	revision := entry.info.Revision + 1
	journalErr := o.journal.append(journalEntry{
		Version: journalVersion, Seq: o.seq, Document: documentID, Revision: revision, Edit: editName, Args: args,
	})
	if journalErr != nil {
		o.revert(entry, kind, applied)

		return appliedEdit{}, fmt.Errorf("%w: %w", ErrNotSaved, journalErr)
	}
	if isIndex(documentID) {
		if err := o.applyLifecycle(editName, args, true); err != nil {
			o.cfg.Logger.Error("journaled project change could not be carried out", "edit", editName, "error", err)
		}
	}
	entry.info.Revision = revision
	entry.dirty = entry.info.Path != ""
	o.history.noteWrite(origin.group, documentID, applied.Touched, o.seq)
	o.broadcast(documentID, protocol.Event{
		Document: documentID, Revision: revision, Edit: editName, Args: args, Origin: origin.id, Request: request,
	})

	return appliedEdit{
		action: action{
			document: documentID, edit: editName, backEdit: applied.InverseEdit, forward: args, back: applied.Inverse,
			touched: applied.Touched, covers: applied.Covers, seq: o.seq,
		},
		revision: revision,
		created:  created,
	}, nil
}

// prepare completes an edit's arguments before it is applied: project index
// edits by the orchestrator itself, other edits by their kind (D4).
func (o *Orchestrator) prepare(
	entry *documentEntry, kind model.EditKind, args json.RawMessage,
) (json.RawMessage, ids.ULID, error) {
	if isIndex(entry.info.ID) {
		return o.prepareLifecycle(kind.Name(), args)
	}
	preparer, ok := kind.(model.Preparer)
	if !ok {
		return args, ids.ULID{}, nil
	}
	var created ids.ULID
	prepared, err := preparer.Prepare(entry.document, args, func() (ids.ULID, error) {
		assigned, err := o.ids.New()
		if err != nil {
			return ids.ULID{}, fmt.Errorf("new id: %w", err)
		}
		created = assigned

		return assigned, nil
	})
	if err != nil {
		return nil, ids.ULID{}, fmt.Errorf("prepare: %w", err)
	}

	return prepared, created, nil
}

// revert undoes an applied edit that could not be journaled.
func (o *Orchestrator) revert(entry *documentEntry, kind model.EditKind, applied model.Applied) {
	inverse := kind
	if applied.InverseEdit != "" {
		if found, ok := o.cfg.Registry.Edit(applied.InverseEdit); ok {
			inverse = found
		}
	}
	if _, err := inverse.Apply(entry.document, applied.Inverse); err != nil {
		o.cfg.Logger.Error("could not revert an unsaved edit", "document", entry.info.ID, "error", err)
	}
}

// undo reverts the window's latest step unless another window has changed any
// of its items since.
func (o *Orchestrator) undo(from *window, request uint64) {
	if o.audioPending(from.group) {
		o.reply(from, request, reasonAudioSaving, 0)

		return
	}
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
	if err := o.refusedRemoval(candidate.actions, true); err != nil {
		o.history.pushUndo(from.group, candidate)
		o.reply(from, request, err.Error(), 0)

		return
	}
	if isAudioStep(candidate) {
		o.queueAudio(audioJob{from: from, request: request, kind: audioUndoJob, edit: noAudioEdit(), step: candidate})

		return
	}
	redone := step{gesture: candidate.gesture, actions: make([]action, len(candidate.actions))}
	var revision uint64
	for index := len(candidate.actions) - 1; index >= 0; index-- {
		reverted := candidate.actions[index]
		applied, err := o.applyEdit(from, reverted.document, reverted.inverseEdit(), reverted.back, 0)
		if err != nil {
			o.reply(from, request, err.Error(), 0)

			return
		}
		revision = applied.revision
		redone.actions[index] = invert(applied.action)
	}
	o.history.pushRedo(from.group, redone)
	o.reply(from, request, "", revision)
}

func (o *Orchestrator) redo(from *window, request uint64) {
	if o.audioPending(from.group) {
		o.reply(from, request, reasonAudioSaving, 0)

		return
	}
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
	if err := o.refusedRemoval(candidate.actions, false); err != nil {
		o.history.pushRedo(from.group, candidate)
		o.reply(from, request, err.Error(), 0)

		return
	}
	if isAudioStep(candidate) {
		o.queueAudio(audioJob{from: from, request: request, kind: audioRedoJob, edit: noAudioEdit(), step: candidate})

		return
	}
	again := step{gesture: candidate.gesture, actions: make([]action, 0, len(candidate.actions))}
	var revision uint64
	for _, reapplied := range candidate.actions {
		applied, err := o.applyEdit(from, reapplied.document, reapplied.edit, reapplied.forward, 0)
		if err != nil {
			o.reply(from, request, err.Error(), 0)

			return
		}
		revision = applied.revision
		again.actions = append(again.actions, applied.action)
	}
	o.history.pushUndo(from.group, again)
	o.reply(from, request, "", revision)
}

// invert turns the result of applying an undo into the action that redoes it.
func invert(applied action) action {
	applied.forward, applied.back = applied.back, applied.forward
	applied.edit, applied.backEdit = applied.inverseEdit(), applied.edit

	return applied
}

// inverseEdit names the edit kind that applies an action's back arguments.
func (a action) inverseEdit() string {
	if a.backEdit != "" {
		return a.backEdit
	}

	return a.edit
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
	if !request.PopOut.IsZero() {
		o.openPopOut(from, message.Request, request)

		return
	}
	if !request.NewView {
		for _, open := range o.windows {
			if open.hello.View == request.View && open.hello.Target == request.Target {
				o.send(open, mustMessage(protocol.KindAttention, 0, protocol.Attention{Item: request.Item}))
				o.reply(from, message.Request, "", 0)

				return
			}
		}
	}
	launched, err := o.launch(request.View, request.Target)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	if request.Item != "" {
		o.attentions[launched] = request.Item
	}
	o.reply(from, message.Request, "", 0)
}

// openPopOut opens a window of its own for a shell's pane.
func (o *Orchestrator) openPopOut(from *window, requested uint64, request protocol.Open) {
	launched, err := o.popOut(request.View, request.Target, request.PopOut)
	if err != nil {
		o.reply(from, requested, err.Error(), 0)

		return
	}
	if request.Item != "" {
		o.attentions[launched] = request.Item
	}
	o.reply(from, requested, "", 0)
}

// reply answers a request; an empty reason means success.
func (o *Orchestrator) reply(target *window, request uint64, reason string, revision uint64) {
	result := protocol.Result{OK: reason == "", Reason: reason, Revision: revision, Created: ids.ULID{}, Item: ""}
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
	// The preview stream would crowd everything else out of a crash report,
	// so only the wire tap keeps it.
	if recent, ok := o.recent[target.id]; ok && message.Kind != protocol.KindPreviewState {
		recent.add(item)
	}
	if o.cfg.Tap != nil {
		if err := writeTap(o.cfg.Tap, item); err != nil {
			o.cfg.Logger.Warn("wire tap failed", "error", err)
		}
	}
}
