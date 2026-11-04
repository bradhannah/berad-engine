package workflow_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/workflow"
)

// lanesOf makes a Together's lanes, l1, l2 and so on, with names.
func lanesOf(names ...string) []workflow.Lane {
	lanes := make([]workflow.Lane, 0, len(names))
	for index, name := range names {
		lanes = append(lanes, workflow.Lane{Port: "l" + string(rune('1'+index)), Label: name})
	}

	return lanes
}

// together is start → a Together of three lanes, Sound, Shake and Say: the
// first leads to a Wait, the second to a Move that goes on to the same Wait,
// the third to nothing; all done leads to a Wait outside.
func together(t *testing.T) *workflow.Workflow {
	t.Helper()
	container, err := node(t, 2, workflow.KindTogether, id(0)).WithParam(workflow.ParamLanes, lanesOf("Sound", "Shake", "Say"))
	if err != nil {
		t.Fatal(err)
	}
	made, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := made.Insert([]workflow.Node{
		container, node(t, 3, workflow.KindWait, id(2)), node(t, 4, workflow.KindMove, id(2)),
		node(t, 5, workflow.KindWait, id(0)),
	}, nil, []workflow.Link{
		{From: id(1), Output: workflow.OutputNext, To: id(2)}, {From: id(2), Output: "l1", To: id(3)},
		{From: id(2), Output: "l2", To: id(4)}, {From: id(4), Output: workflow.OutputNext, To: id(3)},
		{From: id(2), Output: workflow.OutputDone, To: id(5)},
	}); err != nil {
		t.Fatal(err)
	}

	return made
}

// TestTogetherSavesItsLanes saves and reads a Together: its lanes in order,
// each an output, then all done, and the lanes that reach each node inside.
func TestTogetherSavesItsLanes(t *testing.T) {
	t.Parallel()
	data, err := together(t).Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := workflow.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	container, _ := read.Node(id(2))
	outputs := workflow.Outputs(container)
	names := make([]string, 0, len(outputs))
	for _, output := range outputs {
		names = append(names, output.Label)
	}
	if !slices.Equal(names, []string{"Sound", "Shake", "Say", "all done"}) {
		t.Fatalf("each lane, then all done: %v", names)
	}
	reached := read.LanesOf(id(2))
	if !slices.Equal(reached[id(4)], []string{"l2"}) || !slices.Equal(reached[id(3)], []string{"l1", "l2"}) {
		t.Fatalf("the Move is in Shake; the Wait both lanes reach is in both: %v", reached)
	}
	if !workflow.IsInner(container, "l3") || workflow.IsInner(container, workflow.OutputDone) {
		t.Fatal("a lane leads inside; all done leads out")
	}
}

// TestTogetherRefusals refuses a Together of one lane or of nine, two lanes
// by one output, a lane's first step outside it, and a Together inside a
// container.
func TestTogetherRefusals(t *testing.T) {
	t.Parallel()
	for name, lanes := range map[string][]workflow.Lane{
		"one lane":   lanesOf("Only"),
		"nine lanes": lanesOf("1", "2", "3", "4", "5", "6", "7", "8", "9"),
		"one output": {{Port: "l1", Label: "A"}, {Port: "l1", Label: "B"}},
		"all done":   {{Port: "l1", Label: "A"}, {Port: workflow.OutputDone, Label: "B"}},
	} {
		made, err := node(t, 2, workflow.KindTogether, id(0)).WithParam(workflow.ParamLanes, lanes)
		if err != nil {
			t.Fatal(err)
		}
		if err := workflow.CheckParams(made); !errors.Is(err, workflow.ErrNode) {
			t.Fatalf("%s is refused: %v", name, err)
		}
	}
	made, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	outside := made.Insert([]workflow.Node{node(t, 2, workflow.KindTogether, id(0)), node(t, 3, workflow.KindWait, id(0))},
		nil, []workflow.Link{{From: id(2), Output: "l1", To: id(3)}})
	nested := made.Insert([]workflow.Node{
		node(t, 2, workflow.KindContainer, id(0)), node(t, 3, workflow.KindTogether, id(2)),
	}, nil, nil)
	if !errors.Is(outside, workflow.ErrNode) || !errors.Is(nested, workflow.ErrNode) {
		t.Fatalf("a lane leads inside, and a Together sits at the top: %v %v", outside, nested)
	}
}

// TestWhatALaneHolds lets a lane hold steps in a line, not a Call, a Return
// or a container, and names Move and Go to as the nodes that move the NPC.
func TestWhatALaneHolds(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{workflow.KindWait, workflow.KindPlaySound, workflow.KindSay, workflow.KindSetValue} {
		if !workflow.InLane(kind) {
			t.Fatalf("a lane holds %s", kind)
		}
	}
	for _, kind := range []string{
		workflow.KindCall, workflow.KindReturn, workflow.KindContainer, workflow.KindTogether, workflow.KindStart,
	} {
		if workflow.InLane(kind) {
			t.Fatalf("a lane can't hold %s", kind)
		}
	}
	if !workflow.Moves(workflow.KindGoTo) || !workflow.Moves(workflow.KindMove) || workflow.Moves(workflow.KindShake) {
		t.Fatal("only Move and Go to move the NPC")
	}
}

// TestUpgradeFromVersion8 gives a version 8 Play sound the way it carried on
// then, at once.
func TestUpgradeFromVersion8(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(workflow.Format()); err != nil {
		t.Fatal(err)
	}
	eight := `{"start":"` + id(1).String() + `","nodes":[{"id":"` + id(1).String() + `","kind":"start","next":{"next":"` +
		id(2).String() + `"}},{"id":"` + id(2).String() + `","kind":"play-sound","params":{"cue":"","loudness":"as-cue"}}]}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.FormatName, Version: 8, ID: id(9), Name: "Old", Revision: 1, Data: json.RawMessage(eight),
	})
	if err != nil || !changed || upgraded.Version != workflow.FormatVersion {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	read, err := workflow.Decode(upgraded.Data)
	if err != nil {
		t.Fatal(err)
	}
	sound, _ := read.Node(id(2))
	if carry, err := sound.Choice(workflow.ParamCarry); err != nil || carry != workflow.CarryAtOnce {
		t.Fatalf("a version 8 Play sound carries on at once: %q %v", carry, err)
	}
}
