package orchestrator_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

func testLauncher(role, markers string) orchestrator.Launcher {
	return orchestrator.ExecutableLauncher{
		Path: os.Args[0],
		Env:  []string{envRole + "=" + role, envMarkerDir + "=" + markers},
	}
}

func openWindow(t *testing.T, view *client.Client, h *harness) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := view.Open(ctx, "map", h.floor); err != nil {
		t.Fatal(err)
	}
}

// D10: when a window crashes, the orchestrator saves a failure log with the
// window's output and recent traffic, and tells the other windows.
func TestWindowCrashIsReported(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.Launcher = testLauncher(roleCrashWindow, t.TempDir()) })
	observer := h.dial(t, "")
	openWindow(t, observer, h)

	var notice protocol.Notice
	deadline := time.After(15 * time.Second)
	for notice.Kind == "" {
		select {
		case update := <-observer.Updates():
			if update.Session != nil && update.Session.Topic == protocol.TopicNotice {
				if err := json.Unmarshal(update.Session.Value, &notice); err != nil {
					t.Fatal(err)
				}
			}
		case <-deadline:
			t.Fatal("no crash notice")
		}
	}
	if notice.Kind != protocol.NoticeWindowCrashed || notice.Report == "" {
		t.Fatalf("notice = %+v", notice)
	}
	raw, err := os.ReadFile(filepath.Join(dir, notice.Report))
	if err != nil {
		t.Fatal(err)
	}
	var report orchestrator.CrashReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.LogTail, "simulated window crash") {
		t.Fatalf("log tail lacks the panic: %q", report.LogTail)
	}
	if len(report.Recent) == 0 || report.Window != notice.Window {
		t.Fatalf("report lacks recent traffic: %+v", report)
	}
}

// D10: stopping the orchestrator tells every window to clean up and close; no
// window has to be killed.
func TestStopClosesWindowsCleanly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	markers := t.TempDir()
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.Launcher = testLauncher(roleIdleWindow, markers) })
	opener := h.dial(t, "")
	openWindow(t, opener, h)
	openWindow(t, opener, h)
	waitFor(t, "two windows to connect", func() bool {
		entries, _ := os.ReadDir(filepath.Join(dir, ".berad", "logs"))
		return len(entries) >= 2
	})
	time.Sleep(200 * time.Millisecond)

	started := time.Now()
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("stop took %v; windows were probably killed", elapsed)
	}
	clean, err := filepath.Glob(filepath.Join(markers, "*.clean"))
	if err != nil || len(clean) != 2 {
		t.Fatalf("clean exits %v %v", clean, err)
	}
}
