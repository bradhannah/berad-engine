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
const Version = 9

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
	// created, such as a document or an anchor.
	Created ids.ULID `json:"created,omitzero"`
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
// for another view of it.
type Open struct {
	View    string   `json:"view"`
	Target  ids.ULID `json:"target"`
	NewView bool     `json:"newView,omitempty"`
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
