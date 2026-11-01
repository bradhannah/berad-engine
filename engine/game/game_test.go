package game_test

import (
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

const (
	zoneID   = 1
	groundID = 2
	upperID  = 3
	clerkID  = 4
	dayID    = 5
	eight    = 8 * 3600
)

// town is a zone of two 12 by 10 grass floors: the ground has a wall at
// 6,5, stairs up at 2,2, the fountain at 8,5 and Ada Clerk, who rings a bell
// then waits an hour, at 9,8; the zone plays the town's music throughout.
func town(t *testing.T) game.Project {
	t.Helper()
	ground, err := tilefloor.New(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	upper, err := tilefloor.New(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range []struct {
		col, row int
		top      string
	}{{6, 5, "wall.a"}, {2, 2, "stairs.up"}} {
		if _, err := ground.SetTop(set.col, set.row, set.top); err != nil {
			t.Fatal(err)
		}
	}
	if err := ground.InsertSound(0, tilefloor.Sound{ID: id(20), Cue: "town/fountain", Col: 8, Row: 5}); err != nil {
		t.Fatal(err)
	}
	if err := ground.InsertPlacement(0, tilefloor.Placement{ID: id(21), NPC: id(clerkID), Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	zoned := zone.New()
	for index, floorID := range []byte{groundID, upperID} {
		if err := zoned.Insert(index, id(floorID)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zoned.InsertSound(0, "music/town"); err != nil {
		t.Fatal(err)
	}
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: id(zoneID), Floor: id(groundID), Col: 5, Row: 5}

	return game.Project{
		Settings: settings, Zone: zoned,
		Floors:    map[ids.ULID]*tilefloor.Floor{id(groundID): ground, id(upperID): upper},
		NPCs:      map[ids.ULID]npc.NPC{id(clerkID): {Behaviour: id(dayID), Sprite: ids.ULID{}, Notes: ""}},
		Workflows: map[ids.ULID]*flow.Workflow{id(dayID): clerkDay(t)},
		Seconds:   eight, Seed: 1,
	}
}

// clerkDay rings a bell, then waits an hour.
func clerkDay(t *testing.T) *flow.Workflow {
	t.Helper()
	workflow, err := flow.New(id(30))
	if err != nil {
		t.Fatal(err)
	}
	ring, err := flow.NewNode(flow.KindPlaySound)
	if err != nil {
		t.Fatal(err)
	}
	ring.ID = id(31)
	if ring, err = ring.WithParam("cue", "bell"); err != nil {
		t.Fatal(err)
	}
	wait, err := flow.NewNode(flow.KindWait)
	if err != nil {
		t.Fatal(err)
	}
	wait.ID = id(32)
	if wait, err = wait.WithParam("duration", flow.Duration{Amount: 1, Unit: flow.UnitHours, Clock: flow.ClockGame}); err != nil {
		t.Fatal(err)
	}
	if err := workflow.Insert([]flow.Node{ring, wait}, nil, []flow.Link{
		{From: id(30), Output: flow.OutputNext, To: ring.ID}, {From: ring.ID, Output: flow.OutputNext, To: wait.ID},
	}); err != nil {
		t.Fatal(err)
	}

	return workflow
}

// walk holds a direction for a number of ticks, pressing it on the first.
func walk(played *game.Game, direction play.Direction, ticks int) []game.Step {
	steps := make([]game.Step, 0, ticks)
	for tick := range ticks {
		input := play.Input{Held: direction, Pressed: 0}
		if tick == 0 {
			input.Pressed = direction
		}
		steps = append(steps, played.Step(input))
	}

	return steps
}

// TestAGameStartsAtThePlayerStart starts the player on the start's cell,
// centred in a 19 by 11 view, with Ada Clerk on her placement.
func TestAGameStartsAtThePlayerStart(t *testing.T) {
	t.Parallel()
	played, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	player := played.Player()
	if player.Floor != id(groundID) || player.Cell.X != 5 || player.Cell.Y != 5 {
		t.Fatalf("player %+v", player)
	}
	if view := played.View(); view.Left != -4 || view.Top != 0 || view.Columns != 19 || view.Rows != 11 {
		t.Fatalf("view %+v", view)
	}
	characters := played.Characters()
	if len(characters) != 1 || characters[0].NPC != id(clerkID) || characters[0].X != 9 || characters[0].Y != 8 {
		t.Fatalf("characters %+v", characters)
	}
	if sounds := played.ZoneSounds(); len(sounds) != 1 || sounds[0] != "music/town" {
		t.Fatalf("zone sounds %v", sounds)
	}
}

// TestAGameRefusesABadStart refuses no start, and a start on a wall.
func TestAGameRefusesABadStart(t *testing.T) {
	t.Parallel()
	project := town(t)
	project.Settings.Start = game.NoStart()
	if _, err := game.New(project); !errors.Is(err, game.ErrNoStart) {
		t.Fatalf("no start: %v", err)
	}
	project.Settings.Start = game.Start{Zone: id(zoneID), Floor: id(groundID), Col: 6, Row: 5}
	if _, err := game.New(project); !errors.Is(err, game.ErrStart) {
		t.Fatalf("a start on a wall: %v", err)
	}
}

// TestAGameWalksHearsAndClimbs walks the player east, stopped by the wall,
// hears the fountain nearer as they go and the clerk's bell once, then
// climbs the stairs, where the fountain is not heard.
func TestAGameWalksHearsAndClimbs(t *testing.T) {
	t.Parallel()
	played, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	heard := played.Ambient()
	if len(heard) != 1 || heard[0].Cue != "town/fountain" || heard[0].Distance != 3 {
		t.Fatalf("heard %+v", heard)
	}
	steps := walk(played, play.East, 3*play.StepTicks)
	if player := played.Player(); player.Cell.X != 5 || player.Facing.X != 1 {
		t.Fatalf("the wall stops the player: %+v", player)
	}
	bells := 0
	for _, step := range steps {
		for _, cue := range step.Cues {
			if cue.Cue == "bell" {
				bells++
			}
		}
	}
	if bells != 1 {
		t.Fatalf("the bell rings once: %d", bells)
	}
	walk(played, play.North, play.StepTicks)
	walk(played, play.West, 3*play.StepTicks)
	walk(played, play.North, 2*play.StepTicks+1)
	if player := played.Player(); player.Floor != id(upperID) {
		t.Fatalf("the stairs take the player up: %+v", player)
	}
	if heard := played.Ambient(); len(heard) != 0 {
		t.Fatalf("the fountain is not heard upstairs: %+v", heard)
	}
}

func TestAGameRepeats(t *testing.T) {
	t.Parallel()
	first, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	second, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	walk(first, play.South, 2*play.StepTicks)
	walk(second, play.South, 2*play.StepTicks)
	if first.Player() != second.Player() || first.Characters()[0] != second.Characters()[0] {
		t.Fatal("the same input plays the same game")
	}
}
