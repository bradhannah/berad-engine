// Package frametest provides fakes for testing views without an orchestrator:
// documents held in memory and requests that are recorded, with their replies
// delivered when the test chooses.
package frametest

import (
	"encoding/json"
	"image"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// windowSeed is the ID seed of the window under test.
const windowSeed = 99

// Docs holds documents in memory.
type Docs struct {
	docs map[ids.ULID]*client.Document
}

// NewDocs returns an empty document store.
func NewDocs() *Docs {
	return &Docs{docs: map[ids.ULID]*client.Document{}}
}

// Put adds or replaces a document.
func (d *Docs) Put(id ids.ULID, name string, document model.Document) {
	d.docs[id] = &client.Document{ID: id, Name: name, Format: document.Format(), Revision: 0, Model: document}
}

// View calls fn with a document, as a window's client does.
func (d *Docs) View(document ids.ULID, fn func(client.Document)) bool {
	found, ok := d.docs[document]
	if ok {
		fn(*found)
	}

	return ok
}

// Command is a recorded command and its reply callback.
type Command struct {
	Command protocol.Command
	Done    func(protocol.Result, error)
}

// Decode decodes the command's arguments.
func (c Command) Decode(target any) error {
	return json.Unmarshal(c.Command.Args, target) //nolint:wrapcheck // Test helper; callers report the error.
}

// Succeed delivers a successful reply.
func (c Command) Succeed(created ids.ULID) {
	if c.Done != nil {
		c.Done(protocol.Result{OK: true, Reason: "", Revision: 1, Created: created}, nil)
	}
}

// Requests records what a view asks for.
type Requests struct {
	Commands    []Command
	Undos       int
	Redos       int
	Subscribed  []ids.ULID
	Opened      []protocol.Open
	Uses        []func([]protocol.Use, error)
	PickAsks    []func([]protocol.PickMap, error)
	PickStarts  []protocol.PickStart
	PickAnswers []protocol.PickAnswer
	PickCancels []protocol.PickCancel
	// Previews are preview starts waiting for the test to answer.
	Previews      []func(protocol.Result, error)
	PreviewStates []protocol.PreviewState
	PreviewStops  []protocol.PreviewStop
}

// Command records a command.
func (r *Requests) Command(command protocol.Command, done func(protocol.Result, error)) {
	r.Commands = append(r.Commands, Command{Command: command, Done: done})
}

// Undo records an undo.
func (r *Requests) Undo(func(protocol.Result, error)) { r.Undos++ }

// Redo records a redo.
func (r *Requests) Redo(func(protocol.Result, error)) { r.Redos++ }

// Subscribe records a subscription and reports success.
func (r *Requests) Subscribe(document ids.ULID, done func(error)) {
	r.Subscribed = append(r.Subscribed, document)
	if done != nil {
		done(nil)
	}
}

// Open records a window request.
func (r *Requests) Open(view string, target ids.ULID, newView bool) {
	r.Opened = append(r.Opened, protocol.Open{View: view, Target: target, NewView: newView})
}

// WhereUsed records a where-used question; the test answers it.
func (r *Requests) WhereUsed(_ ids.ULID, _ string, done func([]protocol.Use, error)) {
	r.Uses = append(r.Uses, done)
}

// PickAsk records a pick question; the test answers it.
func (r *Requests) PickAsk(_ string, done func([]protocol.PickMap, error)) {
	r.PickAsks = append(r.PickAsks, done)
}

// PickStart records a pick start and reports success.
func (r *Requests) PickStart(start protocol.PickStart, done func(protocol.Result, error)) {
	r.PickStarts = append(r.PickStarts, start)
	if done != nil {
		done(protocol.Result{OK: true, Reason: "", Revision: 0, Created: ids.ULID{}}, nil)
	}
}

// PickAnswer records a pick answer.
func (r *Requests) PickAnswer(answer protocol.PickAnswer) {
	r.PickAnswers = append(r.PickAnswers, answer)
}

// PickCancel records a pick cancellation.
func (r *Requests) PickCancel(cancel protocol.PickCancel) {
	r.PickCancels = append(r.PickCancels, cancel)
}

// PreviewStart records a preview start; the test answers it.
func (r *Requests) PreviewStart(_ ids.ULID, done func(protocol.Result, error)) {
	r.Previews = append(r.Previews, done)
}

// PreviewState records a preview state.
func (r *Requests) PreviewState(state protocol.PreviewState) {
	r.PreviewStates = append(r.PreviewStates, state)
}

// PreviewStop records the end of a preview.
func (r *Requests) PreviewStop(stop protocol.PreviewStop) {
	r.PreviewStops = append(r.PreviewStops, stop)
}

// Of returns the commands of one edit kind.
func (r *Requests) Of(edit string) []Command {
	var matching []Command
	for _, command := range r.Commands {
		if command.Command.Edit == edit {
			matching = append(matching, command)
		}
	}

	return matching
}

// Context returns a view context backed by the fakes, for a window of the
// given size.
func Context(docs *Docs, requests *Requests, size image.Point) *frame.Context {
	return &frame.Context{
		Docs: docs, Requests: requests, Clipboard: nil, Window: ID(windowSeed), Size: size, Frame: frame.NewFrame(time.Now),
	}
}

// ID returns a fixed ID for tests.
func ID(seed byte) ids.ULID {
	var id ids.ULID
	id[15] = seed

	return id
}
