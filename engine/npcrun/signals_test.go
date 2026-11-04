package npcrun_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The test signals, and the guard's NPC, which the baker signals.
const (
	breadSignal = 160
	flourSignal = 161
	guardNPC    = 111
)

// gameMinutes is a Wait of game minutes.
func gameMinutes(t *testing.T, seed byte, minutes int) flow.Node {
	t.Helper()

	return node(t, seed, flow.KindWait, map[string]any{"duration": waitFor(minutes, flow.UnitMinutes, flow.ClockGame)})
}

// sendTo is a Send signal of a signal to one NPC.
func sendTo(t *testing.T, seed, signal, npc byte) flow.Node {
	t.Helper()

	return node(t, seed, flow.KindSendSignal, map[string]any{
		flow.ParamSignal: id(signal).String(), flow.ParamSendTo: flow.ToNPC, flow.ParamNPC: id(npc).String(),
	})
}

// onSignal is a receiver of a signal.
func onSignal(t *testing.T, seed, signal byte) flow.Node {
	t.Helper()

	return node(t, seed, flow.KindOnSignal, map[string]any{flow.ParamSignal: id(signal).String()})
}

// baker waits two game minutes, then signals the guard with each signal,
// and waits an hour.
func baker(t *testing.T, signals ...byte) *flow.Workflow {
	t.Helper()
	nodes := []flow.Node{gameMinutes(t, 2, 2), hour(t, 9)}
	links := []flow.Link{link(1, flow.OutputNext, 2)}
	from := byte(2)
	for index, signal := range signals {
		seed := byte(3 + index)
		nodes = append(nodes, sendTo(t, seed, signal, guardNPC))
		links = append(links, link(from, flow.OutputNext, seed))
		from = seed
	}

	return workflowOf(t, nodes, append(links, link(from, flow.OutputNext, 9))...)
}

// signalTown is the baker at (1, 6) and the guard at (2, 6), the baker
// signalling the guard.
func signalTown(t *testing.T, guard *flow.Workflow, signals ...byte) (*town, *npcrun.NPC) {
	t.Helper()
	made := newTown(t, worldstate.NewStore(worldstate.NewCatalog(worldstate.NewDeclarations(worldstate.HoldsWorldState),
		nil)))
	made.place(100, image.Pt(1, 6), baker(t, signals...))

	return made, made.place(guardNPC-1, image.Pt(2, 6), guard)
}

// TestASignalInterruptsAndResumes signals the guard partway through his
// ten-minute Wait: he stops at once to handle it, keeping where he was,
// then Resume takes him back to his Wait with the time it had left.
func TestASignalInterruptsAndResumes(t *testing.T) {
	t.Parallel()
	guard := workflowOf(t, []flow.Node{
		gameMinutes(t, 2, 10), onSignal(t, 20, breadSignal), gameMinutes(t, 21, 1), node(t, 22, flow.KindResume, nil),
	}, link(1, flow.OutputNext, 2), link(20, flow.OutputNext, 21), link(21, flow.OutputNext, 22))
	town, npc := signalTown(t, guard, breadSignal)
	town.run(121)
	if npc.Node != id(2) || len(town.sent) != 1 {
		t.Fatalf("the baker has sent, and the guard waits on: %v %+v", npc.Node, town.sent)
	}
	town.run(1)
	heard, handling := npc.Handling()
	resumes, doing := npc.Resumes(guard, nil)
	if !handling || heard.Signal != id(breadSignal) || heard.From != id(101) || npc.Node != id(21) {
		t.Fatalf("the guard handles the signal on the next tick: %+v %v", heard, npc.Node)
	}
	if resumes != id(2) || doing != "Wait 7 min 59 s of 10 min (game time)" {
		t.Fatalf("he will resume his Wait: %v %q", resumes, doing)
	}
	if !slices.ContainsFunc(npc.Lines, func(line npcrun.Line) bool { return strings.HasPrefix(line.Text, "Heard ") }) {
		t.Fatalf("hearing it is logged: %+v", npc.Lines)
	}
	town.run(60)
	if _, still := npc.Handling(); still || npc.Node != id(2) || npc.Last.From != id(22) ||
		npc.Activity(guard) != "Wait 7 min 59 s of 10 min (game time)" {
		t.Fatalf("Resume takes him back to his Wait, its time kept: %v %q", npc.Node, npc.Activity(guard))
	}
}

// TestAReceiverTakesOver follows a receiver's line into the flow, which
// carries on from there, the interruption forgotten.
func TestAReceiverTakesOver(t *testing.T) {
	t.Parallel()
	guard := workflowOf(t, []flow.Node{
		gameMinutes(t, 2, 10), hour(t, 3), onSignal(t, 20, breadSignal), gameMinutes(t, 21, 1),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(20, flow.OutputNext, 21),
		link(21, flow.OutputNext, 3))
	town, npc := signalTown(t, guard, breadSignal)
	town.run(122)
	if _, handling := npc.Handling(); !handling {
		t.Fatal("the guard handles the signal")
	}
	town.run(60)
	if _, handling := npc.Handling(); handling || npc.Node != id(3) {
		t.Fatalf("the receiver's line takes over: %v", npc.Node)
	}
	if _, doing := npc.Resumes(guard, nil); doing != "" {
		t.Fatalf("nothing to resume: %q", doing)
	}
}

// TestASecondSignalIsHeld sends two signals at once: the second waits while
// the first is handled, then lands; a signal the guard doesn't answer is
// gone.
func TestASecondSignalIsHeld(t *testing.T) {
	t.Parallel()
	guard := workflowOf(t, []flow.Node{
		gameMinutes(t, 2, 10), onSignal(t, 20, breadSignal), gameMinutes(t, 21, 1), node(t, 22, flow.KindResume, nil),
		onSignal(t, 30, flourSignal), node(t, 31, flow.KindResume, nil),
	}, link(1, flow.OutputNext, 2), link(20, flow.OutputNext, 21), link(21, flow.OutputNext, 22),
		link(30, flow.OutputNext, 31))
	town, npc := signalTown(t, guard, 170, breadSignal, flourSignal)
	town.run(122)
	held := npc.Held()
	if heard, _ := npc.Handling(); heard.Signal != id(breadSignal) || len(held) != 1 || held[0].Signal != id(flourSignal) {
		t.Fatalf("the bread lands first, the flour is held, the unanswered one is gone: %+v %+v", heard, held)
	}
	town.run(60)
	if _, handling := npc.Handling(); handling || len(npc.Held()) != 1 || npc.Node != id(2) {
		t.Fatalf("resumed, the flour still waits for the next tick: %+v %v", npc.Held(), npc.Node)
	}
	town.run(1)
	if len(npc.Held()) != 0 || npc.Last.From != id(31) || npc.Node != id(2) {
		t.Fatalf("the flour lands, is handled and resumed: %+v %+v", npc.Held(), npc.Last)
	}
}

// TestAWalkIsInterruptedAtACell signals the guard as he walks to the desk:
// the signal lands as he reaches a cell, he steps down for it, and
// resuming finds a new way to the desk from where he stands.
func TestAWalkIsInterruptedAtACell(t *testing.T) {
	t.Parallel()
	guard := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindGoTo, map[string]any{
			"destination": tilefloor.AnchorRef{Floor: id(groundID), Anchor: id(deskID)},
		}),
		hour(t, 3), onSignal(t, 20, breadSignal),
		node(t, 21, flow.KindMove, map[string]any{"direction": flow.DirectionDown, "tiles": 1}),
		node(t, 22, flow.KindResume, nil),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(20, flow.OutputNext, 21),
		link(21, flow.OutputNext, 22))
	town := newTown(t, worldstate.NewStore(worldstate.NewCatalog(worldstate.NewDeclarations(worldstate.HoldsWorldState),
		nil)))
	town.place(100, image.Pt(1, 6), workflowOf(t, []flow.Node{
		node(t, 2, flow.KindWait, map[string]any{"duration": waitFor(45, flow.UnitSeconds, flow.ClockGame)}),
		sendTo(t, 3, breadSignal, guardNPC), hour(t, 4),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputNext, 4)))
	npc := town.place(guardNPC-1, image.Pt(2, 6), guard)
	for range 200 {
		town.run(1)
		if _, handling := npc.Handling(); handling {
			break
		}
	}
	if npc.Node != id(21) || npc.To != npc.Cell.Add(image.Pt(0, 1)) {
		t.Fatalf("the signal lands at a cell, and he steps down from it: %v %v → %v", npc.Node, npc.Cell, npc.To)
	}
	town.run(600)
	if npc.Cell != image.Pt(8, 1) || npc.Node != id(3) {
		t.Fatalf("resumed, he finds his way to the desk: %v on %v", npc.Cell, npc.Node)
	}
}

// TestASignalLandsInAModule interrupts the guard inside a call: the
// receiver runs in his behaviour, out of the call, and resuming goes back
// into the module, to its Wait.
func TestASignalLandsInAModule(t *testing.T) {
	t.Parallel()
	module := workflowOf(t, []flow.Node{gameMinutes(t, 2, 10), returnTo(t, 3, "Done", "w1")},
		link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	guard := workflowOf(t, []flow.Node{
		callOf(t, 2, []flow.WayOut{{Port: "w1", Label: "Done"}}, nil, nil), hour(t, 3),
		onSignal(t, 20, breadSignal), gameMinutes(t, 21, 1), node(t, 22, flow.KindResume, nil),
	}, link(1, flow.OutputNext, 2), link(2, "w1", 3), link(20, flow.OutputNext, 21), link(21, flow.OutputNext, 22))
	town, npc := signalTown(t, guard, breadSignal)
	town.modules = func(target ids.ULID) (*flow.Workflow, bool) { return module, target == id(moduleID) }
	town.run(122)
	if _, handling := npc.Handling(); !handling || len(npc.Callers()) != 0 || npc.Node != id(21) {
		t.Fatalf("the receiver runs in his behaviour, out of the call: %v %+v", npc.Node, npc.Callers())
	}
	town.run(60)
	if len(npc.Callers()) != 1 || npc.Module() != id(moduleID) || npc.Node != id(2) {
		t.Fatalf("resuming goes back into the module's Wait: %v %+v", npc.Node, npc.Callers())
	}
}

// TestSignalsRepeatExactly runs the same signalling twice, to the same
// changes.
func TestSignalsRepeatExactly(t *testing.T) {
	t.Parallel()
	runs := make([]npcrun.Change, 0, 2)
	for range 2 {
		guard := workflowOf(t, []flow.Node{
			gameMinutes(t, 2, 10), onSignal(t, 20, breadSignal), gameMinutes(t, 21, 1), node(t, 22, flow.KindResume, nil),
		}, link(1, flow.OutputNext, 2), link(20, flow.OutputNext, 21), link(21, flow.OutputNext, 22))
		town, npc := signalTown(t, guard, breadSignal)
		town.run(300)
		runs = append(runs, npc.Last)
	}
	if runs[0] != runs[1] {
		t.Fatalf("the same run: %+v %+v", runs[0], runs[1])
	}
}
