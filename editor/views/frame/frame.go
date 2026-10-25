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
	// onTopWidth is the width of the On top toggle in the status bar, and
	// statusGap the space kept between the status bar's parts.
	onTopWidth = 76
	statusGap  = 24
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
	WhereUsed(document ids.ULID, item string, done func([]protocol.Use, error))
	// Publish sets session state every window shares under a topic, such as
	// the time the next preview starts at; the other windows are told.
	Publish(topic string, value any)
	Windows
	Picks
	Previews
	Audio
}

// Windows are the requests that open windows (D13).
type Windows interface {
	// Open opens a window, or brings the one already showing it forward.
	Open(view string, target ids.ULID, newView bool)
	// OpenItem opens a window and shows an item in it, such as a cue in the
	// Audio window, or shows it in the window already open.
	OpenItem(view string, target ids.ULID, item string)
}

// Audio are the requests for the project's audio (D8): its library, the cues
// it can play, and a recording's bytes, a part at a time from an offset.
type Audio interface {
	AudioLibrary(done func(protocol.AudioLibrary, error))
	Recording(recording string, offset int64, done func(protocol.Recording, error))
	// AudioEdit changes the audio library (D11); every window is then told
	// it changed.
	AudioEdit(edit protocol.AudioEdit, done func(protocol.Result, error))
	// AudioAnalysis asks for a recording's waveform and suggestions, and
	// AudioCheck for every recording to be decoded; the orchestrator decodes
	// them, never the window.
	AudioAnalysis(asked protocol.AudioAnalysis, done func(protocol.AudioAnalysis, error))
	AudioCheck(done func(protocol.AudioCheck, error))
}

// Files asks the author to choose a file of their own, outside the project,
// such as a sound to add. The window only passes on its path: the
// orchestrator copies it in (D1). done runs on the window's goroutine, with
// chosen false when the author cancels.
type Files interface {
	ChooseSound(done func(path string, chosen bool))
}

// Sounds plays the project's cues in this window. The orchestrator sends the
// recordings (D1, D8), so a cue plays once they have arrived; a cue that
// cannot play says why in the status bar, once.
type Sounds interface {
	// Prepare fetches what cues need, so they play without waiting later.
	Prepare(cues []string)
	// Play plays a cue once, alongside whatever else is playing.
	Play(cue string)
	// Stop stops everything this window plays, and what waits to play.
	Stop()
	// Playing reports whether a cue is playing, or waiting to.
	Playing(cue string) bool
	// PlayRecording plays one recording on its own, and PlayingRecording
	// reports whether it is playing or waiting to.
	PlayRecording(recording string)
	PlayingRecording(recording string) bool
	// Audition plays part of a recording, nil for all of it, at a
	// calibration gain in dB, again and again when loop is set, as the Audio
	// window plays a region it is editing; AuditionAt reports where it is, in
	// the recording's milliseconds, while it plays.
	Audition(recording string, window *protocol.AudioWindow, calibrationDB float64, loop bool)
	AuditionAt() (int64, bool)
}

// Picks are the requests for pick on map (D13).
type Picks interface {
	PickAsk(what string, done func([]protocol.PickMap, error))
	PickStart(start protocol.PickStart, done func(protocol.Result, error))
	PickAnswer(answer protocol.PickAnswer)
	PickCancel(cancel protocol.PickCancel)
}

// Previews are the requests for a preview (D7, D15). PreviewStart starts one
// for a zone, and the result's Created is the run's ID; PreviewState sends
// its latest state and PreviewStop ends it; PreviewControl asks the window
// running it to watch, pause, resume or step its NPCs.
type Previews interface {
	PreviewStart(zone ids.ULID, done func(protocol.Result, error))
	PreviewState(state protocol.PreviewState)
	PreviewStop(stop protocol.PreviewStop)
	PreviewControl(control protocol.PreviewControl)
}

// Context is what a view's Update receives each frame.
type Context struct {
	Docs      Docs
	Requests  Requests
	Sounds    Sounds
	Files     Files
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
	// a snapshot, a pick message or another window's preview.
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

// Frame is the window state every view shares: status messages, notices, the
// command-to-event speed readout, and whether the window stays on top.
type Frame struct {
	status      string
	statusUntil time.Time
	notice      string
	latencies   []time.Duration
	now         func() time.Time
	// Quit is set by a view to close the window.
	Quit bool
	// Closing asks to close the window, from Cmd+W or the window's close
	// button. A Saver is asked about unsaved changes first; any other view
	// closes.
	Closing bool
	// Crash asks the window to crash on purpose, to show failure handling.
	Crash bool
	// OnTop is the status bar's toggle that keeps the window above every
	// other window, session state for each window.
	OnTop *ui.Toggle
}

// NewFrame returns an empty frame.
func NewFrame(now func() time.Time) *Frame {
	return &Frame{
		status: "", statusUntil: time.Time{}, notice: "", latencies: nil, now: now, Quit: false, Closing: false,
		Crash: false, OnTop: &ui.Toggle{ID: "on-top", Label: "On top", Rect: ui.Zero(), On: false},
	}
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

	return "Ready. Typing waits for Save; everything else saves as you go.", false
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

// layoutStatus places the On top toggle at the right end of the status bar.
func (f *Frame) layoutStatus(size image.Point) {
	top := size.Y - StatusHeight
	f.OnTop.Rect = image.Rect(size.X-ui.Inset-onTopWidth, top, size.X-ui.Inset, size.Y)
}

// UpdateStatus handles a click on the status bar's On top toggle.
func (f *Frame) UpdateStatus(input *ui.Input, size image.Point) {
	f.layoutStatus(size)
	f.OnTop.Update(input)
}

// LocateStatus finds the On top toggle, "ui:on-top", and reports
// "state:on-top" while the window is kept on top, for input scripts.
func (f *Frame) LocateStatus(name string) (image.Point, bool) {
	switch name {
	case "ui:" + f.OnTop.ID:
		return ui.Center(f.OnTop.Rect), !f.OnTop.Rect.Empty()
	case "state:" + f.OnTop.ID:
		return image.Pt(0, 0), f.OnTop.On
	}

	return image.Pt(0, 0), false
}

// DrawStatus draws the status bar: the wireframes' monospaced strip along
// the bottom of the window, with the On top toggle at its right end. The
// speed readout is left out when the window is too narrow for it.
func (f *Frame) DrawStatus(screen *ebiten.Image, theme *ui.Theme) {
	bounds := screen.Bounds()
	f.layoutStatus(bounds.Size())
	bar := image.Rect(0, bounds.Max.Y-StatusHeight, bounds.Max.X, bounds.Max.Y)
	ui.FillRect(screen, bar, ui.ColorPanel())
	ui.Divider(screen, image.Pt(0, bar.Min.Y), image.Pt(bar.Max.X, bar.Min.Y))
	message, notice := f.Message()
	ink := ui.ColorInk()
	if notice {
		ink = ui.ColorSelect()
	}
	textY := bar.Min.Y + (StatusHeight-ui.MonoSize)/ui.Half - 1
	message = theme.FitText(message, f.OnTop.Rect.Min.X-statusGap-ui.Inset)
	theme.DrawText(screen, message, ui.Inset, bar.Min.Y+(StatusHeight-ui.TextSize)/ui.Half-1, ink)
	f.OnTop.Draw(screen, theme)
	speed := "command to event: last " + formatLatency(f.last()) + ", median " + formatLatency(f.median())
	speedLeft := f.OnTop.Rect.Min.X - statusGap - theme.MonoWidth(speed)
	if speedLeft >= ui.Inset+theme.TextWidth(message)+statusGap {
		theme.DrawMono(screen, speed, speedLeft, textY, ui.ColorMuted())
	}
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

// DrawInspector draws the inspector panel's divider.
func DrawInspector(screen *ebiten.Image) {
	panel := Inspector(screen.Bounds().Size())
	ui.Divider(screen, panel.Min, image.Pt(panel.Min.X, panel.Max.Y))
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
		ctx.Frame.Closing = true
	case input.Consume(ui.KeyK, ui.CmdShift()):
		ctx.Frame.Crash = true
	default:
		return false
	}

	return true
}
