package protocol_test

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

var testID = ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")

// everyMessage returns one populated message of every kind, so the codec
// contract covers the whole protocol.
func everyMessage(t *testing.T) []protocol.Message {
	t.Helper()
	payloads := map[protocol.Kind]any{
		protocol.KindHello:     protocol.Hello{Protocol: protocol.Version, Build: "b", Window: testID, View: "map", Target: testID, UndoGroup: "g"},
		protocol.KindWelcome:   protocol.Welcome{Window: testID},
		protocol.KindSubscribe: protocol.Subscribe{Document: testID},
		protocol.KindSnapshot:  protocol.Snapshot{Document: testID, Format: "f", Name: "n", Revision: 3, Data: json.RawMessage(`{"a":1}`)},
		protocol.KindCommand:   protocol.Command{Document: testID, Edit: "e", Gesture: "g", Args: json.RawMessage(`{"b":2}`)},
		protocol.KindResult:    protocol.Result{OK: false, Reason: "why", Revision: 4},
		protocol.KindEvent:     protocol.Event{Document: testID, Revision: 5, Edit: "e", Args: json.RawMessage(`[1]`), Origin: testID, Request: 9},
		protocol.KindSession:   protocol.Session{Topic: "t", Value: json.RawMessage(`"v"`)},
		protocol.KindOpen:      protocol.Open{View: "map", Target: testID},
		protocol.KindShutdown:  protocol.Shutdown{Reason: "bye"},
		protocol.KindUndo:      nil,
		protocol.KindRedo:      nil,
	}
	messages := make([]protocol.Message, 0, len(payloads))
	for kind, payload := range payloads {
		message, err := protocol.NewMessage(kind, 7, payload)
		if err != nil {
			t.Fatal(err)
		}
		messages = append(messages, message)
	}
	return messages
}

func TestCodecRoundTripsEveryMessage(t *testing.T) {
	t.Parallel()
	codecs := []protocol.Codec{protocol.JSONCodec{}}
	for _, codec := range codecs {
		for _, message := range everyMessage(t) {
			data, err := codec.Marshal(message)
			if err != nil {
				t.Fatalf("%s %s: %v", codec.Name(), message.Kind, err)
			}
			decoded, err := codec.Unmarshal(data)
			if err != nil {
				t.Fatalf("%s %s: %v", codec.Name(), message.Kind, err)
			}
			if !reflect.DeepEqual(normalise(decoded), normalise(message)) {
				t.Fatalf("%s %s: %+v != %+v", codec.Name(), message.Kind, decoded, message)
			}
		}
	}
}

func normalise(message protocol.Message) protocol.Message {
	if len(message.Payload) == 0 {
		message.Payload = nil
	}
	return message
}

func TestConnSendsAndReceives(t *testing.T) {
	t.Parallel()
	left, right := net.Pipe()
	sender := protocol.NewConn(left, protocol.JSONCodec{})
	receiver := protocol.NewConn(right, protocol.JSONCodec{})
	messages := everyMessage(t)
	go func() {
		for _, message := range messages {
			if err := sender.Send(message); err != nil {
				t.Error(err)
				return
			}
		}
		_ = sender.Close()
	}()
	for _, want := range messages {
		got, err := receiver.Receive()
		if err != nil {
			t.Fatal(err)
		}
		if got.Kind != want.Kind {
			t.Fatalf("kind %s != %s", got.Kind, want.Kind)
		}
	}
	if _, err := receiver.Receive(); !errors.Is(err, io.EOF) {
		t.Fatalf("after close: %v", err)
	}
}

func TestDecodePayload(t *testing.T) {
	t.Parallel()
	message, err := protocol.NewMessage(protocol.KindSubscribe, 1, protocol.Subscribe{Document: testID})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := protocol.DecodePayload[protocol.Subscribe](message)
	if err != nil || payload.Document != testID {
		t.Fatalf("payload %+v %v", payload, err)
	}
	empty := protocol.Message{Kind: protocol.KindSubscribe, Request: 0, Payload: nil}
	if _, err := protocol.DecodePayload[protocol.Subscribe](empty); !errors.Is(err, protocol.ErrPayload) {
		t.Fatalf("empty: %v", err)
	}
}

func TestOversizedFrameRejected(t *testing.T) {
	t.Parallel()
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	sender := protocol.NewConn(left, protocol.JSONCodec{})
	big := protocol.Message{Kind: protocol.KindSession, Request: 0, Payload: json.RawMessage(`"` + string(make([]byte, protocol.MaxFrame)) + `"`)}
	if err := sender.Send(big); err == nil {
		t.Fatal("oversized frame sent")
	}
}
