// Package playview is the Play tab (increment 14): Launch plays the project
// as a game would, with nothing of the editor around it. It takes a snapshot
// of what the game needs when it launches, starts the engine's game runtime
// at the player start, and draws the game's screen, the settings' columns
// and rows of cells, scaled by whole numbers with the player in the middle.
// Every sprite shows, and every sound is heard: what NPCs ask for, the
// ambient sounds on the player's floor by distance, and the zone's sounds.
// Edits made while it runs change nothing until it is launched again.
package playview

import (
	"encoding/json"
	"hash/fnv"
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
)

// ViewName is the Play tab's view name; its target is the game settings.
const ViewName = "play"

// LaunchItem is the item Launch opens the Play tab with: an open Play tab
// that is asked to show it launches again.
const LaunchItem = "launch"

// State is the Play tab.
type State struct {
	window ids.ULID
	theme  *ui.Theme
	size   image.Point
	subs   map[ids.ULID]bool
	// game is the game running, nil until it has launched; problem says why
	// it cannot launch, and launching that it waits to.
	game      *game.Game
	problem   string
	launching bool
	launches  int
	// zone and floors are the zone being played and its floors from the
	// bottom up, for "floor:" states.
	floors []ids.ULID
	// held is the directions held last frame; shown says the tab is in
	// front, so the game runs.
	held  play.Direction
	shown bool
	// standIns and npcSprites are the sprites that show each stand-in and
	// the player, and each NPC, as the launch read them.
	standIns   map[string]ids.ULID
	npcSprites map[ids.ULID]ids.ULID
	art        *artSet
	sound      *sounds
	// canvas is the game's screen at one pixel per tile pixel; scale is how
	// many screen pixels each of its pixels takes.
	canvas *ebiten.Image
	scale  float64
	tile   int
	// clockStart is the time of day the clock starts at, as the Clock window
	// sets it.
	clockStart float64
}

// New returns the Play tab of a window.
func New(window ids.ULID, theme *ui.Theme) *State {
	return &State{
		window: window, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{}, game: nil, problem: "",
		launching: true, launches: 0, floors: nil, held: 0, shown: true, standIns: map[string]ids.ULID{},
		npcSprites: map[ids.ULID]ids.ULID{}, art: newArtSet(), sound: newSounds(),
		canvas: nil, scale: 1, tile: 0, clockStart: protocol.DefaultClockStart,
	}
}

// Title is the tab's title.
func (s *State) Title(frame.Docs) string { return "Play" }

// Focused reports no text field: the Play tab has none.
func (s *State) Focused() *ui.TextField { return nil }

// Start subscribes to what a launch reads first.
func (s *State) Start(ctx *frame.Context) {
	s.prepare(ctx)
}

// SetShown tells the tab whether it is in front. A game behind another tab
// stops and goes quiet, and carries on when it comes back.
func (s *State) SetShown(shown bool) { s.shown = shown }

// Changed follows the orchestrator: Launch asked again, so the game launches
// again, and the time the Clock window sets for the clock to start at.
func (s *State) Changed(update client.Update, _ *frame.Context) {
	if update.Attention && update.Item == LaunchItem {
		s.launching = true
	}
	if update.Session != nil && update.Session.Topic == protocol.TopicClockStart {
		var start protocol.ClockStart
		if err := json.Unmarshal(update.Session.Value, &start); err == nil {
			s.clockStart = start.Seconds
		}
	}
}

// Update launches the game when asked and it can, then runs it a tick while
// the tab is in front: the arrow keys walk the player. Cmd+W closes the tab,
// as in every tab.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	frame.CommonKeys(input, ctx)
	if s.launching {
		s.launch(ctx)
	}
	if s.game == nil {
		return
	}
	if !s.shown {
		s.sound.silence(ctx)

		return
	}
	held := s.heldKeys(input)
	step := s.game.Step(play.Input{Held: held, Pressed: held &^ s.held})
	s.held = held
	if step.Outcome == play.Lost {
		s.stop(ctx, "The player's floor is not in the zone")

		return
	}
	s.sound.update(ctx, s.game, step)
}

// heldKeys reads the arrow keys, taking their presses so nothing repeats.
func (s *State) heldKeys(input *ui.Input) play.Direction {
	var held play.Direction
	for key, direction := range map[ui.Key]play.Direction{
		ui.KeyUp: play.North, ui.KeyRight: play.East, ui.KeyDown: play.South, ui.KeyLeft: play.West,
	} {
		if input.IsDown(key) {
			held |= direction
		}
		for input.Consume(key, ui.Plain()) {
			continue
		}
	}

	return held
}

// launch starts the game from what the project holds now, once every
// document it needs has loaded.
func (s *State) launch(ctx *frame.Context) {
	project, tile, ready := s.prepare(ctx)
	if !ready {
		return
	}
	s.launching = false
	s.sound.silence(ctx)
	s.game, s.problem = nil, ""
	if problem := s.startProblem(project); problem != "" {
		s.problem = problem
		ctx.Frame.Status(problem)

		return
	}
	s.launches++
	project.Seconds, project.Seed = s.clockStart, seed(s.window, s.launches)
	played, err := game.New(project)
	if err != nil {
		s.problem = "The game cannot start: " + err.Error()
		ctx.Frame.Status(s.problem)

		return
	}
	s.game, s.tile, s.floors = played, tile, project.Zone.Floors()
	s.art.draw(ctx.Docs, tile)
	s.sound.launch(ctx, played, s.workflowCues(project))
	if s.launches > 1 {
		ctx.Frame.Status("Launched again with the project as it is now")
	}
}

// startProblem says why a project cannot launch, or "".
func (s *State) startProblem(project game.Project) string {
	switch {
	case project.Settings.Start.IsZero():
		return "No player start yet. Right-click a cell on a Map and choose Set player start here."
	case project.Zone == nil:
		return "The player start's zone was deleted. Set the player start again on a Map."
	}

	return ""
}

// stop ends the game, saying why.
func (s *State) stop(ctx *frame.Context, why string) {
	s.sound.silence(ctx)
	s.game, s.problem = nil, why+". Press Cmd+R to launch again."
	ctx.Frame.Status(s.problem)
}

// seed is a launch's random seed, so each launch of a window differs and a
// test repeats.
func seed(window ids.ULID, launch int) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write(window[:])
	_, _ = hash.Write([]byte{byte(launch)}) //nolint:gosec // Only the low byte varies the seed.

	return hash.Sum64()
}

// Locate answers input scripts: "state:<state>" for the game's states.
func (s *State) Locate(name string, _ frame.Docs) (image.Point, bool) {
	kind, value, _ := strings.Cut(name, ":")
	if kind != "state" {
		return image.Pt(0, 0), false
	}

	return image.Pt(0, 0), s.hasState(value)
}
