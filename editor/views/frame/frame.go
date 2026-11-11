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
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/sound"
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
	// inspectorToggleWidth is the width of the Inspector toggle beside On
	// top, and FoldedWidth the strip a folded inspector leaves at the right.
	inspectorToggleWidth = 88
	FoldedWidth          = 24
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
	// Publish sets session state every window shares under a topic, such as
	// the time the next preview starts at; the other windows are told.
	Publish(topic string, value any)
	Questions
	Windows
	Picks
	Previews
	Audio
}

// Questions are the requests that need the whole project.
type Questions interface {
	// WhereUsed asks which documents refer to a document, or to one item in
	// it.
	WhereUsed(document ids.ULID, item string, done func([]protocol.Use, error))
	// Runners asks which NPCs run a workflow, as their behaviour or
	// through Calls.
	Runners(workflow ids.ULID, done func([]protocol.Use, error))
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
	// AudioAnalysis asks for a recording's waveform and suggestions,
	// AudioCheck for every recording to be decoded and AudioLoudness for how
	// loud every recording measures; the orchestrator decodes them, never the
	// window.
	AudioAnalysis(asked protocol.AudioAnalysis, done func(protocol.AudioAnalysis, error))
	AudioCheck(done func(protocol.AudioCheck, error))
	AudioLoudness(done func(protocol.AudioLoudness, error))
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
	// Play plays a cue once, alongside whatever else is playing; PlayAt plays
	// it once at a step of package loudness instead of its own, as a Play
	// sound asking for one moment louder or quieter does.
	Play(cue string)
	PlayAt(cue, step string)
	// Stop stops everything this window plays but its soundscape, and what
	// waits to play.
	Stop()
	// Playing reports whether a cue is playing, or waiting to.
	Playing(cue string) bool
	// PlayRecording plays one recording on its own, and PlayingRecording
	// reports whether it is playing or waiting to.
	PlayRecording(recording string)
	PlayingRecording(recording string) bool
	// Audition plays part of a recording, nil for all of it, at a gain in
	// dB, again and again when loop is set, as the Audio window plays a region
	// it is editing: as recorded, evened out, or as a cue plays it, louder
	// than a level alone reaches. AuditionAt reports where it is, in the
	// recording's milliseconds, while it plays.
	Audition(recording string, window *protocol.AudioWindow, calibrationDB float64, loop bool)
	AuditionAt() (int64, bool)
}

// Soundscape is a window's soundscape: layers of sound that play together,
// each from its start, and how loud each category plays while its author
// listens. Nothing of it is saved.
type Soundscape interface {
	// PlayLayer plays a layer from its start, replacing what its slot played;
	// SetLayer changes a playing layer's gain and whether it plays again;
	// StopLayer stops one; LayerPlaying reports whether one plays or waits
	// to.
	PlayLayer(layer sound.Layer)
	SetLayer(layer sound.Layer)
	StopLayer(slot string)
	LayerPlaying(slot string) bool
	// SetMonitor sets how loud each category plays.
	SetMonitor(monitor sound.Monitor)
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
	Docs       Docs
	Requests   Requests
	Sounds     Sounds
	Soundscape Soundscape
	Files      Files
	Clipboard  textedit.Clipboard
	// Window is this window's ID; Size is its size in pixels.
	Window ids.ULID
	Size   image.Point
	// Frame is the shared status bar and window state.
	Frame *Frame
	// InspectorFolded is set while the inspector is folded to its strip.
	// Size is then wider than the window by all but the strip, so an
	// inspector at the right edge falls past it; a view that places its
	// inspector anywhere else puts it at the right edge while this is set.
	InspectorFolded bool
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
	// Crash asks the window to crash on purpose, to test failure handling.
	// Only the crash key sets it, and the key works only while CrashKey is
	// set, which the window does for tests alone: an author's stray
	// Cmd+Shift+K must never close a tab.
	Crash    bool
	CrashKey bool
	// OnTop is the status bar's toggle that keeps the window above every
	// other window, session state for each window.
	OnTop *ui.Toggle
	// ShowInspector is the status bar's toggle that shows the inspector or
	// folds it to a strip at the right edge, session state the editor
	// window shares.
	ShowInspector *ui.Toggle
	// history is every message the window was given, and the refusal shown
	// until it is dismissed (increment 39).
	history historyState
}

// NewFrame returns an empty frame.
func NewFrame(now func() time.Time) *Frame {
	return &Frame{
		status: "", statusUntil: time.Time{}, notice: "", latencies: nil, now: now, Quit: false, Closing: false,
		Crash: false, CrashKey: false,
		OnTop:         &ui.Toggle{ID: "on-top", Label: "On top", Rect: ui.Zero(), On: false},
		ShowInspector: &ui.Toggle{ID: "inspector", Label: "Inspector", Rect: ui.Zero(), On: true},
		history:       newHistoryState(),
	}
}

// Status shows a message in the status bar for a few seconds.
func (f *Frame) Status(message string) {
	if message != f.status || !f.now().Before(f.statusUntil) {
		f.keep(message, false)
	}
	f.status = message
	f.statusUntil = f.now().Add(statusDuration)
}

// Notice shows a lasting notice, such as another window crashing.
func (f *Frame) Notice(message string) {
	if message != "" && message != f.notice {
		f.keep("Notice: "+message, false)
	}
	f.notice = message
}

// Kinds of message the status bar shows.
const (
	shownReady = iota
	shownStatus
	shownRefusal
	shownNotice
)

// shown returns what the status bar shows and what kind it is: a fresh
// message, then a refusal until it is dismissed, then a notice.
func (f *Frame) shown() (string, int) {
	switch {
	case f.status != "" && f.now().Before(f.statusUntil):
		return f.status, shownStatus
	case f.history.refusal != "":
		return "Refused: " + f.history.refusal, shownRefusal
	case f.notice != "":
		return "Notice: " + f.notice, shownNotice
	}

	return "Ready. What goes in a field, typed or chosen, waits for Save; everything else saves as you go.", shownReady
}

// Message returns what the status bar shows, and whether it is a notice.
func (f *Frame) Message() (string, bool) {
	text, kind := f.shown()

	return text, kind == shownNotice
}

// Report shows the outcome of a request: its reason when refused.
func (f *Frame) Report(success string) func(protocol.Result, error) {
	return func(result protocol.Result, err error) {
		switch {
		case err != nil && result.Reason != "":
			f.Refuse(result.Reason)
		case err != nil:
			f.Refuse(err.Error())
		case success != "":
			f.Status(success)
		}
	}
}

// ReportStep shows the outcome of an undo or redo: its reason when refused,
// and otherwise what it did and the sentence of the change it took back or
// put back, as "Undid: Painted Ground floor" (increment 39).
func (f *Frame) ReportStep(did string) func(protocol.Result, error) {
	return func(result protocol.Result, err error) {
		switch {
		case err != nil && (result.Reason == protocol.NothingToUndo || result.Reason == protocol.NothingToRedo):
			f.Status(strings.ToUpper(result.Reason[:1]) + result.Reason[1:])
		case err != nil:
			f.Report("")(result, err)
		case result.Said != "":
			f.Status(did + ": " + result.Said)
		default:
			f.Status(did)
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

// AboveStatus is a window's area above its status bar, which a menu opened
// over the window stays inside.
func AboveStatus(size image.Point) image.Rectangle {
	return image.Rect(0, 0, size.X, size.Y-StatusHeight)
}

// Inspector returns the inspector panel's area (D13: embedded on the right of
// every window).
func Inspector(size image.Point) image.Rectangle {
	return image.Rect(size.X-InspectorWidth, 0, size.X, size.Y-StatusHeight)
}

// layoutStatus places the On top toggle at the right end of the status bar,
// and the Inspector toggle before it.
func (f *Frame) layoutStatus(size image.Point) {
	top := size.Y - StatusHeight
	f.OnTop.Rect = image.Rect(size.X-ui.Inset-onTopWidth, top, size.X-ui.Inset, size.Y)
	right := f.OnTop.Rect.Min.X - statusGap/ui.Half
	f.ShowInspector.Rect = image.Rect(right-inspectorToggleWidth, top, right, size.Y)
	f.layoutHistory(size)
}

// UpdateStatus handles a click on the status bar's toggles.
func (f *Frame) UpdateStatus(input *ui.Input, size image.Point) {
	f.layoutStatus(size)
	f.OnTop.Update(input)
	f.ShowInspector.Update(input)
	f.updateHistoryButtons(input)
}

// LocateStatus finds the status bar's toggles, "ui:on-top" and
// "ui:inspector", and reports "state:on-top" while the window is kept on
// top and "state:inspector" while the inspector shows, for input scripts.
func (f *Frame) LocateStatus(name string) (image.Point, bool) {
	if point, found, handled := f.locateHistory(name); handled {
		return point, found
	}
	for _, toggle := range []*ui.Toggle{f.OnTop, f.ShowInspector} {
		switch name {
		case "ui:" + toggle.ID:
			return ui.Center(toggle.Rect), !toggle.Rect.Empty()
		case "state:" + toggle.ID:
			return image.Pt(0, 0), toggle.On
		}
	}

	return image.Pt(0, 0), false
}

// Folded is the strip a folded inspector leaves at the right edge of a
// window of a size, above the status bar.
func Folded(size image.Point) image.Rectangle {
	return image.Rect(size.X-FoldedWidth, 0, size.X, size.Y-StatusHeight)
}

// DrawFolded draws a folded inspector's strip: an arrow to open it, and its
// name down the strip.
func DrawFolded(screen *ebiten.Image, theme *ui.Theme, hover bool) {
	strip := Folded(screen.Bounds().Size())
	fill := ui.ColorDesk()
	if hover {
		fill = ui.ColorSelectWash()
	}
	ui.FillRect(screen, strip, fill)
	ui.Divider(screen, strip.Min, image.Pt(strip.Min.X, strip.Max.Y))
	middle := strip.Min.X + FoldedWidth/ui.Half
	theme.DrawText(screen, "‹", middle-theme.TextWidth("‹")/ui.Half, strip.Min.Y+ui.Inset, ui.ColorInk())
	top := strip.Min.Y + ui.Inset + ui.LineHeight + ui.Gap
	for _, letter := range "INSPECTOR" {
		shown := string(letter)
		theme.DrawLabel(screen, shown, middle-theme.TextWidth(shown)/ui.Half, top, ui.ColorMuted())
		top += ui.LineHeight - ui.Gap/ui.Half
	}
}

// DrawStatus draws the status bar: the wireframes' monospaced strip along
// the bottom of the window, with the Inspector and On top toggles at its
// right end. The
// speed readout is left out when the window is too narrow for it.
func (f *Frame) DrawStatus(screen *ebiten.Image, theme *ui.Theme) {
	bounds := screen.Bounds()
	f.layoutStatus(bounds.Size())
	bar := image.Rect(0, bounds.Max.Y-StatusHeight, bounds.Max.X, bounds.Max.Y)
	ui.FillRect(screen, bar, ui.ColorDesk())
	ui.Divider(screen, image.Pt(0, bar.Min.Y), image.Pt(bar.Max.X, bar.Min.Y))
	message, kind := f.shown()
	ink := ui.ColorInk()
	switch kind {
	case shownNotice:
		ink = ui.ColorSelect()
	case shownRefusal:
		ink = ui.ColorWarn()
	}
	textY := bar.Min.Y + (StatusHeight-ui.MonoSize)/ui.Half - 1
	room := f.statusLeft() - statusGap - ui.Inset
	if hand := f.history.hand; hand != "" {
		hand = theme.FitText(hand, room/ui.Half)
		handLeft := f.statusLeft() - statusGap - theme.TextWidth(hand)
		theme.DrawText(screen, hand, handLeft, bar.Min.Y+(StatusHeight-ui.TextSize)/ui.Half-1, ui.ColorInk())
		room = handLeft - statusGap - ui.Inset
	}
	message = theme.FitText(message, room)
	theme.DrawText(screen, message, ui.Inset, bar.Min.Y+(StatusHeight-ui.TextSize)/ui.Half-1, ink)
	f.ShowInspector.Draw(screen, theme)
	f.OnTop.Draw(screen, theme)
	f.history.button.Draw(screen, theme)
	if !f.history.dismiss.Rect.Empty() {
		f.history.dismiss.Draw(screen, theme)
	}
	if f.history.hand != "" {
		return
	}
	speed := "command to event: last " + formatLatency(f.last()) + ", median " + formatLatency(f.median())
	speedLeft := f.statusLeft() - statusGap - theme.MonoWidth(speed)
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

// CommonKeys handles the shortcuts every window shares: undo, redo and
// close, and for tests the crash key. It returns true if it used a key.
func CommonKeys(input *ui.Input, ctx *Context) bool {
	switch {
	case input.Consume(ui.KeyZ, ui.Cmd()):
		ctx.Requests.Undo(ctx.Frame.ReportStep("Undid"))
	case input.Consume(ui.KeyZ, ui.CmdShift()):
		ctx.Requests.Redo(ctx.Frame.ReportStep("Redid"))
	case input.Consume(ui.KeyW, ui.Cmd()):
		ctx.Frame.Closing = true
	case ctx.Frame.CrashKey && input.Consume(ui.KeyK, ui.CmdShift()):
		ctx.Frame.Crash = true
	default:
		return false
	}

	return true
}
