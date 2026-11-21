package game_test

import (
	"errors"
	"image"
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
	"github.com/bradhannah/berad-engine/engine/zone"
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

// TestPlaceMovesThePlayerAndNothingElse moves the player to another cell,
// and up to the other floor, while the game runs: the clerk carries on, and
// the clock and the tick count are as they were. A wall, a cell off the
// floor and a floor the zone lacks are refused, and the player stays.
func TestPlaceMovesThePlayerAndNothingElse(t *testing.T) {
	t.Parallel()
	played, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	walk(played, play.West, 5)
	ticked, at := played.Ticks(), played.Clock().Seconds()
	if err := played.Place(id(groundID), image.Pt(9, 2)); err != nil {
		t.Fatal(err)
	}
	if player := played.Player(); player.Cell != image.Pt(9, 2) || player.Moving() || played.Ticks() != ticked ||
		played.Clock().Seconds() != at {
		t.Fatalf("the player stands on the new cell, and no time passed: %+v", player)
	}
	if err := played.Place(id(upperID), image.Pt(3, 3)); err != nil || played.Player().Floor != id(upperID) {
		t.Fatalf("the player is moved to the upper floor: %v", err)
	}
	cues, whys := ticks(played, 20)
	if count(cues, "bell") != 0 || len(whys[id(clerkID)]) != 0 {
		t.Fatalf("the clerk, who rang before the move, waits on and does not start again: %v, %v", cues, whys)
	}
	for name, refused := range map[string]error{
		"a wall":                 played.Place(id(groundID), image.Pt(6, 5)),
		"a cell off the floor":   played.Place(id(groundID), image.Pt(40, 40)),
		"a floor not the zone's": played.Place(id(99), image.Pt(1, 1)),
	} {
		if !errors.Is(refused, game.ErrPlace) {
			t.Fatalf("%s is refused: %v", name, refused)
		}
	}
	if player := played.Player(); player.Floor != id(upperID) || player.Cell != image.Pt(3, 3) {
		t.Fatalf("refused, the player stays where they were: %+v", player)
	}
	walk(played, play.East, play.StepTicks)
	if played.Player().Cell != image.Pt(4, 3) {
		t.Fatalf("the player walks on from where they were put: %+v", played.Player().Cell)
	}
}

// harbour is a project gathered for a second zone: one floor of sand with
// Bea at 2,2, who follows a day of her own, its own music, and the player
// arriving at 4,4.
func harbour(t *testing.T) game.Project {
	t.Helper()
	sand, err := tilefloor.New(8, 8, "sand")
	if err != nil {
		t.Fatal(err)
	}
	if err := sand.InsertPlacement(0, tilefloor.Placement{ID: id(beaPlaced), NPC: id(beaID), Col: 2, Row: 2}); err != nil {
		t.Fatal(err)
	}
	zoned := zone.New()
	if err := zoned.Insert(0, id(72)); err != nil {
		t.Fatal(err)
	}
	if err := zoned.InsertSound(0, "music/harbour"); err != nil {
		t.Fatal(err)
	}
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: id(71), Floor: id(72), Col: 4, Row: 4}

	return game.Project{
		Settings: settings, Zone: zoned, Floors: map[ids.ULID]*tilefloor.Floor{id(72): sand},
		NPCs:      map[ids.ULID]npc.NPC{id(beaID): follows(id(8))},
		Workflows: map[ids.ULID]*flow.Workflow{id(8): rings(t, "gull")},
		Values:    nil, Seconds: 0, Seed: 9, Names: nil, Signals: nil,
	}
}

// TestEnterLeavesAZoneAndStartsAnother moves the game into a second zone:
// the clerk of the town stops and Bea of the harbour starts at her
// placement; the harbour's music is the zone's; a value counted in the town
// and the clock's time carry over. Back in the town, the clerk starts her
// day afresh.
func TestEnterLeavesAZoneAndStartsAnother(t *testing.T) {
	t.Parallel()
	greetings := worldstate.New(id(40), "greetings", worldstate.ScopeWorld, worldstate.TypeInteger)
	world := worldstate.NewDeclarations(worldstate.HoldsWorldState)
	if err := world.Insert(greetings); err != nil {
		t.Fatal(err)
	}
	town, away := town(t), harbour(t)
	town.Values, away.Values = worldstate.NewCatalog(world, nil), worldstate.NewCatalog(world, nil)
	played, err := game.New(town)
	if err != nil {
		t.Fatal(err)
	}
	ticks(played, 20)
	if err := played.Values().Set(greetings, ids.ULID{}, worldstate.Value{Type: worldstate.TypeInteger, Integer: 3}); err != nil {
		t.Fatal(err)
	}
	ticked, at := played.Ticks(), played.Clock().Seconds()
	if err := played.Enter(away); err != nil {
		t.Fatal(err)
	}
	player, runners := played.Player(), played.Runners()
	if played.Zone() != id(71) || player.Floor != id(72) || player.Cell != image.Pt(4, 4) {
		t.Fatalf("the player stands in the harbour: zone %v, %+v", played.Zone(), player)
	}
	if len(runners) != 1 || runners[0].NPC.NPC != id(beaID) || runners[0].NPC.Cell != image.Pt(2, 2) {
		t.Fatalf("only Bea runs, from her placement: %+v", runners)
	}
	if sounds := played.ZoneSounds(); len(sounds) != 1 || sounds[0] != "music/harbour" {
		t.Fatalf("the zone's sounds are the harbour's: %v", sounds)
	}
	if played.Values().Get(greetings, ids.ULID{}).Integer != 3 || played.Clock().Seconds() != at || played.Ticks() != ticked {
		t.Fatal("the count and the time carry over")
	}
	cues, _ := ticks(played, 20)
	if count(cues, "gull") != 1 || count(cues, "bell") != 0 {
		t.Fatalf("Bea starts her day; the clerk, in the zone left, does nothing: %v", cues)
	}
	back := town
	back.Settings.Start = game.Start{Zone: id(zoneID), Floor: id(groundID), Col: 1, Row: 1}
	if err := played.Enter(back); err != nil {
		t.Fatal(err)
	}
	cues, _ = ticks(played, 20)
	if count(cues, "bell") != 1 || played.Player().Cell != image.Pt(1, 1) || played.Zone() != id(zoneID) {
		t.Fatalf("back in the town, the clerk starts her day afresh: %v", cues)
	}
}

// TestEnterRefusesWhereThePlayerCannotStand refuses a project with no zone,
// and a start on a cell off the floor: the game goes on where it was.
func TestEnterRefusesWhereThePlayerCannotStand(t *testing.T) {
	t.Parallel()
	played, err := game.New(town(t))
	if err != nil {
		t.Fatal(err)
	}
	off := harbour(t)
	off.Settings.Start.Col = 40
	none := harbour(t)
	none.Zone = nil
	for name, refused := range map[string]error{"a cell off the floor": played.Enter(off), "no zone": played.Enter(none)} {
		if !errors.Is(refused, game.ErrPlace) {
			t.Fatalf("%s is refused: %v", name, refused)
		}
	}
	if played.Zone() != id(zoneID) || len(played.Runners()) != 1 || played.Player().Cell != image.Pt(5, 5) {
		t.Fatal("refused, the game goes on in the town")
	}
}

// chooser is a project whose two NPCs each choose at random, again and
// again, between ringing a bell and sounding a gong. Its documents and
// placements take their IDs from base, as a project made again takes new
// ones.
func chooser(t *testing.T, base byte, seed uint64) game.Project {
	t.Helper()
	ground, err := tilefloor.New(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	for index, col := range []int{2, 9} {
		placement := tilefloor.Placement{ID: id(base + 10 + byte(index)), NPC: id(base + 20 + byte(index)), Col: col, Row: 8}
		if err := ground.InsertPlacement(index, placement); err != nil {
			t.Fatal(err)
		}
	}
	zoned := zone.New()
	if err := zoned.Insert(0, id(base+1)); err != nil {
		t.Fatal(err)
	}
	day, err := flow.New(id(base + 30))
	if err != nil {
		t.Fatal(err)
	}
	choose := node(t, flow.KindRandom, base+31, map[string]any{"options": []flow.Share{
		{Port: "o1", Label: "bell", Share: 1, Constant: flow.NoConstant()},
		{Port: "o2", Label: "gong", Share: 1, Constant: flow.NoConstant()},
	}})
	bell := node(t, flow.KindPlaySound, base+32, map[string]any{"cue": "bell"})
	gong := node(t, flow.KindPlaySound, base+33, map[string]any{"cue": "gong"})
	rest := node(t, flow.KindWait, base+34, map[string]any{
		"duration": flow.Duration{Amount: 1, Unit: flow.UnitMinutes, Clock: flow.ClockGame},
	})
	if err := day.Insert([]flow.Node{choose, bell, gong, rest}, nil, []flow.Link{
		{From: id(base + 30), Output: flow.OutputNext, To: choose.ID}, {From: choose.ID, Output: "o1", To: bell.ID},
		{From: choose.ID, Output: "o2", To: gong.ID}, {From: bell.ID, Output: flow.OutputNext, To: rest.ID},
		{From: gong.ID, Output: flow.OutputNext, To: rest.ID}, {From: rest.ID, Output: flow.OutputNext, To: choose.ID},
	}); err != nil {
		t.Fatal(err)
	}
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: id(base), Floor: id(base + 1), Col: 5, Row: 5}

	return game.Project{
		Settings: settings, Zone: zoned, Floors: map[ids.ULID]*tilefloor.Floor{id(base + 1): ground},
		NPCs: map[ids.ULID]npc.NPC{
			id(base + 20): follows(id(base + 2)), id(base + 21): follows(id(base + 2)),
		},
		Workflows: map[ids.ULID]*flow.Workflow{id(base + 2): day},
		Values:    nil, Seconds: eight, Seed: seed, Names: nil, Signals: nil,
	}
}

// TestAProjectMadeAgainPlaysTheSame plays two copies of a project whose
// documents and placements have different IDs, as a project made again has:
// from the same seed their NPCs make the same random choices, tick for
// tick, and from another seed they make others.
func TestAProjectMadeAgainPlaysTheSame(t *testing.T) {
	t.Parallel()
	heard := func(project game.Project) string {
		played, err := game.New(project)
		if err != nil {
			t.Fatal(err)
		}
		var said strings.Builder
		for range 4000 {
			for _, cue := range played.Tick() {
				said.WriteString(cue.Cue[:1])
			}
			said.WriteByte('.')
		}

		return said.String()
	}
	first, again, other := heard(chooser(t, 100, 7)), heard(chooser(t, 180, 7)), heard(chooser(t, 100, 8))
	if !strings.Contains(first, "b") || !strings.Contains(first, "g") {
		t.Fatal("the NPCs ring bells and sound gongs")
	}
	if first != again {
		t.Fatal("made again with new IDs, the project plays the same from the same seed")
	}
	if first == other {
		t.Fatal("from another seed the NPCs choose otherwise")
	}
}
