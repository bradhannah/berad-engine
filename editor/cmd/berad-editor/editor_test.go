package main_test

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/content"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
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
	gloomyFile := readEnvelope(t, project, "palettes/gloomy--*.json")
	if listed := zoneDoc.(*zoneplugin.Document).Zone.Palettes(); len(listed) != 1 || listed[0] != gloomyFile.ID {
		t.Fatalf("the sample zone lists the sample's palette, gloomy: %v", listed)
	}
	readEnvelope(t, project, "texts/notes--*.json")
	lantern := readEnvelope(t, project, "sprites/lantern--*.json")
	sprite, err := art.DecodeSprite(lantern.Data)
	if err != nil {
		t.Fatal(err)
	}
	tile, err := art.DecodeTile(readEnvelope(t, project, "tiles/lantern--*.json").Data)
	if err != nil || tile.Size() != art.DefaultTileSize || sprite.Tile.IsZero() {
		t.Fatalf("the lantern's tile: %v", err)
	}
	gloomy, err := art.DecodePalette(gloomyFile.Data)
	if err != nil {
		t.Fatal(err)
	}
	ramps := gloomy.Ramps()
	if len(ramps) != 2 || ramps[0].Name != "iron" || len(ramps[0].Shades) != 5 ||
		ramps[1].Name != "glass" || len(ramps[1].Shades) != 2 || ramps[1].Shades[0].Opacity != 40 {
		t.Fatalf("gloomy holds the iron and glass ramps: %+v", ramps)
	}
	ironUsed, glassUsed := 0, 0
	for _, ref := range tile.Colors() {
		switch {
		case ref.Palette != gloomyFile.ID:
			t.Fatalf("the lantern is painted only from gloomy: %+v", ref)
		case ref.Ramp == ramps[0].ID:
			ironUsed++
		case ref.Ramp == ramps[1].ID:
			glassUsed++
		}
	}
	if ironUsed != 5 || glassUsed != 2 {
		t.Fatalf("the lantern is painted with five iron greys and both shades of glass: %+v", tile.Colors())
	}
	defaults, err := art.DecodePalette(readEnvelope(t, project, "palettes/default--*.json").Data)
	if want, _ := art.DefaultPalette(); err != nil || !defaults.Equal(want) {
		t.Fatalf("Default is saved as the engine made it: %v", err)
	}
	standIns, err := art.DecodeStandInSprites(readEnvelope(t, project, "standin-sprites/*.json").Data)
	if shown, ok := standIns.Sprite("pickup.small"); err != nil || !ok || shown != lantern.ID {
		t.Fatalf("the lantern shows the small pickup: %v %v %v", shown, ok, err)
	}
}

// TestOpenMakesAProjectInAnEmptyFolder opens a folder that does not exist yet
// without --sample: it becomes a project holding only what every project
// holds, Default, the art settings and the stand-in sprites, and
// opens again as it is.
func TestOpenMakesAProjectInAnEmptyFolder(t *testing.T) {
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "new project")
	for range 2 {
		cmd := exec.Command(binary, "open", "--project", project, "--no-window", "--exit-after", "1s")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("open: %v\n%s", err, output)
		}
	}
	for _, pattern := range []string{"palettes/*.json", "art-settings/*.json", "standin-sprites/*.json"} {
		readEnvelope(t, project, pattern)
	}
	if zones, _ := filepath.Glob(filepath.Join(project, "zones", "*.json")); len(zones) != 0 {
		t.Fatalf("no sample without --sample: %v", zones)
	}
}

// step is one input-script step (see window/script.go).
type step struct {
	Repeat     int             `json:"repeat,omitempty"`
	At         string          `json:"at,omitempty"`
	Down       bool            `json:"down,omitempty"`
	Pressed    bool            `json:"pressed,omitempty"`
	Released   bool            `json:"released,omitempty"`
	Double     bool            `json:"double,omitempty"`
	Secondary  bool            `json:"secondary,omitempty"`
	Keys       []string        `json:"keys,omitempty"`
	Hold       []string        `json:"hold,omitempty"`
	Text       string          `json:"text,omitempty"`
	Replace    *ui.Replacement `json:"replace,omitempty"`
	Expect     *expectation    `json:"expect,omitempty"`
	Screenshot string          `json:"screenshot,omitempty"`
	Wheel      *[2]float64     `json:"wheel,omitempty"`
	Front      bool            `json:"front,omitempty"`
}

type expectation struct {
	Locate string `json:"locate"`
	Must   bool   `json:"must,omitempty"`
}

func wait(frames int) step { return step{Repeat: frames} }

// shoot saves a screenshot of the editor window with the pane's tab in front.
func shoot(path string) step { return step{Repeat: 2, Screenshot: path, Front: true} }

// front brings the pane's tab to the front, so it is drawn: a hidden tab
// keeps working but is not drawn.
func front() step { return step{Repeat: 2, Front: true} }

// mustInFront is must with the pane's tab kept in front while it waits, for
// parts only drawing makes.
func mustInFront(locate string, frames int) step {
	shown := must(locate, frames)
	shown.Front = true

	return shown
}

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

// must is expect that crashes the pane when the part never appears, so
// noCrashes fails the test.
func must(locate string, frames int) step {
	return step{Repeat: frames, Expect: &expectation{Locate: locate, Must: true}}
}

// noCrashes fails the test if any pane crashed, as a must does when its
// part never appears.
func noCrashes(t *testing.T, project string) {
	t.Helper()
	reports, err := filepath.Glob(filepath.Join(project, ".berad", "logs", "*-crash.json"))
	if err != nil || len(reports) != 0 {
		for _, report := range reports {
			raw, _ := os.ReadFile(report) //nolint:errcheck,gosec // The test's own project.
			t.Logf("%s:\n%.800s", report, raw)
		}
		t.Fatalf("tabs crashed: %v %v", reports, err)
	}
}

// click presses and releases the main button on a named part of the window.
func click(at string) []step {
	return []step{{Repeat: 2, At: at, Down: true, Pressed: true}, {Repeat: 2, At: at, Released: true}}
}

// rightClick presses the right button on a named part of the window.
func rightClick(at string) step { return step{Repeat: 2, At: at, Secondary: true} }

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
		t.Skip("set BERAD_GUI_TESTS=1 to run tests that open the real editor window")
	}
}

// scripts maps each view to the scripts its panes run, in the order the
// panes open.
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

// runEditor opens the sample project in the editor window with scripted
// panes, and waits for the editor to quit by itself.
func runEditor(t *testing.T, binary, project string, panes scripts) {
	t.Helper()
	runEditorWith(t, binary, project, panes)
}

// openZone is the project browser opening the sample zone in a Map: the
// editor opens with the project browser alone.
func openZone() []step {
	return script(must("row:Notes", 600), wait(5), doubleClick("row:Sample zone"))
}

// quitWhenAlone waits until the project browser's tab is the only one left,
// the test's other tabs having closed, then quits the editor from it.
func quitWhenAlone() []step {
	return script(
		must("state:tabs:1", 3600), wait(5), keys("Cmd+Q"), must("ui:quit", 300), wait(5), click("ui:quit"), wait(60),
	)
}

// browseZone is a project browser script that opens the sample zone, then
// quits the editor once the test's other tabs have closed.
func browseZone() []step {
	return script(openZone(), quitWhenAlone())
}

// TestMapPaintsDrawsAndUndoes drives a real Map: a drag paints the
// ground through the orchestrator, a drag with the Walls tool draws walls in
// the chosen family, undo takes the walls back in one step, and the result is
// in the saved file. Then the Resize tool adds a row on top with its button,
// drags the bottom edge down two rows, and undo takes the drag back. Last, a
// right-click on a cell adds an anchor there from its menu, and the Keys
// button lists the keys.
func TestMapPaintsDrawsAndUndoes(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	zoomed, menuShot := shotPath(t, "map-zoom.png"), shotPath(t, "map-menu.png")
	keysShot := shotPath(t, "map-keys.png")
	mapScript := script(
		expect("state:loaded", 600), wait(5), click("ui:zoom-fit"), wait(5),
		keys("3"), drag("cell:4,8", "cell:5,8", "cell:6,8", "cell:7,8"), wait(5),
		keys("W"), keys("3"), drag("cell:4,10", "cell:5,10", "cell:6,10"), wait(10),
		keys("Cmd+Z"), wait(60),
		keys("R"), click("ui:grow-top"), expect("state:bounds:-16,-13,32,25", 300), wait(5),
		drag("edge:bottom", "cell:0,13"), expect("state:bounds:-16,-13,32,27", 300), wait(5),
		keys("Cmd+Z"), expect("state:bounds:-16,-13,32,25", 300), wait(10),
		click("ui:zoom-in"), must("state:zoom:100", 60), click("ui:zoom-fit"), wait(5),
		shoot(zoomed), wait(5),
		rightClick("cell:-10,5"), must("menu:add-anchor", 60), shoot(menuShot),
		click("menu:add-anchor"), wait(30), typed("Gate"), keys("Enter"), wait(30),
		click("ui:keys"), must("state:keys", 60), shoot(keysShot), keys("Escape"), wait(5),
		quit(),
	)
	runEditor(t, binary, project, scripts{"map": {mapScript}, "browser": {browseZone()}})

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
	if gate, found := ground.AnchorAt(-10, 5); !found || gate.Name != "Gate" {
		t.Fatalf("anchor added from the right-click menu: %+v %v", gate, found)
	}
	noCrashes(t, project)
	t.Logf("screenshots: %s %s %s", zoomed, menuShot, keysShot)
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
		shoot(shot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"map": {mapScript}, "browser": {browseZone()}})

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

// TestAnchorTextAndPickOnMap adds and names an anchor in the Map, opens
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
	browserScript := script(openZone(), wait(30), click("row:Notes"), keys("Enter"), wait(30), quitWhenAlone())
	textScript := script(
		expect("state:loaded", 600), wait(5),
		click("ui:body"), keys("Cmd+Down"), keys("Cmd+Enter"),
		typed("caf"), typed("e"), step{Repeat: 2, Replace: &ui.Replacement{Before: 1, After: 0, Text: "é", Caret: 2}},
		typed(" ok"), keys("Cmd+S"), must("state:saved", 300), wait(120),
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

// TestQuitAsksThenClosesEveryTab names a new anchor in the Map without
// saving, then presses Cmd+Q in the project browser: the browser asks
// whether the editor should quit, naming the unsaved anchor, and Save and
// quit saves the name and closes every tab, so the editor exits.
func TestQuitAsksThenClosesEveryTab(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "quit.png")
	mapScript := script(
		expect("state:loaded", 600), wait(5),
		keys("A"), click("cell:4,6"), wait(30), typed("Well"), wait(1200), quit(),
	)
	browserScript := script(
		openZone(), wait(300), keys("Cmd+Q"), must("ui:quit-save", 300),
		shoot(shot), click("ui:quit-save"), wait(1200),
	)
	runEditor(t, binary, project, scripts{"map": {mapScript}, "browser": {browserScript}})

	ground, _ := readFloor(t, project, "ground-floor")
	well, found := ground.AnchorAt(4, 6)
	if !found || well.Name != "Well" {
		t.Fatalf("Save and quit saved the anchor's name: %+v %v", well, found)
	}
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestBrowserCreatesRenamesAndDeletes creates a zone, renames it in the
// inspector by typing a name and clicking Save, deletes the text document
// after confirming, and undoes the deletion. Its status bar keeps the editor
// window on top and lets it go again.
func TestBrowserCreatesRenamesAndDeletes(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "browser.png")
	browserScript := script(
		expect("row:Notes", 600), wait(5),
		click("ui:new-zone"), expect("row:Zone 1", 300), click("row:Zone 1"),
		click("ui:name"), keys("Cmd+A"), typed("Harbour"), shoot(shot), click("ui:save"),
		must("row:Harbour", 300),
		click("row:Notes"), click("ui:delete"), wait(30), click("ui:delete"), wait(30),
		keys("Cmd+Z"), expect("row:Notes", 300), wait(10),
		click("ui:on-top"), must("state:on-top", 60), wait(10), click("ui:on-top"), wait(5), quitWhenAlone(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}})

	harbour := readEnvelope(t, project, "zones/harbour--*.json")
	zoneDoc, err := zoneplugin.Decode(harbour.Data)
	if err != nil || len(zoneDoc.(*zoneplugin.Document).Zone.Floors()) != 1 {
		t.Fatalf("new zone %v", err)
	}
	readFloor(t, project, "floor-1")
	readEnvelope(t, project, "texts/notes--*.json")
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestBrowserFoldersTagsAndSearch makes a text folder from New ▾ and names
// it, drags the text document into it, makes a folder inside that one and
// moves the document there with Move to…, tags the document and finds it by
// searching for the tag. The saved file is in the inner folder, with its
// tag.
func TestBrowserFoldersTagsAndSearch(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot, menuShot := shotPath(t, "browser-folders.png"), shotPath(t, "browser-move.png")
	browserScript := script(
		expect("row:Notes", 600), wait(5),
		click("ui:new"), click("ui:new-folder-text"), wait(30), typed("Letters"), keys("Enter"),
		must("row:Letters", 300), wait(5),
		drag("row:Notes", "row:Letters"), wait(30),
		click("row:Letters"), click("ui:new-folder"), wait(30), typed("Drafts"), keys("Enter"),
		must("row:Drafts", 300), wait(5),
		click("row:Notes"), click("ui:move-to"), must("menu:Letters/Drafts", 60), shoot(menuShot),
		click("menu:Letters/Drafts"), wait(30),
		click("row:Notes"), click("ui:tags"), typed("quest"), keys("Enter"), wait(30),
		click("ui:search"), typed("#quest"), must("row:Notes", 60), shoot(shot),
		keys("Escape"), keys("Escape"), wait(5), quitWhenAlone(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}})

	notes := readEnvelope(t, project, "texts/Letters/Drafts/notes--*.json")
	if len(notes.Tags) != 1 || notes.Tags[0] != "quest" {
		t.Fatalf("the notes are in Letters/Drafts with their tag: %+v", notes.Tags)
	}
	noCrashes(t, project)
	t.Logf("screenshots: %s %s", shot, menuShot)
}

// TestBrowserRightClickMenu makes a workflow folder from the Workflows
// heading's right-click menu and names it, then a workflow inside the folder
// from the folder's own menu. The saved workflow is in the folder.
func TestBrowserRightClickMenu(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "browser-right-click.png")
	browserScript := script(
		expect("row:Notes", 600), wait(5),
		rightClick("section:workflows"), must("menu:new-folder-workflow", 60),
		click("menu:new-folder-workflow"), wait(30), typed("Routines"), keys("Enter"),
		must("row:Routines", 300), wait(5),
		rightClick("row:Routines"), must("menu:new-workflow", 60), shoot(shot),
		click("menu:new-workflow"), must("row:Workflow 1", 300), wait(30), quitWhenAlone(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}})

	readEnvelope(t, project, "workflows/Routines/workflow-1--*[^t].json")
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestBrowserMakesASpriteInAFolder sets the tile size to 24 in the Settings
// row's inspector, makes a sprite folder from the Sprites heading's
// right-click menu and names it, then a sprite inside it from the folder's
// own menu. The saved sprite is in the folder and names its tile, which is
// 24 pixels square.
func TestBrowserMakesASpriteInAFolder(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot, settingsShot := shotPath(t, "browser-sprite.png"), shotPath(t, "browser-settings.png")
	paletteShot := shotPath(t, "browser-palette.png")
	browserScript := script(
		expect("row:Notes", 600), wait(5),
		click("row:Settings"), click("ui:tile-size"), keys("Cmd+A"), typed("24"), shoot(settingsShot),
		click("ui:save"), must("row:Settings", 60), wait(30),
		click("row:Default"), wait(5), shoot(paletteShot),
		rightClick("section:sprites"), must("menu:new-folder-sprite", 60),
		click("menu:new-folder-sprite"), wait(30), typed("Lights"), keys("Enter"),
		must("row:Lights", 300), wait(5),
		rightClick("row:Lights"), must("menu:new-sprite", 60),
		click("menu:new-sprite"), must("row:Sprite 1", 300), wait(30), shoot(shot),
		quitWhenAlone(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}})

	settings, err := art.DecodeSettings(readEnvelope(t, project, "art-settings/*.json").Data)
	if err != nil || settings.TileSize != 24 {
		t.Fatalf("the tile size is saved: %+v %v", settings, err)
	}
	sprite, err := art.DecodeSprite(readEnvelope(t, project, "sprites/Lights/sprite-1--*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	tileFile := readEnvelope(t, project, "tiles/tile-1--*.json")
	tile, err := art.DecodeTile(tileFile.Data)
	if err != nil || tileFile.ID != sprite.Tile || tile.Size() != 24 {
		t.Fatalf("the sprite's tile is 24 pixels square: %+v %v", tileFile, err)
	}
	noCrashes(t, project)
	t.Logf("screenshots: %s %s %s", shot, settingsShot, paletteShot)
}

// TestPixelEditorPaintsAndUndoes makes a sprite in the project browser and
// opens it in a Pixel editor, which paints a stroke in Default's grey 3 and
// another in grey 5, then undoes the second in one step. Then it paints with
// the glass of the sample's palette, gloomy. The saved tile keeps the first stroke and the
// glass.
func TestPixelEditorPaintsAndUndoes(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	painted, undone := shotPath(t, "pixel-editor.png"), shotPath(t, "pixel-editor-undone.png")
	glassShot := shotPath(t, "pixel-editor-glass.png")
	browserScript := script(
		expect("row:Notes", 600), wait(5),
		rightClick("section:sprites"), must("menu:new-sprite", 60), click("menu:new-sprite"),
		must("row:Sprite 1", 300), wait(30), click("ui:open"), quitWhenAlone(),
	)
	spriteScript := script(
		must("state:loaded", 600), wait(5),
		click("swatch:Default / grey 3"), must("state:color:Default / grey 3", 60),
		drag("pixel:1,1", "pixel:2,1", "pixel:3,1"), must("state:pixel:3,1:Default / grey 3", 120),
		click("swatch:Default / grey 5"), drag("pixel:1,3", "pixel:2,3", "pixel:3,3"),
		must("state:pixel:3,3:Default / grey 5", 120), shoot(painted),
		keys("Cmd+Z"), must("state:pixel:1,3:clear", 120), must("state:pixel:3,3:clear", 60),
		must("state:pixel:2,1:Default / grey 3", 60), shoot(undone),
		click("palette:gloomy"), click("swatch:gloomy / glass 1"), must("state:color:gloomy / glass 1", 120),
		drag("pixel:5,0", "pixel:5,1", "pixel:5,2"), must("state:pixel:5,2:gloomy / glass 1", 120),
		shoot(glassShot), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "sprite": {spriteScript}})

	tileFile := readEnvelope(t, project, "tiles/tile-1--*.json")
	tile, err := art.DecodeTile(tileFile.Data)
	if err != nil {
		t.Fatal(err)
	}
	for col := range tile.Size() {
		first, _ := tile.At(col, 1)
		second, _ := tile.At(col, 3)
		if want := col >= 1 && col <= 3 || col == 5; first.IsClear() == want || !second.IsClear() {
			t.Fatalf("pixel %d: the first stroke is saved and the second undone: %+v %+v", col, first, second)
		}
	}
	if first, _ := tile.At(2, 1); first.Shade != art.DefaultShadeID(1, 3) {
		t.Fatalf("the first stroke is grey 3: %+v", first)
	}
	gloomy := readEnvelope(t, project, "palettes/gloomy--*.json")
	for row := range 3 {
		if ref, _ := tile.At(5, row); ref.Palette != gloomy.ID {
			t.Fatalf("pixel 5,%d is gloomy's glass: %+v", row, ref)
		}
	}
	noCrashes(t, project)
	t.Logf("screenshots: %s %s %s", painted, undone, glassShot)
}

// TestSpriteShowsOnTheMap makes a sprite show the small pickup, then draws
// it in the Pixel editor while the Map shows the pickup at 200%. The Pixel
// editor's Edit palette… opens the sample's palette, gloomy, in a Palette
// Editor, which adds a ramp of magenta ink: a stroke in it shows on the map as
// it is drawn, and giving the colour a new value, cyan, recolours it there. A
// fill of a 40% blue pane, another ramp the Palette Editor adds, shows the
// pickup's floor through it. Each is checked in the Map's screenshots, its tab
// brought to the front for each, since a hidden tab is not drawn. The
// sprite takes the small pickup over from the sample's lantern, whose smoky
// amber glass shows the floor through it first.
func TestSpriteShowsOnTheMap(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	stroke, recoloured := shotPath(t, "map-sprite-stroke.png"), shotPath(t, "map-sprite-recoloured.png")
	glassShot, lanternShot := shotPath(t, "map-sprite-glass.png"), shotPath(t, "map-lantern.png")
	paletteShot := shotPath(t, "palette-editor.png")
	browserScript := script(
		openZone(), wait(30),
		rightClick("section:sprites"), must("menu:new-sprite", 60), click("menu:new-sprite"),
		must("row:Sprite 1", 300), wait(30), click("ui:open"), quitWhenAlone(),
	)
	ramp := func(name, colour, opacity string) []step {
		return script(
			click("ui:add-ramp"), must("state:chosen:new ramp 1", 120),
			click("ui:ramp-name"), keys("Cmd+A"), typed(name),
			click("ui:colour"), keys("Cmd+A"), typed(colour),
			click("ui:opacity"), keys("Cmd+A"), typed(opacity),
			click("ui:save"), must("state:colour:"+name+" 1:"+colour, 120), must("state:saved", 120),
		)
	}
	paletteScript := script(
		must("state:loaded", 600), wait(5),
		ramp("ink", "#FF00FF", "100"), wait(240),
		ramp("pane", "#3060C0", "40"), wait(480), shoot(paletteShot),
		click("swatch:ink 1"), must("state:chosen:ink 1", 60),
		click("ui:colour"), keys("Cmd+A"), typed("#00FFFF"), click("ui:save"), must("state:colour:ink 1:#00FFFF", 120),
		wait(240), quit(),
	)
	spriteScript := script(
		must("state:loaded", 600), wait(5),
		click("ui:shows"), must("menu:standin:pickup.small", 60), click("menu:standin:pickup.small"),
		must("state:shows:pickup.small", 120),
		click("swatch:gloomy / glass 1"), must("state:color:gloomy / glass 1", 60), click("ui:edit-palette"),
		must("swatch:gloomy / ink 1", 1200), click("swatch:gloomy / ink 1"),
		drag("pixel:0,4", "pixel:31,4"), must("state:pixel:31,4:gloomy / ink 1", 120),
		must("swatch:gloomy / pane 1", 1200), click("swatch:gloomy / pane 1"),
		keys("G"), click("pixel:0,0"), must("state:pixel:0,0:gloomy / pane 1", 120), wait(900), quit(),
	)
	mapScript := script(
		expect("state:loaded", 600), wait(5),
		click("ui:zoom-fit"), click("ui:zoom-reset"), click("ui:zoom-in"), must("state:zoom:200", 60),
		must("state:sprite:pickup.small", 1800), front(), shoot(lanternShot),
		mustInFront("state:art:pickup.small:16,4:#FF00FF", 1200), shoot(stroke),
		mustInFront("state:art:pickup.small:16,4:#00FFFF", 1200), shoot(recoloured),
		mustInFront("state:art:pickup.small:0,0:#3060C0", 1200), shoot(glassShot),
		quit(),
	)
	runEditor(t, binary, project, scripts{
		"map": {mapScript}, "browser": {browserScript}, "sprite": {spriteScript}, "palette": {paletteScript},
	})
	noCrashes(t, project)

	if got := countColor(t, lanternShot, over(0x6b4a2a, 40, tiles.ColorOf("floor")), 3); got < 64 {
		t.Fatalf("the floor shows through the lantern's glass: %d pixels", got)
	}
	magenta, cyan := color.RGBA{R: 0xff, G: 0, B: 0xff, A: 0xff}, color.RGBA{R: 0, G: 0xff, B: 0xff, A: 0xff}
	if got := countColor(t, stroke, magenta, 0); got < 64 {
		t.Fatalf("the stroke shows on the map: %d magenta pixels", got)
	}
	if got, gone := countColor(t, recoloured, cyan, 0), countColor(t, recoloured, magenta, 0); got < 64 || gone != 0 {
		t.Fatalf("the new colour recolours it: %d cyan pixels, %d magenta", got, gone)
	}
	if got := countColor(t, glassShot, over(0x3060c0, 40, tiles.ColorOf("floor")), 3); got < 256 {
		t.Fatalf("the floor shows through the glass: %d pixels", got)
	}
	sprite := readEnvelope(t, project, "sprites/sprite-1--*.json")
	standIns, err := art.DecodeStandInSprites(readEnvelope(t, project, "standin-sprites/*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	if shown, ok := standIns.Sprite("pickup.small"); !ok || shown != sprite.ID {
		t.Fatalf("Sprite 1 shows the small pickup: %s %v", shown, ok)
	}
	t.Logf("screenshots: %s %s %s %s %s", lanternShot, stroke, recoloured, glassShot, paletteShot)
}

// countColor counts a screenshot's pixels within a tolerance of a colour.
func countColor(t *testing.T, path string, want color.RGBA, tolerance int) int {
	t.Helper()
	file, err := os.Open(path) //nolint:gosec // The test's own screenshot.
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // Read only.
	shot, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	near := func(a, b uint8) bool { return max(int(a)-int(b), int(b)-int(a)) <= tolerance }
	count := 0
	bounds := shot.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			got := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA) //nolint:forcetypeassert // RGBAModel's own type.
			if near(got.R, want.R) && near(got.G, want.G) && near(got.B, want.B) {
				count++
			}
		}
	}
	return count
}

// over is a colour at an opacity, in percent, drawn over an opaque ground.
func over(colour uint32, opacity int, ground uint32) color.RGBA {
	alpha := opacity * 255 / 100
	channel := func(shift int) uint8 {
		top, below := int(colour>>shift&0xff), int(ground>>shift&0xff)
		return uint8(top*alpha/255 + below*(255-alpha)/255)
	}
	return color.RGBA{R: channel(16), G: channel(8), B: channel(0), A: 0xff}
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
	runEditor(t, binary, project, scripts{"map": {first, second}, "browser": {browseZone()}})

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
// The pinned clock's +1h moves the preview's clock on an hour.
func TestPreviewWalksAndClimbs(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	walking, upstairs, seen := shotPath(t, "preview.png"), shotPath(t, "preview-upstairs.png"), shotPath(t, "preview-other-view.png")
	clockShot := shotPath(t, "clock.png")
	first := script(
		expect("state:loaded", 600), wait(5), keys("Cmd+N"), wait(120),
		keys("P"), click("cell:2,-2"), expect("state:previewing", 300),
		walk("Right"), expect("state:player:3,-2", 120), shoot(walking), wait(90),
		walk("Right"), expect("state:floor:2", 120), expect("state:player:4,-2", 120),
		shoot(upstairs), must("state:clock:09:", 1200), wait(30), keys("P"), wait(60), quit(),
	)
	clockScript := script(
		must("state:running", 1800), must("state:time:08:", 300),
		click("ui:hour-on"), must("state:time:09:", 300), shoot(clockShot), wait(10), quit(),
	)
	second := script(
		expect("state:loaded", 600), expect("state:remote-player:3,-2", 900),
		shoot(seen), expect("state:remote-player:4,-2", 600), wait(30), quit(),
	)
	runEditor(t, binary, project, scripts{"map": {first, second}, "browser": {browseZone()}, "clock": {clockScript}})

	ground, envelope := readFloor(t, project, "ground-floor")
	if cellOf(t, ground, tilefloor.LayerTop, 4, -2) != "stairs.up" || envelope.Revision != 0 {
		t.Fatalf("the preview must not change the project: revision %d", envelope.Revision)
	}
	for _, shot := range []string{walking, upstairs, seen, clockShot} {
		if _, err := os.Stat(shot); err != nil {
			t.Fatalf("missing screenshot: %v", err)
		}
		t.Logf("screenshot: %s", shot)
	}
}

// TestWorkflowBuildsARoutine creates a workflow in the project browser, opens
// it, and builds a routine from the palette in a real Workflow: start,
// a time switch, a time-bound container holding a Go to and a Wait, and a
// Wait for the rest of the day. The Go to's destination is picked on the Map
// (D13). The saved logic and its separate layout file are checked.
func TestWorkflowBuildsARoutine(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot, unsaved := shotPath(t, "workflow.png"), shotPath(t, "workflow-unsaved.png")
	browserScript := script(
		openZone(), wait(30), click("ui:new-workflow"), wait(30), click("ui:open"), quitWhenAlone(),
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
		click("ui:param-duration"), keys("Cmd+A"), typed("45"), shoot(unsaved),
		keys("Cmd+S"), wait(20),
		expect("state:nodes:6", 300), click("ui:arrange"), wait(20),
		shoot(shot), wait(10), quit(),
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
	if wait, _ := outer.Duration("duration"); wait.Amount != 45 {
		t.Fatalf("Cmd+S saved the typed amount: %+v", wait)
	}
	noCrashes(t, project)
	t.Logf("screenshots: %s %s", shot, unsaved)
}

// TestWorkflowRandomWalk builds the test routine "move up or down at
// random, wait two seconds, repeat" in a real Workflow: a random
// choice with two named options, a Move each way, and one Wait that both
// Moves lead into, with an entry dot for each, then a line from the Wait
// back to the random choice, drawn dashed as a loop, and auto-arranges it.
func TestWorkflowRandomWalk(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot, held := shotPath(t, "random-walk.png"), shotPath(t, "random-walk-held.png")
	retype := func(field, text string) []step {
		return script(click("ui:"+field), wait(3), keys("Cmd+A"), typed(text), keys("Enter"), wait(10))
	}
	browserScript := script(
		expect("row:Notes", 600), wait(5), click("ui:new-workflow"), wait(30), click("ui:open"), quitWhenAlone(),
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
		click("ui:arrange"), wait(20), step{Repeat: 1, At: "canvas:400,300", Wheel: &[2]float64{-7, 0}}, wait(10),
		drag("port:wait:1:next", "node:random:1"), expect("state:entries:random:1=2", 300), wait(10),
		step{Repeat: 3, At: "entry:random:1:2", Down: true, Pressed: true},
		step{Repeat: 10, At: "canvas:600,420", Down: true}, step{Repeat: 2, At: "canvas:600,420", Down: true, Screenshot: held},
		step{Repeat: 3, At: "node:random:1", Down: true}, step{Repeat: 2, At: "node:random:1", Released: true}, wait(10),
		drag("palette:note", "bend:wait:1:next"), expect("state:nodes:6", 300), wait(10),
		drag("bend:wait:1:next", "canvas:600,8"), expect("state:bent:wait:1:next", 300), wait(10),
		click("node:random:1"), wait(10), shoot(shot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})

	if layout := readEnvelope(t, project, "workflows/workflow-1--*.layout.json"); !strings.Contains(string(layout.Data), `"lines"`) {
		t.Fatalf("the loop moved by hand is saved with the layout: %s", layout.Data)
	}
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
	t.Logf("screenshots: %s %s", shot, held)
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
		openZone(), wait(30), click("ui:new-workflow"), wait(30), click("ui:new-npc"), wait(30),
		click("ui:choose-behaviour"), wait(10), click("row:Workflow 1"), wait(5), click("ui:use"), wait(30),
		click("ui:notes"), wait(5), typed("Opens the shop at eight."), keys("Cmd+S"), wait(30),
		shoot(browserShot), wait(10), quitWhenAlone(),
	)
	mapScript := script(
		expect("state:loaded", 600), wait(5), keys("N"), expect("npc:NPC 1", 1800), click("npc:NPC 1"), wait(5),
		click("cell:1,1"), expect("state:placement:1,1=NPC 1", 300), wait(120),
		shoot(mapShot), wait(10), quit(),
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

// TestNPCFollowsItsWorkflow builds the random walk, makes it an NPC's
// behaviour, places the NPC and starts a preview at 08:00: the NPC must
// leave its cell. Clicking it watches it, and Watch shows it on
// its workflow, pauses the NPCs and steps to its next node.
func TestNPCFollowsItsWorkflow(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	mapShot, watchShot := shotPath(t, "npc-preview-map.png"), shotPath(t, "npc-preview-watch.png")
	stepShot := shotPath(t, "npc-preview-step.png")
	retype := func(field, text string) []step {
		return script(click("ui:"+field), wait(3), keys("Cmd+A"), typed(text), keys("Enter"), wait(10))
	}
	browserScript := script(
		openZone(), wait(30), click("ui:new-workflow"), wait(30), click("ui:open"), wait(60),
		click("ui:new-npc"), wait(30), click("ui:choose-behaviour"), wait(10), click("row:Workflow 1"), wait(5),
		click("ui:use"), wait(30), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:random"), wait(10),
		click("palette:move"), wait(10), click("node:random:1"), wait(5), click("palette:move"), wait(10),
		click("ui:direction-down"), wait(10), click("node:move:1"), wait(5), click("palette:wait"), wait(10),
		retype("param-duration", "1"), click("ui:duration-seconds"), wait(10),
		drag("port:move:2:next", "entry:wait:1"), must("state:entries:wait:1=2", 300), wait(10), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("N"), must("npc:NPC 1", 1800), click("npc:NPC 1"), wait(5),
		click("cell:1,1"), must("state:placement:1,1=NPC 1", 300), wait(400),
		keys("P"), click("cell:2,-2"), must("state:previewing", 300), must("state:npcs-moved", 900),
		click("live-npc:NPC 1"), wait(5), click("live-npc:NPC 1"), must("state:watching", 300), wait(10),
		click("ui:watch"), wait(600), shoot(mapShot), wait(10), keys("P"), wait(30), quit(),
	)
	watchScript := script(
		must("state:previewing", 1800), must("state:watching:NPC 1", 600), wait(60),
		shoot(watchShot), wait(10), click("ui:pause"), must("state:paused", 300), wait(10),
		click("ui:step"), must("state:stepped", 300), wait(10), shoot(stepShot), wait(10), quit(),
	)
	clockScript := script(must("state:running", 3600), wait(30), quit())
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "workflow": {workflowScript}, "map": {mapScript}, "watch": {watchScript},
		"clock": {clockScript},
	})
	noCrashes(t, project)

	routine := readEnvelope(t, project, "workflows/workflow-1--*[^t].json")
	saved := readEnvelope(t, project, "npcs/npc-1--*.json")
	decoded, err := npcplugin.Decode(saved.Data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.(*npcplugin.Document).NPC.Behaviour != routine.ID {
		t.Fatal("the NPC follows the random walk")
	}
	ground, _ := readFloor(t, project, "ground-floor")
	if placement, found := ground.PlacementAt(1, 1); !found || placement.NPC != saved.ID {
		t.Fatal("the preview must not move the NPC's placement")
	}
	t.Logf("screenshots: %s %s %s", mapShot, watchShot, stepShot)
}

// TestSoundsInWorkflows chooses the sample's chime for a Play sound from
// the cue chooser and plays it, follows it with a one-second Wait, gives the
// workflow to an NPC and previews the zone: the Map running the
// preview plays the chime when the NPC reaches it. The saved node keeps the
// cue's ID.
func TestSoundsInWorkflows(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	chooserShot, chosenShot := shotPath(t, "sound-chooser.png"), shotPath(t, "sound-chosen.png")
	previewShot := shotPath(t, "sound-preview.png")
	browserScript := script(
		openZone(), wait(30), click("ui:new-workflow"), wait(30), click("ui:open"), wait(60),
		click("ui:new-npc"), wait(30), click("ui:choose-behaviour"), wait(10), click("row:Workflow 1"), wait(5),
		click("ui:use"), wait(30), quitWhenAlone(),
	)
	workflowScript := script(
		must("state:loaded", 600), wait(5),
		click("node:start:1"), wait(5), click("palette:play-sound"), must("state:cues", 300), wait(5),
		click("ui:cue"), must("cue:sample/chime", 300), wait(5), shoot(chooserShot),
		click("cue:sample/chime"), must("state:cue=sample/chime", 300), wait(5),
		click("ui:listen"), wait(30), shoot(chosenShot),
		click("palette:wait"), wait(10), click("ui:param-duration"), wait(3), keys("Cmd+A"), typed("1"),
		keys("Enter"), wait(10), click("ui:duration-seconds"), wait(10), quit(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("N"), must("npc:NPC 1", 1800), click("npc:NPC 1"), wait(5),
		click("cell:1,1"), must("state:placement:1,1=NPC 1", 300), wait(400),
		keys("P"), click("cell:2,-2"), must("state:previewing", 300), must("state:heard:sample/chime", 900),
		wait(30), shoot(previewShot), keys("P"), wait(30), quit(),
	)
	clockScript := script(must("state:running", 3600), wait(30), quit())
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "workflow": {workflowScript}, "map": {mapScript}, "clock": {clockScript},
	})
	noCrashes(t, project)

	logic := readEnvelope(t, project, "workflows/workflow-1--*[^t].json")
	decoded, err := workflowplugin.Decode(logic.Data)
	if err != nil {
		t.Fatal(err)
	}
	routine := decoded.(*workflowplugin.Document).Workflow
	start, _ := routine.Node(routine.Start())
	sound, found := routine.Node(start.Next[flow.OutputNext])
	if cue, err := sound.Cue("cue"); !found || err != nil || sound.Kind != flow.KindPlaySound || cue != "sample/chime" {
		t.Fatalf("the Play sound keeps the chime's ID: %+v", sound)
	}
	t.Logf("screenshots: %s %s %s", chooserShot, chosenShot, previewShot)
}

// TestCrashedTabLeavesAReport crashes a second view of the Map with the debug
// key: its tab closes alone, the orchestrator saves a crash report with the
// failure, and the other tabs carry on until the editor quits.
func TestCrashedTabLeavesAReport(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	first := script(expect("state:loaded", 600), keys("Cmd+N"), wait(240), quit())
	second := script(wait(30), keys("Cmd+Shift+K"))
	runEditor(t, binary, project, scripts{"map": {first, second}, "browser": {browseZone()}})

	reports, err := filepath.Glob(filepath.Join(project, ".berad", "logs", "*-crash.json"))
	if err != nil || len(reports) != 1 {
		t.Fatalf("crash reports %v %v", reports, err)
	}
	raw, err := os.ReadFile(reports[0])
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		View    string `json:"view"`
		Failure string `json:"failure"`
	}
	if err := json.Unmarshal(raw, &report); err != nil || report.View != "map" ||
		!strings.Contains(report.Failure, "simulated window crash") {
		t.Fatalf("report lacks the panic: %v\n%.500s", err, raw)
	}
}

// TestPaletteEditorEditsAPalette makes a palette from the Palettes section's
// [+], which opens in a Palette Editor. There it copies Default's slate
// ramp, labels a colour, adds a ramp, drags slate's first colour into it and
// adds a gradient. The saved palette holds all of it, with a shadow record of
// the moved colour's old place.
func TestPaletteEditorEditsAPalette(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "palette-editor-made.png")
	browserScript := script(
		expect("row:Notes", 600), wait(5), click("ui:new-palette"), must("row:Palette 1", 300), quitWhenAlone(),
	)
	paletteScript := script(
		must("state:loaded", 600), wait(5),
		click("ui:copy-from"), must("sourceramp:slate", 120), click("sourceramp:slate"), click("ui:copy"),
		must("state:ramps:slate", 120),
		click("swatch:slate 2"), click("ui:label"), typed("dusk"), click("ui:save"),
		must("state:label:slate 2:dusk", 120),
		click("picker:hue 0"), click("picker:square 100 100"), drag("picker:opacity 100", "picker:opacity 50"),
		click("ui:save"), must("state:colour:slate 2:#FF0000", 120),
		click("ui:add-ramp"), must("state:ramps:slate,new ramp", 120),
		drag("swatch:slate 1", "ramp:new ramp"), must("state:colour:new ramp 2:#090D17", 120),
		click("ui:gradient"),
		click("ui:left"), keys("Cmd+A"), typed("#2E2C33"), click("ui:right"), keys("Cmd+A"), typed("#E1E3E6"),
		click("ui:steps"), keys("Cmd+A"), typed("5"), click("ui:apply-gradient"),
		must("state:ramps:slate,new ramp,gradient", 120), shoot(shot), wait(30), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "palette": {paletteScript}})
	noCrashes(t, project)

	palette, err := art.DecodePalette(readEnvelope(t, project, "palettes/palette-1--*.json").Data)
	if err != nil {
		t.Fatal(err)
	}
	ramps := palette.Ramps()
	if len(ramps) != 3 || ramps[0].Name != "slate" || len(ramps[0].Shades) != 14 || ramps[0].Shades[0].Label != "dusk" ||
		ramps[0].Shades[0].Color != 0xFF0000 || ramps[0].Shades[0].Opacity != 50 ||
		ramps[1].Name != "new ramp" || len(ramps[1].Shades) != 2 || ramps[1].Shades[1].Color != 0x090D17 ||
		ramps[2].Name != "gradient" || len(ramps[2].Shades) != 5 || ramps[2].Shades[2].Color != 0x828187 {
		t.Fatalf("the saved palette: %+v", ramps)
	}
	moved := palette.MovedShades()
	if len(moved) != 1 || moved[0].Ramp != ramps[0].ID || moved[0].Name != "slate 1" {
		t.Fatalf("the shadow record of slate 1: %+v", moved)
	}
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}
