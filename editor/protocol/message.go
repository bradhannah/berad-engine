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
const Version = 1

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

// Result answers a Command, Undo, Redo, Subscribe or Open request.
type Result struct {
	OK       bool   `json:"ok"`
	Reason   string `json:"reason,omitempty"`
	Revision uint64 `json:"revision,omitempty"`
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

// Open asks the orchestrator to open a window.
type Open struct {
	View   string   `json:"view"`
	Target ids.ULID `json:"target"`
}

// Shutdown tells a window to clean up and close.
type Shutdown struct {
	Reason string `json:"reason"`
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
