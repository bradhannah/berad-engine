package npcrun_test

import (
	"image"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/play"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// whys runs a number of ticks and lists why each change of node was made.
func (r *runner) whys(ticks int) []string {
	var said []string
	for range ticks {
		r.run(1)
		for _, change := range r.npc.Taken {
			if change.Why != "" {
				said = append(said, change.Why)
			}
		}
	}

	return said
}

// TestEachChangeSaysWhy says what a time switch and a container read, why
// the NPC started again after the clock moved and what that cleared, and
// that a container's time was up (increment 46).
func TestEachChangeSaysWhy(t *testing.T) {
	t.Parallel()
	timeSwitch := node(t, 2, flow.KindTimeSwitch, map[string]any{
		"ranges": []flow.TimeRange{{Port: "r1", From: 9 * 60, To: 17 * 60}},
	})
	container := node(t, 3, flow.KindContainer, map[string]any{
		"bound": flow.TimeRange{Port: "", From: 10 * 60, To: 10*60 + 5},
	})
	inner := node(t, 4, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)})
	inner.Parent = id(3)
	outside := node(t, 5, flow.KindWait, map[string]any{"duration": waitFor(5, flow.UnitHours, flow.ClockGame)})
	afterWork := node(t, 6, flow.KindWait, map[string]any{"duration": waitFor(5, flow.UnitHours, flow.ClockGame)})
	workflow := workflowOf(t, []flow.Node{timeSwitch, container, inner, outside, afterWork},
		link(1, flow.OutputNext, 2), link(2, "r1", 3), link(2, flow.OutputOtherwise, 5),
		link(3, flow.OutputInside, 4), link(3, flow.OutputExit, 6))
	r := newRunner(t, workflow, image.Pt(0, 6), eight, clock.MinutePerSecond)
	if said := r.whys(1); !slices.Equal(said, []string{"Read 8:00: no time holds"}) {
		t.Fatalf("the time switch says what it read: %q", said)
	}
	r.clock.Jump(10 * 3600)
	said := r.whys(1)
	want := []string{
		"Started again: the clock moved to 10:00, clearing what it was doing",
		"Read 10:00: 9:00 to 17:00 holds", "Read 10:00: within 10:00 to 10:05",
	}
	if !slices.Equal(said, want) {
		t.Fatalf("a jump says it started again and what it cleared: %q", said)
	}
	if said := r.whys(6 * clock.TicksPerSecond); !slices.Contains(said, "Its time, 10:00 to 10:05, was up at 10:05") {
		t.Fatalf("leaving a container says its time was up: %q", said)
	}
}

// TestAMoveSaysWhatWasInItsWay says where a Move was blocked, and a Wait how
// long it waited.
func TestAMoveSaysWhatWasInItsWay(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindMove, map[string]any{"direction": flow.DirectionRight, "tiles": 5}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(2, flow.UnitSeconds, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	r := newRunner(t, workflow, image.Pt(3, 2), eight, clock.Real)
	if said := r.whys(5 * play.StepTicks); !slices.Contains(said, "Something was in its way at 5, 2") {
		t.Fatalf("a blocked move says where: %q", said)
	}
	if said := r.whys(3 * clock.TicksPerSecond); !slices.Contains(said, "Waited 2s") {
		t.Fatalf("a wait says how long: %q", said)
	}
}
