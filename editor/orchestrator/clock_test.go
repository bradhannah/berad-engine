package orchestrator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The Clock window shows the preview of whichever zone runs one: it is told
// the preview started and its latest state, its controls reach the window
// running it, and the time the next preview starts at is shared with every
// window, including one that opens later.
func TestClockWindowShowsAnyZonesPreview(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	owner := h.dialView(t, orchestrator.MapView, previewZone, "map:"+previewZone.String())
	clockWindow := h.dialView(t, orchestrator.ClockView, ids.ULID{}, "")

	start, err := json.Marshal(protocol.ClockStart{Seconds: 9 * 3600})
	if err != nil {
		t.Fatal(err)
	}
	if err := clockWindow.Publish(protocol.TopicClockStart, start); err != nil {
		t.Fatal(err)
	}
	shared := nextUpdate(t, owner, "the start time", func(u client.Update) bool {
		return u.Session != nil && u.Session.Topic == protocol.TopicClockStart
	})
	if string(shared.Session.Value) != string(start) {
		t.Fatalf("start time %s", shared.Session.Value)
	}
	run, err := startPreview(t, owner, previewZone)
	if err != nil {
		t.Fatal(err)
	}
	nextUpdate(t, clockWindow, "the preview start", func(u client.Update) bool { return u.PreviewStart != nil })
	if err := owner.PreviewState(playerAt(run, 1, 0)); err != nil {
		t.Fatal(err)
	}
	nextUpdate(t, clockWindow, "the state", func(u client.Update) bool { return u.PreviewState != nil })
	jump := protocol.PreviewControl{Run: run, Action: protocol.ControlJump, Placement: ids.ULID{}, Rate: 0, Seconds: 3600}
	if err := clockWindow.PreviewControl(jump); err != nil {
		t.Fatal(err)
	}
	control := nextUpdate(t, owner, "the jump", func(u client.Update) bool { return u.PreviewControl != nil })
	if *control.PreviewControl != jump {
		t.Fatalf("control %+v", control.PreviewControl)
	}
	later := h.dialView(t, orchestrator.MapView, otherZone, "map:"+otherZone.String())
	nextUpdate(t, later, "the start time for a window opened later", func(u client.Update) bool {
		return u.Session != nil && u.Session.Topic == protocol.TopicClockStart
	})
}

// Starting a preview opens the Clock window when none is open, and only
// then.
func TestPreviewOpensTheClockWindow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	markers := t.TempDir()
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.Launcher = testLauncher(roleIdleWindow, markers) })
	owner := h.dialView(t, orchestrator.MapView, previewZone, "map:"+previewZone.String())
	windows := func() int {
		ask, err := owner.QuitAsk(pickContext(t))
		if err != nil {
			t.Fatal(err)
		}

		return ask.Windows
	}
	run, err := startPreview(t, owner, previewZone)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the Clock window to connect", func() bool { return windows() == 2 })
	if err := owner.PreviewStop(protocol.PreviewStop{Run: run, Reason: "stopped"}); err != nil {
		t.Fatal(err)
	}
	if _, err := startPreview(t, owner, previewZone); err != nil {
		t.Fatal(err)
	}
	logs, err := os.ReadDir(filepath.Join(dir, ".berad", "logs"))
	if err != nil || len(logs) != 1 || windows() != 2 {
		t.Fatalf("one Clock window: %d logs, %d windows, %v", len(logs), windows(), err)
	}
}
