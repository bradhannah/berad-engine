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
	"github.com/bradhannah/berad-engine/editor/sound"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const (
	requestTimeout   = 5 * time.Second
	audioEditTimeout = 2 * time.Minute
	maxUpdates       = 1024
	callbackQueue    = 256
	screenshotEnv    = "BERAD_SCREENSHOT"
	screenshotFrame  = 90
)

// starter is implemented by views that subscribe to more documents once the
// window is running.
type starter interface {
	Start(ctx *frame.Context)
}

// host runs one view as an Ebitengine game: it applies what the orchestrator
// sends, builds input, runs the view and carries out its requests.
type host struct {
	conn   *client.Client
	view   frame.View
	theme  *ui.Theme
	frame  *frame.Frame
	ctx    frame.Context
	reader *inputReader
	script *inputScript
	// sounds plays the project's cues in this window.
	sounds     *sound.Player
	callbacks  chan func()
	outbox     []outgoing
	pending    map[uint64]func(protocol.Result, error)
	screenshot string
	shot       string
	frames     int
	title      string
	started    bool
	// quitDialog asks whether the author is done, while quitAsking waits
	// for the orchestrator to say what quitting would close; quitSaving is
	// set while the window saves before closing for a quit. reported is the
	// unsaved changes last told to the orchestrator.
	quitDialog *ui.Dialog
	quitAsking bool
	quitSaving bool
	reported   protocol.Unsaved
	// floating is whether the window was last asked to stay on top, and
	// float asks; tests replace it.
	floating bool
	float    func(bool)
}

func newHost(conn *client.Client, view frame.View, viewName string, theme *ui.Theme) *host {
	created := &host{
		conn: conn, view: view, theme: theme, frame: frame.NewFrame(time.Now),
		ctx: frame.Context{
			Docs: nil, Requests: nil, Sounds: nil, Soundscape: nil, Files: nil, Clipboard: nil,
			Window: conn.Window(), Size: image.Pt(0, 0), Frame: nil,
		},
		reader: newInputReader(), script: nil, sounds: nil, callbacks: make(chan func(), callbackQueue),
		outbox:     nil,
		pending:    map[uint64]func(protocol.Result, error){},
		screenshot: strings.ReplaceAll(os.Getenv(screenshotEnv), "{view}", viewName),
		shot:       "", frames: 0, title: "", started: false,
		quitDialog: nil, quitAsking: false, quitSaving: false, reported: protocol.Unsaved{Count: 0, Name: ""},
		floating: false, float: ebiten.SetWindowFloating,
	}
	created.ctx.Docs = conn
	created.ctx.Requests = created
	created.sounds = sound.New(created, created.frame.Status)
	created.ctx.Sounds, created.ctx.Soundscape = created.sounds, created.sounds
	created.ctx.Files = nativeFiles{post: created.post}
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
	h.sounds.Update()
	if h.quitSaving && h.savedForQuit() {
		return ebiten.Termination
	}
	var input ui.Input
	if h.script != nil {
		input = h.script.next(h, h.conn)
	} else {
		input = h.reader.read(h.view.Focused())
	}
	h.frame.UpdateStatus(&input, h.ctx.Size)
	h.applyOnTop()
	h.windowKeys(&input)
	h.updateQuitDialog(&input)
	h.view.Update(&input, &h.ctx)
	h.close()
	h.reportUnsaved()
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

// applyOnTop keeps the window above every other window while the status
// bar's On top toggle is on.
func (h *host) applyOnTop() {
	if h.frame.OnTop.On == h.floating {
		return
	}
	h.floating = h.frame.OnTop.On
	h.float(h.floating)
}

// save saves a Saver's typed changes for Cmd+S, or says there is nothing
// waiting to be saved.
func (h *host) save() {
	if saver, ok := h.view.(frame.Saver); ok && saver.Unsaved(h.conn) > 0 {
		saver.Save(&h.ctx)

		return
	}
	h.frame.Status("Nothing to save: every other change saves as you make it")
}

// close carries out a request to close: a Saver asks about unsaved changes
// first, and any other view closes.
func (h *host) close() {
	if !h.frame.Closing {
		return
	}
	h.frame.Closing = false
	if saver, ok := h.view.(frame.Saver); ok {
		saver.Leave(&h.ctx, func() { h.frame.Quit = true })

		return
	}
	h.frame.Quit = true
}

// drainUpdates applies what the orchestrator sent and reports whether the
// window must close. A shutdown for a quit with Save first saves the
// window's unsaved changes, and the window closes once they are saved.
func (h *host) drainUpdates() bool {
	for range maxUpdates {
		select {
		case update, open := <-h.conn.Updates():
			if !open {
				return true
			}
			if update.Shutdown != nil {
				if !update.Shutdown.Save || !h.saveForQuit() {
					return true
				}

				continue
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
		if update.Item == "" {
			return
		}
	}
	if update.AudioChanged != nil {
		h.sounds.Changed()
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
			reply(item.done, protocol.Result{OK: false, Reason: err.Error(), Revision: 0, Created: ids.ULID{}, Item: ""}, err)

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

// OpenItem asks for a window showing an item.
func (h *host) OpenItem(view string, target ids.ULID, item string) {
	h.background(func(ctx context.Context) func() {
		result, err := h.conn.OpenItem(ctx, view, target, item)
		if err == nil {
			return nil
		}

		return func() { h.frame.Report("")(result, err) }
	})
}

// AudioEdit asks for a change to the audio library. Copying a large file in
// can take a while, so it may wait longer than other requests.
func (h *host) AudioEdit(edit protocol.AudioEdit, done func(protocol.Result, error)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), audioEditTimeout)
		defer cancel()
		result, err := h.conn.AudioEdit(ctx, edit)
		h.post(func() { reply(done, result, err) })
	}()
}

// AudioAnalysis asks for a recording's waveform and suggestions. Decoding a
// long recording can take a while, so it may wait longer than other requests.
func (h *host) AudioAnalysis(asked protocol.AudioAnalysis, done func(protocol.AudioAnalysis, error)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), audioEditTimeout)
		defer cancel()
		answer, err := h.conn.AudioAnalysis(ctx, asked)
		h.post(func() { done(answer, err) })
	}()
}

// AudioCheck asks for every recording to be decoded.
func (h *host) AudioCheck(done func(protocol.AudioCheck, error)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), audioEditTimeout)
		defer cancel()
		answer, err := h.conn.AudioCheck(ctx)
		h.post(func() { done(answer, err) })
	}()
}

// Publish sets session state every window shares.
func (h *host) Publish(topic string, value any) {
	encoded, err := json.Marshal(value)
	if err == nil {
		err = h.conn.Publish(topic, encoded)
	}
	if err != nil {
		h.frame.Status(err.Error())
	}
}

// WhereUsed asks where a document or item is used.
func (h *host) WhereUsed(document ids.ULID, item string, done func([]protocol.Use, error)) {
	h.background(func(ctx context.Context) func() {
		uses, err := h.conn.WhereUsed(ctx, document, item)

		return func() { done(uses, err) }
	})
}

// AudioLibrary asks for the project's audio library.
func (h *host) AudioLibrary(done func(protocol.AudioLibrary, error)) {
	h.background(func(ctx context.Context) func() {
		library, err := h.conn.AudioLibrary(ctx)

		return func() { done(library, err) }
	})
}

// Recording asks for part of a recording's bytes.
func (h *host) Recording(recording string, offset int64, done func(protocol.Recording, error)) {
	h.background(func(ctx context.Context) func() {
		part, err := h.conn.Recording(ctx, recording, offset)

		return func() { done(part, err) }
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
	if h.quitDialog != nil {
		h.quitDialog.Draw(screen, h.theme)
	}
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
