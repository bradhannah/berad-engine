package main_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/engine/content"
)

// buildEditor compiles berad-editor once per test into a temporary folder.
func buildEditor(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "berad-editor")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	return binary
}

func floorTiles(t *testing.T, project string) func(col, row int) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(project, "tilefloors", "*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("floor files %v %v", matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := content.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	document, err := tilefloor.Decode(envelope.Data)
	if err != nil {
		t.Fatal(err)
	}
	floor := document.(*tilefloor.Document).Floor
	return func(col, row int) string {
		tile, err := floor.At(col, row)
		if err != nil {
			t.Fatal(err)
		}
		return tile
	}
}

// TestHeadlessOpenCreatesAndSavesSample runs the real program without a window.
func TestHeadlessOpenCreatesAndSavesSample(t *testing.T) {
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	cmd := exec.Command(binary, "open", "--project", project, "--sample", "--no-window", "--exit-after", "1s")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("open: %v\n%s", err, output)
	}
	tile := floorTiles(t, project)
	if tile(6, 5) != "wall" || tile(0, 0) != "grass" {
		t.Fatal("sample floor not saved")
	}
}

type step struct {
	Repeat   int     `json:"repeat,omitempty"`
	Cell     *[2]int `json:"cell,omitempty"`
	Down     bool    `json:"down,omitempty"`
	Pressed  bool    `json:"pressed,omitempty"`
	Released bool    `json:"released,omitempty"`
	Undo     bool    `json:"undo,omitempty"`
	NewView  bool    `json:"newView,omitempty"`
	Quit     bool    `json:"quit,omitempty"`
	Crash    bool    `json:"crash,omitempty"`
	TileKey  int     `json:"tileKey,omitempty"`
}

func cell(col, row int) *[2]int { return &[2]int{col, row} }

func wait(frames int) step { return step{Repeat: frames} }

// stroke presses on the first cell, drags through the rest and releases.
func stroke(tileKey int, cells ...[2]int) []step {
	steps := []step{{Repeat: 2, TileKey: tileKey}}
	for index, position := range cells {
		steps = append(steps, step{Repeat: 3, Cell: cell(position[0], position[1]), Down: true, Pressed: index == 0})
	}
	last := cells[len(cells)-1]
	return append(steps, step{Repeat: 2, Cell: cell(last[0], last[1]), Released: true})
}

func writeScript(t *testing.T, name string, steps []step) string {
	t.Helper()
	raw, err := json.Marshal(steps)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func requireGUI(t *testing.T) {
	t.Helper()
	if os.Getenv("BERAD_GUI_TESTS") == "" {
		t.Skip("set BERAD_GUI_TESTS=1 to run tests that open real windows")
	}
}

// runEditor opens the project with scripted windows and waits for the editor
// to quit by itself once its last window closes.
func runEditor(t *testing.T, binary, project string, scripts ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	screenshot := filepath.Join(t.TempDir(), "window.png")
	cmd := exec.CommandContext(ctx, binary, "open", "--project", project, "--sample")
	cmd.Env = append(os.Environ(), "BERAD_INPUT_SCRIPT="+strings.Join(scripts, ","), "BERAD_SCREENSHOT="+screenshot)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("editor did not quit after its windows closed\n%s", output)
	}
	if err != nil {
		t.Fatalf("editor: %v\n%s", err, output)
	}
	return screenshot
}

// TestWindowPaintsAndUndoesAStroke drives a real Map window: a drag paints
// through the orchestrator, a second stroke is undone in one step, and the
// result is in the saved file after the window closes.
func TestWindowPaintsAndUndoesAStroke(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	var steps []step
	steps = append(steps, wait(10))
	steps = append(steps, stroke(3, [2]int{20, 18}, [2]int{21, 18}, [2]int{22, 18}, [2]int{23, 18})...)
	steps = append(steps, wait(10))
	steps = append(steps, stroke(5, [2]int{20, 20}, [2]int{21, 20}, [2]int{22, 20})...)
	steps = append(steps, wait(10), step{Undo: true, Repeat: 40}, wait(90), step{Quit: true})
	screenshot := runEditor(t, binary, project, writeScript(t, "window.json", steps))

	tile := floorTiles(t, project)
	for col := 20; col <= 23; col++ {
		if tile(col, 18) != "wall" {
			t.Fatalf("stroke 1 cell %d = %q", col, tile(col, 18))
		}
	}
	for col := 20; col <= 22; col++ {
		if tile(col, 20) != "grass" {
			t.Fatalf("undone stroke 2 cell %d = %q", col, tile(col, 20))
		}
	}
	if _, err := os.Stat(screenshot); err != nil {
		t.Fatalf("no screenshot: %v", err)
	}
	t.Logf("screenshot: %s", screenshot)
}

// TestViewsShareUndo opens a second view with Cmd+N; the second view undoes
// the first view's stroke, because views of one map share one undo list.
func TestViewsShareUndo(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	var first []step
	first = append(first, wait(10))
	first = append(first, stroke(4, [2]int{2, 20}, [2]int{3, 20}, [2]int{4, 20})...)
	first = append(first, wait(10), step{NewView: true}, wait(300), step{Quit: true})
	second := []step{wait(30), {Undo: true, Repeat: 30}, wait(30), {Quit: true}}
	runEditor(t, binary, project, writeScript(t, "first.json", first), writeScript(t, "second.json", second))

	tile := floorTiles(t, project)
	for col := 2; col <= 4; col++ {
		if tile(col, 20) != "grass" {
			t.Fatalf("cell %d = %q; the second view should have undone the first view's stroke", col, tile(col, 20))
		}
	}
}

// TestCrashedWindowLeavesAReport crashes a second view with the debug key and
// checks the orchestrator saved a crash report.
func TestCrashedWindowLeavesAReport(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	first := []step{wait(10), {NewView: true}, wait(240), {Quit: true}}
	second := []step{wait(30), {Crash: true}}
	runEditor(t, binary, project, writeScript(t, "first.json", first), writeScript(t, "second.json", second))

	reports, err := filepath.Glob(filepath.Join(project, ".berad", "logs", "*-crash.json"))
	if err != nil || len(reports) != 1 {
		t.Fatalf("crash reports %v %v", reports, err)
	}
	raw, err := os.ReadFile(reports[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "simulated window crash") {
		t.Fatalf("report lacks the panic:\n%.500s", raw)
	}
}
