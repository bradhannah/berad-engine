package window

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// scriptEnv names a comma-separated list of JSON files of input steps, each
// optionally prefixed with the view it is for ("map=/tmp/a.json"). Each window
// claims the first unclaimed file for its view, or else an unprefixed one, and
// runs it instead of real input, so a whole window can be exercised by an
// automated test.
const scriptEnv = "BERAD_INPUT_SCRIPT"

// scriptStep is one scripted input, held for Repeat frames. At places the
// cursor on a named part of the view ("cell:3,4", "ui:pick", "row:Town");
// until a step names one, the cursor stays where it was. Keys are pressed on
// the first frame, written as "Cmd+Shift+Z". Text and Replace stand in for
// the input method: Replace is what pressing and holding a key for an accent
// produces. Hold names keys held down for every frame of the step.
type scriptStep struct {
	Repeat   int              `json:"repeat"`
	At       string           `json:"at"`
	Down     bool             `json:"down"`
	Pressed  bool             `json:"pressed"`
	Released bool             `json:"released"`
	Double   bool             `json:"double"`
	Keys     []string         `json:"keys"`
	Hold     []string         `json:"hold"`
	Text     string           `json:"text"`
	Replace  *ui.Replacement  `json:"replace"`
	Wheel    *[2]float64      `json:"wheel"`
	Held     string           `json:"held"`
	Space    bool             `json:"space"`
	Expect   *scriptExpection `json:"expect"`
	// Screenshot saves the window to this path after the first frame of the
	// step.
	Screenshot string `json:"screenshot"`
}

// scriptExpection makes a step wait, up to Repeat frames, until a named part
// of the view exists, so scripts can wait for documents to load or dialogs to
// open. A Must expectation that is not met by then crashes the window, so the
// crash report fails the test that ran it.
type scriptExpection struct {
	Locate string `json:"locate"`
	Must   bool   `json:"must"`
}

type inputScript struct {
	steps  []scriptStep
	index  int
	frame  int
	cursor image.Point
	shot   string
}

// takeScreenshot returns a screenshot path a step asked for, once.
func (s *inputScript) takeScreenshot() string {
	shot := s.shot
	s.shot = ""

	return shot
}

// scripted reports whether input scripts drive the editor's windows.
func scripted() bool { return os.Getenv(scriptEnv) != "" }

func loadScript(view string) (*inputScript, error) {
	path := claimScript(os.Getenv(scriptEnv), view)
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

	return &inputScript{steps: steps, index: 0, frame: 0, cursor: image.Pt(0, 0), shot: ""}, nil
}

// next returns this frame's input. When the script runs out it returns no
// input, leaving the window idle.
// locator finds named parts of a window for input scripts.
type locator interface {
	Locate(name string, docs frame.Docs) (image.Point, bool)
}

func (s *inputScript) next(view locator, docs frame.Docs) ui.Input {
	input := ui.NewInput(s.cursor)
	if s.index >= len(s.steps) {
		input.Cursor = s.cursor

		return input
	}
	step := s.steps[s.index]
	if step.Expect != nil {
		s.expect(step, view, docs)
		input.Cursor = s.cursor

		return input
	}
	if step.At != "" {
		if point, found := view.Locate(step.At, docs); found {
			s.cursor = point
		}
	}
	input.Cursor = s.cursor
	first := s.frame == 0
	input.Down = step.Down
	input.Pressed = step.Pressed && first
	input.Released = step.Released && first
	input.DoubleClick = step.Double && first
	input.Held = ui.ParsePress(step.Held + "+x").Mods
	input.Space = step.Space
	for _, key := range step.Hold {
		input.Keys = append(input.Keys, ui.Key(key))
	}
	if first {
		for _, key := range step.Keys {
			input.Presses = append(input.Presses, ui.ParsePress(key))
		}
		input.Text = step.Text
		input.Replace = step.Replace
		s.shot = step.Screenshot
		if step.Wheel != nil {
			input.WheelX, input.WheelY = step.Wheel[0], step.Wheel[1]
		}
	}
	s.frame++
	if s.frame >= max(step.Repeat, 1) {
		s.advance()
	}

	return input
}

// expect waits a frame for an expectation, and moves on once it is met or
// its frames run out; a Must expectation that is not met crashes the window.
func (s *inputScript) expect(step scriptStep, view locator, docs frame.Docs) {
	_, found := view.Locate(step.Expect.Locate, docs)
	if !found && s.frame < max(step.Repeat, 1) {
		s.frame++

		return
	}
	if !found && step.Expect.Must {
		panic(fmt.Sprintf("input script: %q was not met in %d frames", step.Expect.Locate, step.Repeat))
	}
	s.advance()
}

func (s *inputScript) advance() {
	s.index++
	s.frame = 0
}

// claimScript takes the first unclaimed script for a view from a
// comma-separated list by renaming it, so each window started in a test runs
// its own script once.
func claimScript(list, view string) string {
	if list == "" {
		return ""
	}
	entries := strings.Split(list, ",")
	for _, wantPrefix := range []bool{true, false} {
		for _, entry := range entries {
			prefix, path, prefixed := strings.Cut(entry, "=")
			if !prefixed {
				path = entry
			}
			if prefixed != wantPrefix || (prefixed && prefix != view) {
				continue
			}
			claimed := path + ".claimed"
			// Test scripts come from the developer's own environment.
			if err := os.Rename(path, claimed); err == nil { //nolint:gosec // See above.
				return claimed
			}
		}
	}

	return ""
}
