package window

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bradhannah/berad-engine/editor/views/mapview"
)

// scriptEnv names a JSON file of input steps that replaces real input, so the
// whole window can be exercised by an automated test.
const scriptEnv = "BERAD_INPUT_SCRIPT"

// scriptStep is one scripted input, held for Repeat frames. Cell, when set,
// places the cursor at the centre of that floor cell.
type scriptStep struct {
	Repeat   int     `json:"repeat"`
	Cell     *[2]int `json:"cell"`
	Down     bool    `json:"down"`
	Pressed  bool    `json:"pressed"`
	Released bool    `json:"released"`
	Undo     bool    `json:"undo"`
	Redo     bool    `json:"redo"`
	NewView  bool    `json:"newView"`
	Quit     bool    `json:"quit"`
	Crash    bool    `json:"crash"`
	TileKey  int     `json:"tileKey"`
}

type inputScript struct {
	steps []scriptStep
	index int
	frame int
}

func loadScript() (*inputScript, error) {
	path := claimScript(os.Getenv(scriptEnv))
	if path == "" {
		return nil, nil //nolint:nilnil // No script is the normal case, not an error.
	}
	raw, err := os.ReadFile(path) //nolint:gosec // The path comes from the developer's own environment.
	if err != nil {
		return nil, fmt.Errorf("read input script: %w", err)
	}
	var steps []scriptStep
	if err := json.Unmarshal(raw, &steps); err != nil {
		return nil, fmt.Errorf("parse input script: %w", err)
	}

	return &inputScript{steps: steps, index: 0, frame: 0}, nil
}

// next returns this frame's input. When the script runs out it returns no
// input, leaving the window idle.
func (s *inputScript) next(state *mapview.State, layout mapview.Layout) mapview.Input {
	var input mapview.Input
	if s.index >= len(s.steps) {
		return input
	}
	step := s.steps[s.index]
	if step.Cell != nil {
		center := state.CellCenter(step.Cell[0], step.Cell[1], layout)
		input.CursorX, input.CursorY = center.X, center.Y
	}
	input.Down = step.Down
	input.Pressed = step.Pressed && s.frame == 0
	input.Released = step.Released && s.frame == 0
	input.Undo = step.Undo && s.frame == 0
	input.Redo = step.Redo && s.frame == 0
	input.NewView = step.NewView && s.frame == 0
	input.Quit = step.Quit && s.frame == 0
	input.SimulateCrash = step.Crash && s.frame == 0
	if s.frame == 0 {
		input.TileKey = step.TileKey
	}
	s.frame++
	if s.frame >= max(step.Repeat, 1) {
		s.index++
		s.frame = 0
	}

	return input
}

// claimScript takes the first unclaimed script from a comma-separated list by
// renaming it, so each window started in a test runs its own script once.
func claimScript(list string) string {
	if list == "" {
		return ""
	}
	for _, path := range strings.Split(list, ",") {
		claimed := path + ".claimed"
		// Test scripts come from the developer's own environment.
		if err := os.Rename(path, claimed); err == nil { //nolint:gosec // See above.
			return claimed
		}
	}

	return ""
}
