package npcrun_test

import (
	"encoding/json"
	"testing"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestAZoneLoopRuns runs a zone's loop that adds to the zone's count, rings
// a bell and waits a game minute: it does so once a minute, never moving,
// as a runner with no placement, NPC or floor.
func TestAZoneLoopRuns(t *testing.T) {
	t.Parallel()
	loop, err := flow.NewFor(id(1), flow.PurposeZoneLoop)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []flow.Node{
		node(t, 2, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeZone, countID), "how": flow.HowAdd,
			"to": json.RawMessage("1")}),
		node(t, 3, flow.KindPlaySound, map[string]any{"cue": "market/bell"}),
		node(t, 4, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitMinutes, flow.ClockGame)}),
	}
	if err := loop.Insert(nodes, nil, []flow.Link{
		link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputNext, 4),
	}); err != nil {
		t.Fatal(err)
	}
	state := worldstate.NewStore(catalog(t))
	runner := npcrun.StartLoop(id(zoneID), 7)
	if !runner.IsLoop() || !runner.NPC.IsZero() || !runner.Placement.IsZero() || !runner.Floor.IsZero() {
		t.Fatalf("a zone loop has no placement, NPC or floor: %+v", runner)
	}
	levels := play.Levels{{ID: id(groundID), Floor: office(t)}}
	gameClock := clock.New(eight, clock.MinutePerSecond)
	bells := 0
	for tick := uint64(1); tick <= 3*clock.TicksPerSecond; tick++ {
		passed := gameClock.Advance()
		runner.Step(npcrun.World{
			Zone: levels, Workflow: loop, Behaviour: id(1), Workflows: nil, Clock: gameClock, Passed: passed,
			State: state, Here: id(zoneID), Others: nil,
		}, tick)
		if soundsOf(runner.Sounds) == "market/bell" {
			bells++
		}
		if runner.Moving() {
			t.Fatal("a zone loop never moves")
		}
	}
	declared, _, _ := state.Catalog().Declared(id(countID))
	count := state.Get(declared, id(zoneID))
	if bells != 3 || count.Integer != 3 {
		t.Fatalf("three game minutes ring the bell and count three times: %d bells, count %d", bells, count.Integer)
	}
}
