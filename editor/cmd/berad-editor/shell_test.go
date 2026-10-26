package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
)

// runShellEditor opens the sample project as one window (increment 10) with
// scripted panes, and waits for the editor to quit by itself.
func runShellEditor(t *testing.T, binary, project string, panes scripts) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "open", "--project", project, "--sample", "--shell")
	cmd.Env = append(os.Environ(), panes.env(t))
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("editor did not quit after its window closed\n%s", output)
	}
	if err != nil {
		t.Fatalf("editor: %v\n%s", err, output)
	}
}

// shellLog returns the log of the editor's only window process, failing the
// test if there was another, or if a pane crashed: a pane's failure closes
// it alone, so it leaves no crash report, only a line in this log.
func shellLog(t *testing.T, project string) string {
	t.Helper()
	logs, err := filepath.Glob(filepath.Join(project, ".berad", "logs", "*.log"))
	if err != nil || len(logs) != 1 {
		t.Fatalf("window processes' logs: %v %v", logs, err)
	}
	raw, err := os.ReadFile(logs[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "crashed") {
		t.Fatalf("a pane crashed:\n%.2000s", raw)
	}
	return string(raw)
}

// TestShellShowsWindowsAsTabs opens the editor as one window (increment 10).
// The project browser opens a Map, a text document and the Audio window,
// each as a tab of its own window, and no other window process starts. The
// Map paints while another tab is in front, so hidden tabs keep working, and
// closing the project browser's tab quits the editor after asking.
func TestShellShowsWindowsAsTabs(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "shell-tabs.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Sample zone"), wait(60),
		doubleClick("row:Notes"), wait(60), must("section:audio", 600), doubleClick("row:Hum"), wait(240),
		step{Repeat: 2, Screenshot: shot}, keys("Cmd+W"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	mapScript := script(must("state:loaded", 600), wait(240), keys("3"), drag("cell:4,8", "cell:5,8"), wait(30))
	audioScript := script(must("state:selected=cue:sample/hum", 900))
	runShellEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {mapScript}, "audio": {audioScript},
	})
	noCrashes(t, project)
	shellLog(t, project)

	ground, _ := readFloor(t, project, "ground-floor")
	for col := 4; col <= 5; col++ {
		if got := cellOf(t, ground, tilefloor.LayerGround, col, 8); got != "road" {
			t.Fatalf("ground cell %d painted from a hidden tab = %q", col, got)
		}
	}
	t.Logf("screenshot: %s", shot)
}

// TestShellPinsTheClock previews the sample zone in a Map tab, then brings a
// text document's tab to the front (increment 10). The pinned clock reads
// the preview out on the bar, its panel opens by itself, and its +1h moves
// the hidden Map's preview on an hour.
func TestShellPinsTheClock(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "shell-clock.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Sample zone"), wait(90), doubleClick("row:Notes"),
		wait(300), step{Repeat: 2, Screenshot: shot}, keys("Cmd+W"), must("ui:quit", 300), wait(5),
		click("ui:quit"), wait(60),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("P"), click("cell:2,-2"), must("state:previewing", 300),
	)
	clockScript := script(
		must("state:running", 1200), wait(10), click("ui:hour-on"), must("state:time:09:", 300), wait(10),
	)
	runShellEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {mapScript}, "clock": {clockScript},
	})
	noCrashes(t, project)
	shellLog(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestShellShowsWatchBesideItsMap previews the sample zone and asks for Watch
// from the Map (increment 10): Watch opens beside the Map, in its tab, and
// follows the preview the Map still runs.
func TestShellShowsWatchBesideItsMap(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "shell-split.png")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Sample zone"), wait(600),
		keys("Cmd+W"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("P"), click("cell:2,-2"), must("state:previewing", 300),
		click("ui:watch"), wait(120), step{Repeat: 2, Screenshot: shot}, walk("ArrowRight"), wait(10),
	)
	watchScript := script(must("state:previewing", 900), wait(10))
	runShellEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {mapScript}, "watch": {watchScript}})
	noCrashes(t, project)
	shellLog(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestShellPopsATabOut paints two strokes in a Map tab, then pops the tab out
// with Cmd+Shift+O (increment 10): the Map opens in a window process of its
// own, the tab closes once that window has joined, and undo in the new window
// takes back the second stroke made in the tab, because their undo list
// carried over, leaving the first.
func TestShellPopsATabOut(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	browserScript := script(
		must("row:Notes", 600), wait(5), doubleClick("row:Sample zone"), wait(600),
		keys("Cmd+W"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	inTab := script(
		must("state:loaded", 600), wait(5), keys("3"), drag("cell:4,8", "cell:5,8"), wait(30),
		drag("cell:6,8", "cell:7,8"), wait(30), keys("Cmd+Shift+O"), wait(60),
	)
	poppedOut := script(must("state:loaded", 600), wait(30), keys("Cmd+Z"), wait(60))
	runShellEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {inTab, poppedOut}})
	noCrashes(t, project)

	logs, err := filepath.Glob(filepath.Join(project, ".berad", "logs", "*.log"))
	if err != nil || len(logs) != 2 {
		t.Fatalf("the shell and the popped-out window: %v %v", logs, err)
	}
	ground, _ := readFloor(t, project, "ground-floor")
	for col := 4; col <= 7; col++ {
		if got, first := cellOf(t, ground, tilefloor.LayerGround, col, 8), col <= 5; (got == "road") != first {
			t.Fatalf("ground cell %d = %q: the first stroke stays, undo in the popped-out window takes the second",
				col, got)
		}
	}
}
