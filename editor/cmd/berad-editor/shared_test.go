package main_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/engine/content"
)

// runEditorWhile runs the editor as runEditor does, and once the screenshot
// at signal is saved, while the editor still runs, calls during: the walk's
// panes wait meanwhile for what it changes.
func runEditorWhile(t *testing.T, binary, project string, panes scripts, signal string, during func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "open", "--project", project, "--sample")
	cmd.Env = append(os.Environ(), panes.env(t))
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for waiting := true; waiting; {
		select {
		case err := <-done:
			t.Fatalf("the editor quit before %s was saved: %v\n%s", signal, err, output.String())
		case <-time.After(20 * time.Millisecond):
		}
		_, err := os.Stat(signal)
		waiting = err != nil
	}
	during()
	if err := <-done; err != nil || ctx.Err() != nil {
		t.Fatalf("editor: %v %v\n%s", err, ctx.Err(), output.String())
	}
}

// renameFloor renames the floor called from to to in its file, as another
// program would.
func renameFloor(t *testing.T, project, from, to string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(project, "tilefloors", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := content.Decode(raw)
		if err != nil || envelope.Name != from || strings.HasSuffix(path, ".layout.json") {
			continue
		}
		envelope.Name = to
		encoded, err := content.Encode(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no floor called %s", from)
}

// TestAFloorChangedOnDiskIsTakenIn opens the sample's ground floor in a
// Map, then renames it in its file, as another program would: the Map takes
// in the file's version and its status bar says so (increment 57).
func TestAFloorChangedOnDiskIsTakenIn(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	signal, shot := shotPath(t, "taken-in-before.png"), shotPath(t, "taken-in.png")
	mapScript := script(
		must("state:loaded", 600), wait(30), shoot(signal),
		must("state:said:Took in Cellar floor as changed on disk", 900), shoot(shot), quit(),
	)
	runEditorWhile(t, binary, project, scripts{"browser": {browseZone()}, "map": {mapScript}}, signal,
		func() { renameFloor(t, project, "Ground floor", "Cellar floor") })
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestAProjectThatCannotOpenAsksFirst puts a file with a merge's conflict
// markers into a project: the Project tab asks before anything opens, Open
// without them leaves it out with a notice, and the notice's Dismiss ends
// it (increment 57).
func TestAProjectThatCannotOpenAsksFirst(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	create := exec.Command(binary, "open", "--project", project, "--sample", "--no-window", "--exit-after", "2s")
	if output, err := create.CombinedOutput(); err != nil {
		t.Fatalf("make the sample: %v\n%s", err, output)
	}
	broken := filepath.Join(project, "plan--01j00000.json")
	conflicted := []byte("<<<<<<< HEAD\n{}\n=======\n{}\n>>>>>>> theirs\n")
	if err := os.WriteFile(broken, conflicted, 0o600); err != nil {
		t.Fatal(err)
	}
	asked, told := shotPath(t, "opening-asked.png"), shotPath(t, "opening-told.png")
	browserScript := script(
		must("ui:opening-open", 900), wait(10), shoot(asked), click("ui:opening-open"),
		must("state:notices=1", 300), wait(5), shoot(told), click("ui:notice-dismiss"),
		must("state:notices=0", 120), wait(5), keys("Cmd+Q"), must("ui:quit", 300), wait(5), click("ui:quit"),
		wait(60),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}})
	noCrashes(t, project)
	if after, err := os.ReadFile(broken); err != nil || !bytes.Equal(after, conflicted) {
		t.Fatalf("the file left out is never written over: %v", err)
	}
	t.Logf("screenshots: %s %s", asked, told)
}

// TestDeletingAZoneNamesItsFloors asks to delete the sample zone: the
// question names the floors that go with it, and Cancel leaves everything
// as it was (increment 58).
func TestDeletingAZoneNamesItsFloors(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "delete-zone.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("row:Sample zone"), wait(5), click("ui:delete"),
		must("state:dialog-line=go with it: Ground floor", 300), wait(5), shoot(shot), click("ui:cancel"),
		must("row:Sample zone", 120), wait(5), keys("Cmd+Q"), must("ui:quit", 300), wait(5), click("ui:quit"),
		wait(60),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestKeysInAFieldStayWithIt types a Wait's duration, presses Escape and
// Cmd+Z: Escape leaves the field and keeps what was typed, and Cmd+Z stops
// at the field once its typing is undone, saying so (increment 59).
func TestKeysInAFieldStayWithIt(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "field-keys.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"),
		must("state:tabs:2", 600), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(30), click("node:start:1"), wait(5), addNode("wait"),
		must("state:selected:wait", 300), wait(5), click("ui:param-duration"), keys("Cmd+A"), typed("45"), wait(5),
		keys("Escape"), must("state:said:Left the field. What you typed waits for Save.", 60), wait(5),
		click("ui:param-duration"), keys("Cmd+Z"), keys("Cmd+Z"), keys("Cmd+Z"), keys("Cmd+Z"),
		must("state:said:Nothing more to undo in this field", 60), must("state:nodes:2", 60), wait(5), shoot(shot),
		keys("Escape"), wait(5), click("ui:revert"), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}
