package npcrun_test

import (
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

func id(seed byte) ids.ULID {
	var made ids.ULID
	made[15] = seed

	return made
}

const (
	groundID = 200
	upperID  = 201
	deskID   = 202
)

// office is a 12×8 floor with a wall from (5, 0) down to (5, 5), and a desk
// anchor at (8, 1), behind the wall.
func office(t *testing.T) *tilefloor.Floor {
	t.Helper()
	floor, err := tilefloor.New(12, 8, "floor")
	if err != nil {
		t.Fatal(err)
	}
	for row := range 6 {
		if _, err := floor.SetTop(5, row, "wall.a"); err != nil {
			t.Fatal(err)
		}
	}
	if err := floor.InsertAnchor(0, tilefloor.Anchor{ID: id(deskID), Name: "Desk", Col: 8, Row: 1}); err != nil {
		t.Fatal(err)
	}

	return floor
}

// node makes a node of a kind with parameters set.
func node(t *testing.T, seed byte, kind string, params map[string]any) flow.Node {
	t.Helper()
	made, err := flow.NewNode(kind)
	if err != nil {
		t.Fatal(err)
	}
	made.ID = id(seed)
	for name, value := range params {
		if made, err = made.WithParam(name, value); err != nil {
			t.Fatal(err)
		}
	}

	return made
}

// workflowOf builds a workflow from its start and nodes, with links from
// one node's output to another.
func workflowOf(t *testing.T, nodes []flow.Node, links ...flow.Link) *flow.Workflow {
	t.Helper()
	workflow, err := flow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.Insert(nodes, nil, links); err != nil {
		t.Fatal(err)
	}

	return workflow
}

func link(from byte, output string, to byte) flow.Link {
	return flow.Link{From: id(from), Output: output, To: id(to)}
}

func waitFor(amount int, unit, clockName string) flow.Duration {
	return flow.Duration{Amount: amount, Unit: unit, Clock: clockName}
}

// runner is an NPC on the office floor with a clock and a workflow.
type runner struct {
	npc      *npcrun.NPC
	zone     play.Levels
	clock    *clock.Clock
	workflow *flow.Workflow
	tick     uint64
}

func newRunner(t *testing.T, workflow *flow.Workflow, at image.Point, start float64, rate clock.Rate) *runner {
	t.Helper()
	zone := play.Levels{{ID: id(groundID), Floor: office(t)}, {ID: id(upperID), Floor: office(t)}}
	placement := tilefloor.Placement{ID: id(100), NPC: id(101), Col: at.X, Row: at.Y}

	return &runner{
		npc: npcrun.Start(placement, id(groundID), 7), zone: zone, clock: clock.New(start, rate), workflow: workflow, tick: 0,
	}
}

func (r *runner) run(ticks int) {
	for range ticks {
		r.tick++
		passed := r.clock.Advance()
		r.npc.Step(npcrun.World{Zone: r.zone, Workflow: r.workflow, Clock: r.clock, Passed: passed}, r.tick)
	}
}

const eight = 8 * 3600

// TestMoveThenWait moves two tiles, one every play.StepTicks, then waits two
// game seconds and goes back to the start to move again.
func TestMoveThenWait(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindMove, map[string]any{"direction": flow.DirectionRight, "tiles": 2}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(4, flow.UnitSeconds, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	r := newRunner(t, workflow, image.Pt(0, 6), eight, clock.Real)
	r.run(1)
	if r.npc.Node != id(2) || r.npc.Activity(workflow) != "Move right, 1 of 2 tiles" {
		t.Fatalf("moving: %v %q", r.npc.Node, r.npc.Activity(workflow))
	}
	r.run(2 * play.StepTicks)
	if r.npc.Cell != image.Pt(2, 6) || r.npc.Node != id(3) {
		t.Fatalf("two tiles then the wait: %v on %v", r.npc.Cell, r.npc.Node)
	}
	if activity := r.npc.Activity(workflow); activity != "Wait 4 s of 4 s (game time)" {
		t.Fatalf("waiting: %q", activity)
	}
	r.run(clock.TicksPerSecond)
	if !strings.HasPrefix(r.npc.Activity(workflow), "Wait 3 s") {
		t.Fatalf("a second of game time at real rate: %q", r.npc.Activity(workflow))
	}
	r.run(3*clock.TicksPerSecond + 1)
	if r.npc.Node != id(2) || r.npc.Last.From != id(1) {
		t.Fatalf("back to the start and moving again: on %v after %+v", r.npc.Node, r.npc.Last)
	}
}

// TestMoveStopsAtAWall carries on when the wall is in the way, saying so on
// the change it carried on by.
func TestMoveStopsAtAWall(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindMove, map[string]any{"direction": flow.DirectionRight, "tiles": 5}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	r := newRunner(t, workflow, image.Pt(3, 2), eight, clock.Real)
	r.run(5 * play.StepTicks)
	if r.npc.Cell != image.Pt(4, 2) || r.npc.Node != id(3) || r.npc.Last.Status != npcrun.StatusBlocked {
		t.Fatalf("stopped beside the wall and carried on: %v on %v, %+v", r.npc.Cell, r.npc.Node, r.npc.Last)
	}
	if r.npc.Status != "" || !strings.HasPrefix(r.npc.Activity(workflow), "Wait 59 min") {
		t.Fatalf("the wait after it is not blocked: %q, %q", r.npc.Status, r.npc.Activity(workflow))
	}
}

// TestTakenListsTheChangesOfOneStep lists every node passed in one tick: a
// finished wait, the start, a random choice and a blocked move, back to the
// wait.
func TestTakenListsTheChangesOfOneStep(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindRandom, map[string]any{"options": []flow.Share{{Port: "o1", Label: "only", Share: 1}}}),
		node(t, 3, flow.KindMove, map[string]any{"direction": flow.DirectionUp, "tiles": 1}),
		node(t, 4, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, "o1", 3), link(3, flow.OutputNext, 4))
	r := newRunner(t, workflow, image.Pt(1, 0), eight, clock.Real)
	r.run(1)
	if r.npc.Node != id(4) || len(r.npc.Taken) != 4 {
		t.Fatalf("start, choice, blocked move, wait: on %v after %+v", r.npc.Node, r.npc.Taken)
	}
	r.run(1)
	if len(r.npc.Taken) != 0 {
		t.Fatalf("a tick of waiting takes nothing: %+v", r.npc.Taken)
	}
	for range 2 * clock.TicksPerSecond {
		if r.run(1); len(r.npc.Taken) > 0 {
			break
		}
	}
	taken := r.npc.Taken
	if len(taken) != 4 || taken[0].From != id(4) || taken[0].To != id(1) || taken[1].To != id(2) ||
		taken[2].Output != "o1" || taken[3].From != id(3) || taken[3].Status != npcrun.StatusBlocked ||
		taken[3].To != id(4) || taken[2].Status != "" {
		t.Fatalf("the wait ends and the NPC comes round to it again: %+v", taken)
	}
}

// TestGoToWalksAroundTheWall finds a way round the wall to the desk.
func TestGoToWalksAroundTheWall(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindGoTo, map[string]any{
			"destination": tilefloor.AnchorRef{Floor: id(groundID), Anchor: id(deskID)},
		}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	r := newRunner(t, workflow, image.Pt(2, 1), eight, clock.Real)
	r.run(1)
	if activity := r.npc.Activity(workflow); !strings.HasPrefix(activity, "Walking to Desk, ") {
		t.Fatalf("walking: %q", activity)
	}
	for range 40 * play.StepTicks {
		before := r.npc.Cell
		r.run(1)
		if r.npc.Cell.X == 5 && r.npc.Cell.Y < 6 {
			t.Fatalf("walked through the wall at %v from %v", r.npc.Cell, before)
		}
	}
	if r.npc.Cell != image.Pt(8, 1) || r.npc.Node != id(3) {
		t.Fatalf("arrived at the desk: %v on %v (%q)", r.npc.Cell, r.npc.Node, r.npc.Status)
	}
}

// TestGoToAnotherFloorCarriesOn says it cannot reach a destination upstairs.
func TestGoToAnotherFloorCarriesOn(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindGoTo, map[string]any{
			"destination": tilefloor.AnchorRef{Floor: id(upperID), Anchor: id(deskID)},
		}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	r := newRunner(t, workflow, image.Pt(2, 1), eight, clock.Real)
	r.run(1)
	if r.npc.Node != id(3) || r.npc.Last.Status != npcrun.StatusOtherFloor || r.npc.Moving() {
		t.Fatalf("carried on: on %v, %+v", r.npc.Node, r.npc.Last)
	}
}

// TestTimeSwitchContainerAndJump waits outside working hours; after a jump
// into them it restarts, goes into the container and repeats its contents;
// when the container's time is up it leaves by its exit at once.
func TestTimeSwitchContainerAndJump(t *testing.T) {
	t.Parallel()
	timeSwitch := node(t, 2, flow.KindTimeSwitch, map[string]any{
		"ranges": []flow.TimeRange{{Port: "r1", From: 9 * 60, To: 17 * 60}},
	})
	container := node(t, 3, flow.KindContainer, map[string]any{
		"bound": flow.TimeRange{Port: "", From: 10 * 60, To: 10*60 + 5},
	})
	inner := node(t, 4, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)})
	inner.Parent = id(3)
	outside := node(t, 5, flow.KindWait, map[string]any{"duration": waitFor(5, flow.UnitHours, flow.ClockGame)})
	afterWork := node(t, 6, flow.KindWait, map[string]any{"duration": waitFor(5, flow.UnitHours, flow.ClockGame)})
	workflow := workflowOf(t, []flow.Node{timeSwitch, container, inner, outside, afterWork},
		link(1, flow.OutputNext, 2), link(2, "r1", 3), link(2, flow.OutputOtherwise, 5),
		link(3, flow.OutputInside, 4), link(3, flow.OutputExit, 6))
	r := newRunner(t, workflow, image.Pt(0, 6), eight, clock.MinutePerSecond)
	r.run(1)
	if r.npc.Node != id(5) {
		t.Fatalf("outside the times it takes otherwise: %v", r.npc.Node)
	}
	r.clock.Jump(10 * 3600)
	r.run(1)
	if r.npc.Node != id(4) || r.npc.Container != id(3) {
		t.Fatalf("a jump restarts into the container: %v in %v", r.npc.Node, r.npc.Container)
	}
	changes := r.npc.Changes
	r.run(3)
	if r.npc.Node != id(4) || r.npc.Changes == changes {
		t.Fatalf("inside, a free output repeats the contents: %v", r.npc.Node)
	}
	r.run(5 * clock.TicksPerSecond)
	if r.npc.Node != id(6) || r.npc.Container != (ids.ULID{}) || r.npc.Last.Output != flow.OutputExit {
		t.Fatalf("when its time is up it leaves by its exit: %v %+v", r.npc.Node, r.npc.Last)
	}
}

// TestRealWaitIgnoresTheClock finishes a real-time wait while the game
// clock is stopped.
func TestRealWaitIgnoresTheClock(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockReal)}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	r := newRunner(t, workflow, image.Pt(0, 6), eight, clock.Real)
	r.clock.Stop(true)
	r.run(clock.TicksPerSecond + 1)
	if r.npc.Node != id(3) {
		t.Fatalf("a real second passes with the clock stopped: on %v (%q)", r.npc.Node, r.npc.Activity(workflow))
	}
}

// TestLoopingWithoutWaitingStops gives up for the tick on a workflow that
// never takes time.
func TestLoopingWithoutWaitingStops(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, nil)
	r := newRunner(t, workflow, image.Pt(0, 6), eight, clock.Real)
	r.run(1)
	if r.npc.Status != npcrun.StatusLooping {
		t.Fatalf("status %q", r.npc.Status)
	}
	r.workflow = nil
	r.run(1)
	if r.npc.Status != npcrun.StatusNoBehaviour || r.npc.Activity(nil) != npcrun.StatusNoBehaviour {
		t.Fatalf("no behaviour: %q", r.npc.Status)
	}
}

// TestRandomChoiceFollowsItsShares takes the option with three shares about
// three times as often as the one with one.
func TestRandomChoiceFollowsItsShares(t *testing.T) {
	t.Parallel()
	random := node(t, 2, flow.KindRandom, map[string]any{"options": []flow.Share{
		{Port: "o1", Label: "one", Share: 1}, {Port: "o2", Label: "three", Share: 3},
	}})
	workflow := workflowOf(t, []flow.Node{
		random,
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)}),
		node(t, 4, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, "o1", 3), link(2, "o2", 4))
	r := newRunner(t, workflow, image.Pt(0, 6), eight, clock.MinutePerSecond)
	counts := map[ids.ULID]int{}
	for range 4000 {
		r.run(1)
		if r.npc.Last.Tick == r.tick && r.npc.Last.From == id(2) {
			counts[r.npc.Last.To]++
		}
	}
	one, three := counts[id(3)], counts[id(4)]
	if one == 0 || float64(three)/float64(one) < 2.5 || float64(three)/float64(one) > 3.5 {
		t.Fatalf("three shares to one: %d and %d", three, one)
	}
}

// TestRandomWalkRepeats runs the random walk twice with one seed: the NPC
// takes the same path.
func TestRandomWalkRepeats(t *testing.T) {
	t.Parallel()
	random := node(t, 2, flow.KindRandom, map[string]any{"options": []flow.Share{
		{Port: "o1", Label: "up", Share: 1}, {Port: "o2", Label: "down", Share: 3},
	}})
	workflow := workflowOf(t, []flow.Node{
		random,
		node(t, 3, flow.KindMove, map[string]any{"direction": flow.DirectionUp, "tiles": 1}),
		node(t, 4, flow.KindMove, map[string]any{"direction": flow.DirectionDown, "tiles": 1}),
		node(t, 5, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, "o1", 3), link(2, "o2", 4), link(3, flow.OutputNext, 5),
		link(4, flow.OutputNext, 5), link(5, flow.OutputNext, 2))
	first := newRunner(t, workflow, image.Pt(0, 3), eight, clock.MinutePerSecond)
	second := newRunner(t, workflow, image.Pt(0, 3), eight, clock.MinutePerSecond)
	moved := false
	for range 4000 {
		first.run(1)
		second.run(1)
		if first.npc.Cell != second.npc.Cell {
			t.Fatal("one seed, one path")
		}
		moved = moved || first.npc.Cell != image.Pt(0, 3)
	}
	if !moved {
		t.Fatal("the random walk moves")
	}
}

// soundsOf lists what Play sounds asked for, a step after its cue's @ when
// one asked for one.
func soundsOf(sounds []npcrun.Sound) string {
	listed := make([]string, 0, len(sounds))
	for _, sound := range sounds {
		if sound.Loudness != "" {
			listed = append(listed, sound.Cue+"@"+sound.Loudness)
		} else {
			listed = append(listed, sound.Cue)
		}
	}

	return strings.Join(listed, ",")
}

// TestPlaySoundAsksForItsCue asks for the cue, at the loudness it asks for,
// in the tick it reaches a Play sound and carries on at once, without a cue
// says why it carried on, and asks again only when it comes round again.
func TestPlaySoundAsksForItsCue(t *testing.T) {
	t.Parallel()
	nodes := []flow.Node{
		node(t, 2, flow.KindPlaySound, map[string]any{"cue": "door/open", "loudness": "louder"}),
		node(t, 3, flow.KindPlaySound, nil),
		node(t, 4, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockReal)}),
	}
	workflow := workflowOf(t, nodes, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputNext, 4))
	run := newRunner(t, workflow, image.Pt(1, 1), eight, clock.MinutePerSecond)
	run.run(1)
	if soundsOf(run.npc.Sounds) != "door/open@louder" || run.npc.Node != id(4) {
		t.Fatalf("one tick plays the cue and reaches the Wait: %v at %v", run.npc.Sounds, run.npc.Node)
	}
	if run.npc.Last.From != id(3) || run.npc.Last.Status != npcrun.StatusNoCue {
		t.Fatalf("a Play sound with no cue carries on and says why: %+v", run.npc.Last)
	}
	run.run(1)
	if len(run.npc.Sounds) != 0 {
		t.Fatalf("the cue is asked for once, not every tick: %v", run.npc.Sounds)
	}
	asked := 0
	for range clock.TicksPerSecond {
		run.run(1)
		if soundsOf(run.npc.Sounds) == "door/open@louder" {
			asked++
		}
	}
	if asked != 1 {
		t.Fatalf("after the Wait the branch comes round and asks once more, not %d times", asked)
	}
}

// TestLoopingPlaysNothing plays nothing from a loop that never waits, which
// would otherwise ask for its sound every tick.
func TestLoopingPlaysNothing(t *testing.T) {
	t.Parallel()
	nodes := []flow.Node{node(t, 2, flow.KindPlaySound, map[string]any{"cue": "bell"})}
	run := newRunner(t, workflowOf(t, nodes, link(1, flow.OutputNext, 2)), image.Pt(1, 1), eight, clock.MinutePerSecond)
	run.run(1)
	if run.npc.Status != npcrun.StatusLooping || len(run.npc.Sounds) != 0 {
		t.Fatalf("a loop without a wait plays nothing: %q %v", run.npc.Status, run.npc.Sounds)
	}
}
