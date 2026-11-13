package main_test

import (
	"image/color"
	"path/filepath"
	"testing"

	"github.com/bradhannah/berad-engine/engine/play"
)

// hold holds a key long enough to walk a number of cells, letting go before
// another step starts.
func hold(key string, cells int) step {
	return step{Repeat: cells*play.StepTicks - play.StepTicks/2, Hold: []string{key}, Front: true}
}

// TestLaunchPlaysTheSample presses Cmd+R in the project browser, which opens
// the Play tab at the sample's player start, inside the door: the walls show
// brick, the player and the clerk their figures, the tune plays throughout
// and the hum, three cells away, within its distance; the clerk, following
// the clerk's day, goes to the desk and rings the chime. The player walks to the
// stairs and up them; Cmd+R launches again, back at the start.
func TestLaunchPlaysTheSample(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	started, upstairs := shotPath(t, "play-start.png"), shotPath(t, "play-upstairs.png")
	browserScript := script(must("row:Sample zone", 600), wait(5), keys("Cmd+R"), must("state:tabs:2", 1800), quitWhenAlone())
	playScript := script(
		must("state:playing", 1800), mustInFront("state:sprite:player", 120), must("state:sprite:wall.a", 60),
		must("state:player:0,2", 60), must("state:layer:sample/tune", 900), must("state:layer:sample/hum", 900),
		must("state:heard:sample/chime", 900),
		wait(30), shoot(started),
		hold("Right", 4), must("state:player:4,2", 120), hold("Up", 4), mustInFront("state:floor:2", 120),
		wait(30), shoot(upstairs), keys("Cmd+R"), must("state:launches:2", 900), must("state:player:0,2", 120),
		must("state:floor:1", 60), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "play": {playScript}})
	noCrashes(t, project)
	brick := color.RGBA{R: 0x30, G: 0x2c, B: 0x38, A: 0xff}
	bricks := countColor(t, started, brick, 0)
	if bricks < 1000 {
		t.Fatalf("the walls show brick: %d pixels", bricks)
	}
	head := color.RGBA{R: 0x54, G: 0x4e, B: 0x60, A: 0xff}
	if got := countColor(t, started, head, 0); got < 100 {
		t.Fatalf("the player and the clerk show their figures: %d pixels", got)
	}
	if got := countColor(t, upstairs, brick, 0); got*4 >= bricks {
		t.Fatalf("upstairs, family B's walls have no sprite, only the hero is that grey: %d pixels", got)
	}
	t.Logf("screenshots: %s %s", started, upstairs)
}

// TestSampleRoutineShowsEachNode opens the clerk's day in a Workflow tab, laid
// out as Auto-arrange lays it out, with a node of every kind.
func TestSampleRoutineShowsEachNode(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "clerks-day.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Clerk's day"), must("state:tabs:2", 600), quitWhenAlone(),
	)
	workflowScript := script(must("state:loaded", 900), must("state:nodes:25", 60), wait(20), shoot(shot), quit())
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "workflow": {workflowScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}

// TestPlayFromHere right-clicks a cell of the sample's ground floor and
// chooses Play from here: the Play tab launches with the player on that
// cell, and Cmd+R launches again at the player start (increment 47).
func TestPlayFromHere(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "play-from-here.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(5), doubleClick("row:Sample zone"), must("state:tabs:2", 600),
		quitWhenAlone(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(30), rightClick("cell:3,1"), must("menu:play-here", 60),
		click("menu:play-here"), wait(1500), quit(),
	)
	playScript := script(
		must("state:playing", 1800), must("state:player:3,1", 120), wait(30), shoot(shot), keys("Cmd+R"),
		must("state:launches:2", 900), must("state:player:0,2", 120), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "map": {mapScript}, "play": {playScript}})
	noCrashes(t, project)
	t.Logf("screenshot: %s", shot)
}
