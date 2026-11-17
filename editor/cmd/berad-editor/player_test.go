package main_test

import (
	"encoding/json"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/player"
)

// buildPlayer builds berad-play, the engine's program that plays a project
// in a window of its own.
func buildPlayer(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "berad-play")
	build := exec.Command("go", "build", "-o", binary, "./cmd/berad-play")
	build.Dir = filepath.Join("..", "..", "..", "engine")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build berad-play: %v\n%s", err, output)
	}
	return binary
}

// TestPlayerPlaysTheSampleFromItsFolder makes the sample with the editor,
// closes the editor, and plays the sample's folder with berad-play, in a
// window of its own (increment 62): the walk TestLaunchPlaysTheSample takes
// in the Play tab. The walls show brick, the player and the clerk their
// figures, the tune and the hum play and the clerk rings the chime; the
// player walks to the stairs and up them. Playing changes no file.
func TestPlayerPlaysTheSampleFromItsFolder(t *testing.T) {
	requireGUI(t)
	project := filepath.Join(t.TempDir(), "project")
	sample := exec.Command(buildEditor(t), "open", "--project", project, "--sample", "--no-window", "--exit-after", "1s")
	if output, err := sample.CombinedOutput(); err != nil {
		t.Fatalf("make the sample: %v\n%s", err, output)
	}
	before := folderFiles(t, project)
	started, upstairs := shotPath(t, "player-start.png"), shotPath(t, "player-upstairs.png")
	walking := func(key string, cells int) player.ScriptStep {
		return player.ScriptStep{Repeat: cells*play.StepTicks - play.StepTicks/2, Hold: []string{key}}
	}
	must := func(state string, ticks int) player.ScriptStep { return player.ScriptStep{Repeat: ticks, Must: state} }
	steps := []player.ScriptStep{
		must("playing", 1800), must("sprite:player", 120), must("sprite:wall.a", 60), must("player:0,2", 60),
		must("layer:sample/tune", 900), must("layer:sample/hum", 900), must("heard:sample/chime", 900),
		{Repeat: 30, Screenshot: started},
		walking("Right", 4), must("player:4,2", 120), walking("Up", 4), must("floor:2", 120),
		{Repeat: 30, Screenshot: upstairs}, {Repeat: 2, Quit: true},
	}
	encoded, err := json.Marshal(steps)
	if err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(t.TempDir(), "walk.json")
	if err := os.WriteFile(scriptPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	played := exec.Command(buildPlayer(t), "--project", project, "--seed", "1")
	played.Env = append(os.Environ(), "BERAD_PLAY_SCRIPT="+scriptPath)
	if output, err := played.CombinedOutput(); err != nil || len(output) != 0 {
		t.Fatalf("berad-play plays the sample and has nothing to report: %v\n%s", err, output)
	}
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
	if after := folderFiles(t, project); !sameFiles(before, after) {
		t.Fatalf("playing changes no file of the project:\nbefore %v\nafter  %v", before, after)
	}
	t.Logf("screenshots: %s %s", started, upstairs)
}

// folderFiles lists every file under a folder with its size and when it was
// last written.
func folderFiles(t *testing.T, dir string) map[string][2]int64 {
	t.Helper()
	files := map[string][2]int64{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		files[path] = [2]int64{info.Size(), info.ModTime().UnixNano()}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func sameFiles(a, b map[string][2]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for path, stamp := range a {
		if b[path] != stamp {
			return false
		}
	}
	return true
}
