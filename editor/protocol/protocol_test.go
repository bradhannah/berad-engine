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
		protocol.KindResult:    protocol.Result{OK: false, Reason: "why", Revision: 4, Created: testID, Item: "door/open"},
		protocol.KindEvent:     protocol.Event{Document: testID, Revision: 5, Edit: "e", Args: json.RawMessage(`[1]`), Origin: testID, Request: 9},
		protocol.KindSession:   protocol.Session{Topic: "t", Value: json.RawMessage(`"v"`)},
		protocol.KindOpen:      protocol.Open{View: "map", Target: testID, NewView: true, Item: "door/open"},
		protocol.KindShutdown:  protocol.Shutdown{Reason: "bye", Save: true},
		protocol.KindUnsaved:   protocol.Unsaved{Count: 2, Name: "NPC 1"},
		protocol.KindQuitAsk: protocol.QuitAsk{Windows: 3, Unsaved: []protocol.UnsavedWindow{
			{Window: testID, View: "browser", Name: "NPC 1", Count: 2},
		}},
		protocol.KindQuit:         protocol.Quit{Save: true},
		protocol.KindUndo:         nil,
		protocol.KindRedo:         nil,
		protocol.KindQuery:        protocol.Query{Question: protocol.QuestionWhereUsed, Document: testID, Item: "i"},
		protocol.KindAnswer:       protocol.Answer{Uses: []protocol.Use{{Document: testID, Name: "n", Format: "f", Label: "l"}}},
		protocol.KindAttention:    protocol.Attention{Item: "door/open"},
		protocol.KindPickAsk:      protocol.PickAsk{What: protocol.PickAnchor, Maps: []protocol.PickMap{{Window: testID, Zone: testID, Shows: true}}},
		protocol.KindPickStart:    protocol.PickStart{Pick: "p", What: protocol.PickAnchor, Label: "l", Window: testID, Zone: testID, From: testID},
		protocol.KindPickAnswer:   protocol.PickAnswer{Pick: "p", Floor: testID, Item: "i"},
		protocol.KindPickCancel:   protocol.PickCancel{Pick: "p", Reason: "r"},
		protocol.KindPreviewStart: protocol.PreviewStart{Zone: testID, Run: testID, Window: testID},
		protocol.KindPreviewState: protocol.PreviewState{Run: testID, Tick: 12, Entities: []protocol.PreviewEntity{
			{ID: protocol.EntityPlayer, Floor: testID, X: 1.5, Y: -2, Face: [2]int{1, -1}},
			{
				ID: testID.String(), Floor: testID, X: 3, Y: 4, Face: [2]int{0, 1}, NPC: testID, Workflow: testID,
				Node: testID, Activity: "Wait 1 s of 2 s (game time)", Status: "s",
				Last: protocol.PreviewChange{From: testID, Output: "next", To: testID, Tick: 9, Status: "b"}, Changes: 3,
			},
		}, Clock: 8 * 3600, Rate: 60, ClockStopped: true, NPCsPaused: true, Watched: testID, Stepping: true,
			Step: []protocol.PreviewChange{{From: testID, Output: "o1", To: testID, Tick: 9, Status: "b"}}},
		protocol.KindPreviewStop: protocol.PreviewStop{Run: testID, Reason: "r"},
		protocol.KindAudioLibrary: protocol.AudioLibrary{Problem: "p", Cues: []protocol.AudioCue{{
			ID: "door/open", Name: "Door opens", Category: protocol.CategorySFX, CategorySet: true, Gain: 0.8,
			HowToHear: "Open a door.", SearchTerms: []string{"hinge"},
			Recordings: []protocol.AudioRecording{{
				ID: "door_a", Extension: ".wav", Version: "v", Size: 9, CalibrationDB: -4.5, StartMs: 10, EndMs: 900,
				Status: protocol.RecordingReady, File: "imported/door_a_01.wav", OriginalFile: "door A.wav",
				Note: "first take", SearchTerms: []string{"creak"},
				Attribution: protocol.Attribution{Creator: "c", Source: "https://example.com", License: "CC0-1.0",
					Commercial: true},
			}},
		}}, Schema: 4, Unused: []protocol.AudioRecording{{ID: "spare", Status: protocol.RecordingNoFile,
			Attribution: protocol.Attribution{Creator: protocol.Unknown, Source: protocol.Unknown,
				License: protocol.Unknown}}}},
		protocol.KindAudioEdit: protocol.AudioEdit{Edit: protocol.AudioAddSound, Gesture: "g",
			Args: json.RawMessage(`{"cue":"door/open","path":"/sounds/door.wav","attribution":{"creator":"c"}}`)},
		protocol.KindAudioChanged: protocol.AudioChanged{Revision: 3},
		protocol.KindRecording:    protocol.Recording{ID: "door_a", Offset: 3, Version: "v", Size: 9, Data: []byte{1, 2, 3}},
		protocol.KindPreviewControl: protocol.PreviewControl{
			Run: testID, Action: protocol.ControlJump, Placement: testID, Rate: 60, Seconds: 9 * 3600,
		},
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

// TestPayloadsRoundTrip checks that every payload type decodes back to the
// value that was encoded, so no field is lost on the wire.
func TestPayloadsRoundTrip(t *testing.T) {
	t.Parallel()
	for _, message := range everyMessage(t) {
		if len(message.Payload) == 0 {
			continue
		}
		var check func() (any, any, error)
		switch message.Kind {
		case protocol.KindResult:
			check = roundTrip[protocol.Result](message)
		case protocol.KindOpen:
			check = roundTrip[protocol.Open](message)
		case protocol.KindQuery:
			check = roundTrip[protocol.Query](message)
		case protocol.KindAnswer:
			check = roundTrip[protocol.Answer](message)
		case protocol.KindPickAsk:
			check = roundTrip[protocol.PickAsk](message)
		case protocol.KindPickStart:
			check = roundTrip[protocol.PickStart](message)
		case protocol.KindPickAnswer:
			check = roundTrip[protocol.PickAnswer](message)
		case protocol.KindPickCancel:
			check = roundTrip[protocol.PickCancel](message)
		case protocol.KindPreviewStart:
			check = roundTrip[protocol.PreviewStart](message)
		case protocol.KindPreviewState:
			check = roundTrip[protocol.PreviewState](message)
		case protocol.KindPreviewStop:
			check = roundTrip[protocol.PreviewStop](message)
		case protocol.KindPreviewControl:
			check = roundTrip[protocol.PreviewControl](message)
		case protocol.KindShutdown:
			check = roundTrip[protocol.Shutdown](message)
		case protocol.KindUnsaved:
			check = roundTrip[protocol.Unsaved](message)
		case protocol.KindQuitAsk:
			check = roundTrip[protocol.QuitAsk](message)
		case protocol.KindQuit:
			check = roundTrip[protocol.Quit](message)
		case protocol.KindHello:
			check = roundTrip[protocol.Hello](message)
		case protocol.KindEvent:
			check = roundTrip[protocol.Event](message)
		case protocol.KindAudioLibrary:
			check = roundTrip[protocol.AudioLibrary](message)
		case protocol.KindRecording:
			check = roundTrip[protocol.Recording](message)
		case protocol.KindAttention:
			check = roundTrip[protocol.Attention](message)
		case protocol.KindAudioEdit:
			check = roundTrip[protocol.AudioEdit](message)
		case protocol.KindAudioChanged:
			check = roundTrip[protocol.AudioChanged](message)
		default:
			continue
		}
		decoded, again, err := check()
		if err != nil || !reflect.DeepEqual(decoded, again) {
			t.Fatalf("%s: %+v != %+v (%v)", message.Kind, decoded, again, err)
		}
	}
}

// TestAudioEditArgsRoundTrip checks that the arguments of every audio edit
// decode back to the value that was encoded.
func TestAudioEditArgsRoundTrip(t *testing.T) {
	t.Parallel()
	name, gain, terms := "Door opens", 0.5, []string{"hinge"}
	attribution := protocol.Attribution{Creator: "c", Source: "s", License: "l", Commercial: true}
	for edit, args := range map[string]any{
		protocol.AudioCreateCue: protocol.NewCue{ID: "door/open", Name: name, Category: protocol.CategorySFX,
			HowToHear: "Open a door."},
		protocol.AudioAddSound: protocol.AddSound{Cue: "", NewCue: &protocol.NewCue{ID: "door/open", Name: name,
			Category: "", HowToHear: "h"}, Path: "/sounds/door.wav", Attribution: attribution},
		protocol.AudioReplaceFile: protocol.ReplaceFile{Recording: "door_a", Path: "/sounds/door.wav",
			Attribution: attribution},
		protocol.AudioSetCue: protocol.CueChange{Cue: "door/open", Name: &name, Gain: &gain, HowToHear: &name,
			SearchTerms: &terms},
		protocol.AudioSetCategory:   protocol.CategoryChange{Cue: "door/open", Category: protocol.CategoryMusic},
		protocol.AudioSetRecordings: protocol.CueRecordings{Cue: "door/open", Recordings: []string{"a", "b"}},
		protocol.AudioSetRecording:  protocol.RecordingChange{Recording: "door_a", Note: &name, SearchTerms: &terms},
		protocol.AudioSetAttribution: protocol.AttributionChange{Recording: "door_a",
			Attribution: attribution},
		protocol.AudioDeleteCue:       protocol.AudioItem{ID: "door/open"},
		protocol.AudioDeleteRecording: protocol.AudioItem{ID: "door_a"},
	} {
		data, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		decoded := reflect.New(reflect.TypeOf(args))
		if err := json.Unmarshal(data, decoded.Interface()); err != nil {
			t.Fatal(edit, err)
		}
		if !reflect.DeepEqual(decoded.Elem().Interface(), args) {
			t.Fatalf("%s: %+v != %+v", edit, decoded.Elem().Interface(), args)
		}
	}
}

// roundTrip decodes the payload of a message, encodes the result again and
// decodes that, so the two decoded values must match.
func roundTrip[T any](message protocol.Message) func() (any, any, error) {
	return func() (any, any, error) {
		first, err := protocol.DecodePayload[T](message)
		if err != nil {
			return nil, nil, err
		}
		encoded, err := protocol.NewMessage(message.Kind, message.Request, first)
		if err != nil {
			return nil, nil, err
		}
		second, err := protocol.DecodePayload[T](encoded)
		return first, second, err
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
