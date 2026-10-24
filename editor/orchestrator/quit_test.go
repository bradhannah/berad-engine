package orchestrator_test

import (
	"context"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// TestQuitNamesUnsavedChangesThenClosesEveryWindow reports one window's
// unsaved changes, asks what quitting would close from another, then quits
// with Save: every window is told to save and close.
func TestQuitNamesUnsavedChangesThenClosesEveryWindow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	editing := h.dialView(t, orchestrator.MapView, previewZone, "")
	quitting := h.dialView(t, orchestrator.WatchView, previewZone, "")
	if err := editing.Unsaved(protocol.Unsaved{Count: 2, Name: "the anchor Desk"}); err != nil {
		t.Fatal(err)
	}
	ask, err := quitting.QuitAsk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ask.Windows != 2 || len(ask.Unsaved) != 1 || ask.Unsaved[0].Name != "the anchor Desk" ||
		ask.Unsaved[0].Count != 2 || ask.Unsaved[0].View != orchestrator.MapView {
		t.Fatalf("quit ask %+v", ask)
	}
	if result, err := quitting.Quit(context.Background(), true); err != nil || !result.OK {
		t.Fatalf("quit %+v %v", result, err)
	}
	for _, window := range []*client.Client{editing, quitting} {
		got := nextUpdate(t, window, "the shutdown", func(u client.Update) bool { return u.Shutdown != nil })
		if !got.Shutdown.Save {
			t.Fatalf("shutdown %+v", got.Shutdown)
		}
	}
}
