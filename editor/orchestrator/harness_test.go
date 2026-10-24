package orchestrator_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const testBuild = "test-build"

func TestMain(m *testing.M) {
	if role := os.Getenv(envRole); role != "" {
		os.Exit(runChildRole(role))
	}
	os.Exit(m.Run())
}

func newRegistry(t testing.TB) *plugin.Registry {
	t.Helper()
	registry, err := plugin.NewRegistry(
		project.Plugin(), tilefloor.Plugin(), zone.Plugin(), textdoc.Plugin(), workflow.Plugin(), npc.Plugin(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// shortSocket returns a socket path short enough for macOS's 104-byte limit.
func shortSocket(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "berad")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s.sock")
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type harness struct {
	t      testing.TB
	dir    string
	socket string
	floor  ids.ULID
	cancel context.CancelFunc
	done   chan error
	once   sync.Once
	err    error
}

func baseConfig(t testing.TB, dir, socket string) orchestrator.Config {
	t.Helper()
	return orchestrator.Config{
		ProjectDir:    dir,
		Registry:      newRegistry(t),
		Build:         testBuild,
		SocketPath:    socket,
		FlushInterval: time.Hour,
		Logger:        quietLogger(),
	}
}

// createProject makes a project with one 16x16 grass floor and returns its ID.
func createProject(t testing.TB, dir string) ids.ULID {
	t.Helper()
	orch, err := orchestrator.New(context.Background(), baseConfig(t, dir, shortSocket(t)))
	if err != nil {
		t.Fatal(err)
	}
	document, err := tilefloor.NewDocument(16, 16, "grass")
	if err != nil {
		t.Fatal(err)
	}
	id, err := orch.CreateDocument("Floor 1", document)
	if err != nil {
		t.Fatal(err)
	}
	if err := orch.Close(); err != nil {
		t.Fatal(err)
	}
	return id
}

// start runs an orchestrator for a project created with createProject.
func start(t testing.TB, dir string, configure func(*orchestrator.Config)) *harness {
	t.Helper()
	socket := shortSocket(t)
	cfg := baseConfig(t, dir, socket)
	if configure != nil {
		configure(&cfg)
	}
	orch, err := orchestrator.New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := orch.Documents()
	if err != nil || len(documents) == 0 {
		t.Fatalf("documents: %v %v", documents, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &harness{t: t, dir: dir, socket: cfg.SocketPath, floor: documents[0].ID, cancel: cancel, done: make(chan error, 1)}
	go func() { h.done <- orch.Run(ctx) }()
	t.Cleanup(func() { _ = h.stop() })
	return h
}

func (h *harness) stop() error {
	h.once.Do(func() {
		h.cancel()
		select {
		case h.err = <-h.done:
		case <-time.After(10 * time.Second):
			h.t.Error("orchestrator did not stop")
		}
	})
	return h.err
}

func (h *harness) dial(t testing.TB, group string) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := client.Dial(ctx, h.socket, protocol.Hello{
		Build: testBuild, View: "map", Target: h.floor, UndoGroup: group,
	}, newRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := connection.Subscribe(ctx, h.floor); err != nil {
		t.Fatal(err)
	}
	return connection
}

func paintArgs(t testing.TB, cells ...tilefloor.Cell) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(tilefloor.PaintArgs{Cells: cells})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func paint(t testing.TB, c *client.Client, floor ids.ULID, col, row int, tile, gesture string) (protocol.Result, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.Command(ctx, protocol.Command{
		Document: floor, Edit: tilefloor.PaintEdit, Gesture: gesture,
		Args: paintArgs(t, tilefloor.Cell{Col: col, Row: row, Layer: tilefloor.LayerGround, Tile: tile}),
	})
}

func mustPaint(t testing.TB, c *client.Client, floor ids.ULID, col, row int, tile, gesture string) protocol.Result {
	t.Helper()
	result, err := paint(t, c, floor, col, row, tile, gesture)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func tileAt(t testing.TB, c *client.Client, floor ids.ULID, col, row int) string {
	t.Helper()
	var tile string
	ok := c.View(floor, func(document client.Document) {
		var err error
		tile, err = document.Model.(*tilefloor.Document).Floor.Ground(col, row)
		if err != nil {
			t.Error(err)
		}
	})
	if !ok {
		t.Fatal("document not loaded")
	}
	return tile
}

func revision(c *client.Client, floor ids.ULID) uint64 {
	var rev uint64
	c.View(floor, func(document client.Document) { rev = document.Revision })
	return rev
}

// waitFor polls until cond holds or fails the test after a few seconds.
func waitFor(t testing.TB, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func undo(t testing.TB, c *client.Client) (protocol.Result, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.Undo(ctx)
}

func redo(t testing.TB, c *client.Client) (protocol.Result, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.Redo(ctx)
}
