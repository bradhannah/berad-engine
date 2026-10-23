// Package client is the window side of the protocol: it connects to the
// orchestrator, keeps local copies of subscribed documents up to date from
// events, and sends commands.
//
// Updates are strict: a local document changes only when the orchestrator's
// event arrives, never optimistically.
package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const updateQueue = 4096

// ErrRefused reports a handshake the orchestrator refused.
var ErrRefused = errors.New("orchestrator refused the connection")

// ErrClosed reports use of a closed client.
var ErrClosed = errors.New("client closed")

// ErrRejected reports a request the orchestrator answered with a failure.
var ErrRejected = errors.New("request rejected")

// Document is a window's copy of a subscribed document.
type Document struct {
	ID       ids.ULID
	Name     string
	Format   string
	Revision uint64
	Model    model.Document
}

// Update tells the window something changed. Exactly one field is set.
type Update struct {
	// Event is an applied edit, already reflected in the local document.
	Event *protocol.Event
	// Latency is the command-to-event time when Event answers this window's
	// own command; zero otherwise.
	Latency time.Duration
	// Result answers a command sent with Send.
	Result *protocol.Result
	// Request is the request number Result answers.
	Request uint64
	// Snapshot reports a document (re)loaded from a snapshot.
	Snapshot *ids.ULID
	// Session is a session-state update, such as a notice.
	Session *protocol.Session
	// Shutdown asks the window to clean up and close.
	Shutdown *protocol.Shutdown
	// Attention asks the window to draw attention to itself.
	Attention bool
	// PickStart asks this Map window to answer a pick.
	PickStart *protocol.PickStart
	// PickAnswer answers a pick this window asked for.
	PickAnswer *protocol.PickAnswer
	// PickCancel ends a pick this window asked for or was answering.
	PickCancel *protocol.PickCancel
	// PreviewStart reports a preview another window started in this
	// window's zone; PreviewState is its latest state and PreviewStop its
	// end.
	PreviewStart *protocol.PreviewStart
	PreviewState *protocol.PreviewState
	PreviewStop  *protocol.PreviewStop
}

// Client is one window's connection.
type Client struct {
	conn     *protocol.Conn
	registry *plugin.Registry
	window   ids.ULID
	clock    func() time.Time
	next     atomic.Uint64

	mu       sync.Mutex
	pending  map[uint64]chan protocol.Message
	sentAt   map[uint64]time.Time
	docs     map[ids.ULID]*Document
	resyncs  map[uint64]bool
	dropped  bool
	updates  chan Update
	done     chan struct{}
	closeErr error
	once     sync.Once
}

// Dial connects and completes the handshake.
func Dial(ctx context.Context, socket string, hello protocol.Hello, registry *plugin.Registry) (*Client, error) {
	var dialer net.Dialer
	connection, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", socket, err)
	}
	conn := protocol.NewConn(connection, protocol.JSONCodec{})
	if hello.Protocol == 0 {
		hello.Protocol = protocol.Version
	}
	welcome, err := handshake(conn, hello)
	if err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	client := &Client{
		conn:     conn,
		registry: registry,
		window:   welcome.Window,
		clock:    time.Now,
		next:     atomic.Uint64{},
		mu:       sync.Mutex{},
		pending:  map[uint64]chan protocol.Message{},
		sentAt:   map[uint64]time.Time{},
		docs:     map[ids.ULID]*Document{},
		resyncs:  map[uint64]bool{},
		dropped:  false,
		updates:  make(chan Update, updateQueue),
		done:     make(chan struct{}),
		closeErr: nil,
		once:     sync.Once{},
	}
	go client.readLoop()

	return client, nil
}

func handshake(conn *protocol.Conn, hello protocol.Hello) (protocol.Welcome, error) {
	message, err := protocol.NewMessage(protocol.KindHello, 0, hello)
	if err != nil {
		return protocol.Welcome{}, fmt.Errorf("hello: %w", err)
	}
	if err := conn.Send(message); err != nil {
		return protocol.Welcome{}, fmt.Errorf("hello: %w", err)
	}
	reply, err := conn.Receive()
	if err != nil {
		return protocol.Welcome{}, fmt.Errorf("%w: %w", ErrRefused, err)
	}
	if reply.Kind == protocol.KindShutdown {
		shutdown, decodeErr := protocol.DecodePayload[protocol.Shutdown](reply)
		if decodeErr != nil {
			return protocol.Welcome{}, fmt.Errorf("%w: %w", ErrRefused, decodeErr)
		}

		return protocol.Welcome{}, fmt.Errorf("%w: %s", ErrRefused, shutdown.Reason)
	}

	return protocol.DecodePayload[protocol.Welcome](reply)
}

// Window returns the window's ID.
func (c *Client) Window() ids.ULID { return c.window }

// Updates delivers changes for the window to react to. If the window stops
// reading and the queue fills, further updates are dropped (see Dropped); the
// documents themselves stay current.
func (c *Client) Updates() <-chan Update { return c.updates }

// Done is closed when the connection ends.
func (c *Client) Done() <-chan struct{} { return c.done }

// Dropped reports whether any update was dropped because the queue was full.
func (c *Client) Dropped() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.dropped
}

// Close ends the connection.
func (c *Client) Close() error {
	err := c.conn.Close()
	<-c.done
	if err != nil {
		return fmt.Errorf("close client: %w", err)
	}

	return nil
}

// View calls fn with the current local copy of a document while holding the
// client's lock. fn must not keep the document or call the client.
func (c *Client) View(document ids.ULID, fn func(Document)) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	local, ok := c.docs[document]
	if ok {
		fn(*local)
	}

	return ok
}

// Subscribe loads a document and keeps it current from then on.
func (c *Client) Subscribe(ctx context.Context, document ids.ULID) error {
	reply, err := c.request(ctx, protocol.KindSubscribe, protocol.Subscribe{Document: document})
	if err != nil {
		return err
	}
	if reply.Kind == protocol.KindResult {
		return rejected(reply)
	}

	return nil
}

// Command sends a command and waits for its result.
func (c *Client) Command(ctx context.Context, command protocol.Command) (protocol.Result, error) {
	reply, err := c.request(ctx, protocol.KindCommand, command)
	if err != nil {
		return protocol.Result{}, err
	}

	return resultOf(reply)
}

// Send sends a command without waiting. Its result arrives as an Update.
func (c *Client) Send(command protocol.Command) (uint64, error) {
	request := c.next.Add(1)
	message, err := protocol.NewMessage(protocol.KindCommand, request, command)
	if err != nil {
		return 0, fmt.Errorf("command: %w", err)
	}
	c.mu.Lock()
	c.sentAt[request] = c.clock()
	c.mu.Unlock()
	if err := c.conn.Send(message); err != nil {
		return 0, fmt.Errorf("command: %w", err)
	}

	return request, nil
}

// Undo reverts this window's latest undo step.
func (c *Client) Undo(ctx context.Context) (protocol.Result, error) {
	reply, err := c.request(ctx, protocol.KindUndo, nil)
	if err != nil {
		return protocol.Result{}, err
	}

	return resultOf(reply)
}

// Redo re-applies this window's latest undone step.
func (c *Client) Redo(ctx context.Context) (protocol.Result, error) {
	reply, err := c.request(ctx, protocol.KindRedo, nil)
	if err != nil {
		return protocol.Result{}, err
	}

	return resultOf(reply)
}

// Open asks the orchestrator to open a window, or to draw attention to the
// window already showing that view and target unless newView is set.
func (c *Client) Open(ctx context.Context, view string, target ids.ULID, newView bool) (protocol.Result, error) {
	reply, err := c.request(ctx, protocol.KindOpen, protocol.Open{View: view, Target: target, NewView: newView})
	if err != nil {
		return protocol.Result{}, err
	}

	return resultOf(reply)
}

// WhereUsed asks which documents refer to a document, or to one item in it.
func (c *Client) WhereUsed(ctx context.Context, document ids.ULID, item string) ([]protocol.Use, error) {
	reply, err := c.request(ctx, protocol.KindQuery, protocol.Query{
		Question: protocol.QuestionWhereUsed, Document: document, Item: item,
	})
	if err != nil {
		return nil, err
	}
	if reply.Kind == protocol.KindResult {
		return nil, rejected(reply)
	}
	answer, err := protocol.DecodePayload[protocol.Answer](reply)
	if err != nil {
		return nil, err
	}

	return answer.Uses, nil
}

// PickAsk asks which open Map windows a pick could use.
func (c *Client) PickAsk(ctx context.Context, what string) ([]protocol.PickMap, error) {
	reply, err := c.request(ctx, protocol.KindPickAsk, protocol.PickAsk{What: what, Maps: nil})
	if err != nil {
		return nil, err
	}
	if reply.Kind == protocol.KindResult {
		return nil, rejected(reply)
	}
	ask, err := protocol.DecodePayload[protocol.PickAsk](reply)
	if err != nil {
		return nil, err
	}

	return ask.Maps, nil
}

// PickStart starts a pick in a Map window, or on a zone in a new one.
func (c *Client) PickStart(ctx context.Context, start protocol.PickStart) (protocol.Result, error) {
	reply, err := c.request(ctx, protocol.KindPickStart, start)
	if err != nil {
		return protocol.Result{}, err
	}

	return resultOf(reply)
}

// PickAnswer answers the pick this Map window is showing.
func (c *Client) PickAnswer(answer protocol.PickAnswer) error {
	return c.notify(protocol.KindPickAnswer, answer)
}

// PickCancel ends a pick from either side.
func (c *Client) PickCancel(cancel protocol.PickCancel) error {
	return c.notify(protocol.KindPickCancel, cancel)
}

// PreviewStart starts a preview of a zone (D15). The result's Created is the
// run's ID.
func (c *Client) PreviewStart(ctx context.Context, zone ids.ULID) (protocol.Result, error) {
	reply, err := c.request(ctx, protocol.KindPreviewStart, protocol.PreviewStart{
		Zone: zone, Run: ids.ULID{}, Window: ids.ULID{},
	})
	if err != nil {
		return protocol.Result{}, err
	}

	return resultOf(reply)
}

// PreviewState sends the latest state of this window's preview (D7).
func (c *Client) PreviewState(state protocol.PreviewState) error {
	return c.notify(protocol.KindPreviewState, state)
}

// PreviewStop ends this window's preview.
func (c *Client) PreviewStop(stop protocol.PreviewStop) error {
	return c.notify(protocol.KindPreviewStop, stop)
}

// notify sends a message that expects no reply.
func (c *Client) notify(kind protocol.Kind, payload any) error {
	message, err := protocol.NewMessage(kind, 0, payload)
	if err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}
	if err := c.conn.Send(message); err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}

	return nil
}

// Publish shares a session-state value with the other windows.
func (c *Client) Publish(topic string, value []byte) error {
	return c.notify(protocol.KindSession, protocol.Session{Topic: topic, Value: value})
}

func (c *Client) request(ctx context.Context, kind protocol.Kind, payload any) (protocol.Message, error) {
	request := c.next.Add(1)
	message, err := protocol.NewMessage(kind, request, payload)
	if err != nil {
		return protocol.Message{}, fmt.Errorf("%s: %w", kind, err)
	}
	reply := make(chan protocol.Message, 1)
	c.mu.Lock()
	c.pending[request] = reply
	c.sentAt[request] = c.clock()
	c.mu.Unlock()
	if err := c.conn.Send(message); err != nil {
		c.forget(request)

		return protocol.Message{}, fmt.Errorf("%s: %w", kind, err)
	}
	select {
	case answer := <-reply:
		return answer, nil
	case <-ctx.Done():
		c.forget(request)

		return protocol.Message{}, fmt.Errorf("%s: %w", kind, ctx.Err())
	case <-c.done:
		return protocol.Message{}, ErrClosed
	}
}

func (c *Client) forget(request uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pending, request)
	delete(c.sentAt, request)
}

func resultOf(reply protocol.Message) (protocol.Result, error) {
	result, err := protocol.DecodePayload[protocol.Result](reply)
	if err != nil {
		return protocol.Result{}, err
	}
	if !result.OK {
		return result, fmt.Errorf("%w: %s", ErrRejected, result.Reason)
	}

	return result, nil
}

func rejected(reply protocol.Message) error {
	_, err := resultOf(reply)
	if err == nil {
		return fmt.Errorf("%w: unexpected result", ErrRejected)
	}

	return err
}
