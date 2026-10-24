package client

import (
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

func (c *Client) readLoop() {
	defer c.once.Do(func() {
		close(c.done)
		close(c.updates)
	})
	for {
		message, err := c.conn.Receive()
		if err != nil {
			return
		}
		c.dispatch(message)
	}
}

func (c *Client) dispatch(message protocol.Message) {
	switch message.Kind {
	case protocol.KindSnapshot:
		c.onSnapshot(message)
	case protocol.KindResult:
		c.onResult(message)
	case protocol.KindEvent:
		c.onEvent(message)
	case protocol.KindSession:
		if session, err := protocol.DecodePayload[protocol.Session](message); err == nil {
			update := newUpdate()
			update.Session = &session
			c.emit(update)
		}
	case protocol.KindShutdown:
		if shutdown, err := protocol.DecodePayload[protocol.Shutdown](message); err == nil {
			update := newUpdate()
			update.Shutdown = &shutdown
			c.emit(update)
		}
	case protocol.KindAnswer, protocol.KindQuitAsk:
		c.deliver(message)
	case protocol.KindAttention:
		update := newUpdate()
		update.Attention = true
		c.emit(update)
	case protocol.KindPickAsk, protocol.KindPickStart, protocol.KindPickAnswer, protocol.KindPickCancel:
		c.dispatchPick(message)
	case protocol.KindPreviewStart, protocol.KindPreviewState, protocol.KindPreviewStop,
		protocol.KindPreviewControl:
		c.dispatchPreview(message)
	case protocol.KindHello, protocol.KindWelcome, protocol.KindSubscribe, protocol.KindCommand,
		protocol.KindUndo, protocol.KindRedo, protocol.KindOpen, protocol.KindQuery, protocol.KindUnsaved,
		protocol.KindQuit:
		return
	default:
		return
	}
}

// dispatchPick delivers pick-on-map messages: a reply to PickAsk, or a pick
// starting, answered or cancelled.
func (c *Client) dispatchPick(message protocol.Message) {
	update := newUpdate()
	switch message.Kind {
	case protocol.KindPickAsk:
		c.deliver(message)

		return
	case protocol.KindPickStart:
		start, err := protocol.DecodePayload[protocol.PickStart](message)
		if err != nil {
			return
		}
		update.PickStart = &start
	case protocol.KindPickAnswer:
		answer, err := protocol.DecodePayload[protocol.PickAnswer](message)
		if err != nil {
			return
		}
		update.PickAnswer = &answer
	case protocol.KindPickCancel:
		cancel, err := protocol.DecodePayload[protocol.PickCancel](message)
		if err != nil {
			return
		}
		update.PickCancel = &cancel
	case protocol.KindHello, protocol.KindWelcome, protocol.KindSubscribe, protocol.KindCommand,
		protocol.KindUndo, protocol.KindRedo, protocol.KindOpen, protocol.KindQuery, protocol.KindSnapshot,
		protocol.KindResult, protocol.KindEvent, protocol.KindSession, protocol.KindShutdown,
		protocol.KindAnswer, protocol.KindAttention, protocol.KindPreviewStart, protocol.KindPreviewState,
		protocol.KindPreviewStop, protocol.KindPreviewControl, protocol.KindUnsaved, protocol.KindQuitAsk,
		protocol.KindQuit:
		return
	default:
		return
	}
	c.emit(update)
}

// dispatchPreview delivers a preview another window runs in this window's
// zone: its start, its latest state and its end.
func (c *Client) dispatchPreview(message protocol.Message) {
	update := newUpdate()
	var err error
	switch message.Kind {
	case protocol.KindPreviewStart:
		var start protocol.PreviewStart
		start, err = protocol.DecodePayload[protocol.PreviewStart](message)
		update.PreviewStart = &start
	case protocol.KindPreviewState:
		var state protocol.PreviewState
		state, err = protocol.DecodePayload[protocol.PreviewState](message)
		update.PreviewState = &state
	case protocol.KindPreviewStop:
		var stop protocol.PreviewStop
		stop, err = protocol.DecodePayload[protocol.PreviewStop](message)
		update.PreviewStop = &stop
	case protocol.KindPreviewControl:
		var control protocol.PreviewControl
		control, err = protocol.DecodePayload[protocol.PreviewControl](message)
		update.PreviewControl = &control
	case protocol.KindHello, protocol.KindWelcome, protocol.KindSubscribe, protocol.KindCommand,
		protocol.KindUndo, protocol.KindRedo, protocol.KindOpen, protocol.KindQuery, protocol.KindSnapshot,
		protocol.KindResult, protocol.KindEvent, protocol.KindSession, protocol.KindShutdown,
		protocol.KindAnswer, protocol.KindAttention, protocol.KindPickAsk, protocol.KindPickStart,
		protocol.KindPickAnswer, protocol.KindPickCancel, protocol.KindUnsaved, protocol.KindQuitAsk,
		protocol.KindQuit:
		return
	default:
		return
	}
	if err == nil {
		c.emit(update)
	}
}

// onSnapshot installs a document before the reply is delivered, so events
// that follow always apply on top of it.
func (c *Client) onSnapshot(message protocol.Message) {
	snapshot, err := protocol.DecodePayload[protocol.Snapshot](message)
	if err == nil {
		c.install(snapshot)
	}
	c.mu.Lock()
	resync := c.resyncs[message.Request]
	delete(c.resyncs, message.Request)
	c.mu.Unlock()
	if err == nil {
		document := snapshot.Document
		update := newUpdate()
		update.Snapshot = &document
		c.emit(update)
	}
	if !resync {
		c.deliver(message)
	}
}

func (c *Client) install(snapshot protocol.Snapshot) {
	documentType, ok := c.registry.Document(snapshot.Format)
	if !ok {
		return
	}
	decoded, err := documentType.DecodeWithLayout(snapshot.Data, snapshot.Layout)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.docs[snapshot.Document] = &Document{
		ID:       snapshot.Document,
		Name:     snapshot.Name,
		Format:   snapshot.Format,
		Revision: snapshot.Revision,
		Model:    decoded,
	}
}

func (c *Client) onResult(message protocol.Message) {
	if c.deliver(message) {
		return
	}
	result, err := protocol.DecodePayload[protocol.Result](message)
	if err != nil {
		return
	}
	c.mu.Lock()
	delete(c.sentAt, message.Request)
	c.mu.Unlock()
	update := newUpdate()
	update.Result = &result
	update.Request = message.Request
	c.emit(update)
}

// deliver hands a reply to a waiting request; it reports whether one waited.
func (c *Client) deliver(message protocol.Message) bool {
	c.mu.Lock()
	reply, waiting := c.pending[message.Request]
	delete(c.pending, message.Request)
	if waiting {
		delete(c.sentAt, message.Request)
	}
	c.mu.Unlock()
	if waiting {
		reply <- message
	}

	return waiting
}

// onEvent applies an edit to the local document. A missed revision means the
// copy can no longer be trusted, so the window resubscribes for a snapshot.
func (c *Client) onEvent(message protocol.Message) {
	event, err := protocol.DecodePayload[protocol.Event](message)
	if err != nil {
		return
	}
	applied, gap, latency := c.applyEvent(event)
	if gap {
		c.resubscribe(event.Document)

		return
	}
	if applied {
		update := newUpdate()
		update.Event = &event
		update.Latency = latency
		c.emit(update)
	}
}

func (c *Client) applyEvent(event protocol.Event) (bool, bool, time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	local, loaded := c.docs[event.Document]
	if !loaded || event.Revision <= local.Revision {
		return false, false, 0
	}
	if event.Revision != local.Revision+1 {
		delete(c.docs, event.Document)

		return false, true, 0
	}
	kind, ok := c.registry.Edit(event.Edit)
	if !ok {
		delete(c.docs, event.Document)

		return false, true, 0
	}
	if _, err := kind.Apply(local.Model, event.Args); err != nil {
		delete(c.docs, event.Document)

		return false, true, 0
	}
	local.Revision = event.Revision
	var latency time.Duration
	if event.Origin == c.window && event.Request != 0 {
		if sent, ok := c.sentAt[event.Request]; ok {
			latency = c.clock().Sub(sent)
		}
	}

	return true, false, latency
}

func (c *Client) resubscribe(document ids.ULID) {
	request := c.next.Add(1)
	message, err := protocol.NewMessage(protocol.KindSubscribe, request, protocol.Subscribe{Document: document})
	if err != nil {
		return
	}
	c.mu.Lock()
	c.resyncs[request] = true
	c.mu.Unlock()
	if err := c.conn.Send(message); err != nil {
		c.mu.Lock()
		delete(c.resyncs, request)
		c.mu.Unlock()
	}
}

func (c *Client) emit(update Update) {
	select {
	case c.updates <- update:
	default:
		c.mu.Lock()
		c.dropped = true
		c.mu.Unlock()
	}
}

func newUpdate() Update {
	return Update{
		Event: nil, Latency: 0, Result: nil, Request: 0, Snapshot: nil, Session: nil, Shutdown: nil,
		Attention: false, PickStart: nil, PickAnswer: nil, PickCancel: nil,
		PreviewStart: nil, PreviewState: nil, PreviewStop: nil, PreviewControl: nil,
	}
}
