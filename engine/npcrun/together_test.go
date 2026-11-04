package npcrun_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// togetherOf makes a Together of named lanes, l1, l2 and so on.
func togetherOf(t *testing.T, seed byte, names ...string) flow.Node {
	t.Helper()
	lanes := make([]flow.Lane, 0, len(names))
	for index, name := range names {
		lanes = append(lanes, flow.Lane{Port: "l" + string(rune('1'+index)), Label: name})
	}

	return node(t, seed, flow.KindTogether, map[string]any{flow.ParamLanes: lanes})
}

// inside puts a node inside a container.
func inside(made flow.Node, container byte) flow.Node {
	made.Parent = id(container)

	return made
}

// realWait is a Wait of whole real seconds.
func realWait(t *testing.T, seed byte, seconds int) flow.Node {
	t.Helper()

	return node(t, seed, flow.KindWait, map[string]any{"duration": waitFor(seconds, flow.UnitSeconds, flow.ClockReal)})
}

// TestTogetherWaitsForEveryLane runs three lanes of one, three and two real
// seconds at once: the NPC goes on by all done only when the longest is
// done, and each lane says what it is doing or that it is done.
func TestTogetherWaitsForEveryLane(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		togetherOf(t, 2, "Sound", "Shake", "Say"),
		inside(realWait(t, 3, 1), 2), inside(realWait(t, 4, 3), 2), inside(realWait(t, 5, 2), 2),
		hour(t, 9),
	}, link(1, flow.OutputNext, 2), link(2, "l1", 3), link(2, "l2", 4), link(2, "l3", 5), link(2, flow.OutputDone, 9))
	r := newRunner(t, workflow, image.Pt(2, 6), eight, clock.Real)
	r.run(1)
	if r.npc.Node != id(2) || r.npc.Activity(workflow) != "Together, 3 of 3 lanes running" {
		t.Fatalf("every lane starts at once: %v %q", r.npc.Node, r.npc.Activity(workflow))
	}
	lanes := r.npc.Lanes(workflow)
	if len(lanes) != 3 || lanes[1].Label != "Shake" || lanes[1].Node != id(4) || lanes[1].Doing != "Wait 3 s of 3 s (real time)" {
		t.Fatalf("each lane says what it is doing: %+v", lanes)
	}
	r.run(clock.TicksPerSecond + 1)
	lanes = r.npc.Lanes(workflow)
	if !lanes[0].Done || lanes[0].Doing != npcrun.LaneDone || lanes[1].Done || lanes[2].Done {
		t.Fatalf("the first lane is done after a second, the others run on: %+v", lanes)
	}
	r.run(clock.TicksPerSecond)
	if r.npc.Node != id(2) || r.npc.Activity(workflow) != "Together, 1 of 3 lanes running" {
		t.Fatalf("the NPC waits for the last lane: %v %q", r.npc.Node, r.npc.Activity(workflow))
	}
	r.run(clock.TicksPerSecond)
	if r.npc.Node != id(9) || r.npc.Last.From != id(2) || r.npc.Last.Output != flow.OutputDone {
		t.Fatalf("all done once the last lane is: on %v after %+v", r.npc.Node, r.npc.Last)
	}
	if lanes := r.npc.Lanes(workflow); len(lanes) != 0 {
		t.Fatalf("no lanes once it has gone on: %+v", lanes)
	}
}

// TestALaneGoesOnInItsLine follows a lane through two steps, each change
// naming the lane, while a lane with no steps is done at once.
func TestALaneGoesOnInItsLine(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		togetherOf(t, 2, "Steps", "Empty"),
		inside(realWait(t, 3, 1), 2), inside(realWait(t, 4, 1), 2),
		hour(t, 9),
	}, link(1, flow.OutputNext, 2), link(2, "l1", 3), link(3, flow.OutputNext, 4), link(2, flow.OutputDone, 9))
	r := newRunner(t, workflow, image.Pt(2, 6), eight, clock.Real)
	r.run(1)
	if lanes := r.npc.Lanes(workflow); !lanes[1].Done || lanes[0].Node != id(3) {
		t.Fatalf("an empty lane is done at once: %+v", lanes)
	}
	r.run(clock.TicksPerSecond + 1)
	if lanes := r.npc.Lanes(workflow); lanes[0].Node != id(4) || r.npc.Last.Lane != "l1" || r.npc.Last.To != id(4) {
		t.Fatalf("the lane goes on to its next step, the change naming it: %+v %+v", lanes, r.npc.Last)
	}
	r.run(clock.TicksPerSecond)
	if r.npc.Node != id(9) {
		t.Fatalf("all done when the line runs out: %v", r.npc.Node)
	}
}

// TestOneLaneMovesWhileOthersRun moves the NPC three tiles in one lane
// while another lane waits a second; a lane that also moves waits until the
// NPC stands still, so the moves never overlap.
func TestOneLaneMovesWhileOthersRun(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		togetherOf(t, 2, "Walk", "Wait", "Step down"),
		inside(node(t, 3, flow.KindMove, map[string]any{"direction": flow.DirectionRight, "tiles": 3}), 2),
		inside(realWait(t, 4, 1), 2),
		inside(node(t, 5, flow.KindMove, map[string]any{"direction": flow.DirectionDown, "tiles": 1}), 2),
		hour(t, 9),
	}, link(1, flow.OutputNext, 2), link(2, "l1", 3), link(2, "l2", 4), link(2, "l3", 5), link(2, flow.OutputDone, 9))
	r := newRunner(t, workflow, image.Pt(0, 6), eight, clock.Real)
	r.run(1)
	if lanes := r.npc.Lanes(workflow); !r.npc.Moving() || lanes[0].Doing != "Move right, 1 of 3 tiles" ||
		lanes[2].Doing != "Move" {
		t.Fatalf("the first lane walks, the third waits to: %+v", lanes)
	}
	r.run(play.StepTicks)
	if lanes := r.npc.Lanes(workflow); lanes[1].Done || lanes[1].Doing == "Wait 1 s of 1 s (real time)" ||
		r.npc.Cell != image.Pt(1, 6) {
		t.Fatalf("the waiting lane counts down while the NPC walks: %+v at %v", lanes, r.npc.Cell)
	}
	r.run(2 * play.StepTicks)
	if r.npc.Cell != image.Pt(3, 6) || r.npc.To != image.Pt(3, 7) {
		t.Fatalf("three tiles right, then the third lane steps down: %v to %v", r.npc.Cell, r.npc.To)
	}
	r.run(play.StepTicks)
	if lanes := r.npc.Lanes(workflow); r.npc.Cell != image.Pt(3, 7) || r.npc.Node != id(2) || lanes[1].Done {
		t.Fatalf("stepped down, still waiting for the second lane: %v on %v, %+v", r.npc.Cell, r.npc.Node, lanes)
	}
	r.run(clock.TicksPerSecond)
	if r.npc.Node != id(9) {
		t.Fatalf("all done once the wait is: on %v", r.npc.Node)
	}
}

// TestPlaySoundWaitsUntilPlayed waits on a Play sound for its cue's ticks
// when it carries on once the sound has played, asking for the cue once;
// without the cue's length it carries on at once.
func TestPlaySoundWaitsUntilPlayed(t *testing.T) {
	t.Parallel()
	const bellTicks = 30
	workflow := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindPlaySound, map[string]any{"cue": "bell", flow.ParamCarry: flow.CarryWhenDone}),
		hour(t, 3),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	for name, lengths := range map[string]func(string) int{
		"known":   func(cue string) int { return map[string]int{"bell": bellTicks}[cue] },
		"unknown": nil,
	} {
		r := newRunner(t, workflow, image.Pt(2, 6), eight, clock.Real)
		heard := 0
		for range bellTicks + 2 {
			r.tick++
			passed := r.clock.Advance()
			r.npc.Step(npcrun.World{Zone: r.zone, Workflow: workflow, Clock: r.clock, Passed: passed, CueTicks: lengths}, r.tick)
			heard += len(r.npc.Sounds)
			if r.tick == bellTicks && name == "known" && r.npc.Node != id(2) {
				t.Fatalf("still playing the bell after %d ticks: on %v", r.tick, r.npc.Node)
			}
		}
		if heard != 1 || r.npc.Node != id(3) {
			t.Fatalf("%s: the bell asked for once, then on: %d on %v", name, heard, r.npc.Node)
		}
	}
}
