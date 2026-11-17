package player

import (
	"fmt"
	"io/fs"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
)

// How much of the display a game's window may take when it opens, and the
// colours of the room around the game and of what the window says.
const (
	windowShare = 0.8
	hexMessage  = 0xd8dce3
)

// Options are what a game's window plays.
type Options struct {
	// Project is the project folder. It is only read.
	Project fs.FS
	// Title is the window's title.
	Title string
	// Seed fixes the NPCs' random choices, so a run repeats.
	Seed uint64
	// Report is told, once each, what could not be read or played and why;
	// nil says nothing.
	Report func(string)
	// Script plays the game by itself in place of the keyboard, for tests
	// that open the real window; nil leaves the keyboard.
	Script []ScriptStep
}

// Window is a game in a window of its own: the project read from its
// folder, played by the engine's game runtime, drawn and heard through the
// player. It is an Ebitengine game.
type Window struct {
	screen *Screen
	sound  *Sound
	audio  *Audio
	faces  *Type
	hands  Hands
	// played is the game running, nil when it cannot start or has ended;
	// problem says why.
	played  *game.Game
	problem string
	// floors are the zone's floors from the bottom up, for "floor:" states.
	floors []ids.ULID
	// device is the screen's pixels to a point.
	device float64
	script *script
	// failed is why the script ended the game, if it did.
	failed error
}

// Open reads a project folder and starts its game at the player start. A
// project that cannot start, such as one with no player start, still opens:
// the window says why. An error is a folder that cannot be read at all.
func Open(options Options) (*Window, error) {
	report := options.Report
	if report == nil {
		report = func(string) {}
	}
	folder, err := gameload.ReadFolder(options.Project)
	if err != nil {
		return nil, fmt.Errorf("open project: %w", err)
	}
	for _, unread := range folder.Unread() {
		report(unread.Path + " cannot be read: " + unread.Reason)
	}
	faces, err := NewType()
	if err != nil {
		return nil, err
	}
	library, err := ReadAudio(options.Project)
	if err != nil {
		report("Sound is off: " + err.Error())
		if library, err = ReadAudio(noFiles{}); err != nil {
			return nil, err
		}
	}
	window := &Window{
		screen: NewScreen(), sound: NewSound(), audio: NewAudio(library, report), faces: faces, hands: Hands{held: 0},
		played: nil, problem: "", floors: nil, device: 1, script: nil, failed: nil,
	}
	if options.Script != nil {
		window.script = &script{steps: options.Script, index: 0, tick: 0, shot: "", quit: false}
	}
	loaded, _ := gameload.Load(folder, nil)
	loaded.Project.Seconds, loaded.Project.Seed = game.DefaultStartSeconds, options.Seed
	switch {
	case loaded.Project.Settings.Start.IsZero():
		window.problem = "This project has no player start yet."
	case loaded.Project.Zone == nil:
		window.problem = "The player start's zone is not in the project."
	}
	if window.problem != "" {
		return window, nil
	}
	// A project that cannot start is not an error of the window's: it opens
	// and says why.
	if played, startErr := game.New(loaded.Project); startErr != nil {
		window.problem = "The game cannot start: " + startErr.Error()
	} else {
		window.played, window.floors = played, loaded.Project.Zone.Floors()
		window.screen.Start(loaded, folder)
		played.KnowCues(library.CueTicks)
		window.sound.KnowRadius(library.Radius)
		window.sound.Start(window.audio, window.audio, played, loaded.Cues)
	}

	return window, nil
}

// noFiles is a project folder with nothing in it.
type noFiles struct{}

func (noFiles) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// Size is the game's screen in its own pixels, or zero when no game runs.
func (w *Window) Size() (int, int) {
	if w.played == nil {
		return 0, 0
	}
	view := w.played.View()

	return view.Columns * w.screen.Tile(), view.Rows * w.screen.Tile()
}

// Problem is why no game runs, or "".
func (w *Window) Problem() string { return w.problem }

// Update runs the game a tick: the arrow keys walk the player, and Enter,
// Space or a click closes a box that waits.
func (w *Window) Update() error {
	if w.script != nil && w.script.quit && w.script.shot == "" {
		return ebiten.Termination
	}
	held, closeBox := w.keys()
	if w.played != nil {
		step := w.hands.Step(w.played, held, closeBox)
		if step.Outcome == play.Lost {
			w.sound.Silence(w.audio, w.audio)
			w.played, w.problem = nil, "The player's floor is not in the zone."
		} else {
			w.sound.Update(w.audio, w.audio, w.played, step)
		}
	}
	w.audio.Update()
	if w.script != nil {
		if err := w.script.advance(w.State); err != nil {
			w.failed = err

			return ebiten.Termination
		}
	}

	return nil
}

// keys reads what is held and asked this tick, from the script when there
// is one.
func (w *Window) keys() (play.Direction, bool) {
	if w.script != nil {
		return w.script.hands()
	}
	var held play.Direction
	for key, direction := range map[ebiten.Key]play.Direction{
		ebiten.KeyArrowUp: play.North, ebiten.KeyArrowRight: play.East, ebiten.KeyArrowDown: play.South,
		ebiten.KeyArrowLeft: play.West,
	} {
		if ebiten.IsKeyPressed(key) {
			held |= direction
		}
	}
	closeBox := inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	return held, closeBox
}

// State answers what a script waits on: "playing" while a game runs,
// "problem:<start>" when none does and the window says why, and the states
// of State.
func (w *Window) State(state string) bool {
	switch {
	case state == "playing":
		return w.played != nil
	case len(state) > len(problemState) && state[:len(problemState)] == problemState:
		return w.problem != "" && len(w.problem) >= len(state)-len(problemState) &&
			w.problem[:len(state)-len(problemState)] == state[len(problemState):]
	case w.played == nil:
		return false
	}
	in, _ := State(state, w.played, w.screen, w.sound, w.floors)

	return in
}

// problemState starts the state that says why no game runs.
const problemState = "problem:"

// Draw draws the game as large as the window allows, on black, or why there
// is no game.
func (w *Window) Draw(screen *ebiten.Image) {
	screen.Fill(hex(hexDark))
	write := Writer{Type: w.faces, Scale: w.device}
	if w.played == nil {
		width := write.Width(w.problem, w.faces.Face)
		bounds := screen.Bounds()
		write.Draw(screen, w.problem, w.faces.Face, (int(float64(bounds.Dx())/w.device)-width)/half,
			int(float64(bounds.Dy())/w.device)/half, hex(hexMessage))
	} else {
		w.screen.Draw(screen, w.played)
		w.screen.DrawShown(screen, w.played, write)
	}
	if w.script != nil {
		if err := w.script.shoot(screen); err != nil && w.failed == nil {
			w.failed = err
		}
	}
}

// Layout answers the window's size in the screen's own pixels, so the game
// is drawn at the screen's resolution.
func (w *Window) Layout(outsideWidth, outsideHeight int) (int, int) {
	w.device = 1
	if monitor := ebiten.Monitor(); monitor != nil {
		if scale := monitor.DeviceScaleFactor(); scale > 0 {
			w.device = scale
		}
	}

	return int(math.Ceil(float64(outsideWidth) * w.device)), int(math.Ceil(float64(outsideHeight) * w.device))
}

// Err is why the window's script ended the game, or nil.
func (w *Window) Err() error { return w.failed }

// Close stops the game's sounds, when the window has closed.
func (w *Window) Close() error {
	if err := w.audio.Close(); err != nil {
		return fmt.Errorf("close sound: %w", err)
	}

	return nil
}

// Run opens a project folder in a window of its own and plays it until the
// window closes. The window opens at the largest whole number of times the
// game's screen that fits the display, and can be resized.
func Run(options Options) error {
	window, err := Open(options)
	if err != nil {
		return err
	}
	width, height := window.Size()
	if width == 0 || height == 0 {
		settings := game.DefaultSettings()
		width, height = settings.Columns*defaultTile, settings.Rows*defaultTile
	}
	times := 1
	if monitor := ebiten.Monitor(); monitor != nil {
		wide, high := monitor.Size()
		times = max(1, int(min(windowShare*float64(wide)/float64(width), windowShare*float64(high)/float64(height))))
	}
	ebiten.SetWindowSize(width*times, height*times)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle(options.Title)
	runErr := ebiten.RunGame(window)
	closeErr := window.Close()
	switch {
	case window.Err() != nil:
		return window.Err()
	case runErr != nil:
		return fmt.Errorf("run game: %w", runErr)
	}

	return closeErr
}

// defaultTile is the tile size a window with no game is sized for.
const defaultTile = 24
