package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// dialHello connects with a hello of the test's own, as the editor window or
// one of its panes does.
func (h *harness) dialHello(t testing.TB, hello protocol.Hello) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := client.Dial(ctx, h.socket, hello, newRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return connection
}

// editorWindow connects as the editor window's own connection, the project
// browser's, which the orchestrator asks to show every window it opens.
func (h *harness) editorWindow(t testing.TB) *client.Client {
	t.Helper()
	return h.dialHello(t, protocol.Hello{View: "browser", Target: project.IndexID(), Hosts: true})
}

// hostNext waits for the editor window to be asked for a pane, and connects
// that pane as the window the orchestrator opened.
func (h *harness) hostNext(t *testing.T, shell *client.Client) (*client.Client, protocol.Host) {
	t.Helper()
	asked := nextUpdate(t, shell, "a pane to show", func(u client.Update) bool { return u.Host != nil })
	pane := h.dialHello(t, protocol.Hello{
		Window: asked.Host.Window, View: asked.Host.View, Target: asked.Host.Target,
	})
	if pane.Window() != asked.Host.Window {
		t.Fatalf("pane joined as %s, not %s", pane.Window(), asked.Host.Window)
	}
	return pane, *asked.Host
}

// Increment 13: a window asked for is shown as a pane of the editor window.
// The pane connects as the window the orchestrator opened, so opening it
// again draws its attention and quitting counts it.
func TestEditorWindowShowsTheWindowsItIsAskedFor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	shell := h.editorWindow(t)
	asker := h.dialView(t, "text", project.IndexID(), "")

	if _, err := asker.Open(pickContext(t), orchestrator.MapView, h.floor, false); err != nil {
		t.Fatal(err)
	}
	pane, asked := h.hostNext(t, shell)
	if asked.View != orchestrator.MapView || asked.Target != h.floor {
		t.Fatalf("host %+v", asked)
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

// Increment 13: with the editor window not connected, there is nowhere to
// show a window, so opening one is refused.
func TestOpenWithoutTheEditorWindowIsRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	view := h.dial(t, "")
	result, err := view.Open(pickContext(t), orchestrator.MapView, h.floor, true)
	if err == nil || result.Reason != orchestrator.ErrNoWindow.Error() {
		t.Fatalf("open %+v %v", result, err)
	}
}

// D10: once a quit has closed every window, the orchestrator stops by
// itself, saving first.
func TestQuitStopsOnceEveryWindowHasLeft(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	shell := h.editorWindow(t)
	other := h.dial(t, "")
	if result, err := shell.Quit(pickContext(t), true); err != nil || !result.OK {
		t.Fatalf("quit %+v %v", result, err)
	}
	for _, window := range []*client.Client{shell, other} {
		nextUpdate(t, window, "the shutdown", func(u client.Update) bool { return u.Shutdown != nil })
		_ = window.Close()
	}
	if err := h.stopped(); err != nil {
		t.Fatal(err)
	}
}
