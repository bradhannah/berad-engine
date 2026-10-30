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
	"github.com/bradhannah/berad-engine/editor/sound"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// windowSeed is the ID seed of the window under test.
const windowSeed = 99

// Docs holds documents in memory.
type Docs struct {
	docs    map[ids.ULID]*client.Document
	viewing bool
}

// NewDocs returns an empty document store.
func NewDocs() *Docs {
	return &Docs{docs: map[ids.ULID]*client.Document{}, viewing: false}
}

// Put adds or replaces a document; a replaced document's revision goes up,
// as it would after an edit.
func (d *Docs) Put(id ids.ULID, name string, document model.Document) {
	revision := uint64(0)
	if previous, exists := d.docs[id]; exists {
		revision = previous.Revision + 1
	}
	d.docs[id] = &client.Document{ID: id, Name: name, Format: document.Format(), Revision: revision, Model: document}
}

// Touch raises a document's revision, as an edit applied to it in place
// does.
func (d *Docs) Touch(id ids.ULID) {
	if found, exists := d.docs[id]; exists {
		found.Revision++
	}
}

// View calls visit with a document, as a window's client does. Like the
// client's, it is not re-entrant: viewing a document from inside visit would
// deadlock a real window, so here it panics.
func (d *Docs) View(document ids.ULID, visit func(client.Document)) bool {
	if d.viewing {
		panic("frametest: View called inside View; a real window would deadlock")
	}
	found, exists := d.docs[document]
	if exists {
		d.viewing = true
		defer func() { d.viewing = false }()
		visit(*found)
	}

	return exists
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
		c.Done(protocol.Result{OK: true, Reason: "", Revision: 1, Created: created, Item: ""}, nil)
	}
}

// Requests records what a view asks for.
type Requests struct {
	Commands   []Command
	Undos      int
	Redos      int
	Subscribed []ids.ULID
	Opened     []protocol.Open
	Uses       []func([]protocol.Use, error)
	// UsesOf are the document and item each where-used question in Uses
	// asked about, in the same order.
	UsesOf      []UsesQuestion
	PickAsks    []func([]protocol.PickMap, error)
	PickStarts  []protocol.PickStart
	PickAnswers []protocol.PickAnswer
	PickCancels []protocol.PickCancel
	// Previews are preview starts waiting for the test to answer.
	Previews      []func(protocol.Result, error)
	PreviewStates []protocol.PreviewState
	PreviewStops  []protocol.PreviewStop
	// Published is the session state set, in order.
	Published       []protocol.Session
	PreviewControls []protocol.PreviewControl
	// AudioAsks are audio library questions, and RecordingAsks recording
	// questions, waiting for the test to answer.
	AudioAsks     []func(protocol.AudioLibrary, error)
	RecordingAsks []RecordingAsk
	// AudioEdits are audio edits waiting for the test to answer.
	AudioEdits []AudioEdit
	// Analyses are waveform questions, Checks file checks and Loudness
	// loudness questions, waiting for the test to answer.
	Analyses []Analysis
	Checks   []func(protocol.AudioCheck, error)
	Loudness []func(protocol.AudioLoudness, error)
}

// Analysis is a waveform question and how to answer it.
type Analysis struct {
	Asked protocol.AudioAnalysis
	Done  func(protocol.AudioAnalysis, error)
}

// AudioAnalysis records a waveform question; the test answers it.
func (r *Requests) AudioAnalysis(asked protocol.AudioAnalysis, done func(protocol.AudioAnalysis, error)) {
	r.Analyses = append(r.Analyses, Analysis{Asked: asked, Done: done})
}

// AudioCheck records a file check; the test answers it.
func (r *Requests) AudioCheck(done func(protocol.AudioCheck, error)) {
	r.Checks = append(r.Checks, done)
}

// AudioLoudness records a loudness question; the test answers it.
func (r *Requests) AudioLoudness(done func(protocol.AudioLoudness, error)) {
	r.Loudness = append(r.Loudness, done)
}

// AudioEdit is a recorded audio edit and its reply callback.
type AudioEdit struct {
	Edit protocol.AudioEdit
	Done func(protocol.Result, error)
}

// Decode decodes the edit's arguments.
func (e AudioEdit) Decode(target any) error {
	return json.Unmarshal(e.Edit.Args, target) //nolint:wrapcheck // Test helper; callers report the error.
}

// Succeed delivers a successful reply naming what the edit made.
func (e AudioEdit) Succeed(item string) {
	if e.Done != nil {
		e.Done(protocol.Result{OK: true, Reason: "", Revision: 1, Created: ids.ULID{}, Item: item}, nil)
	}
}

// AudioEdit records an audio edit; the test answers it.
func (r *Requests) AudioEdit(edit protocol.AudioEdit, done func(protocol.Result, error)) {
	r.AudioEdits = append(r.AudioEdits, AudioEdit{Edit: edit, Done: done})
}

// AudioOf returns the audio edits of one kind.
func (r *Requests) AudioOf(edit string) []AudioEdit {
	var matching []AudioEdit
	for _, recorded := range r.AudioEdits {
		if recorded.Edit.Edit == edit {
			matching = append(matching, recorded)
		}
	}

	return matching
}

// Files answers a choice of file with Chosen, or a cancel when it is empty,
// and counts the choices asked for.
type Files struct {
	Chosen string
	Asked  int
}

// ChooseSound answers at once.
func (f *Files) ChooseSound(done func(string, bool)) {
	f.Asked++
	done(f.Chosen, f.Chosen != "")
}

// RecordingAsk is a recording question: which recording, from where, and
// how to answer.
type RecordingAsk struct {
	Recording string
	Offset    int64
	Done      func(protocol.Recording, error)
}

// AudioLibrary records an audio library question; the test answers it.
func (r *Requests) AudioLibrary(done func(protocol.AudioLibrary, error)) {
	r.AudioAsks = append(r.AudioAsks, done)
}

// Recording records a recording question; the test answers it.
func (r *Requests) Recording(recording string, offset int64, done func(protocol.Recording, error)) {
	r.RecordingAsks = append(r.RecordingAsks, RecordingAsk{Recording: recording, Offset: offset, Done: done})
}

// Sounds records what a view asks to play. Sounding holds the cues the test
// says are playing, and recordings played on their own under "/" and their
// ID.
type Sounds struct {
	Prepared   [][]string
	Played     []string
	Stops      int
	Sounding   map[string]bool
	Auditioned []Audition
}

// Audition records an audition, as "//audition" played, and keeps it in
// Auditioned.
func (s *Sounds) Audition(recording string, window *protocol.AudioWindow, calibrationDB float64, loop bool) {
	s.Auditioned = append(s.Auditioned, Audition{
		Recording: recording, Window: window, CalibrationDB: calibrationDB, Loop: loop,
	})
	s.Play("//audition")
}

// AuditionAt reports the start of the latest audition while it plays.
func (s *Sounds) AuditionAt() (int64, bool) {
	if !s.Sounding["//audition"] || len(s.Auditioned) == 0 {
		return 0, false
	}
	if window := s.Auditioned[len(s.Auditioned)-1].Window; window != nil {
		return window.StartMs, true
	}

	return 0, true
}

// Audition is an audition a view asked for.
type Audition struct {
	Recording     string
	Window        *protocol.AudioWindow
	CalibrationDB float64
	Loop          bool
}

// PlayRecording records a recording played on its own, as "/" and its ID.
func (s *Sounds) PlayRecording(recording string) { s.Play("/" + recording) }

// PlayingRecording reports whether a recording played on its own plays.
func (s *Sounds) PlayingRecording(recording string) bool { return s.Sounding["/"+recording] }

// Prepare records cues to fetch.
func (s *Sounds) Prepare(cues []string) { s.Prepared = append(s.Prepared, cues) }

// Play records a cue played, which then counts as playing.
func (s *Sounds) Play(cue string) {
	s.Played = append(s.Played, cue)
	if s.Sounding == nil {
		s.Sounding = map[string]bool{}
	}
	s.Sounding[cue] = true
}

// PlayAt records a cue played at a step, as the cue, "@" and the step.
func (s *Sounds) PlayAt(cue, step string) {
	s.Play(cue)
	s.Played[len(s.Played)-1] = cue + "@" + step
}

// Stop records a stop; nothing plays after it.
func (s *Sounds) Stop() {
	s.Stops++
	s.Sounding = nil
}

// Playing reports whether a cue was played and not stopped.
func (s *Sounds) Playing(cue string) bool { return s.Sounding[cue] }

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
	r.Opened = append(r.Opened, protocol.Open{View: view, Target: target, NewView: newView, Item: "", PopOut: ids.ULID{}})
}

// OpenItem records a window request that shows an item.
func (r *Requests) OpenItem(view string, target ids.ULID, item string) {
	r.Opened = append(r.Opened, protocol.Open{View: view, Target: target, NewView: false, Item: item, PopOut: ids.ULID{}})
}

// Publish records session state set.
func (r *Requests) Publish(topic string, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	r.Published = append(r.Published, protocol.Session{Topic: topic, Value: encoded})
}

// UsesQuestion is what a where-used question asked about.
type UsesQuestion struct {
	Document ids.ULID
	Item     string
}

// WhereUsed records a where-used question; the test answers it.
func (r *Requests) WhereUsed(document ids.ULID, item string, done func([]protocol.Use, error)) {
	r.Uses = append(r.Uses, done)
	r.UsesOf = append(r.UsesOf, UsesQuestion{Document: document, Item: item})
}

// PickAsk records a pick question; the test answers it.
func (r *Requests) PickAsk(_ string, done func([]protocol.PickMap, error)) {
	r.PickAsks = append(r.PickAsks, done)
}

// PickStart records a pick start and reports success.
func (r *Requests) PickStart(start protocol.PickStart, done func(protocol.Result, error)) {
	r.PickStarts = append(r.PickStarts, start)
	if done != nil {
		done(protocol.Result{OK: true, Reason: "", Revision: 0, Created: ids.ULID{}, Item: ""}, nil)
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

// PreviewControl records a preview control.
func (r *Requests) PreviewControl(control protocol.PreviewControl) {
	r.PreviewControls = append(r.PreviewControls, control)
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
		Docs: docs, Requests: requests, Sounds: &Sounds{
			Prepared: nil, Played: nil, Stops: 0, Sounding: nil, Auditioned: nil,
		},
		Soundscape: &Soundscape{Layers: map[string]sound.Layer{}, Started: nil, Monitor: sound.FullMonitor()},
		Files:      &Files{Chosen: "", Asked: 0}, Clipboard: nil, Window: ID(windowSeed), Size: size,
		Frame: frame.NewFrame(time.Now),
	}
}

// Soundscape keeps the layers a view plays: Layers by slot while they play,
// each layer started in Started, and the monitor levels last set.
type Soundscape struct {
	Layers  map[string]sound.Layer
	Started []sound.Layer
	Monitor sound.Monitor
}

// PlayLayer records a layer started.
func (s *Soundscape) PlayLayer(layer sound.Layer) {
	s.Layers[layer.Slot] = layer
	s.Started = append(s.Started, layer)
}

// SetLayer changes a playing layer.
func (s *Soundscape) SetLayer(layer sound.Layer) {
	if playing, found := s.Layers[layer.Slot]; found && playing.Cue == layer.Cue {
		s.Layers[layer.Slot] = layer
	}
}

// StopLayer stops a layer.
func (s *Soundscape) StopLayer(slot string) { delete(s.Layers, slot) }

// LayerPlaying reports whether a layer plays.
func (s *Soundscape) LayerPlaying(slot string) bool {
	_, found := s.Layers[slot]

	return found
}

// SetMonitor records the monitor levels.
func (s *Soundscape) SetMonitor(monitor sound.Monitor) { s.Monitor = monitor }

// ID returns a fixed ID for tests.
func ID(seed byte) ids.ULID {
	var id ids.ULID
	id[15] = seed

	return id
}
