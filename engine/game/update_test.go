package game_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

const (
	beaID      = 6
	stockID    = 7
	beaPlaced  = 22
	stockStart = 60
)

// ticks runs the world a number of ticks and returns the cues asked for and
// why each NPC, by its NPC, last started a node.
func ticks(played *game.Game, count int) ([]string, map[ids.ULID][]string) {
	var cues []string
	whys := map[ids.ULID][]string{}
	for range count {
		for _, cue := range played.Tick() {
			cues = append(cues, cue.Cue)
		}
		for _, runner := range played.Runners() {
			for _, change := range runner.NPC.Taken {
				if change.Why != "" {
					whys[runner.NPC.NPC] = append(whys[runner.NPC.NPC], change.Why)
				}
			}
		}
	}

	return cues, whys
}

func count(cues []string, cue string) int {
	found := 0
	for _, heard := range cues {
		if heard == cue {
			found++
		}
	}

	return found
}

func startedAgain(whys []string, why string) bool {
	for _, said := range whys {
		if strings.HasPrefix(said, why) {
			return true
		}
	}

	return false
}

// rings is a day that plays a cue, then waits an hour.
func rings(t *testing.T, cue string) *flow.Workflow {
	t.Helper()
	day, err := flow.New(id(30))
	if err != nil {
		t.Fatal(err)
	}
	ring := node(t, flow.KindPlaySound, 31, map[string]any{"cue": cue})
	rest := node(t, flow.KindWait, 32, map[string]any{
		"duration": flow.Duration{Amount: 1, Unit: flow.UnitHours, Clock: flow.ClockGame},
	})
	if err := day.Insert([]flow.Node{ring, rest}, nil, []flow.Link{
		{From: id(30), Output: flow.OutputNext, To: ring.ID}, {From: ring.ID, Output: flow.OutputNext, To: rest.ID},
	}); err != nil {
		t.Fatal(err)
	}

	return day
}

// TestUpdateWithNothingChangedChangesNothing gives a running game the same
// project, gathered again: nobody starts again.
func TestUpdateWithNothingChangedChangesNothing(t *testing.T) {
	t.Parallel()
	played, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	before, _ := ticks(played, 20)
	played.Update(town(t))
	after, whys := ticks(played, 20)
	if count(before, "bell") != 1 || count(after, "bell") != 0 || len(whys[id(clerkID)]) != 0 {
		t.Fatalf("the clerk carries on, ringing once in all: %v then %v, %v", before, after, whys)
	}
}

// TestUpdateRestartsAnNPCWhoseBehaviourChanged edits the clerk's day: she
// starts again, saying why, and follows the day as it is now. Her behaviour
// taken away, she follows none; given back, she starts it without a word.
func TestUpdateRestartsAnNPCWhoseBehaviourChanged(t *testing.T) {
	t.Parallel()
	played, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	ticks(played, 20)
	edited := town(t)
	edited.Workflows[id(dayID)] = rings(t, "gong")
	played.Update(edited)
	cues, whys := ticks(played, 20)
	if count(cues, "gong") != 1 || count(cues, "bell") != 0 || !startedAgain(whys[id(clerkID)], game.RestartedByEdit) {
		t.Fatalf("the clerk starts her changed day again: %v, %v", cues, whys)
	}
	none := town(t)
	none.NPCs[id(clerkID)] = npc.New()
	played.Update(none)
	if runner := played.Runners()[0]; runner.Workflow != nil || !runner.Behaviour.IsZero() {
		t.Fatalf("with her behaviour taken away she follows none: %+v", runner)
	}
	played.Update(edited)
	cues, whys = ticks(played, 20)
	if played.Runners()[0].Workflow == nil || startedAgain(whys[id(clerkID)], game.RestartedByEdit) {
		t.Fatalf("given a behaviour again, she follows it without saying she started again: %v, %v", cues, whys)
	}
}

// TestUpdateStartsAndStopsPlacements adds a placement to a floor, and its
// NPC starts running; takes one away, and its NPC stops.
func TestUpdateStartsAndStopsPlacements(t *testing.T) {
	t.Parallel()
	played, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	more := town(t)
	bea := tilefloor.Placement{ID: id(beaPlaced), NPC: id(beaID), Col: 10, Row: 8}
	if err := more.Floors[id(groundID)].InsertPlacement(1, bea); err != nil {
		t.Fatal(err)
	}
	more.NPCs[id(beaID)] = follows(id(dayID))
	played.Update(more)
	runners := played.Runners()
	if len(runners) != 2 || runners[1].NPC.NPC != id(beaID) || runners[1].Workflow == nil {
		t.Fatalf("Bea starts running, following the day: %+v", runners)
	}
	cues, _ := ticks(played, 20)
	if count(cues, "bell") != 1 {
		t.Fatalf("Bea rings as she starts; the clerk, already waiting, does not: %v", cues)
	}
	fewer := town(t)
	if _, _, err := fewer.Floors[id(groundID)].RemovePlacement(id(21)); err != nil {
		t.Fatal(err)
	}
	played.Update(fewer)
	if len(played.Runners()) != 0 || len(played.Characters()) != 0 {
		t.Fatalf("with no placement left, nobody runs: %+v", played.Runners())
	}
}

// TestUpdateWalksTheFloorAsPainted paints a wall beside the player while
// the game runs: the player cannot walk into it.
func TestUpdateWalksTheFloorAsPainted(t *testing.T) {
	t.Parallel()
	played, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	painted := town(t)
	if _, err := painted.Floors[id(groundID)].SetTop(4, 5, "wall.a"); err != nil {
		t.Fatal(err)
	}
	played.Update(painted)
	walk(played, play.West, 3*play.StepTicks)
	if player := played.Player(); player.Cell.X != 5 {
		t.Fatalf("the new wall stops the player: %+v", player.Cell)
	}
}

// TestUpdateKeepsWhatValuesHeld declares another value while the game
// runs: the value already counted keeps its count.
func TestUpdateKeepsWhatValuesHeld(t *testing.T) {
	t.Parallel()
	greetings := worldstate.New(id(40), "greetings", worldstate.ScopeWorld, worldstate.TypeInteger)
	declared := func(more bool) *worldstate.Catalog {
		world := worldstate.NewDeclarations(worldstate.HoldsWorldState)
		if err := world.Insert(greetings); err != nil {
			t.Fatal(err)
		}
		if more {
			if err := world.Insert(worldstate.New(id(43), "visits", worldstate.ScopeWorld, worldstate.TypeInteger)); err != nil {
				t.Fatal(err)
			}
		}

		return worldstate.NewCatalog(world, nil)
	}
	project := town(t)
	project.Values = declared(false)
	played, err := game.New(project)
	if err != nil {
		t.Fatal(err)
	}
	if err := played.Values().Set(greetings, ids.ULID{}, worldstate.Value{Type: worldstate.TypeInteger, Integer: 3}); err != nil {
		t.Fatal(err)
	}
	again := town(t)
	again.Values = declared(true)
	played.Update(again)
	if got := played.Values().Get(greetings, ids.ULID{}); got.Integer != 3 {
		t.Fatalf("the count is kept: %+v", got)
	}
	if _, known := played.Values().Catalog().Lookup(worldstate.ScopeWorld, ids.ULID{}, id(43)); !known {
		t.Fatal("the value declared since is known")
	}
	bare, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	bare.Update(again)
	if bare.Values() == nil {
		t.Fatal("a game that kept no values keeps them once the project declares some")
	}
}

// TestUpdateRestartsAnNPCInsideAChangedCall has the clerk call Fetch stock,
// which waits an hour, while Bea follows a day of her own. Fetch stock is
// edited: the clerk, inside the call, starts her day again, saying why; Bea
// carries on.
func TestUpdateRestartsAnNPCInsideAChangedCall(t *testing.T) {
	t.Parallel()
	calling := func(minutes int) game.Project {
		project := town(t)
		day, err := flow.New(id(30))
		if err != nil {
			t.Fatal(err)
		}
		call := node(t, flow.KindCall, 31, map[string]any{flow.ParamWorkflow: id(stockID).String()})
		if err := day.Insert([]flow.Node{call}, nil, []flow.Link{{From: id(30), Output: flow.OutputNext, To: call.ID}}); err != nil {
			t.Fatal(err)
		}
		stock, err := flow.New(id(stockStart))
		if err != nil {
			t.Fatal(err)
		}
		rest := node(t, flow.KindWait, stockStart+1, map[string]any{
			"duration": flow.Duration{Amount: minutes, Unit: flow.UnitMinutes, Clock: flow.ClockGame},
		})
		if err := stock.Insert([]flow.Node{rest}, nil, []flow.Link{
			{From: id(stockStart), Output: flow.OutputNext, To: rest.ID},
		}); err != nil {
			t.Fatal(err)
		}
		project.Workflows[id(dayID)], project.Workflows[id(stockID)] = day, stock
		bea := tilefloor.Placement{ID: id(beaPlaced), NPC: id(beaID), Col: 10, Row: 8}
		if err := project.Floors[id(groundID)].InsertPlacement(1, bea); err != nil {
			t.Fatal(err)
		}
		project.NPCs[id(beaID)] = follows(id(8))
		project.Workflows[id(8)] = rings(t, "bell")

		return project
	}
	played, err := game.New(calling(60))
	if err != nil {
		t.Fatal(err)
	}
	ticks(played, 20)
	var clerk *npcrun.NPC
	for _, runner := range played.Runners() {
		if runner.NPC.NPC == id(clerkID) {
			clerk = runner.NPC
		}
	}
	if clerk == nil || clerk.Module() != id(stockID) {
		t.Fatalf("the clerk is inside her call to Fetch stock: %+v", clerk)
	}
	played.Update(calling(90))
	cues, whys := ticks(played, 20)
	if !startedAgain(whys[id(clerkID)], game.RestartedByCall) || startedAgain(whys[id(beaID)], game.RestartedByCall) ||
		startedAgain(whys[id(beaID)], game.RestartedByEdit) || count(cues, "bell") != 0 {
		t.Fatalf("only the clerk, inside the changed call, starts again: %v, %v", whys, cues)
	}
}
