package npcrun_test

import (
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// stairsAt puts a stand-in on a floor's top layer.
func stairsAt(t *testing.T, floor *tilefloor.Floor, cell image.Point, tile string) {
	t.Helper()
	if _, err := floor.SetTop(cell.X, cell.Y, tile); err != nil {
		t.Fatal(err)
	}
}

// goAndBack goes to the upstairs desk, waits, and comes back down to the
// downstairs one.
func goAndBack(t *testing.T) *flow.Workflow {
	t.Helper()

	return workflowOf(t, []flow.Node{
		node(t, 2, flow.KindGoTo, map[string]any{
			"destination": tilefloor.AnchorRef{Floor: id(upperID), Anchor: id(deskID)},
		}),
		node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockReal)}),
		node(t, 4, flow.KindGoTo, map[string]any{
			"destination": tilefloor.AnchorRef{Floor: id(groundID), Anchor: id(deskID)},
		}),
		node(t, 5, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputNext, 4), link(4, flow.OutputNext, 5))
}

// TestGoToTakesTwoWayStairs walks up the nearer of two stairs to a desk
// upstairs, saying the stairs on its way, then back down the only stairs
// down to the desk downstairs, walking around the stairs up below the wall,
// which it does not mean to take.
func TestGoToTakesTwoWayStairs(t *testing.T) {
	t.Parallel()
	workflow := goAndBack(t)
	r := newRunner(t, workflow, image.Pt(2, 6), eight, clock.Real)
	stairsAt(t, r.zone[0].Floor, image.Pt(5, 6), "stairs.up")
	stairsAt(t, r.zone[0].Floor, image.Pt(1, 3), "stairs.up")
	stairsAt(t, r.zone[1].Floor, image.Pt(1, 3), "stairs.down")
	r.run(1)
	if way := r.npc.Way(); len(way) != 1 || way[0].At != image.Pt(5, 6) || way[0].From != id(groundID) ||
		way[0].To != id(upperID) {
		t.Fatalf("the way up is the nearer stairs, at 5, 6: %+v (%q)", way, r.npc.Last.Status)
	}
	if activity := r.npc.Activity(workflow); !strings.HasPrefix(activity, "Walking to Desk, ") {
		t.Fatalf("walking: %q", activity)
	}
	climbed := false
	for range 60 * play.StepTicks {
		r.run(1)
		climbed = climbed || r.npc.Floor == id(upperID)
		if r.npc.Node == id(3) {
			break
		}
	}
	if r.npc.Floor != id(upperID) || r.npc.Cell != image.Pt(8, 1) || len(r.npc.Way()) != 0 {
		t.Fatalf("arrived at the upstairs desk: %v on %v, way %+v (%q)", r.npc.Cell, r.npc.Floor, r.npc.Way(),
			r.npc.Last.Status)
	}
	for range 80 * play.StepTicks {
		r.run(1)
		if r.npc.Node == id(5) {
			break
		}
	}
	if !climbed || r.npc.Floor != id(groundID) || r.npc.Cell != image.Pt(8, 1) {
		t.Fatalf("came back down to the desk: %v on %v (%q)", r.npc.Cell, r.npc.Floor, r.npc.Last.Status)
	}
}

// TestGoToFallsDownAHole takes one-way stairs, a hole, down to a desk
// below, and cannot come back up, so it carries on.
func TestGoToFallsDownAHole(t *testing.T) {
	t.Parallel()
	workflow := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindGoTo, map[string]any{
			"destination": tilefloor.AnchorRef{Floor: id(groundID), Anchor: id(deskID)},
		}),
		node(t, 3, flow.KindGoTo, map[string]any{
			"destination": tilefloor.AnchorRef{Floor: id(upperID), Anchor: id(deskID)},
		}),
		node(t, 4, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputNext, 4))
	zone := play.Levels{{ID: id(groundID), Floor: office(t)}, {ID: id(upperID), Floor: office(t)}}
	stairsAt(t, zone[1].Floor, image.Pt(2, 3), "stairs.down")
	placement := tilefloor.Placement{ID: id(100), NPC: id(101), Col: 2, Row: 6}
	r := &runner{npc: npcrun.Start(placement, id(upperID), 7), zone: zone, clock: clock.New(eight, clock.Real),
		workflow: workflow, tick: 0}
	for range 60 * play.StepTicks {
		r.run(1)
		if r.npc.Node == id(3) {
			break
		}
	}
	if r.npc.Floor != id(groundID) || r.npc.Cell != image.Pt(8, 1) {
		t.Fatalf("fell down the hole to the desk below: %v on %v (%q)", r.npc.Cell, r.npc.Floor, r.npc.Last.Status)
	}
	r.run(2)
	if r.npc.Node != id(4) || r.npc.Last.Status != npcrun.StatusOtherFloor {
		t.Fatalf("no stairs lead back up, so it carries on: on %v, %+v", r.npc.Node, r.npc.Last)
	}
}
