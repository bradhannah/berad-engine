package client_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

var (
	floorID  = ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")
	windowID = ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHD")
)

// fakeOrchestrator accepts one connection and lets the test script replies.
type fakeOrchestrator struct {
	t        *testing.T
	accepted chan *protocol.Conn
}

func newFake(t *testing.T) *fakeOrchestrator {
	t.Helper()
	return &fakeOrchestrator{t: t, accepted: make(chan *protocol.Conn, 1)}
}

// connect opens an in-memory connection, as the orchestrator's Connect does,
// and hands its other end to the test.
func (f *fakeOrchestrator) connect(context.Context) (net.Conn, error) {
	pane, served := net.Pipe()
	f.t.Cleanup(func() { _ = served.Close() })
	f.accepted <- protocol.NewConn(served, protocol.JSONCodec{})
	return pane, nil
}

func (f *fakeOrchestrator) expect(conn *protocol.Conn, kind protocol.Kind) protocol.Message {
	f.t.Helper()
	message, err := conn.Receive()
	if err != nil || message.Kind != kind {
		f.t.Fatalf("expected %s, got %v %v", kind, message.Kind, err)
	}
	return message
}

func (f *fakeOrchestrator) send(conn *protocol.Conn, kind protocol.Kind, request uint64, payload any) {
	f.t.Helper()
	message, err := protocol.NewMessage(kind, request, payload)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := conn.Send(message); err != nil {
		f.t.Fatal(err)
	}
}

func snapshot(t *testing.T, revision uint64, tile string) protocol.Snapshot {
	t.Helper()
	document, err := tilefloor.NewDocument(4, 4, tile)
	if err != nil {
		t.Fatal(err)
	}
	data, err := document.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return protocol.Snapshot{Document: floorID, Format: "tilefloor", Name: "Floor", Revision: revision, Data: data}
}

func paintEvent(t *testing.T, revision uint64, tile string) protocol.Event {
	t.Helper()
	args, err := json.Marshal(tilefloor.PaintArgs{Cells: []tilefloor.Cell{{Col: 0, Row: 0, Layer: tilefloor.LayerGround, Tile: tile}}})
	if err != nil {
		t.Fatal(err)
	}
	return protocol.Event{Document: floorID, Revision: revision, Edit: tilefloor.PaintEdit, Args: args, Origin: windowID}
}

// D6: a window that misses a revision does not trust its copy; it asks for a
// fresh snapshot and carries on from there.
func TestMissedRevisionTriggersResubscribe(t *testing.T) {
	t.Parallel()
	fake := newFake(t)
	registry, err := plugin.NewRegistry(project.Plugin(), tilefloor.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialed := make(chan *client.Client, 1)
	go func() {
		connection, err := client.Dial(ctx, fake.connect, protocol.Hello{}, registry)
		if err != nil {
			t.Error(err)
		}
		dialed <- connection
	}()
	server := <-fake.accepted
	fake.expect(server, protocol.KindHello)
	fake.send(server, protocol.KindWelcome, 0, protocol.Welcome{Window: windowID})
	view := <-dialed
	defer view.Close()

	subscribed := make(chan error, 1)
	go func() { subscribed <- view.Subscribe(ctx, floorID) }()
	subscribe := fake.expect(server, protocol.KindSubscribe)
	fake.send(server, protocol.KindSnapshot, subscribe.Request, snapshot(t, 1, "grass"))
	if err := <-subscribed; err != nil {
		t.Fatal(err)
	}

	fake.send(server, protocol.KindEvent, 0, paintEvent(t, 2, "wall"))
	fake.send(server, protocol.KindEvent, 0, paintEvent(t, 4, "road"))
	resync := fake.expect(server, protocol.KindSubscribe)
	fake.send(server, protocol.KindSnapshot, resync.Request, snapshot(t, 4, "sand"))

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var revision uint64
		var tile string
		view.View(floorID, func(document client.Document) {
			revision = document.Revision
			tile, _ = document.Model.(*tilefloor.Document).Floor.Ground(1, 1)
		})
		if revision == 4 && tile == "sand" {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("client did not reload from a fresh snapshot after the gap")
}
