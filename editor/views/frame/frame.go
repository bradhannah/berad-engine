// Package frame is what every editor window has in common, kept apart from
// Ebitengine's game loop and from the connection so views can be tested
// without either: the interface a view implements, the requests it can make,
// and the status bar and inspector panel every window shows (D13).
//
// Views never call the orchestrator directly. They make requests during
// Update; replies come back as callbacks run at the start of a later Update,
// on the window's own goroutine, so a view never needs a lock.
package frame

import (
	"image"
	"slices"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Layout sizes, in screen pixels.
const (
	StatusHeight   = 30
	InspectorWidth = 280
	statusDuration = 4 * time.Second
)

// Docs gives read access to the window's subscribed documents. fn runs while
// the copy is locked and must not keep the document.
type Docs interface {
	View(document ids.ULID, fn func(client.Document)) bool
}

// Requests are what a view can ask of the orchestrator. Every reply arrives
// as a callback on the window's goroutine; a nil callback ignores it.
type Requests interface {
	Command(command protocol.Command, done func(protocol.Result, error))
	Undo(done func(protocol.Result, error))
	Redo(done func(protocol.Result, error))
	Subscribe(document ids.ULID, done func(error))
	Open(view string, target ids.ULID, newView bool)
	WhereUsed(document ids.ULID, item string, done func([]protocol.Use, error))
	PickAsk(what string, done func([]protocol.PickMap, error))
	PickStart(start protocol.PickStart, done func(protocol.Result, error))
	PickAnswer(answer protocol.PickAnswer)
	PickCancel(cancel protocol.PickCancel)
}

// Context is what a view's Update receives each frame.
type Context struct {
	Docs      Docs
	Requests  Requests
	Clipboard textedit.Clipboard
	// Window is this window's ID; Size is its size in pixels.
	Window ids.ULID
	Size   image.Point
	// Frame is the shared status bar and window state.
	Frame *Frame
}

// View is one kind of editor window: Map, Project browser, Text.
type View interface {
	// Title is the window title.
	Title(docs Docs) string
	// Update handles a frame of input and makes requests.
	Update(input *ui.Input, ctx *Context)
	// Changed tells the view that the orchestrator sent something: an event,
	// a snapshot or a pick message.
	Changed(update client.Update, ctx *Context)
	// Draw renders the view.
	Draw(screen *ebiten.Image, docs Docs)
	// Focused returns the text field with keyboard focus, for the input
	// method, or nil.
	Focused() *ui.TextField
	// Locate finds a named part of the view for input scripts: "ui:<id>" for
	// a widget, or a view-specific name such as "cell:3,4".
	Locate(name string, docs Docs) (image.Point, bool)
}

// Frame is the window state every view shares: status messages, notices and
// the command-to-event speed readout.
type Frame struct {
	status      string
	statusUntil time.Time
	notice      string
	latencies   []time.Duration
	now         func() time.Time
	// Quit is set by a view to close the window.
	Quit bool
	// Crash asks the window to crash on purpose, to show failure handling.
	Crash bool
}

// NewFrame returns an empty frame.
func NewFrame(now func() time.Time) *Frame {
	return &Frame{status: "", statusUntil: time.Time{}, notice: "", latencies: nil, now: now, Quit: false, Crash: false}
}

// Status shows a message in the status bar for a few seconds.
func (f *Frame) Status(message string) {
	f.status = message
	f.statusUntil = f.now().Add(statusDuration)
}

// Notice shows a lasting notice, such as another window crashing.
func (f *Frame) Notice(message string) { f.notice = message }

// Message returns what the status bar shows, and whether it is a notice.
func (f *Frame) Message() (string, bool) {
	if f.status != "" && f.now().Before(f.statusUntil) {
		return f.status, false
	}
	if f.notice != "" {
		return "Notice: " + f.notice, true
	}

	return "Ready. Every change is saved automatically.", false
}

// Report shows the outcome of a request: its reason when refused.
func (f *Frame) Report(success string) func(protocol.Result, error) {
	return func(result protocol.Result, err error) {
		switch {
		case err != nil && result.Reason != "":
			f.Status(result.Reason)
		case err != nil:
			f.Status(err.Error())
		case success != "":
			f.Status(success)
		}
	}
}

// AddLatency records a command-to-event time.
func (f *Frame) AddLatency(latency time.Duration) {
	const samples = 120
	f.latencies = append(f.latencies, latency)
	if len(f.latencies) > samples {
		f.latencies = f.latencies[len(f.latencies)-samples:]
	}
}

// Latencies returns the recent command-to-event times.
func (f *Frame) Latencies() []time.Duration { return f.latencies }

// Content returns the area left for a view's own content: everything above
// the status bar and left of the inspector.
func Content(size image.Point) image.Rectangle {
	return image.Rect(0, 0, size.X-InspectorWidth, size.Y-StatusHeight)
}

// Inspector returns the inspector panel's area (D13: embedded on the right of
// every window).
func Inspector(size image.Point) image.Rectangle {
	return image.Rect(size.X-InspectorWidth, 0, size.X, size.Y-StatusHeight)
}

// DrawStatus draws the status bar.
func (f *Frame) DrawStatus(screen *ebiten.Image, theme *ui.Theme) {
	bounds := screen.Bounds()
	bar := image.Rect(0, bounds.Max.Y-StatusHeight, bounds.Max.X, bounds.Max.Y)
	ui.FillRect(screen, bar, ui.ColorPanel())
	ui.FillRect(screen, image.Rect(0, bar.Min.Y, bar.Max.X, bar.Min.Y+ui.Hairline), ui.ColorRule())
	message, notice := f.Message()
	ink := ui.ColorMuted()
	if notice {
		ink = ui.ColorSelect()
	}
	theme.DrawText(screen, message, ui.Inset, bar.Min.Y+(StatusHeight-ui.TextSize)/2-1, ink)
	speed := "command to event: last " + formatLatency(f.last()) + ", median " + formatLatency(f.median())
	textY := bar.Min.Y + (StatusHeight-ui.TextSize)/ui.Half - 1
	theme.DrawText(screen, speed, bar.Max.X-ui.Inset-theme.TextWidth(speed), textY, ui.ColorSubdued())
}

func (f *Frame) last() time.Duration {
	if len(f.latencies) == 0 {
		return 0
	}

	return f.latencies[len(f.latencies)-1]
}

func (f *Frame) median() time.Duration {
	if len(f.latencies) == 0 {
		return 0
	}
	sorted := slices.Clone(f.latencies)
	slices.Sort(sorted)

	return sorted[len(sorted)/2]
}

func formatLatency(latency time.Duration) string {
	if latency == 0 {
		return "-"
	}

	return strconv.FormatFloat(float64(latency)/float64(time.Microsecond), 'f', 0, 64) + " µs"
}

// DrawInspector draws the inspector panel's background and divider.
func DrawInspector(screen *ebiten.Image) {
	panel := Inspector(screen.Bounds().Size())
	ui.FillRect(screen, image.Rect(panel.Min.X, panel.Min.Y, panel.Min.X+ui.Hairline, panel.Max.Y), ui.ColorRule())
}

// CommonKeys handles the shortcuts every window shares: undo, redo, close and
// the test crash. It returns true if it used a key.
func CommonKeys(input *ui.Input, ctx *Context) bool {
	switch {
	case input.Consume(ui.KeyZ, ui.Cmd()):
		ctx.Requests.Undo(ctx.Frame.Report("Undone"))
	case input.Consume(ui.KeyZ, ui.CmdShift()):
		ctx.Requests.Redo(ctx.Frame.Report("Redone"))
	case input.Consume(ui.KeyW, ui.Cmd()):
		ctx.Frame.Quit = true
	case input.Consume(ui.KeyK, ui.CmdShift()):
		ctx.Frame.Crash = true
	default:
		return false
	}

	return true
}
