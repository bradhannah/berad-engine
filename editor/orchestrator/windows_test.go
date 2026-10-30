package orchestrator_test

import (
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

// D10: stopping the orchestrator tells every window to clean up and close,
// and closes their connections.
func TestStopClosesWindowsCleanly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	panes := []*client.Client{h.dial(t, ""), h.dial(t, "")}

	started := time.Now()
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("stop took %v", elapsed)
	}
	for _, pane := range panes {
		nextUpdate(t, pane, "the shutdown", func(u client.Update) bool { return u.Shutdown != nil })
		select {
		case <-pane.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("a window's connection stayed open")
		}
	}
}

// D10, increment 13: a pane that fails tells the orchestrator as it closes;
// the orchestrator saves a crash report with its failure and recent traffic,
// and tells the other windows.
func TestPaneCrashIsReported(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	observer := h.dial(t, "")
	pane := h.dial(t, "")
	if err := pane.Crashed("simulated window crash\ngoroutine 1 [running]"); err != nil {
		t.Fatal(err)
	}
	_ = pane.Close()

	var notice protocol.Notice
	nextUpdate(t, observer, "the crash notice", func(u client.Update) bool {
		return u.Session != nil && u.Session.Topic == protocol.TopicNotice &&
			json.Unmarshal(u.Session.Value, &notice) == nil
	})
	if notice.Kind != protocol.NoticeWindowCrashed || notice.Report == "" || notice.Window != pane.Window() {
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
	if !strings.Contains(report.Failure, "simulated window crash") || report.View != orchestrator.MapView {
		t.Fatalf("report lacks the failure: %+v", report)
	}
	if len(report.Recent) == 0 || report.Window != pane.Window() {
		t.Fatalf("report lacks recent traffic: %+v", report)
	}
}
