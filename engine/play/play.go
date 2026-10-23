// Package play runs a zone: the player walking through its floors one cell at
// a time. A run changes only when it is stepped, by its input and the zone's
// floors as they are on that tick, so the editor's preview and a game behave
// alike and a test repeats exactly.
//
// A tap moves exactly one cell; holding a direction keeps walking. The player
// slides between cells over StepTicks ticks, in any of eight directions, and
// cannot squeeze diagonally between two cells that block. Walking onto stairs
// takes the player to the same cell of the floor above or below; every floor
// of a zone shares the zone origin, so stairs line up.
package play

import (
	"image"
	"slices"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// Timing, in ticks of the game loop (60 a second).
const (
	// StepTicks is how long a step from one cell to the next takes.
	StepTicks = 12
	// JoinTicks is how soon after a step starts a second direction can turn
	// it into a diagonal, so two keys pressed almost together move
	// diagonally.
	JoinTicks = 4
)

// Direction is a set of directions, as held keys give them.
type Direction uint8

// Directions. Rows count downwards, so North is up the screen.
const (
	North Direction = 1 << iota
	East
	South
	West
)

// Delta returns the step a set of directions asks for, one cell in each
// direction it holds. Opposite directions cancel.
func (d Direction) Delta() image.Point {
	var delta image.Point
	if d&North != 0 {
		delta.Y--
	}
	if d&South != 0 {
		delta.Y++
	}
	if d&West != 0 {
		delta.X--
	}
	if d&East != 0 {
		delta.X++
	}

	return delta
}

// Input is what the player asks for on one tick.
type Input struct {
	// Held is the directions held now.
	Held Direction
	// Pressed is the directions first pressed on this tick. A tap that starts
	// and ends during a step is remembered and taken when the step ends.
	Pressed Direction
}

// Zone gives a run the zone's floors as they are now, so edits made during a
// run apply at once.
type Zone interface {
	// Floors lists the zone's floors from the bottom up.
	Floors() []ids.ULID
	// Floor calls fn with a floor, if it has loaded, and reports whether it
	// did. A run never asks for a second floor from inside fn.
	Floor(id ids.ULID, fn func(*tilefloor.Floor)) bool
}

// Level is one floor of a zone held in memory. Floor is nil until the floor
// has loaded.
type Level struct {
	ID    ids.ULID
	Floor *tilefloor.Floor
}

// Levels is a zone held in memory, its floors from the bottom up.
type Levels []Level

// Floors lists the floors from the bottom up.
func (l Levels) Floors() []ids.ULID {
	floors := make([]ids.ULID, 0, len(l))
	for _, level := range l {
		floors = append(floors, level.ID)
	}

	return floors
}

// Floor calls fn with a floor that has loaded.
func (l Levels) Floor(id ids.ULID, fn func(*tilefloor.Floor)) bool {
	index := slices.IndexFunc(l, func(level Level) bool { return level.ID == id })
	if index < 0 || l[index].Floor == nil {
		return false
	}
	fn(l[index].Floor)

	return true
}

// Player is where the player is.
type Player struct {
	// Floor and Cell are where the player stands, or the cell a step is
	// leaving.
	Floor ids.ULID
	Cell  image.Point
	// To is the cell the current step moves to; it equals Cell when the
	// player stands still.
	To image.Point
	// Progress counts the ticks of the current step.
	Progress int
	// Facing is the direction of the latest step or attempt, one cell.
	Facing image.Point
}

// Moving reports whether the player is between two cells.
func (p Player) Moving() bool { return p.To != p.Cell }

// At returns the player's position in cells, between Cell and To during a
// step.
func (p Player) At() (float64, float64) {
	share := float64(p.Progress) / StepTicks

	return float64(p.Cell.X) + float64(p.To.X-p.Cell.X)*share, float64(p.Cell.Y) + float64(p.To.Y-p.Cell.Y)*share
}

// Outcome is what a tick did that the player may need telling.
type Outcome int

// Outcomes of a tick.
const (
	// Walking is a tick with nothing to report.
	Walking Outcome = iota
	// Climbed is a tick on which stairs took the player to another floor.
	Climbed
	// NoFloor is a step onto stairs leading to a floor the zone lacks.
	NoFloor
	// StairsBlocked is a step onto stairs whose other end is outside that
	// floor or cannot be stood on.
	StairsBlocked
	// Lost is a tick on which the player's floor is no longer in the zone.
	Lost
)

// Run is one run of a zone.
type Run struct {
	tick   uint64
	player Player
	queued Direction
}

// Start begins a run with the player standing on a cell.
func Start(floor ids.ULID, cell image.Point) *Run {
	return &Run{
		tick:   0,
		player: Player{Floor: floor, Cell: cell, To: cell, Progress: 0, Facing: image.Pt(0, 1)},
		queued: 0,
	}
}

// CanStand reports whether the player can stand on a cell of a floor.
func CanStand(zone Zone, floor ids.ULID, cell image.Point) bool {
	can := false
	zone.Floor(floor, func(loaded *tilefloor.Floor) { can = standable(loaded, cell) })

	return can
}

// Tick returns how many ticks the run has taken.
func (r *Run) Tick() uint64 { return r.tick }

// Player returns where the player is.
func (r *Run) Player() Player { return r.player }

// Step advances the run by one tick.
func (r *Run) Step(zone Zone, input Input) Outcome {
	r.tick++
	floors := zone.Floors()
	index := slices.Index(floors, r.player.Floor)
	if index < 0 {
		return Lost
	}
	climb := 0
	zone.Floor(r.player.Floor, func(floor *tilefloor.Floor) { climb = r.walk(floor, input) })
	if climb == 0 {
		return Walking
	}
	r.queued = 0

	return r.climb(zone, floors, index+climb)
}

// walk moves the player on their floor for one tick. On arriving on stairs
// it returns where they lead, and takes no further step until they are
// resolved.
func (r *Run) walk(floor *tilefloor.Floor, input Input) int {
	if r.player.Moving() {
		r.player.Progress++
		if !r.join(floor, input.Held) && input.Pressed != 0 {
			r.queued = input.Held | input.Pressed
		}
		if r.player.Progress < StepTicks {
			return 0
		}
		r.player.Cell, r.player.Progress = r.player.To, 0
		if climb := climbAt(floor, r.player.Cell); climb != 0 {
			return climb
		}
	}
	want := input.Held | input.Pressed
	if want.Delta() == image.Pt(0, 0) {
		want = r.queued
	}
	r.queued = 0
	r.begin(floor, want.Delta())

	return 0
}

// begin starts a step if the player can take it.
func (r *Run) begin(floor *tilefloor.Floor, delta image.Point) {
	if delta == image.Pt(0, 0) {
		return
	}
	r.player.Facing = delta
	if canStep(floor, r.player.Cell, delta) {
		r.player.To = r.player.Cell.Add(delta)
	}
}

// join turns a step that has just started into a diagonal when a second
// direction arrives soon after the first, and reports whether it did.
func (r *Run) join(floor *tilefloor.Floor, held Direction) bool {
	step := r.player.To.Sub(r.player.Cell)
	wanted := held.Delta()
	if r.player.Progress > JoinTicks || (step.X != 0 && step.Y != 0) || wanted.X == 0 || wanted.Y == 0 {
		return false
	}
	if (step.X == 0 || step.X == wanted.X) && (step.Y == 0 || step.Y == wanted.Y) &&
		canStep(floor, r.player.Cell, wanted) {
		r.player.To = r.player.Cell.Add(wanted)
		r.player.Facing = wanted

		return true
	}

	return false
}

// climb takes the player up or down stairs to a floor, by its index, when
// the same cell there can be stood on.
func (r *Run) climb(zone Zone, floors []ids.ULID, target int) Outcome {
	if target < 0 || target >= len(floors) {
		return NoFloor
	}
	if !CanStand(zone, floors[target], r.player.Cell) {
		return StairsBlocked
	}
	r.player.Floor = floors[target]

	return Climbed
}

// climbAt reports where stairs on a cell lead, or 0 for none.
func climbAt(floor *tilefloor.Floor, cell image.Point) int {
	if !floor.In(cell.X, cell.Y) {
		return 0
	}
	top, _ := floor.Top(cell.X, cell.Y) //nolint:errcheck // Inside the floor.

	return tiles.Climb(top)
}

// canStep reports whether a step can be taken: its cell can be stood on and,
// for a diagonal, the two cells beside it do not both block.
func canStep(floor *tilefloor.Floor, from, delta image.Point) bool {
	if !standable(floor, from.Add(delta)) {
		return false
	}
	if delta.X == 0 || delta.Y == 0 {
		return true
	}

	return standable(floor, from.Add(image.Pt(delta.X, 0))) || standable(floor, from.Add(image.Pt(0, delta.Y)))
}

// standable reports whether a cell is on a floor and nothing on it blocks.
func standable(floor *tilefloor.Floor, cell image.Point) bool {
	if !floor.In(cell.X, cell.Y) {
		return false
	}
	ground, _ := floor.Ground(cell.X, cell.Y) //nolint:errcheck // Inside the floor.
	top, _ := floor.Top(cell.X, cell.Y)       //nolint:errcheck // Inside the floor.

	return tiles.Passable(ground) && tiles.Passable(top)
}
