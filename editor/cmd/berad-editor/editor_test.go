package main_test

import (
	"context"
	"encoding/json"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/content"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// The sample building's walls (see createSample), in cells from the zone
// origin in the middle of its room.
const (
	left, top, right, bottom, door = -6, -4, 6, 4, 0
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

func readEnvelope(t *testing.T, project, pattern string) content.Envelope {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(project, pattern))
	if err != nil || len(matches) != 1 {
		t.Fatalf("files matching %s: %v %v", pattern, matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := content.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func readFloor(t *testing.T, project, name string) (*floor.Floor, content.Envelope) {
	t.Helper()
	envelope := readEnvelope(t, project, "tilefloors/"+name+"--*.json")
	document, err := tilefloor.Decode(envelope.Data)
	if err != nil {
		t.Fatal(err)
	}
	return document.(*tilefloor.Document).Floor, envelope
}

func cellOf(t *testing.T, fl *floor.Floor, layer string, col, row int) string {
	t.Helper()
	read := fl.Ground
	if layer == tilefloor.LayerTop {
		read = fl.Top
	}
	tile, err := read(col, row)
	if err != nil {
		t.Fatal(err)
	}
	return tile
}

// buildingWalls returns the wall cells of the sample building, door excluded.
func buildingWalls() [][2]int {
	var cells [][2]int
	for col := left; col <= right; col++ {
		for row := top; row <= bottom; row++ {
			edge := col == left || col == right || row == top || row == bottom
			if edge && !(row == bottom && col == door) {
				cells = append(cells, [2]int{col, row})
			}
		}
	}
	return cells
}

// TestHeadlessOpenCreatesAndSavesSample runs the real program without a window.
func TestHeadlessOpenCreatesAndSavesSample(t *testing.T) {
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	cmd := exec.Command(binary, "open", "--project", project, "--sample", "--no-window", "--exit-after", "1s")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("open: %v\n%s", err, output)
	}
	ground, envelope := readFloor(t, project, "ground-floor")
	if cellOf(t, ground, tilefloor.LayerTop, left, top) != "wall.a" || cellOf(t, ground, tilefloor.LayerGround, -16, -12) != "grass" ||
		cellOf(t, ground, tilefloor.LayerTop, door, bottom) != "door" || len(ground.Anchors()) != 1 {
		t.Fatal("sample ground floor not saved")
	}
	if ground.Bounds() != floor.Centered(32, 24) || envelope.Version != floor.FormatVersion {
		t.Fatalf("sample floor bounds %v, version %d", ground.Bounds(), envelope.Version)
	}
	zoneEnvelope := readEnvelope(t, project, "zones/sample-zone--*.json")
	zoneDoc, err := zoneplugin.Decode(zoneEnvelope.Data)
	if err != nil || len(zoneDoc.(*zoneplugin.Document).Zone.Floors()) != 2 || zoneDoc.(*zoneplugin.Document).Zone.Floors()[0] != envelope.ID {
		t.Fatalf("sample zone: %v", err)
	}
	readEnvelope(t, project, "texts/notes--*.json")
}

// step is one input-script step (see window/script.go).
type step struct {
	Repeat     int             `json:"repeat,omitempty"`
	At         string          `json:"at,omitempty"`
	Down       bool            `json:"down,omitempty"`
	Pressed    bool            `json:"pressed,omitempty"`
	Released   bool            `json:"released,omitempty"`
	Double     bool            `json:"double,omitempty"`
	Keys       []string        `json:"keys,omitempty"`
	Hold       []string        `json:"hold,omitempty"`
	Text       string          `json:"text,omitempty"`
	Replace    *ui.Replacement `json:"replace,omitempty"`
	Expect     *expectation    `json:"expect,omitempty"`
	Screenshot string          `json:"screenshot,omitempty"`
	Wheel      *[2]float64     `json:"wheel,omitempty"`
}

type expectation struct {
	Locate string `json:"locate"`
}

func wait(frames int) step { return step{Repeat: frames} }

func keys(names ...string) step { return step{Repeat: 2, Keys: names} }

func typed(text string) step { return step{Repeat: 2, Text: text} }

// walk holds an arrow key for one frame: in a preview, a tap moves the player
// exactly one cell.
func walk(key string) step { return step{Repeat: 1, Hold: []string{key}} }

// shotPath is where a test saves a screenshot: BERAD_SHOT_DIR when set, so
// they can be looked at afterwards, or else a temporary folder.
func shotPath(t *testing.T, name string) string {
	t.Helper()
	if dir := os.Getenv("BERAD_SHOT_DIR"); dir != "" {
		return filepath.Join(dir, name)
	}
	return filepath.Join(t.TempDir(), name)
}

func quit() step { return keys("Cmd+W") }

// expect waits up to frames for a named part of the window to exist.
func expect(locate string, frames int) step {
	return step{Repeat: frames, Expect: &expectation{Locate: locate}}
}

// click presses and releases the main button on a named part of the window.
func click(at string) []step {
	return []step{{Repeat: 2, At: at, Down: true, Pressed: true}, {Repeat: 2, At: at, Released: true}}
}

func doubleClick(at string) []step {
	return append(click(at), step{Repeat: 2, At: at, Down: true, Pressed: true, Double: true}, step{Repeat: 2, At: at, Released: true})
}

// drag presses on the first cell, drags through the rest and releases.
func drag(cells ...string) []step {
	steps := []step{}
	for index, at := range cells {
		steps = append(steps, step{Repeat: 3, At: at, Down: true, Pressed: index == 0})
	}
	return append(steps, step{Repeat: 2, At: cells[len(cells)-1], Released: true})
}

func script(parts ...any) []step {
	var steps []step
	for _, part := range parts {
		switch value := part.(type) {
		case step:
			steps = append(steps, value)
		case []step:
			steps = append(steps, value...)
		}
	}
	return steps
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

// scripts maps each view to the scripts its windows run, in the order the
// windows open.
type scripts map[string][][]step

func (s scripts) env(t *testing.T) string {
	t.Helper()
	var entries []string
	for view, list := range s {
		for index, steps := range list {
			entries = append(entries, view+"="+writeScript(t, view+string(rune('a'+index))+".json", steps))
		}
	}
	return "BERAD_INPUT_SCRIPT=" + strings.Join(entries, ",")
}

// runEditor opens the sample project with scripted windows and waits for the
// editor to quit by itself once its last window closes.
func runEditor(t *testing.T, binary, project string, windows scripts) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "open", "--project", project, "--sample")
	cmd.Env = append(os.Environ(), windows.env(t))
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("editor did not quit after its windows closed\n%s", output)
	}
	if err != nil {
		t.Fatalf("editor: %v\n%s", err, output)
	}
}

// closeBrowser is a project browser script that waits for the project and
// closes, leaving the other windows to the test.
func closeBrowser() []step {
	return script(expect("row:Notes", 600), quit())
}

// TestMapPaintsDrawsAndUndoes drives a real Map window: a drag paints the
// ground through the orchestrator, a drag with the Walls tool draws walls in
// the chosen family, undo takes the walls back in one step, and the result is
// in the saved file. Then the Resize tool adds a row on top with its button,
// drags the bottom edge down two rows, and undo takes the drag back.
func TestMapPaintsDrawsAndUndoes(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	mapScript := script(
		expect("state:loaded", 600), wait(5),
		keys("3"), drag("cell:4,8", "cell:5,8", "cell:6,8", "cell:7,8"), wait(5),
		keys("W"), keys("3"), drag("cell:4,10", "cell:5,10", "cell:6,10"), wait(10),
		keys("Cmd+Z"), wait(60),
		keys("R"), click("ui:grow-top"), expect("state:bounds:-16,-13,32,25", 300), wait(5),
		drag("edge:bottom", "cell:0,13"), expect("state:bounds:-16,-13,32,27", 300), wait(5),
		keys("Cmd+Z"), expect("state:bounds:-16,-13,32,25", 300), wait(60), quit(),
	)
	runEditor(t, binary, project, scripts{"map": {mapScript}, "browser": {closeBrowser()}})

	ground, _ := readFloor(t, project, "ground-floor")
	for col := 4; col <= 7; col++ {
		if got := cellOf(t, ground, tilefloor.LayerGround, col, 8); got != "road" {
			t.Fatalf("painted ground cell %d = %q", col, got)
		}
	}
	for col := 4; col <= 6; col++ {
		if got := cellOf(t, ground, tilefloor.LayerTop, col, 10); got != "" {
			t.Fatalf("undone wall cell %d = %q", col, got)
		}
	}
	if ground.Bounds() != image.Rect(-16, -13, 16, 12) {
		t.Fatalf("resized bounds %v", ground.Bounds())
	}
	if got := cellOf(t, ground, tilefloor.LayerGround, 0, -13); got != "grass" {
		t.Fatalf("the added row has ground %q, want the floor's most common ground", got)
	}
}

// TestWallFamiliesForARoomAndARun selects a room by double-clicking inside it
// and changes its walls' family in one go; then, on the upper floor, selects
// a connected run of walls by clicking one and changes that run.
func TestWallFamiliesForARoomAndARun(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := filepath.Join(t.TempDir(), "upper.png")
	mapScript := script(
		expect("state:loaded", 600), wait(5),
		keys("S"), doubleClick("cell:0,0"), wait(5), click("tile:wall.c"), wait(20),
		keys("PageUp"), expect("state:floor:2", 300), expect("state:loaded", 300), wait(10),
		click("cell:-6,-4"), wait(5), click("tile:wall"), wait(20),
		step{Repeat: 2, Screenshot: shot}, wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"map": {mapScript}, "browser": {closeBrowser()}})

	ground, _ := readFloor(t, project, "ground-floor")
	upper, _ := readFloor(t, project, "upper-floor")
	for _, cell := range buildingWalls() {
		if got := cellOf(t, ground, tilefloor.LayerTop, cell[0], cell[1]); got != "wall.c" {
			t.Fatalf("room wall %v = %q, want the new family", cell, got)
		}
	}
	if got := cellOf(t, ground, tilefloor.LayerTop, door, bottom); got != "door" {
		t.Fatalf("the door became %q", got)
	}
	for _, cell := range buildingWalls() {
		if got := cellOf(t, upper, tilefloor.LayerTop, cell[0], cell[1]); got != "wall" {
			t.Fatalf("upper run wall %v = %q", cell, got)
		}
	}
	if _, err := os.Stat(shot); err != nil {
		t.Fatalf("no screenshot of the upper floor: %v", err)
	}
	t.Logf("upper floor with the floor below: %s", shot)
}

// TestAnchorTextAndPickOnMap adds and names an anchor in the Map window, opens
// the text document from the project browser, writes in it (a new line with
// Cmd+Enter, an accent the way press-and-hold sends it, Enter to save), then
// picks the anchor as its location through the orchestrator.
func TestAnchorTextAndPickOnMap(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	mapScript := script(
		expect("state:loaded", 600), wait(5),
		keys("A"), click("cell:4,6"), wait(30), typed("Well"), keys("Enter"), wait(20),
		expect("state:picking", 1800), wait(5), click("cell:4,6"), wait(40), quit(),
	)
	browserScript := script(
		expect("row:Notes", 600), click("row:Notes"), keys("Enter"), wait(30), quit(),
	)
	textScript := script(
		expect("state:loaded", 600), wait(5),
		click("ui:body"), keys("Cmd+Down"), keys("Cmd+Enter"),
		typed("caf"), typed("e"), step{Repeat: 2, Replace: &ui.Replacement{Before: 1, After: 0, Text: "é", Caret: 2}},
		typed(" ok"), keys("Enter"), expect("state:saved", 300), wait(120),
		click("ui:pick"), expect("state:located", 1800), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"map": {mapScript}, "browser": {browserScript}, "text": {textScript}})

	ground, groundEnvelope := readFloor(t, project, "ground-floor")
	well, found := ground.AnchorAt(4, 6)
	if !found || well.Name != "Well" {
		t.Fatalf("anchor %+v %v", well, found)
	}
	envelope := readEnvelope(t, project, "texts/notes--*.json")
	decoded, err := textdoc.Decode(envelope.Data)
	if err != nil {
		t.Fatal(err)
	}
	notes := decoded.(*textdoc.Document)
	if !strings.HasSuffix(notes.Body, "\ncafé ok") {
		t.Fatalf("body %q", notes.Body)
	}
	if notes.Location.Floor != groundEnvelope.ID || notes.Location.Anchor != well.ID {
		t.Fatalf("location %+v, want the Well anchor", notes.Location)
	}
}

// TestBrowserCreatesRenamesAndDeletes creates a zone, renames it in the
// inspector by typing a name and clicking Rename, deletes the text document
// after confirming, and undoes the deletion.
func TestBrowserCreatesRenamesAndDeletes(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	browserScript := script(
		expect("row:Notes", 600), wait(5),
		click("ui:new-zone"), expect("row:Zone 1", 300), click("row:Zone 1"),
		click("ui:name"), keys("Cmd+A"), typed("Harbour"), click("ui:rename"), expect("row:Harbour", 300),
		click("row:Notes"), click("ui:delete"), wait(30), click("ui:delete"), wait(30),
		keys("Cmd+Z"), expect("row:Notes", 300), wait(10), quit(),
	)
	mapScript := script(expect("state:loaded", 600), quit())
	runEditor(t, binary, project, scripts{"map": {mapScript}, "browser": {browserScript}})

	harbour := readEnvelope(t, project, "zones/harbour--*.json")
	zoneDoc, err := zoneplugin.Decode(harbour.Data)
	if err != nil || len(zoneDoc.(*zoneplugin.Document).Zone.Floors()) != 1 {
		t.Fatalf("new zone %v", err)
	}
	readFloor(t, project, "floor-1")
	readEnvelope(t, project, "texts/notes--*.json")
}

// TestViewsShareUndo opens a second view with Cmd+N; the second view undoes
// the first view's stroke, because views of one zone share one undo list.
func TestViewsShareUndo(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	first := script(
		expect("state:loaded", 600), wait(5), keys("4"), drag("cell:2,9", "cell:3,9", "cell:4,9"),
		wait(10), keys("Cmd+N"), wait(300), quit(),
	)
	second := script(expect("state:loaded", 600), wait(20), keys("Cmd+Z"), wait(30), quit())
	runEditor(t, binary, project, scripts{"map": {first, second}, "browser": {closeBrowser()}})

	ground, _ := readFloor(t, project, "ground-floor")
	for col := 2; col <= 4; col++ {
		if got := cellOf(t, ground, tilefloor.LayerGround, col, 9); got != "grass" {
			t.Fatalf("cell %d = %q; the second view should have undone the first view's stroke", col, got)
		}
	}
}

// TestPreviewWalksAndClimbs previews the sample zone (D15): P and a click
// start the player inside the building, arrow keys walk them onto the stairs
// and up to the same cell of the upper floor, and a second view of the zone
// shows the player as they walk. The preview changes nothing in the project.
func TestPreviewWalksAndClimbs(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	walking, upstairs, seen := shotPath(t, "preview.png"), shotPath(t, "preview-upstairs.png"), shotPath(t, "preview-other-view.png")
	first := script(
		expect("state:loaded", 600), wait(5), keys("Cmd+N"), wait(120),
		keys("P"), click("cell:2,-2"), expect("state:previewing", 300),
		walk("Right"), expect("state:player:3,-2", 120), step{Repeat: 2, Screenshot: walking}, wait(90),
		walk("Right"), expect("state:floor:2", 120), expect("state:player:4,-2", 120),
		step{Repeat: 2, Screenshot: upstairs}, wait(60), keys("P"), wait(60), quit(),
	)
	second := script(
		expect("state:loaded", 600), expect("state:remote-player:3,-2", 900),
		step{Repeat: 2, Screenshot: seen}, expect("state:remote-player:4,-2", 600), wait(30), quit(),
	)
	runEditor(t, binary, project, scripts{"map": {first, second}, "browser": {closeBrowser()}})

	ground, envelope := readFloor(t, project, "ground-floor")
	if cellOf(t, ground, tilefloor.LayerTop, 4, -2) != "stairs.up" || envelope.Revision != 0 {
		t.Fatalf("the preview must not change the project: revision %d", envelope.Revision)
	}
	for _, shot := range []string{walking, upstairs, seen} {
		if _, err := os.Stat(shot); err != nil {
			t.Fatalf("missing screenshot: %v", err)
		}
		t.Logf("screenshot: %s", shot)
	}
}

// TestWorkflowBuildsARoutine creates a workflow in the project browser, opens
// it, and builds a routine from the palette in a real Workflow window: start,
// a time switch, a time-bound container holding a Go to and a Wait, and a
// Wait for the rest of the day. The Go to's destination is picked on the Map
// window (D13). The saved logic and its separate layout file are checked.
func TestWorkflowBuildsARoutine(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "workflow.png")
	browserScript := script(
		expect("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), wait(60), quit(),
	)
	mapScript := script(
		expect("state:loaded", 600), expect("state:picking", 1800), wait(5), click("cell:-3,-1"), wait(300), quit(),
	)
	workflowScript := script(
		expect("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:time-switch"), wait(10),
		click("palette:container"), wait(10), click("palette:go-to"), wait(10),
		click("ui:pick"), expect("state:destination", 1800), wait(5),
		click("palette:wait"), wait(10),
		click("node:time-switch:1"), wait(5), click("palette:wait"), wait(10),
		expect("state:nodes:6", 300), click("ui:arrange"), wait(20),
		step{Repeat: 2, Screenshot: shot}, wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {mapScript}, "workflow": {workflowScript},
	})

	logic := readEnvelope(t, project, "workflows/workflow-1--*[^t].json")
	layout := readEnvelope(t, project, "workflows/workflow-1--*.layout.json")
	decoded, err := workflowplugin.Decode(logic.Data)
	if err != nil {
		t.Fatal(err)
	}
	routine := decoded.(*workflowplugin.Document).Workflow
	if err := routine.DecodeLayout(layout.Data); err != nil {
		t.Fatal(err)
	}
	byKind := map[string][]flow.Node{}
	for _, node := range routine.Nodes() {
		byKind[node.Kind] = append(byKind[node.Kind], node)
		if _, placed := routine.Box(node.ID); !placed {
			t.Fatalf("auto-arrange placed every node; %s has no box", node.Kind)
		}
	}
	start, _ := routine.Node(routine.Start())
	timeSwitch, container := byKind[flow.KindTimeSwitch][0], byKind[flow.KindContainer][0]
	goTo, waits := byKind[flow.KindGoTo][0], byKind[flow.KindWait]
	if len(routine.Nodes()) != 6 || len(waits) != 2 {
		t.Fatalf("nodes %+v", byKind)
	}
	inner, outer := waits[0], waits[1]
	if start.Next[flow.OutputNext] != timeSwitch.ID || timeSwitch.Next["r1"] != container.ID ||
		timeSwitch.Next[flow.OutputOtherwise] != outer.ID || container.Next[flow.OutputInside] != goTo.ID ||
		goTo.Next[flow.OutputNext] != inner.ID || goTo.Parent != container.ID || inner.Parent != container.ID {
		t.Fatalf("the routine is not linked as built: %+v", routine.Nodes())
	}
	ground, groundEnvelope := readFloor(t, project, "ground-floor")
	desk, found := ground.AnchorAt(-3, -1)
	destination, err := goTo.Anchor("destination")
	if err != nil || !found || destination.Floor != groundEnvelope.ID || destination.Anchor != desk.ID {
		t.Fatalf("destination %+v, want the desk anchor", destination)
	}
	t.Logf("screenshot: %s", shot)
}

// TestWorkflowRandomWalk builds the test routine "move up or down at
// random, wait two seconds, repeat" in a real Workflow window: a random
// choice with two named options, a Move each way, and one Wait that both
// Moves lead into, with an entry dot for each, then a line from the Wait
// back to the random choice, drawn dashed as a loop, and auto-arranges it.
func TestWorkflowRandomWalk(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "random-walk.png")
	retype := func(field, text string) []step {
		return script(click("ui:"+field), wait(3), keys("Cmd+A"), typed(text), keys("Enter"), wait(10))
	}
	browserScript := script(
		expect("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), wait(60), quit(),
	)
	workflowScript := script(
		expect("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:random"), wait(10),
		retype("options-1-label", "up"), retype("options-2-label", "down"),
		click("palette:move"), wait(10), click("node:random:1"), wait(5), click("palette:move"), wait(10),
		click("ui:direction-down"), wait(10),
		click("node:move:1"), wait(5), click("palette:wait"), wait(10),
		retype("param-duration", "2"), click("ui:duration-seconds"), wait(10),
		drag("port:move:2:next", "entry:wait:1"), expect("state:entries:wait:1=2", 300), wait(10),
		click("ui:arrange"), wait(20), step{Repeat: 1, At: "canvas:400,300", Wheel: &[2]float64{7, 0}}, wait(10),
		drag("port:wait:1:next", "node:random:1"), expect("state:entries:random:1=2", 300), wait(10),
		click("node:random:1"), wait(10), step{Repeat: 2, Screenshot: shot}, wait(10), quit(),
	)
	mapScript := script(expect("state:loaded", 600), quit())
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {mapScript}, "workflow": {workflowScript}})

	decoded, err := workflowplugin.Decode(readEnvelope(t, project, "workflows/workflow-1--*[^t].json").Data)
	if err != nil {
		t.Fatal(err)
	}
	byKind := map[string][]flow.Node{}
	for _, node := range decoded.(*workflowplugin.Document).Workflow.Nodes() {
		byKind[node.Kind] = append(byKind[node.Kind], node)
	}
	random, moves, waits := byKind[flow.KindRandom], byKind[flow.KindMove], byKind[flow.KindWait]
	if len(random) != 1 || len(moves) != 2 || len(waits) != 1 {
		t.Fatalf("nodes %+v", byKind)
	}
	options, _ := random[0].Shares("options")
	direction, _ := moves[1].Choice("direction")
	duration, _ := waits[0].Duration("duration")
	if len(options) != 2 || options[0].Label != "up" || options[1].Label != "down" ||
		random[0].Next[options[0].Port] != moves[0].ID || random[0].Next[options[1].Port] != moves[1].ID ||
		moves[0].Next[flow.OutputNext] != waits[0].ID || moves[1].Next[flow.OutputNext] != waits[0].ID ||
		waits[0].Next[flow.OutputNext] != random[0].ID ||
		direction != flow.DirectionDown || duration.Amount != 2 || duration.Unit != flow.UnitSeconds {
		t.Fatalf("the routine was saved as %+v", byKind)
	}
	t.Logf("screenshot: %s", shot)
}

// TestNPCPlacedWithBehaviour makes a workflow and an NPC in the project
// browser, gives the NPC the workflow as its behaviour and some notes, and
// places the NPC on the map with the Place NPC tool.
func TestNPCPlacedWithBehaviour(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	browserShot, mapShot := shotPath(t, "npc-browser.png"), shotPath(t, "npc-map.png")
	browserScript := script(
		expect("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:new-npc"), wait(30),
		click("ui:choose-behaviour"), wait(10), click("row:Workflow 1"), wait(5), click("ui:use"), wait(30),
		click("ui:notes"), wait(5), typed("Opens the shop at eight."), keys("Enter"), wait(30),
		step{Repeat: 2, Screenshot: browserShot}, wait(10), quit(),
	)
	mapScript := script(
		expect("state:loaded", 600), wait(5), keys("N"), expect("npc:NPC 1", 1800), click("npc:NPC 1"), wait(5),
		click("cell:1,1"), expect("state:placement:1,1=NPC 1", 300), wait(120),
		step{Repeat: 2, Screenshot: mapShot}, wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {mapScript}})

	routine := readEnvelope(t, project, "workflows/workflow-1--*[^t].json")
	saved := readEnvelope(t, project, "npcs/npc-1--*.json")
	decoded, err := npcplugin.Decode(saved.Data)
	if err != nil {
		t.Fatal(err)
	}
	clerk := decoded.(*npcplugin.Document).NPC
	if clerk.Behaviour != routine.ID || clerk.Notes != "Opens the shop at eight." {
		t.Fatalf("the NPC was saved as %+v", clerk)
	}
	ground, _ := readFloor(t, project, "ground-floor")
	placement, found := ground.PlacementAt(1, 1)
	if !found || placement.NPC != saved.ID || placement.ID.IsZero() {
		t.Fatalf("placement %+v %v", placement, found)
	}
	for _, shot := range []string{browserShot, mapShot} {
		t.Logf("screenshot: %s", shot)
	}
}

// TestWindowsCloseWhenOrchestratorIsKilled kills the orchestrator process
// outright (no chance to say goodbye) and checks its windows notice the lost
// connection and close themselves instead of lingering (D10).
func TestWindowsCloseWhenOrchestratorIsKilled(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	cmd := exec.Command(binary, "open", "--project", project, "--sample")
	cmd.Env = append(os.Environ(), scripts{"map": {{wait(3600)}}, "browser": {{wait(3600)}}}.env(t))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	windows := func() int {
		output, _ := exec.Command("pgrep", "-f", binary+" window").Output()
		return len(strings.Fields(string(output)))
	}
	deadline := time.Now().Add(20 * time.Second)
	for windows() < 2 {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatal("two windows did not open")
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Logf("%d windows running; killing the orchestrator", windows())
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	deadline = time.Now().Add(10 * time.Second)
	for windows() > 0 {
		if time.Now().After(deadline) {
			_ = exec.Command("pkill", "-f", binary+" window").Run()
			t.Fatal("windows kept running after the orchestrator was killed")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestCrashedWindowLeavesAReport crashes a second view with the debug key and
// checks the orchestrator saved a crash report.
func TestCrashedWindowLeavesAReport(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	first := script(expect("state:loaded", 600), keys("Cmd+N"), wait(240), quit())
	second := script(wait(30), keys("Cmd+Shift+K"))
	runEditor(t, binary, project, scripts{"map": {first, second}, "browser": {closeBrowser()}})

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
