package play_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

func floorID(seed byte) ids.ULID {
	var id ids.ULID
	id[15] = seed

	return id
}

// newFloor is a grass floor of 10 by 10 cells around the zone origin, with
// tiles placed on top at the given cells.
func newFloor(t *testing.T, tops map[image.Point]string) *tilefloor.Floor {
	t.Helper()
	floor, err := tilefloor.NewAt(tilefloor.Centered(10, 10), "grass")
	if err != nil {
		t.Fatal(err)
	}
	for cell, top := range tops {
		if _, err := floor.SetTop(cell.X, cell.Y, top); err != nil {
			t.Fatal(err)
		}
	}

	return floor
}

func oneLevel(floor *tilefloor.Floor) play.Levels {
	return play.Levels{{ID: floorID(1), Floor: floor}}
}

// steps runs ticks with the same input and returns the last outcome.
func steps(run *play.Run, levels play.Levels, input play.Input, ticks int) play.Outcome {
	outcome := play.Walking
	for range ticks {
		outcome = run.Step(levels, input)
	}

	return outcome
}

func held(direction play.Direction) play.Input {
	return play.Input{Held: direction, Pressed: 0}
}

// tap is a key pressed on one tick and released on the next.
func tap(run *play.Run, levels play.Levels, direction play.Direction) {
	run.Step(levels, play.Input{Held: direction, Pressed: direction})
}

func standsAt(t *testing.T, run *play.Run, cell image.Point) {
	t.Helper()
	player := run.Player()
	if player.Moving() || player.Cell != cell {
		t.Fatalf("player at %v moving to %v, want standing at %v", player.Cell, player.To, cell)
	}
}

func TestTapMovesExactlyOneCell(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, nil))
	run := play.Start(floorID(1), image.Pt(0, 0))
	tap(run, levels, play.East)
	if !run.Player().Moving() {
		t.Fatal("a tap starts a step at once")
	}
	steps(run, levels, held(0), play.StepTicks*3)
	standsAt(t, run, image.Pt(1, 0))
}

func TestHoldingKeepsWalking(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, nil))
	run := play.Start(floorID(1), image.Pt(0, 0))
	run.Step(levels, play.Input{Held: play.South, Pressed: play.South})
	steps(run, levels, held(play.South), 3*play.StepTicks-1)
	steps(run, levels, held(0), play.StepTicks)
	standsAt(t, run, image.Pt(0, 3))
}

func TestSlideBetweenCells(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, nil))
	run := play.Start(floorID(1), image.Pt(0, 0))
	tap(run, levels, play.East)
	steps(run, levels, held(0), play.StepTicks/2)
	x, y := run.Player().At()
	if x != 0.5 || y != 0 {
		t.Fatalf("half way through a step: %v,%v", x, y)
	}
}

func TestTapDuringAStepIsNotLost(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, nil))
	run := play.Start(floorID(1), image.Pt(0, 0))
	tap(run, levels, play.East)
	steps(run, levels, held(0), 2)
	tap(run, levels, play.East)
	steps(run, levels, held(0), play.StepTicks*3)
	standsAt(t, run, image.Pt(2, 0))
}

func TestWallsAndWaterBlock(t *testing.T) {
	t.Parallel()
	floor := newFloor(t, map[image.Point]string{{X: 1, Y: 0}: "wall.a"})
	if _, err := floor.SetGround(0, 1, "water"); err != nil {
		t.Fatal(err)
	}
	levels := oneLevel(floor)
	run := play.Start(floorID(1), image.Pt(0, 0))
	for _, direction := range []play.Direction{play.East, play.South} {
		tap(run, levels, direction)
		steps(run, levels, held(0), play.StepTicks)
		standsAt(t, run, image.Pt(0, 0))
	}
	if run.Player().Facing != image.Pt(0, 1) {
		t.Fatalf("a blocked step still turns the player: %v", run.Player().Facing)
	}
}

func TestTheFloorEdgeBlocks(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, nil))
	run := play.Start(floorID(1), image.Pt(-5, -5))
	tap(run, levels, play.North|play.West)
	steps(run, levels, held(0), play.StepTicks)
	standsAt(t, run, image.Pt(-5, -5))
}

func TestDoorsAndPickupsDoNotBlock(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, map[image.Point]string{{X: 1, Y: 0}: "door", {X: 2, Y: 0}: "pickup.small"}))
	run := play.Start(floorID(1), image.Pt(0, 0))
	run.Step(levels, play.Input{Held: play.East, Pressed: play.East})
	steps(run, levels, held(play.East), 2*play.StepTicks-1)
	steps(run, levels, held(0), play.StepTicks)
	standsAt(t, run, image.Pt(2, 0))
}

func TestDiagonals(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, nil))
	run := play.Start(floorID(1), image.Pt(0, 0))
	tap(run, levels, play.North|play.East)
	steps(run, levels, held(0), play.StepTicks)
	standsAt(t, run, image.Pt(1, -1))
}

func TestSecondKeySoonAfterJoinsADiagonal(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, nil))
	run := play.Start(floorID(1), image.Pt(0, 0))
	run.Step(levels, play.Input{Held: play.North, Pressed: play.North})
	steps(run, levels, held(play.North), play.JoinTicks-1)
	run.Step(levels, play.Input{Held: play.North | play.East, Pressed: play.East})
	steps(run, levels, held(0), play.StepTicks)
	standsAt(t, run, image.Pt(1, -1))

	late := play.Start(floorID(1), image.Pt(0, 0))
	late.Step(levels, play.Input{Held: play.North, Pressed: play.North})
	steps(late, levels, held(play.North), play.JoinTicks+1)
	steps(late, levels, held(play.North|play.East), 1)
	steps(late, levels, held(0), play.StepTicks)
	standsAt(t, late, image.Pt(0, -1))
}

func TestNoSqueezingBetweenDiagonalWalls(t *testing.T) {
	t.Parallel()
	squeeze := oneLevel(newFloor(t, map[image.Point]string{{X: 1, Y: 0}: "wall", {X: 0, Y: -1}: "wall"}))
	run := play.Start(floorID(1), image.Pt(0, 0))
	tap(run, squeeze, play.North|play.East)
	steps(run, squeeze, held(0), play.StepTicks)
	standsAt(t, run, image.Pt(0, 0))

	corner := oneLevel(newFloor(t, map[image.Point]string{{X: 1, Y: 0}: "wall"}))
	run = play.Start(floorID(1), image.Pt(0, 0))
	tap(run, corner, play.North|play.East)
	steps(run, corner, held(0), play.StepTicks)
	standsAt(t, run, image.Pt(1, -1))
}

func TestOppositeKeysCancel(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, nil))
	run := play.Start(floorID(1), image.Pt(0, 0))
	tap(run, levels, play.North|play.South|play.East)
	steps(run, levels, held(0), play.StepTicks)
	standsAt(t, run, image.Pt(1, 0))
}

func stairsLevels(t *testing.T) play.Levels {
	t.Helper()
	ground := newFloor(t, map[image.Point]string{{X: 1, Y: 0}: "stairs.up"})
	upper := newFloor(t, map[image.Point]string{{X: 1, Y: 0}: "stairs.down", {X: 2, Y: 0}: "wall"})

	return play.Levels{{ID: floorID(1), Floor: ground}, {ID: floorID(2), Floor: upper}}
}

func TestStairsChangeFloor(t *testing.T) {
	t.Parallel()
	levels := stairsLevels(t)
	run := play.Start(floorID(1), image.Pt(0, 0))
	tap(run, levels, play.East)
	if outcome := steps(run, levels, held(0), play.StepTicks); outcome != play.Climbed {
		t.Fatalf("outcome %v", outcome)
	}
	if player := run.Player(); player.Floor != floorID(2) || player.Cell != image.Pt(1, 0) {
		t.Fatalf("after the stairs up: %+v", player)
	}
	steps(run, levels, held(0), play.StepTicks)
	if run.Player().Floor != floorID(2) {
		t.Fatal("arriving on stairs down does not take them")
	}
	tap(run, levels, play.West)
	steps(run, levels, held(0), play.StepTicks)
	tap(run, levels, play.East)
	steps(run, levels, held(0), play.StepTicks)
	if player := run.Player(); player.Floor != floorID(1) || player.Cell != image.Pt(1, 0) {
		t.Fatalf("after the stairs down: %+v", player)
	}
}

func TestStairsThatLeadNowhere(t *testing.T) {
	t.Parallel()
	levels := stairsLevels(t)
	if _, err := levels[0].Floor.SetTop(-1, 0, "stairs.down"); err != nil {
		t.Fatal(err)
	}
	run := play.Start(floorID(1), image.Pt(0, 0))
	tap(run, levels, play.West)
	if outcome := steps(run, levels, held(0), play.StepTicks); outcome != play.NoFloor {
		t.Fatalf("no floor below: %v", outcome)
	}
	if _, err := levels[0].Floor.SetTop(-1, 0, "stairs.up"); err != nil {
		t.Fatal(err)
	}
	if _, err := levels[1].Floor.SetTop(-1, 0, "wall"); err != nil {
		t.Fatal(err)
	}
	tap(run, levels, play.East)
	steps(run, levels, held(0), play.StepTicks)
	tap(run, levels, play.West)
	if outcome := steps(run, levels, held(0), play.StepTicks); outcome != play.StairsBlocked {
		t.Fatalf("blocked at the top: %v", outcome)
	}
	if player := run.Player(); player.Floor != floorID(1) || player.Cell != image.Pt(-1, 0) {
		t.Fatalf("the player stays on the stairs: %+v", player)
	}
}

func TestEditsApplyAtOnce(t *testing.T) {
	t.Parallel()
	floor := newFloor(t, nil)
	levels := oneLevel(floor)
	run := play.Start(floorID(1), image.Pt(0, 0))
	if _, err := floor.SetTop(1, 0, "wall.b"); err != nil {
		t.Fatal(err)
	}
	tap(run, levels, play.East)
	steps(run, levels, held(0), play.StepTicks)
	standsAt(t, run, image.Pt(0, 0))
}

func TestLosingTheFloor(t *testing.T) {
	t.Parallel()
	run := play.Start(floorID(1), image.Pt(0, 0))
	if outcome := run.Step(oneLevel(newFloor(t, nil))[:0], held(0)); outcome != play.Lost {
		t.Fatalf("outcome %v", outcome)
	}
}

func TestCanStand(t *testing.T) {
	t.Parallel()
	levels := oneLevel(newFloor(t, map[image.Point]string{{X: 1, Y: 0}: "wall"}))
	if !play.CanStand(levels, floorID(1), image.Pt(0, 0)) || play.CanStand(levels, floorID(1), image.Pt(1, 0)) ||
		play.CanStand(levels, floorID(1), image.Pt(20, 0)) || play.CanStand(levels, floorID(2), image.Pt(0, 0)) {
		t.Fatal("can stand")
	}
	unloaded := play.Levels{{ID: floorID(1), Floor: nil}}
	if play.CanStand(unloaded, floorID(1), image.Pt(0, 0)) {
		t.Fatal("not before the floor has loaded")
	}
}

func TestRunsRepeatExactly(t *testing.T) {
	t.Parallel()
	inputs := []play.Input{
		{Held: play.East, Pressed: play.East}, held(play.East), held(play.East | play.South),
		held(play.South), held(0), {Held: play.West, Pressed: play.West}, held(0),
	}
	final := func() play.Player {
		levels := oneLevel(newFloor(t, map[image.Point]string{{X: 2, Y: 2}: "wall"}))
		run := play.Start(floorID(1), image.Pt(0, 0))
		for index := range 200 {
			run.Step(levels, inputs[index*7/200])
		}

		return run.Player()
	}
	if first, second := final(), final(); first != second {
		t.Fatalf("runs differ: %+v and %+v", first, second)
	}
}
