package player

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/engine/play"
)

// ErrScript reports a script that cannot be read, or one whose game never
// reached a state it must.
var ErrScript = errors.New("the game's script failed")

// ScriptStep is one step of a script that plays a game by itself, in place
// of the keyboard, so a test can walk a game in its real window. A step
// lasts Repeat ticks. Hold names the arrow keys held throughout it, as "Up",
// "Right", "Down" and "Left"; Close asks, on its first tick, to close a box
// that waits. Must names a state the step waits for, ending as soon as the
// game is in it; one never reached in Repeat ticks ends the game with an
// error. Screenshot saves the window to a PNG at that path when the step
// ends, and Quit ends the game when it does.
type ScriptStep struct {
	Repeat     int      `json:"repeat"`
	Hold       []string `json:"hold,omitempty"`
	Close      bool     `json:"close,omitempty"`
	Must       string   `json:"must,omitempty"`
	Screenshot string   `json:"screenshot,omitempty"`
	Quit       bool     `json:"quit,omitempty"`
}

// ReadScript reads a script from a JSON file of steps.
func ReadScript(path string) ([]ScriptStep, error) {
	data, err := os.ReadFile(path) //nolint:gosec // The path comes from the developer's own environment.
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrScript, err)
	}
	var steps []ScriptStep
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrScript, err)
	}

	return steps, nil
}

// script runs a window's script: the step it is on, the ticks that step has
// run, and the screenshot waiting to be taken when the window next draws.
type script struct {
	steps []ScriptStep
	index int
	tick  int
	shot  string
	// quit says the script has ended the game, once its last screenshot is
	// taken.
	quit bool
}

// hands is what the script holds this tick.
func (s *script) hands() (play.Direction, bool) {
	if s.index >= len(s.steps) {
		return 0, false
	}
	step := s.steps[s.index]
	var held play.Direction
	for _, key := range step.Hold {
		held |= map[string]play.Direction{"Up": play.North, "Right": play.East, "Down": play.South, "Left": play.West}[key]
	}

	return held, step.Close && s.tick == 0
}

// advance moves the script on after a tick, given whether the game is in a
// state; a state a step must reach and never does is an error.
func (s *script) advance(inState func(state string) bool) error {
	if s.index >= len(s.steps) {
		return nil
	}
	step := s.steps[s.index]
	s.tick++
	reached := step.Must != "" && inState(step.Must)
	if !reached && s.tick < step.Repeat {
		return nil
	}
	if step.Must != "" && !reached {
		return fmt.Errorf("%w: step %d: the game never reached %q in %d ticks", ErrScript, s.index+1, step.Must, step.Repeat)
	}
	s.shot, s.quit = step.Screenshot, s.quit || step.Quit
	s.index, s.tick = s.index+1, 0

	return nil
}

// shoot saves the window's pixels when a step asked for a screenshot.
func (s *script) shoot(screen *ebiten.Image) error {
	if s.shot == "" {
		return nil
	}
	path := s.shot
	s.shot = ""
	pixels := image.NewRGBA(screen.Bounds())
	screen.ReadPixels(pixels.Pix)
	file, err := os.Create(path) //nolint:gosec // The path comes from the developer's own script.
	if err != nil {
		return fmt.Errorf("%w: %w", ErrScript, err)
	}
	if err := errors.Join(png.Encode(file, pixels), file.Close()); err != nil {
		return fmt.Errorf("%w: %w", ErrScript, err)
	}

	return nil
}
