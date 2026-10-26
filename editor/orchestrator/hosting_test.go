package orchestrator_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// dialHello connects with a hello of the test's own, as a shell or its pane
// does.
func (h *harness) dialHello(t testing.TB, hello protocol.Hello) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hello.Build = testBuild
	connection, err := client.Dial(ctx, h.socket, hello, newRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return connection
}

// Increment 10: with a shell connected, a window asked for is shown as one of
// its panes rather than started as a process. The pane connects as the
// window the orchestrator opened, so opening it again draws its attention
// and quitting counts it.
func TestShellHostsTheWindowsItIsAskedFor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	shell := h.dialHello(t, protocol.Hello{View: "browser", Target: project.IndexID(), Hosts: true})
	asker := h.dialView(t, "text", ids.ULID{}, "")

	if _, err := asker.Open(pickContext(t), orchestrator.MapView, h.floor, false); err != nil {
		t.Fatal(err)
	}
	asked := nextUpdate(t, shell, "the map to host", func(u client.Update) bool { return u.Host != nil })
	if asked.Host.View != orchestrator.MapView || asked.Host.Target != h.floor || asked.Host.Window.IsZero() {
		t.Fatalf("host %+v", asked.Host)
	}
	pane := h.dialHello(t, protocol.Hello{
		Window: asked.Host.Window, View: orchestrator.MapView, Target: h.floor, UndoGroup: "map:" + h.floor.String(),
	})
	if pane.Window() != asked.Host.Window {
		t.Fatalf("pane joined as %s, not %s", pane.Window(), asked.Host.Window)
	}
	if _, err := asker.OpenItem(pickContext(t), orchestrator.MapView, h.floor, "anchor"); err != nil {
		t.Fatal(err)
	}
	attention := nextUpdate(t, pane, "attention", func(u client.Update) bool { return u.Attention })
	if attention.Item != "anchor" {
		t.Fatalf("attention %+v", attention)
	}
	ask, err := asker.QuitAsk(pickContext(t))
	if err != nil || ask.Windows != 3 {
		t.Fatalf("quit ask %+v %v", ask, err)
	}
}

// Increment 10: popping a pane out opens its window as a process of its own,
// even with a shell connected, and closes the pane once that window has
// joined.
func TestPopOutClosesThePaneOnceItsWindowJoins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	markers := t.TempDir()
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.Launcher = testLauncher(roleIdleWindow, markers) })
	shell := h.dialHello(t, protocol.Hello{View: "browser", Target: project.IndexID(), Hosts: true})
	if _, err := shell.Open(pickContext(t), orchestrator.MapView, h.floor, false); err != nil {
		t.Fatal(err)
	}
	asked := nextUpdate(t, shell, "the map to host", func(u client.Update) bool { return u.Host != nil })
	pane := h.dialHello(t, protocol.Hello{Window: asked.Host.Window, View: orchestrator.MapView, Target: h.floor})

	result, err := shell.PopOut(pickContext(t), orchestrator.MapView, h.floor, pane.Window())
	if err != nil || !result.OK {
		t.Fatalf("pop out %+v %v", result, err)
	}
	closing := nextUpdate(t, pane, "the pane closing", func(u client.Update) bool { return u.Shutdown != nil })
	if !closing.Shutdown.Save {
		t.Fatalf("shutdown %+v", closing.Shutdown)
	}
	select {
	case update := <-shell.Updates():
		if update.Host != nil {
			t.Fatalf("the popped-out window was hosted: %+v", update.Host)
		}
	default:
	}
}

// Increment 10: closing the shell's process quits the editor, and tells any
// window still open on its own to close.
func TestShellClosingQuitsTheEditor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	markers := t.TempDir()
	cfg := baseConfig(t, dir, shortSocket(t))
	cfg.Launcher = testLauncher(roleShellWindow, markers)
	// Without this, only the shell closing can quit the editor.
	cfg.QuitWhenLastWindowCloses = false
	orch, err := orchestrator.New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orch.LaunchShell("browser", project.IndexID()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- orch.Run(context.Background()) }()
	h := &harness{t: t, dir: dir, socket: cfg.SocketPath, floor: ids.ULID{}}
	other := h.dialView(t, "text", ids.ULID{}, "")
	waitFor(t, "the shell to join", func() bool {
		ask, err := other.QuitAsk(pickContext(t))
		return err == nil && ask.Windows == 2
	})

	if err := os.WriteFile(filepath.Join(markers, closeShell), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	closing := nextUpdate(t, other, "the other window closing", func(u client.Update) bool { return u.Shutdown != nil })
	if !closing.Shutdown.Save {
		t.Fatalf("shutdown %+v", closing.Shutdown)
	}
	_ = other.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the editor did not quit")
	}
}
