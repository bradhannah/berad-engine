package window

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const (
	requestTimeout  = 5 * time.Second
	maxUpdates      = 1024
	callbackQueue   = 256
	screenshotEnv   = "BERAD_SCREENSHOT"
	screenshotFrame = 90
)

// starter is implemented by views that subscribe to more documents once the
// window is running.
type starter interface {
	Start(ctx *frame.Context)
}

// host runs one view as an Ebitengine game: it applies what the orchestrator
// sends, builds input, runs the view and carries out its requests.
type host struct {
	conn       *client.Client
	view       frame.View
	theme      *ui.Theme
	frame      *frame.Frame
	ctx        frame.Context
	reader     *inputReader
	script     *inputScript
	callbacks  chan func()
	outbox     []outgoing
	pending    map[uint64]func(protocol.Result, error)
	screenshot string
	shot       string
	frames     int
	title      string
	started    bool
}

func newHost(conn *client.Client, view frame.View, viewName string, theme *ui.Theme) *host {
	created := &host{
		conn: conn, view: view, theme: theme, frame: frame.NewFrame(time.Now),
		ctx: frame.Context{
			Docs: nil, Requests: nil, Clipboard: nil, Window: conn.Window(), Size: image.Pt(0, 0), Frame: nil,
		},
		reader: newInputReader(), script: nil, callbacks: make(chan func(), callbackQueue), outbox: nil,
		pending:    map[uint64]func(protocol.Result, error){},
		screenshot: strings.ReplaceAll(os.Getenv(screenshotEnv), "{view}", viewName),
		shot:       "", frames: 0, title: "", started: false,
	}
	created.ctx.Docs = conn
	created.ctx.Requests = created
	created.ctx.Clipboard = &systemClipboard{mu: sync.Mutex{}, memory: ""}
	created.ctx.Frame = created.frame

	return created
}

// Update runs once per tick.
func (h *host) Update() error {
	if !h.started {
		h.started = true
		if view, ok := h.view.(starter); ok {
			view.Start(&h.ctx)
		}
	}
	if h.drainUpdates() {
		return ebiten.Termination
	}
	h.runCallbacks()
	var input ui.Input
	if h.script != nil {
		input = h.script.next(h.view, h.conn)
	} else {
		input = h.reader.read(h.view.Focused())
	}
	h.view.Update(&input, &h.ctx)
	h.flush()
	if h.script != nil {
		if shot := h.script.takeScreenshot(); shot != "" {
			h.shot = shot
		}
	}
	if h.frame.Crash {
		panic("simulated window crash (debug key)")
	}
	if h.frame.Quit {
		return ebiten.Termination
	}
	if title := h.view.Title(h.conn); title != h.title {
		h.title = title
		ebiten.SetWindowTitle(title)
	}

	return nil
}

// drainUpdates applies what the orchestrator sent and reports whether the
// window must close.
func (h *host) drainUpdates() bool {
	for range maxUpdates {
		select {
		case update, open := <-h.conn.Updates():
			if !open || update.Shutdown != nil {
				return true
			}
			h.onUpdate(update)
		default:
			return false
		}
	}

	return false
}

func (h *host) onUpdate(update client.Update) {
	if update.Latency > 0 {
		h.frame.AddLatency(update.Latency)
	}
	if update.Result != nil {
		if done, ok := h.pending[update.Request]; ok {
			delete(h.pending, update.Request)
			h.answer(done, *update.Result)
		}

		return
	}
	if update.Attention {
		ebiten.RequestAttention()

		return
	}
	if update.Session != nil && update.Session.Topic == protocol.TopicNotice {
		var notice protocol.Notice
		if err := json.Unmarshal(update.Session.Value, &notice); err == nil {
			h.frame.Notice(notice.Text)
		}

		return
	}
	h.view.Changed(update, &h.ctx)
}

func (h *host) answer(done func(protocol.Result, error), result protocol.Result) {
	if done == nil {
		return
	}
	if !result.OK {
		done(result, fmt.Errorf("%w: %s", client.ErrRejected, result.Reason))

		return
	}
	done(result, nil)
}

func (h *host) runCallbacks() {
	for {
		select {
		case callback := <-h.callbacks:
			callback()
		default:
			return
		}
	}
}

// post queues a callback to run on the window goroutine.
func (h *host) post(callback func()) {
	select {
	case h.callbacks <- callback:
	case <-h.conn.Done():
	}
}

// background runs a blocking request off the frame loop and posts its reply.
func (h *host) background(request func(context.Context) func()) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		reply := request(ctx)
		if reply != nil {
			h.post(reply)
		}
	}()
}

// outgoing is a command waiting to be sent at the end of the frame.
type outgoing struct {
	command protocol.Command
	done    func(protocol.Result, error)
}

// Command queues a command. Views make requests while reading documents,
// which holds the connection's lock, so commands go out after the view's
// Update, in order; each result comes back as an update.
func (h *host) Command(command protocol.Command, done func(protocol.Result, error)) {
	h.outbox = append(h.outbox, outgoing{command: command, done: done})
}

func (h *host) flush() {
	for _, item := range h.outbox {
		request, err := h.conn.Send(item.command)
		if err != nil {
			reply(item.done, protocol.Result{OK: false, Reason: err.Error(), Revision: 0, Created: ids.ULID{}}, err)

			continue
		}
		h.pending[request] = item.done
	}
	h.outbox = nil
}

// Undo asks the orchestrator to undo this window's latest step.
func (h *host) Undo(done func(protocol.Result, error)) {
	h.background(func(ctx context.Context) func() {
		result, err := h.conn.Undo(ctx)

		return func() { reply(done, result, err) }
	})
}

// Redo asks the orchestrator to redo this window's latest undone step.
func (h *host) Redo(done func(protocol.Result, error)) {
	h.background(func(ctx context.Context) func() {
		result, err := h.conn.Redo(ctx)

		return func() { reply(done, result, err) }
	})
}

func reply(done func(protocol.Result, error), result protocol.Result, err error) {
	if done != nil {
		done(result, err)
	}
}

// Subscribe loads a document and keeps it current.
func (h *host) Subscribe(document ids.ULID, done func(error)) {
	h.background(func(ctx context.Context) func() {
		err := h.conn.Subscribe(ctx, document)

		return func() {
			if done != nil {
				done(err)
			}
		}
	})
}

// Open asks for a window.
func (h *host) Open(view string, target ids.ULID, newView bool) {
	h.background(func(ctx context.Context) func() {
		result, err := h.conn.Open(ctx, view, target, newView)
		if err == nil {
			return nil
		}

		return func() { h.frame.Report("")(result, err) }
	})
}

// WhereUsed asks where a document or item is used.
func (h *host) WhereUsed(document ids.ULID, item string, done func([]protocol.Use, error)) {
	h.background(func(ctx context.Context) func() {
		uses, err := h.conn.WhereUsed(ctx, document, item)

		return func() { done(uses, err) }
	})
}

// PickAsk asks which maps could answer a pick.
func (h *host) PickAsk(what string, done func([]protocol.PickMap, error)) {
	h.background(func(ctx context.Context) func() {
		maps, err := h.conn.PickAsk(ctx, what)

		return func() { done(maps, err) }
	})
}

// PickStart starts a pick.
func (h *host) PickStart(start protocol.PickStart, done func(protocol.Result, error)) {
	h.background(func(ctx context.Context) func() {
		result, err := h.conn.PickStart(ctx, start)

		return func() { reply(done, result, err) }
	})
}

// PickAnswer answers a pick.
func (h *host) PickAnswer(answer protocol.PickAnswer) {
	if err := h.conn.PickAnswer(answer); err != nil {
		h.frame.Status(err.Error())
	}
}

// PickCancel ends a pick.
func (h *host) PickCancel(cancel protocol.PickCancel) {
	if err := h.conn.PickCancel(cancel); err != nil {
		h.frame.Status(err.Error())
	}
}

// PreviewStart starts a preview of a zone.
func (h *host) PreviewStart(zone ids.ULID, done func(protocol.Result, error)) {
	h.background(func(ctx context.Context) func() {
		result, err := h.conn.PreviewStart(ctx, zone)

		return func() { reply(done, result, err) }
	})
}

// PreviewState sends the preview's latest state.
func (h *host) PreviewState(state protocol.PreviewState) {
	if err := h.conn.PreviewState(state); err != nil {
		h.frame.Status(err.Error())
	}
}

// PreviewStop ends the preview.
func (h *host) PreviewStop(stop protocol.PreviewStop) {
	if err := h.conn.PreviewStop(stop); err != nil {
		h.frame.Status(err.Error())
	}
}

// PreviewControl asks the window running the preview to watch, pause, resume
// or step its NPCs.
func (h *host) PreviewControl(control protocol.PreviewControl) {
	if err := h.conn.PreviewControl(control); err != nil {
		h.frame.Status(err.Error())
	}
}

// Draw renders the view, the status bar and, once, a screenshot if asked for.
func (h *host) Draw(screen *ebiten.Image) {
	h.view.Draw(screen, h.conn)
	h.frame.DrawStatus(screen, h.theme)
	h.frames++
	if h.screenshot != "" && h.frames == screenshotFrame {
		h.shot = h.screenshot
	}
	if h.shot != "" {
		if err := saveScreenshot(screen, h.shot); err != nil {
			h.frame.Status(err.Error())
		}
		h.shot = ""
	}
}

// Layout uses the window's real size, one screen pixel per pixel.
func (h *host) Layout(outsideWidth, outsideHeight int) (int, int) {
	h.ctx.Size = image.Pt(outsideWidth, outsideHeight)

	return outsideWidth, outsideHeight
}
