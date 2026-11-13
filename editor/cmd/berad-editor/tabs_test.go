package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/engine/art"
)

// TestViewsOpenAsTabs opens a Map, a workflow and Audio from the
// project browser, each as a tab of the editor window (increments 10 and 13).
// The Map paints while another tab is in front, so hidden tabs keep working,
// and closing the project browser's tab quits the editor after asking.
func TestViewsOpenAsTabs(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "tabs.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), wait(60),
		doubleClick("row:Fetch stock"), wait(60), must("section:audio", 600), doubleClick("row:Hum"), wait(240),
		step{Repeat: 2, Screenshot: shot}, keys("Cmd+W"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	mapScript := script(must("state:loaded", 600), wait(240), keys("3"), drag("cell:4,8", "cell:5,8"), wait(30))
	audioScript := script(must("state:selected=cue:sample/hum", 900))
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {mapScript}, "audio": {audioScript},
	})
	noCrashes(t, project)

	ground, _ := readFloor(t, project, "ground-floor")
	for col := 4; col <= 5; col++ {
		if got := cellOf(t, ground, tilefloor.LayerGround, col, 8); got != "road" {
			t.Fatalf("ground cell %d painted from a hidden tab = %q", col, got)
		}
	}
	t.Logf("screenshot: %s", shot)
}

// TestClockIsPinned previews the sample zone in a Map tab, then brings a
// workflow's tab to the front (increment 10). The pinned clock reads the
// preview out on the bar, and its +1h moves the hidden Map's preview on an
// hour. Since increment 41 its panel no longer opens by itself.
func TestClockIsPinned(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "tabs-clock.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), wait(90), doubleClick("row:Fetch stock"),
		wait(300), step{Repeat: 2, Screenshot: shot}, keys("Cmd+W"), must("ui:quit", 300), wait(5),
		click("ui:quit"), wait(60),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("P"), click("cell:2,-2"), must("state:previewing", 300),
	)
	clockScript := script(
		must("state:running", 1200), wait(10), click("ui:hour-on"), must("state:time:09:", 300), wait(10),
	)
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {mapScript}, "clock": {clockScript},
	})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestWatchOpensBesideItsMap previews the sample zone and asks for Watch
// from the Map (increment 10): Watch opens beside the Map, in its tab, and
// follows the preview the Map still runs.
func TestWatchOpensBesideItsMap(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "tabs-split.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), wait(600),
		keys("Cmd+W"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("P"), click("cell:2,-2"), must("state:previewing", 300),
		click("ui:watch"), wait(120), step{Repeat: 2, Screenshot: shot}, walk("ArrowRight"), wait(10),
	)
	watchScript := script(must("state:previewing", 900), wait(10))
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {mapScript}, "watch": {watchScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// frameScenario is the scenario increment 10 measures: a preview runs in the
// sample zone with Watch open and the clock showing, eight strokes are
// painted where the Map shows them beside Watch too, then everything runs for
// ten seconds before the editor quits.
func frameScenario() scripts {
	strokes := []any{must("state:loaded", 600), wait(5), keys("P"), click("cell:2,-2"),
		must("state:previewing", 300), click("ui:watch"), wait(60), keys("3")}
	for row := 4; row < 12; row++ {
		at := func(col int) string { return "cell:" + strconv.Itoa(col) + "," + strconv.Itoa(row) }
		strokes = append(strokes, drag(at(-14), at(-13), at(-12)), wait(10))
	}
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), wait(1100),
		keys("Cmd+Q"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	return scripts{
		"browser": {browserScript}, "map": {script(append(strokes, wait(600))...)},
		"watch": {script(must("state:previewing", 900))},
	}
}

// measureFrames runs the frame scenario and returns the editor window's frame
// statistics, a line for each time it closed.
func measureFrames(t *testing.T, binary string) []string {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	stats := filepath.Join(t.TempDir(), "frames.jsonl")
	args := []string{"open", "--project", project, "--sample"}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), frameScenario().env(t), "BERAD_FRAME_STATS="+stats)
	if output, err := cmd.CombinedOutput(); err != nil || ctx.Err() != nil {
		t.Fatalf("editor: %v %v\n%s", err, ctx.Err(), output)
	}
	noCrashes(t, project)
	raw, err := os.ReadFile(stats)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// TestFrameTimes measures increment 10's scenario in the editor window,
// and logs its tick and draw times, frame rate and command-to-event times,
// to check against the latency budget (D12).
func TestFrameTimes(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	stats := measureFrames(t, binary)
	if len(stats) != 1 || !strings.Contains(stats[0], `"view":"shell"`) {
		t.Fatalf("the editor window's statistics: %q", stats)
	}
	t.Logf("the editor window:\n%s", stats[0])
}

// TestEveryDocumentHasATab opens two workflows, then a Map and two more
// views of it: the bar shows each as a tab of its own, on one row
// (increment 38).
func TestEveryDocumentHasATab(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "tabs-row.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5),
		click("ui:new-workflow"), wait(30), click("ui:open"), wait(60),
		click("ui:new-workflow"), wait(30), click("ui:open"), wait(60),
		doubleClick("row:Sample zone"), wait(240), step{Repeat: 2, Screenshot: shot},
		keys("Cmd+W"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	mapScript := script(must("state:loaded", 600), wait(5), keys("Cmd+N"), wait(60), keys("Cmd+N"), wait(60))
	idle := script(must("state:loaded", 600))
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {mapScript, idle, idle}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestKeysMoveBetweenTabs opens a workflow and a Map, then moves between
// the three tabs by key from the workflow's tab (increment 38): Cmd+1 to
// the Project, Cmd+Shift+] to its neighbour, Ctrl+Tab back to the tab last
// used, and Cmd+9, with no ninth tab, nowhere.
func TestKeysMoveBetweenTabs(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("row:Workflows"), wait(20), doubleClick("row:Clerk's day"),
		must("state:tabs:2", 600), wait(30), click("row:Zones"), wait(20), doubleClick("row:Sample zone"),
		must("state:tabs:1", 3600), wait(5), keys("Cmd+Q"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	workflowScript := script(
		must("state:tabs:3", 900), wait(30), keys("Cmd+1"), must("state:front:Project", 60),
		keys("Cmd+Shift+]"), must("state:front:Clerk's day", 60), keys("Ctrl+Tab"), must("state:front:Project", 60),
		keys("Cmd+9"), wait(5), must("state:front:Project", 5), keys("Cmd+3"), wait(30), keys("Cmd+W"), wait(30),
	)
	mapScript := script(must("state:loaded", 600), must("state:tabs:2", 1800), wait(5), keys("Cmd+W"), wait(30))
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}, "map": {mapScript}})
	noCrashes(t, project)
}

// TestBackReturnsToANode selects two nodes of Clerk's day in turn, goes to
// the Project's tab, then steps back with Cmd+[ to the workflow with the
// second selected, back again to the first, and forward with Cmd+]
// (increment 38).
func TestBackReturnsToANode(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("row:Workflows"), wait(20), doubleClick("row:Clerk's day"),
		must("state:tabs:2", 600), must("state:tabs:1", 3600), wait(5), keys("Cmd+Q"), must("ui:quit", 300), wait(5),
		click("ui:quit"), wait(60),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(30), click("node:start:1"), must("state:selected:start", 60),
		click("node:time-switch:1"), must("state:selected:time-switch", 60), wait(5), keys("Cmd+1"),
		must("state:front:Project", 60), wait(5), keys("Cmd+["), must("state:front:Clerk's day", 60),
		must("state:selected:time-switch", 60), wait(5), keys("Cmd+["), must("state:selected:start", 60), wait(5),
		keys("Cmd+]"), must("state:selected:time-switch", 60), wait(5), keys("Cmd+W"), wait(30),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
}

// TestQuickOpenFromAMap presses Cmd+P in a Map's tab, finds Clerk's day in
// the sheet and opens it with Enter in a tab of its own; then finds Fetch
// stock and opens it with Cmd+Enter beside the Map, in the Map's tab
// (increment 38).
func TestQuickOpenFromAMap(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "quick-open.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), click("row:Zones"), wait(20), doubleClick("row:Sample zone"),
		must("state:tabs:2", 600), must("state:quick-open", 900), wait(5), typed("Clerk's"),
		must("quick:Clerk's day", 120), step{Repeat: 2, Screenshot: shot, Front: false}, click("quick:Clerk's day"),
		keys("Enter"), must("state:tabs:3", 600), must("state:quick-open", 1800), wait(5), typed("Fetch"),
		must("quick:Fetch stock", 120), click("quick:Fetch stock"), keys("Cmd+Enter"),
		must("state:tabs:1", 3600), wait(5), keys("Cmd+Q"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	mapScript := script(
		must("state:loaded", 600), wait(30), keys("Cmd+P"), must("state:tabs:3", 900),
		must("state:front:Clerk's day", 60), must("state:tabs:2", 1800), wait(10), keys("Cmd+P"),
		must("state:front:Fetch stock", 900), must("state:tabs:2", 5), wait(120), keys("Cmd+W"), wait(30),
	)
	clerkScript := script(must("state:loaded", 600), wait(30), keys("Cmd+W"), wait(30))
	fetchScript := script(must("state:loaded", 600), wait(60), keys("Cmd+W"), wait(30))
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {mapScript}, "workflow": {clerkScript, fetchScript},
	})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestSpriteOpensAsATab makes a sprite in the project browser and opens it
// as a tab under Sprites, after Audio on the bar (increment 11). The Pixel
// editor paints a stroke there.
func TestSpriteOpensAsATab(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "tabs-sprite.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5),
		rightClick("section:sprites"), must("menu:new-sprite", 60), click("menu:new-sprite"),
		must("row:Sprite 1", 300), wait(30), click("ui:open"), wait(360), step{Repeat: 2, Screenshot: shot},
		keys("Cmd+W"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
	spriteScript := script(
		must("state:loaded", 600), wait(5), click("swatch:Default / grey 2"),
		drag("pixel:2,2", "pixel:6,2", "pixel:6,6"), must("state:pixel:6,6:Default / grey 2", 120),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "sprite": {spriteScript}})
	noCrashes(t, project)

	tile, err := art.DecodeTile(readEnvelope(t, project, "tiles/tile-1--*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, pixel := range [][2]int{{2, 2}, {4, 2}, {6, 2}, {6, 4}, {6, 6}} {
		if ref, _ := tile.At(pixel[0], pixel[1]); ref.Shade != art.DefaultShadeID(1, 2) {
			t.Fatalf("pixel %v painted in the Sprites tab is %+v", pixel, ref)
		}
	}
	t.Logf("screenshot: %s", shot)
}
