package main_test

import (
	"os"
	"path/filepath"
	"testing"
)

// smallWindow opens a scripted run at 150% in a window of 1200 × 832 of the
// screen's points, which is 800 × 555 of the window's (increment 61).
func smallWindow() []string { return []string{"BERAD_TEXT_SCALE=150", "BERAD_WINDOW_SIZE=1200x832"} }

// inspectorPoint is a point in the middle of a view's inspector in the small
// window, for scrolling it with the wheel.
const inspectorPoint = "point:660,300"

// scrolled scrolls the inspector's fields down with the wheel.
func scrolled() step {
	return step{Repeat: 2, At: inspectorPoint, Wheel: &[2]float64{0, -20}}
}

// TestViewsFitASmallWindow opens each view whose layout did not fit a small
// window at 150% in a window of 1200 × 832, which is 800 × 555 of the
// window's points, and saves what it shows, to be looked at: the Project
// window's pictures and cards with their views' buttons below the path,
// World state's value with its fields scrolled above the save bar, the
// Palette Editor's colour and its gradient, the Pixel editor's toolbar on
// two lines, Audio's toolbar, panel and inspector, and a previewing Map.
// Watch, with no room beside its Map, opens in a tab of its own, and a
// larger step of text and controls is refused (increment 61).
func TestViewsFitASmallWindow(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	shots := map[string]string{}
	for _, name := range []string{
		"small-project-pictures", "small-project-cards", "small-world-state", "small-world-state-scrolled",
		"small-palette-colour", "small-palette-gradient", "small-pixel-editor", "small-map-preview",
		"small-watch-own-tab", "small-text-refused", "small-audio-cue", "small-audio-scrolled",
	} {
		shots[name] = shotPath(t, name+".png")
	}
	smallProject(t, binary, shots)
	smallWorldState(t, binary, shots)
	smallPalette(t, binary, shots)
	smallPixelEditor(t, binary, shots)
	smallMap(t, binary, shots)
	smallAudio(t, binary, shots)
	t.Logf("screenshots: %v", shots)
}

func smallProject(t *testing.T, binary string, shots map[string]string) {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	browserScript := script(
		must("place:Home", 600), wait(30), click("place:Sprites"), must("state:pane=pictures", 60), wait(60),
		shoot(shots["small-project-pictures"]), click("place:NPCs"), must("state:pane=cards", 60), wait(60),
		shoot(shots["small-project-cards"]), quitWhenAlone(),
	)
	runEditorWith(t, binary, project, scripts{"browser": {browserScript}}, smallWindow()...)
	noCrashes(t, project)
}

func smallWorldState(t *testing.T, binary string, shots map[string]string) {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	browserScript := script(
		must("row:Values", 600), wait(5), doubleClick("row:Values"), must("state:tabs:2", 600), quitWhenAlone(),
	)
	stateScript := script(
		must("state:loaded", 600), must("row:npc.shop.waiting", 600), wait(5), click("ui:add-value"),
		must("state:selected=world.value", 300), wait(5), click("ui:value-type-choice"),
		must("state:value=world.value:choice:first", 300), wait(30), shoot(shots["small-world-state"]),
		scrolled(), wait(30), shoot(shots["small-world-state-scrolled"]), wait(10), quit(),
	)
	runEditorWith(t, binary, project, scripts{"browser": {browserScript}, "world-state": {stateScript}},
		smallWindow()...)
	noCrashes(t, project)
}

func smallPalette(t *testing.T, binary string, shots map[string]string) {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	browserScript := script(
		expect("row:Sample zone", 600), wait(5), click("ui:new-palette"), must("row:Palette 1", 300),
		must("state:tabs:2", 600), quitWhenAlone(),
	)
	paletteScript := script(
		must("state:loaded", 600), wait(5),
		click("ui:copy-from"), must("sourceramp:slate", 120), click("sourceramp:slate"), click("ui:copy"),
		must("state:ramps:slate", 120), click("swatch:slate 2"), wait(30), shoot(shots["small-palette-colour"]),
		click("ui:gradient"), click("ui:mix-opacity"), wait(30), shoot(shots["small-palette-gradient"]), wait(10),
		quit(),
	)
	runEditorWith(t, binary, project, scripts{"browser": {browserScript}, "palette": {paletteScript}},
		smallWindow()...)
	noCrashes(t, project)
}

func smallPixelEditor(t *testing.T, binary string, shots map[string]string) {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	browserScript := script(
		expect("row:Sample zone", 600), wait(5),
		rightClick("section:sprites"), must("menu:new-sprite", 60), click("menu:new-sprite"),
		must("row:Sprite 1", 300), wait(30), click("ui:open"), quitWhenAlone(),
	)
	spriteScript := script(
		must("state:loaded", 600), wait(5), click("swatch:Default / grey 3"),
		drag("pixel:1,1", "pixel:2,1", "pixel:3,1"), must("state:pixel:3,1:Default / grey 3", 120), wait(30),
		shoot(shots["small-pixel-editor"]), quit(),
	)
	runEditorWith(t, binary, project, scripts{"browser": {browserScript}, "sprite": {spriteScript}},
		smallWindow()...)
	noCrashes(t, project)
}

// smallMap previews the sample zone: Watch, with no room beside the Map,
// opens in a tab of its own, so the editor has three, and a larger step of
// text and controls, at which the Map would not fit, leaves them at 150%.
func smallMap(t *testing.T, binary string, shots map[string]string) {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), must("state:tabs:2", 600),
		must("state:tabs:3", 900), quitWhenAlone(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(5), keys("P"), click("cell:2,-2"), must("state:previewing", 300),
		click("ui:watch"), wait(120), shoot(shots["small-map-preview"]), must("state:text-scale=150", 60),
		keys("Cmd+Shift+Equal"), wait(10), must("state:text-scale=150", 60), shoot(shots["small-text-refused"]),
		wait(10), keys("P"), wait(10), quit(),
	)
	watchScript := script(
		must("state:previewing", 900), wait(30), shoot(shots["small-watch-own-tab"]), must("state:tabs:3", 60),
		wait(300), quit(),
	)
	runEditorWith(t, binary, project, scripts{"browser": {browserScript}, "map": {mapScript}, "watch": {watchScript}},
		smallWindow()...)
	noCrashes(t, project)
}

func smallAudio(t *testing.T, binary string, shots map[string]string) {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	source := filepath.Join(t.TempDir(), "padded_knock.wav")
	if err := os.WriteFile(source, paddedKnockWAV(), 0o600); err != nil {
		t.Fatal(err)
	}
	browserScript := script(must("section:audio", 600), wait(5), click("ui:new-sound"),
		must("row:Padded knock", 1800), wait(5), quitWhenAlone())
	audioScript := script(
		must("state:dialog", 900), wait(5), click("ui:dialog-ok"),
		must("state:selected=cue:sfx/padded_knock", 900), must("state:waveform", 900), wait(30),
		shoot(shots["small-audio-cue"]), scrolled(), wait(30), shoot(shots["small-audio-scrolled"]), quit(),
	)
	env := append(smallWindow(), "BERAD_TEST_SOUND_FILE="+source)
	runEditorWith(t, binary, project, scripts{"browser": {browserScript}, "audio": {audioScript}}, env...)
	noCrashes(t, project)
}
