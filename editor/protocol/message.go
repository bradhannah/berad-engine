// Package protocol defines the messages exchanged between the orchestrator and
// editor window processes, and how they travel over a connection.
//
// Every message is a Kind, an optional request number for correlating replies,
// and a payload. Payload types are defined once here as Go types; the encoding
// sits behind the Codec interface so it can change without touching them.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Version is the protocol version exchanged in the handshake.
const Version = 14

// ErrPayload reports a payload that does not match its message kind.
var ErrPayload = errors.New("invalid message payload")

// Kind names a message type.
type Kind string

// Message kinds.
const (
	KindHello     Kind = "hello"
	KindWelcome   Kind = "welcome"
	KindSubscribe Kind = "subscribe"
	KindSnapshot  Kind = "snapshot"
	KindCommand   Kind = "command"
	KindUndo      Kind = "undo"
	KindRedo      Kind = "redo"
	KindResult    Kind = "result"
	KindEvent     Kind = "event"
	KindSession   Kind = "session"
	KindOpen      Kind = "open"
	KindShutdown  Kind = "shutdown"
	// KindQuery asks the orchestrator a question about the whole project;
	// KindAnswer replies.
	KindQuery  Kind = "query"
	KindAnswer Kind = "answer"
	// KindAttention asks a window to draw attention to itself, for example
	// when something already open in it is opened again.
	KindAttention Kind = "attention"
	// The four pick-on-map messages (D13).
	KindPickAsk    Kind = "pick-ask"
	KindPickStart  Kind = "pick-start"
	KindPickAnswer Kind = "pick-answer"
	KindPickCancel Kind = "pick-cancel"
	// The preview messages (D7, D15): a Map window starts a preview, streams
	// its live state through the orchestrator, and stops it; other windows
	// of the zone control its NPCs.
	KindPreviewStart   Kind = "preview-start"
	KindPreviewState   Kind = "preview-state"
	KindPreviewStop    Kind = "preview-stop"
	KindPreviewControl Kind = "preview-control"
	// Quitting: a window reports its unsaved changes as they change, asks
	// what quitting would close, and asks the orchestrator to quit, which
	// closes every window.
	KindUnsaved Kind = "unsaved"
	KindQuitAsk Kind = "quit-ask"
	KindQuit    Kind = "quit"
	// The audio messages (D8): a window asks for the project's audio
	// library, the cues it can play, and for a recording's bytes, a part at
	// a time. The orchestrator replies to each with the same kind, filled in.
	KindAudioLibrary Kind = "audio-library"
	KindRecording    Kind = "recording"
	// KindAudioEdit asks the orchestrator to change the audio library, and is
	// answered with a Result. KindAudioChanged tells every window that the
	// library changed, so windows that show it ask for it again.
	KindAudioEdit    Kind = "audio-edit"
	KindAudioChanged Kind = "audio-changed"
	// KindAudioAnalysis asks for a recording's waveform and suggestions,
	// KindAudioCheck for every recording to be decoded, and KindAudioLoudness
	// for how loud every recording measures; the orchestrator replies to each
	// with the same kind, filled in.
	KindAudioAnalysis Kind = "audio-analysis"
	KindAudioCheck    Kind = "audio-check"
	KindAudioLoudness Kind = "audio-loudness"
)

// Message is one framed protocol message.
type Message struct {
	Kind    Kind            `json:"kind"`
	Request uint64          `json:"request,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Hello opens a window's connection. The orchestrator refuses a different
// protocol version or build.
type Hello struct {
	Protocol int      `json:"protocol"`
	Build    string   `json:"build"`
	Window   ids.ULID `json:"window"`
	View     string   `json:"view"`
	Target   ids.ULID `json:"target"`
	// UndoGroup names the undo list this window shares. Views of the same map
	// share one; other windows use their own.
	UndoGroup string `json:"undoGroup"`
}

// Welcome accepts a Hello.
type Welcome struct {
	Window ids.ULID `json:"window"`
}

// Subscribe asks for a document's snapshot and its future change events.
type Subscribe struct {
	Document ids.ULID `json:"document"`
}

// Snapshot is a document's full current state.
type Snapshot struct {
	Document ids.ULID        `json:"document"`
	Format   string          `json:"format"`
	Name     string          `json:"name"`
	Revision uint64          `json:"revision"`
	Data     json.RawMessage `json:"data"`
	// Layout is the document's layout, for documents that keep one (D17).
	Layout json.RawMessage `json:"layout,omitempty"`
}

// Command asks the orchestrator to apply one edit to one document.
type Command struct {
	Document ids.ULID `json:"document"`
	Edit     string   `json:"edit"`
	// Gesture groups consecutive commands (for example one paint stroke) into
	// one undo step. Empty means the command is its own step.
	Gesture string          `json:"gesture,omitempty"`
	Args    json.RawMessage `json:"args"`
}

// Result answers a Command, Undo, Redo, Subscribe, Open, PickStart or
// PreviewStart request.
type Result struct {
	OK       bool   `json:"ok"`
	Reason   string `json:"reason,omitempty"`
	Revision uint64 `json:"revision,omitempty"`
	// Created is the ID the orchestrator assigned to something the command
	// created, such as a document or an anchor. Item names something it
	// created that has no ID of its own, such as a cue.
	Created ids.ULID `json:"created,omitzero"`
	Item    string   `json:"item,omitempty"`
}

// Event reports an applied edit. Windows apply the same edit with the same
// arguments to their copy of the document.
type Event struct {
	Document ids.ULID        `json:"document"`
	Revision uint64          `json:"revision"`
	Edit     string          `json:"edit"`
	Args     json.RawMessage `json:"args"`
	Origin   ids.ULID        `json:"origin"`
	// Request is the originating window's request number, so it can measure
	// command-to-event time. Zero for undo and redo replays.
	Request uint64 `json:"request,omitempty"`
}

// Session carries shared, unsaved, never-undone session state.
type Session struct {
	Topic string          `json:"topic"`
	Value json.RawMessage `json:"value"`
}

// TopicNotice is the session topic for user-facing notices.
const TopicNotice = "notice"

// TopicClockStart is the session topic for the time the next preview starts
// at, a ClockStart. The Clock window sets it; until then a preview starts at
// DefaultClockStart.
const TopicClockStart = "clock-start"

// DefaultClockStart is the time of day, in seconds, a preview starts at until
// the Clock window sets another: 08:00.
const DefaultClockStart = 8 * 3600

// ClockStart is the time of day, in seconds, the next preview starts at.
type ClockStart struct {
	Seconds float64 `json:"seconds"`
}

// NoticeWindowCrashed is the notice kind for a window that closed unexpectedly.
const NoticeWindowCrashed = "window-crashed"

// Notice is a user-facing message, published on TopicNotice.
type Notice struct {
	Kind   string   `json:"kind"`
	Text   string   `json:"text"`
	Window ids.ULID `json:"window"`
	Report string   `json:"report,omitempty"`
}

// Open asks the orchestrator to open a window. Opening a view and target that
// is already open draws attention to that window instead, unless NewView asks
// for another view of it. Item, when set, is something in the window to show,
// such as a cue in the Audio window; the window is sent an Attention naming it.
type Open struct {
	View    string   `json:"view"`
	Target  ids.ULID `json:"target"`
	NewView bool     `json:"newView,omitempty"`
	Item    string   `json:"item,omitempty"`
}

// Attention asks a window to draw attention to itself and, when Item is set,
// to show that item.
type Attention struct {
	Item string `json:"item,omitempty"`
}

// QuestionWhereUsed asks which documents refer to a document, or to one item
// inside it.
const QuestionWhereUsed = "where-used"

// Query asks a question that needs the whole project.
type Query struct {
	Question string   `json:"question"`
	Document ids.ULID `json:"document"`
	Item     string   `json:"item,omitempty"`
}

// Use is one place that refers to the thing asked about.
type Use struct {
	Document ids.ULID `json:"document"`
	Name     string   `json:"name"`
	Format   string   `json:"format"`
	Label    string   `json:"label"`
}

// Answer replies to a Query.
type Answer struct {
	Uses []Use `json:"uses"`
}

// PickAnchor is what a pick of an anchor asks for.
const PickAnchor = "anchor"

// PickMap describes an open Map window a pick could use.
type PickMap struct {
	Window ids.ULID `json:"window"`
	Zone   ids.ULID `json:"zone"`
	// Shows is true when the map holds something that can be picked.
	Shows bool `json:"shows"`
}

// PickAsk asks which open maps a pick could use. The orchestrator replies
// with the same kind, Maps filled in.
type PickAsk struct {
	What string    `json:"what"`
	Maps []PickMap `json:"maps,omitempty"`
}

// PickStart starts a pick. The requesting window names an open Map window, or
// a zone for the orchestrator to open one on. The orchestrator forwards it to
// the Map window with From set.
type PickStart struct {
	Pick   string   `json:"pick"`
	What   string   `json:"what"`
	Label  string   `json:"label"`
	Window ids.ULID `json:"window,omitzero"`
	Zone   ids.ULID `json:"zone,omitzero"`
	From   ids.ULID `json:"from,omitzero"`
}

// PickAnswer reports what was picked; the requesting window then makes an
// ordinary edit.
type PickAnswer struct {
	Pick  string   `json:"pick"`
	Floor ids.ULID `json:"floor"`
	Item  string   `json:"item"`
}

// PickCancel ends a pick without an answer, from either side.
type PickCancel struct {
	Pick   string `json:"pick"`
	Reason string `json:"reason,omitempty"`
}

// PreviewStart asks to start a preview of a zone (D15); one preview runs at
// a time. The orchestrator answers with a Result whose Created is the new
// run's ID, then tells the zone's other Map windows with this kind, Run and
// Window filled in.
type PreviewStart struct {
	Zone   ids.ULID `json:"zone"`
	Run    ids.ULID `json:"run,omitzero"`
	Window ids.ULID `json:"window,omitzero"`
}

// EntityPlayer is the player's ID among a preview's entities.
const EntityPlayer = "player"

// PreviewEntity is one live entity of a preview: where it is, in cells,
// part-way between two cells while it moves. The player's ID is
// EntityPlayer; an NPC's is its placement's.
type PreviewEntity struct {
	ID    string   `json:"id"`
	Floor ids.ULID `json:"floor"`
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
	// Face is the direction the entity faces, one cell.
	Face [2]int `json:"face"`
	// An NPC also says which NPC it is, the workflow it follows, the node it
	// is on, what it is doing there, what keeps it from following its
	// workflow, its latest change of node and how many changes it has made.
	NPC      ids.ULID      `json:"npc,omitzero"`
	Workflow ids.ULID      `json:"workflow,omitzero"`
	Node     ids.ULID      `json:"node,omitzero"`
	Activity string        `json:"activity,omitempty"`
	Status   string        `json:"status,omitempty"`
	Last     PreviewChange `json:"last,omitzero"`
	Changes  uint64        `json:"changes,omitempty"`
}

// PreviewChange is an NPC's change from one node to another: the output it
// took, the tick, and, when the node it left could not do its work and
// carried on, why.
type PreviewChange struct {
	From   ids.ULID `json:"from,omitzero"`
	Output string   `json:"output,omitempty"`
	To     ids.ULID `json:"to,omitzero"`
	Tick   uint64   `json:"tick,omitempty"`
	Status string   `json:"status,omitempty"`
}

// PreviewState is a preview's latest live state, sent at a limited rate; the
// latest value wins (D7). Tick orders the states of one run, so a receiver
// discards a state older than the one it holds.
type PreviewState struct {
	Run      ids.ULID        `json:"run"`
	Tick     uint64          `json:"tick"`
	Entities []PreviewEntity `json:"entities"`
	// Clock is the game clock's time of day in seconds, Rate how many game
	// seconds pass in a real one, and ClockStopped whether it is stopped.
	Clock        float64 `json:"clock"`
	Rate         int     `json:"rate"`
	ClockStopped bool    `json:"clockStopped,omitempty"`
	// NPCsPaused says the NPCs are paused; Watched is the placement of the
	// NPC being watched, if any.
	NPCsPaused bool     `json:"npcsPaused,omitempty"`
	Watched    ids.ULID `json:"watched,omitzero"`
	// Stepping says a step is running. Step is the changes the watched NPC
	// made in the latest step, oldest first, kept until the NPCs resume or
	// another step starts.
	Stepping bool            `json:"stepping,omitempty"`
	Step     []PreviewChange `json:"step,omitempty"`
}

// Preview controls.
const (
	ControlWatch      = "watch"
	ControlPause      = "pause"
	ControlResume     = "resume"
	ControlStep       = "step"
	ControlRate       = "rate"
	ControlJump       = "jump"
	ControlStopClock  = "stop-clock"
	ControlStartClock = "start-clock"
)

// PreviewControl asks the window running a preview to watch an NPC, by its
// placement, or to pause, resume or step its NPCs: a step runs them, as fast
// as it can, until the watched NPC changes node, then pauses them. It also
// runs the preview's clock: a rate, a jump to a time of day in Seconds, and
// stopping and starting it. It comes from a Watch window, the Clock window
// or another Map window of the zone, and the orchestrator passes it to the
// window running the preview.
type PreviewControl struct {
	Run       ids.ULID `json:"run"`
	Action    string   `json:"action"`
	Placement ids.ULID `json:"placement,omitzero"`
	Rate      int      `json:"rate,omitempty"`
	Seconds   float64  `json:"seconds,omitempty"`
}

// PreviewStop ends a preview. The window running it sends it; the
// orchestrator tells the zone's other Map windows, and sends it itself when
// that window closes.
type PreviewStop struct {
	Run    ids.ULID `json:"run"`
	Reason string   `json:"reason,omitempty"`
}

// Shutdown tells a window to clean up and close. With Save, the window first
// saves its unsaved changes, and stays open if it cannot.
type Shutdown struct {
	Reason string `json:"reason"`
	Save   bool   `json:"save,omitempty"`
}

// Unsaved is a window's typed changes not saved yet: how many, and what they
// belong to, such as an NPC's name. A window sends it whenever it changes.
type Unsaved struct {
	Count int    `json:"count"`
	Name  string `json:"name,omitempty"`
}

// UnsavedWindow is one window with unsaved changes.
type UnsavedWindow struct {
	Window ids.ULID `json:"window"`
	View   string   `json:"view"`
	Name   string   `json:"name"`
	Count  int      `json:"count"`
}

// QuitAsk asks what quitting would close; the orchestrator replies with how
// many windows are open and which have unsaved changes.
type QuitAsk struct {
	Windows int             `json:"windows"`
	Unsaved []UnsavedWindow `json:"unsaved"`
}

// Quit asks the orchestrator to quit the editor: every window closes, saving
// its unsaved changes first when Save is set.
type Quit struct {
	Save bool `json:"save,omitempty"`
}

// Cue categories, as berad-audio's CueCategory names them.
const (
	CategoryMusic   = "music"
	CategorySFX     = "sfx"
	CategoryAmbient = "ambient"
)

// Recording statuses: ready to play, an intentional silent slot with no
// file, a file the project does not have, or one too large to send.
const (
	RecordingReady    = "ready"
	RecordingNoFile   = "no file"
	RecordingMissing  = "missing"
	RecordingTooLarge = "too large"
)

// MaxRecording bounds the size of a recording the orchestrator sends, and
// MaxRecordingPart the bytes one reply carries, so it fits in a frame.
const (
	MaxRecording     = 128 << 20
	MaxRecordingPart = 8 << 20
)

// AudioLibrary asks for the project's audio library. The reply lists every
// cue, ordered by ID, or says in Problem why there is nothing to play, such
// as a project with no audio library. Schema is the library's schema version
// and Unused lists the recordings no cue plays, ordered by ID.
type AudioLibrary struct {
	Problem string           `json:"problem,omitempty"`
	Cues    []AudioCue       `json:"cues,omitempty"`
	Schema  int              `json:"schema,omitempty"`
	Unused  []AudioRecording `json:"unused,omitempty"`
	// Profiles are the classification profiles a cue can have, and Families
	// the acoustic families a recording can be in, each by name.
	Profiles []AudioProfile `json:"profiles,omitempty"`
	Families []string       `json:"families,omitempty"`
	// AmbientPreset is what the audio project gives a new ambient cue, if
	// anything; SliderGains are its 11 volume levels, 0 to 10, as gains from 0
	// to 1, or none for even steps.
	AmbientPreset *AmbientPreset `json:"ambientPreset,omitempty"`
	SliderGains   []float64      `json:"sliderGains,omitempty"`
	// ReferenceDB is the loudness every measured recording is evened out to;
	// CategoryGainsDB the adjustment, in dB, each category's cues apply on top
	// of their step, none for 0; MaxCueGain the largest gain a cue may have in
	// the library's schema.
	ReferenceDB     float64            `json:"referenceDb,omitempty"`
	CategoryGainsDB map[string]float64 `json:"categoryGainsDb,omitempty"`
	MaxCueGain      float64            `json:"maxCueGain,omitempty"`
}

// AudioProfile is a classification profile: the kind of moment a cue plays
// for, which a search finds it by. Its fields are berad-audio's.
type AudioProfile struct {
	Name        string   `json:"name"`
	Family      string   `json:"family"`
	Phase       string   `json:"phase"`
	Outcome     string   `json:"outcome"`
	MixRole     string   `json:"mixRole"`
	Emitters    []string `json:"emitters,omitempty"`
	Contexts    []string `json:"contexts,omitempty"`
	SearchTerms []string `json:"searchTerms,omitempty"`
}

// AmbientSettings say how an ambient cue is heard: from up to RadiusTiles
// tiles away, louder closer, or everywhere when zero; fading in and out over
// the given milliseconds.
type AmbientSettings struct {
	RadiusTiles float64 `json:"radiusTiles"`
	FadeInMs    int64   `json:"fadeInMs"`
	FadeOutMs   int64   `json:"fadeOutMs"`
}

// AmbientPreset is what a new ambient cue starts with: its ID's first part,
// its profile, its first recording's family, how to hear it and its gain.
type AmbientPreset struct {
	CuePrefix string  `json:"cuePrefix"`
	Profile   string  `json:"profile"`
	Family    string  `json:"family"`
	HowToHear string  `json:"howToHear"`
	Gain      float64 `json:"gain"`
}

// AudioCue is one cue of the audio library: its ID, which a cue parameter
// stores; its name; its category, and whether the audio project names it
// rather than it following from the cue's routing; its gain; how to hear it;
// the words a search also finds it by; and its recordings, in the order a
// cycle plays them. For the Audio window it also has its classification
// profile and, for an ambient cue, how far away it is heard.
type AudioCue struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Category    string           `json:"category"`
	CategorySet bool             `json:"categorySet,omitempty"`
	Gain        float64          `json:"gain"`
	HowToHear   string           `json:"howToHear,omitempty"`
	SearchTerms []string         `json:"searchTerms,omitempty"`
	Recordings  []AudioRecording `json:"recordings"`
	Profile     string           `json:"profile,omitempty"`
	Ambient     *AmbientSettings `json:"ambient,omitempty"`
}

// AudioRecording is one recording a cue plays: its ID; its file's
// extension, which says how to decode it; a version that changes whenever
// the file does, so a window keeps bytes it already holds; its size in
// bytes; its calibration gain in dB; the part of it that plays, in
// milliseconds, both zero for all of it; and its status. For the Audio
// window it also has how its calibration was set (CalibrationAuto, manual,
// or empty for none) and its target, zero for its family's; its acoustic
// family; its file, relative to the audio folder; the file it was copied
// from; its note and search terms; and its attribution.
type AudioRecording struct {
	ID              string      `json:"id"`
	Extension       string      `json:"extension,omitempty"`
	Version         string      `json:"version,omitempty"`
	Size            int64       `json:"size,omitempty"`
	CalibrationDB   float64     `json:"calibrationDb,omitempty"`
	CalibrationMode string      `json:"calibrationMode,omitempty"`
	TargetDB        float64     `json:"targetDb,omitempty"`
	Family          string      `json:"family,omitempty"`
	StartMs         int64       `json:"startMs,omitempty"`
	EndMs           int64       `json:"endMs,omitempty"`
	Status          string      `json:"status"`
	File            string      `json:"file,omitempty"`
	OriginalFile    string      `json:"originalFile,omitempty"`
	Note            string      `json:"note,omitempty"`
	SearchTerms     []string    `json:"searchTerms,omitempty"`
	Attribution     Attribution `json:"attribution"`
}

// Unknown is how the audio library spells an attribution nobody recorded.
const Unknown = "unknown"

// How a recording's calibration was set: measured from it, or by hand.
const (
	CalibrationAuto   = "auto"
	CalibrationManual = "manual"
)

// AudioWindow is a part of a recording, in source milliseconds.
type AudioWindow struct {
	StartMs int64 `json:"startMs"`
	EndMs   int64 `json:"endMs"`
}

// AudioAnalysis asks for a recording's waveform: Columns peaks between From
// and To, in source milliseconds, both zero for all of it, and a calibration
// suggestion for Region, the recording's own when nil. The reply adds the
// recording's version and length, the peaks, each the loudest moment of its
// column from 0 for −72 dB or quieter to 255 for 0 dB, the region its sound
// fills, each sound in it that silence separates from the next, and the
// suggestion; or Problem, such as a file that cannot be decoded.
type AudioAnalysis struct {
	Recording   string            `json:"recording"`
	From        int64             `json:"from,omitempty"`
	To          int64             `json:"to,omitempty"`
	Columns     int               `json:"columns"`
	Region      *AudioWindow      `json:"region,omitempty"`
	Version     string            `json:"version,omitempty"`
	DurationMs  int64             `json:"durationMs,omitempty"`
	Peaks       []byte            `json:"peaks,omitempty"`
	Suggested   *AudioWindow      `json:"suggested,omitempty"`
	Sounds      []AudioWindow     `json:"sounds,omitempty"`
	Calibration *AudioCalibration `json:"calibration,omitempty"`
	Problem     string            `json:"problem,omitempty"`
}

// AudioLoudness asks how loud every recording with a file measures. The
// reply lists each, ordered by ID.
type AudioLoudness struct {
	Recordings []RecordingLoudness `json:"recordings,omitempty"`
}

// RecordingLoudness is how loud a recording's region measures, its loudest
// sample and the target measuring aims it at, all in dB; or Problem when it
// cannot be measured, such as silence or a file that cannot be decoded.
type RecordingLoudness struct {
	ID         string  `json:"id"`
	MeasuredDB float64 `json:"measuredDb,omitempty"`
	PeakDB     float64 `json:"peakDb,omitempty"`
	TargetDB   float64 `json:"targetDb,omitempty"`
	Problem    string  `json:"problem,omitempty"`
}

// AudioCalibration is a calibration suggestion: how loud the region measures
// and how it was measured, the target, its loudest sample, the gain that
// brings it to the target, and whether the gain was limited to stop it
// clipping or to stay in range. All in dB; estimates, never a listening
// judgement.
type AudioCalibration struct {
	Method     string  `json:"method"`
	MeasuredDB float64 `json:"measuredDb"`
	TargetDB   float64 `json:"targetDb"`
	PeakDB     float64 `json:"peakDb"`
	GainDB     float64 `json:"gainDb"`
	Limited    bool    `json:"limited,omitempty"`
}

// AudioCheck asks for every recording with a file to be decoded. The reply
// lists those that cannot play, with why.
type AudioCheck struct {
	Broken []AudioProblem `json:"broken,omitempty"`
}

// AudioProblem is why a recording cannot play.
type AudioProblem struct {
	Recording string `json:"recording"`
	Reason    string `json:"reason"`
}

// Attribution is who made a recording, where it came from, what kind of
// source that was and on what licence, and what was done to it before it
// came in, as its author recorded them, Unknown where nobody did; and
// whether the author cleared it for use in a commercial game. The editor
// records these; it verifies no licence.
type Attribution struct {
	Creator    string           `json:"creator"`
	Source     string           `json:"source"`
	SourceType string           `json:"sourceType"`
	License    string           `json:"license"`
	Processing []ProcessingStep `json:"processing,omitempty"`
	Commercial bool             `json:"commercial,omitempty"`
}

// ProcessingStep is one thing done to a recording before it came in, such
// as trimming it, with the tool used and who did it.
type ProcessingStep struct {
	Description string `json:"description"`
	Tool        string `json:"tool"`
	PerformedBy string `json:"performedBy"`
}

// Audio edits, as AudioEdit names them, with the arguments each takes.
const (
	// AudioCreateCue makes a cue, NewCue, with an empty recording slot.
	AudioCreateCue = "audio.create-cue"
	// AudioAddSound copies a file into the project as a new recording, AddSound.
	AudioAddSound = "audio.add-sound"
	// AudioReplaceFile copies a file in for an existing recording, ReplaceFile.
	AudioReplaceFile = "audio.replace-file"
	// AudioSetCue, AudioSetCategory and AudioSetRecordings change a cue:
	// CueChange, CategoryChange and CueRecordings.
	AudioSetCue        = "audio.set-cue"
	AudioSetCategory   = "audio.set-category"
	AudioSetRecordings = "audio.set-recordings"
	// AudioSetRecording and AudioSetAttribution change a recording:
	// RecordingChange and AttributionChange.
	AudioSetRecording   = "audio.set-recording"
	AudioSetAttribution = "audio.set-attribution"
	// AudioDeleteCue and AudioDeleteRecording delete one, AudioItem; a
	// recording only when no cue plays it.
	AudioDeleteCue       = "audio.delete-cue"
	AudioDeleteRecording = "audio.delete-recording"
	// AudioSetRegion sets the part of a recording that plays, RegionChange;
	// AudioDuplicateRegion makes a new recording of part of one, DuplicateRegion.
	AudioSetRegion       = "audio.set-region"
	AudioDuplicateRegion = "audio.duplicate-region"
	// AudioAcceptCalibration measures a recording and sets its gain from the
	// measurement, AudioItem; AudioSetCalibration sets its gain or its target
	// by hand, CalibrationChange.
	AudioAcceptCalibration = "audio.accept-calibration"
	AudioSetCalibration    = "audio.set-calibration"
	// AudioSetAmbient sets how far away an ambient cue is heard and its
	// fades, AmbientChange.
	AudioSetAmbient = "audio.set-ambient"
	// AudioSetProfile sets a cue's classification profile, ProfileChange;
	// AudioSetFamily a recording's acoustic family, FamilyChange.
	AudioSetProfile = "audio.set-profile"
	AudioSetFamily  = "audio.set-family"
	// AudioSetLoudness sets how loud a cue is meant to sound, LoudnessChange.
	// AudioSetCategoryGain sets a category's adjustment, CategoryGainChange,
	// and AudioSetReference the loudness recordings are evened out to,
	// ReferenceChange; each changes every cue or recording it affects.
	AudioSetLoudness     = "audio.set-loudness"
	AudioSetCategoryGain = "audio.set-category-gain"
	AudioSetReference    = "audio.set-reference"
	// AudioMeasureAll measures every recording no level was set for, with no
	// arguments; AudioUseReference measures one again, AudioItem, dropping a
	// level or target set by hand.
	AudioMeasureAll   = "audio.measure-all"
	AudioUseReference = "audio.use-reference"
	// AudioSetFileAttribution sets the attribution of every recording of a
	// file, FileAttributionChange; AudioMakePieces makes a recording of each
	// of several parts of one's file, MakePieces.
	AudioSetFileAttribution = "audio.set-file-attribution"
	AudioMakePieces         = "audio.make-pieces"
)

// AudioEdit asks for one change to the audio library: Edit names it and Args
// holds its arguments, as for a Command. Gesture groups consecutive edits
// into one undo step. The Result's Item names a cue or recording it created.
type AudioEdit struct {
	Edit    string          `json:"edit"`
	Gesture string          `json:"gesture,omitempty"`
	Args    json.RawMessage `json:"args"`
}

// AudioChanged tells windows the audio library changed: an edit, an undo or
// a change another program made to its files. Revision counts the changes
// the orchestrator has seen.
type AudioChanged struct {
	Revision uint64 `json:"revision"`
}

// NewCue is a cue to create: its ID, which never changes; its name; its
// category, or empty for the default; and how to hear it in the game.
type NewCue struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category,omitempty"`
	HowToHear string `json:"howToHear"`
}

// AddSound copies the file at Path into the project as a new recording with
// its attribution: into a new cue, NewCue, or as another recording of Cue,
// filling its empty slot when it has one.
type AddSound struct {
	Cue         string      `json:"cue,omitempty"`
	NewCue      *NewCue     `json:"newCue,omitempty"`
	Path        string      `json:"path"`
	Attribution Attribution `json:"attribution"`
}

// ReplaceFile copies the file at Path into the project for a recording, with
// the new file's attribution.
type ReplaceFile struct {
	Recording   string      `json:"recording"`
	Path        string      `json:"path"`
	Attribution Attribution `json:"attribution"`
}

// CueChange changes some of a cue's fields; a nil field stays as it is.
type CueChange struct {
	Cue         string    `json:"cue"`
	Name        *string   `json:"name,omitempty"`
	Gain        *float64  `json:"gain,omitempty"`
	HowToHear   *string   `json:"howToHear,omitempty"`
	SearchTerms *[]string `json:"searchTerms,omitempty"`
}

// CategoryChange sets a cue's category; empty returns it to the default.
type CategoryChange struct {
	Cue      string `json:"cue"`
	Category string `json:"category,omitempty"`
}

// CueRecordings sets the recordings a cue plays, in turn when there are
// several.
type CueRecordings struct {
	Cue        string   `json:"cue"`
	Recordings []string `json:"recordings"`
}

// RecordingChange changes a recording's note or search terms; a nil field
// stays as it is.
type RecordingChange struct {
	Recording   string    `json:"recording"`
	Note        *string   `json:"note,omitempty"`
	SearchTerms *[]string `json:"searchTerms,omitempty"`
}

// AttributionChange sets a recording's attribution.
type AttributionChange struct {
	Recording   string      `json:"recording"`
	Attribution Attribution `json:"attribution"`
}

// AudioItem names the cue or recording an edit deletes, or the recording it
// measures.
type AudioItem struct {
	ID string `json:"id"`
}

// RegionChange sets the part of a recording that plays; nil plays all of it.
type RegionChange struct {
	Recording string       `json:"recording"`
	Region    *AudioWindow `json:"region,omitempty"`
}

// DuplicateRegion makes a new recording of part of one, nil for all of it, and
// adds it to the recordings Cue plays in turn when Cue is set.
type DuplicateRegion struct {
	Recording string       `json:"recording"`
	Region    *AudioWindow `json:"region,omitempty"`
	Cue       string       `json:"cue,omitempty"`
}

// CalibrationChange sets a recording's gain by hand, in dB, or the loudness
// its measurement aims at, in dB; DefaultTarget returns that to its family's.
// A nil field stays as it is.
type CalibrationChange struct {
	Recording     string   `json:"recording"`
	GainDB        *float64 `json:"gainDb,omitempty"`
	TargetDB      *float64 `json:"targetDb,omitempty"`
	DefaultTarget bool     `json:"defaultTarget,omitempty"`
}

// AmbientChange sets an ambient cue's settings.
type AmbientChange struct {
	Cue     string          `json:"cue"`
	Ambient AmbientSettings `json:"ambient"`
}

// ProfileChange sets a cue's classification profile, one the library has.
type ProfileChange struct {
	Cue     string `json:"cue"`
	Profile string `json:"profile"`
}

// FamilyChange puts a recording in an acoustic family, a new one when no
// recording is in it yet; a measured recording is measured again.
type FamilyChange struct {
	Recording string `json:"recording"`
	Family    string `json:"family"`
}

// LoudnessChange sets how loud a cue is meant to sound: a step of package
// loudness, such as "louder".
type LoudnessChange struct {
	Cue  string `json:"cue"`
	Step string `json:"step"`
}

// CategoryGainChange sets the adjustment, in dB, every cue of a category
// applies on top of its step.
type CategoryGainChange struct {
	Category string  `json:"category"`
	GainDB   float64 `json:"gainDb"`
}

// ReferenceChange sets the loudness, in dB, every measured recording is
// evened out to.
type ReferenceChange struct {
	TargetDB float64 `json:"targetDb"`
}

// FileAttributionChange sets the attribution of every recording whose file
// is File, relative to the audio folder.
type FileAttributionChange struct {
	File        string      `json:"file"`
	Attribution Attribution `json:"attribution"`
}

// MakePieces makes a new recording of each region of Recording's file, and
// adds them, in order, to the recordings Cue plays in turn when Cue is set.
type MakePieces struct {
	Recording string        `json:"recording"`
	Regions   []AudioWindow `json:"regions"`
	Cue       string        `json:"cue,omitempty"`
}

// Recording asks for a recording's bytes from Offset. The reply carries its
// Version and Size and at most MaxRecordingPart bytes from Offset; a window
// asks again from the end of each part until it has Size bytes.
type Recording struct {
	ID      string `json:"id"`
	Offset  int64  `json:"offset,omitempty"`
	Version string `json:"version,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Data    []byte `json:"data,omitempty"`
}

// NewMessage encodes a payload into a message.
func NewMessage(kind Kind, request uint64, payload any) (Message, error) {
	if payload == nil {
		return Message{Kind: kind, Request: request, Payload: nil}, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("%w: %s: %w", ErrPayload, kind, err)
	}

	return Message{Kind: kind, Request: request, Payload: data}, nil
}

// DecodePayload decodes a message's payload as T.
//
//nolint:ireturn // T is the caller-chosen concrete payload type.
func DecodePayload[T any](message Message) (T, error) {
	var payload T
	if len(message.Payload) == 0 {
		return payload, fmt.Errorf("%w: %s has no payload", ErrPayload, message.Kind)
	}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return payload, fmt.Errorf("%w: %s: %w", ErrPayload, message.Kind, err)
	}

	return payload, nil
}
