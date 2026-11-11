package orchestrator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// nextNotice waits for the next notice a window is sent.
func nextNotice(t *testing.T, c *client.Client) protocol.Notice {
	t.Helper()
	var notice protocol.Notice
	nextUpdate(t, c, "a notice", func(update client.Update) bool {
		if update.Session == nil || update.Session.Topic != protocol.TopicNotice {
			return false
		}
		if err := json.Unmarshal(update.Session.Value, &notice); err != nil {
			t.Fatal(err)
		}
		return true
	})
	return notice
}

// A save that fails is told to every window, not only written to the log,
// and a window opened while saves fail is told too. Once a save succeeds
// the windows are told that, and a window opened afterwards hears nothing.
func TestAFailedSaveIsToldUntilASaveSucceeds(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder this user cannot write to")
	}
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.FlushInterval = 20 * time.Millisecond })
	view := h.dial(t, "")
	floors := filepath.Join(dir, "tilefloors")
	if err := os.Chmod(floors, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(floors, 0o700) })
	mustPaint(t, view, h.floor, 1, 1, "wall", "")
	failed := nextNotice(t, view)
	if failed.Kind != protocol.NoticeSaveFailed || !strings.Contains(failed.Text, "permission denied") ||
		!strings.Contains(failed.Text, "journal") {
		t.Fatalf("a failed save says so, why, and where the changes are: %+v", failed)
	}
	if late := nextNotice(t, h.dial(t, "")); late.Kind != protocol.NoticeSaveFailed {
		t.Fatalf("a window opened while saves fail is told: %+v", late)
	}
	if readFloorFile(t, dir).Revision != 0 {
		t.Fatal("the floor's file was written after all")
	}

	if err := os.Chmod(floors, 0o700); err != nil {
		t.Fatal(err)
	}
	if saved := nextNotice(t, view); saved.Kind != protocol.NoticeSaved {
		t.Fatalf("a save that succeeds says so: %+v", saved)
	}
	if readFloorFile(t, dir).Revision != 1 {
		t.Fatal("the floor's file holds the paint once saving works")
	}
	after := h.dial(t, "")
	mustPaint(t, view, h.floor, 2, 2, "wall", "")
	waitFor(t, "the later window's copy", func() bool { return revision(after, h.floor) == 2 })
	select {
	case update := <-after.Updates():
		if update.Session != nil && update.Session.Topic == protocol.TopicNotice {
			t.Fatalf("a window opened after saving works is told nothing: %s", update.Session.Value)
		}
	default:
	}
}
