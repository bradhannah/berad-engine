package workflow_test

import (
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/workflow"
)

// TestTheShowNodes saves and reads a Say, a Show message and a Shake
// screen, each with its settings, finds which use their seconds, and
// refuses seconds out of range.
func TestTheShowNodes(t *testing.T) {
	t.Parallel()
	say := node(t, 2, workflow.KindSay, id(0))
	say, err := say.WithParam(workflow.ParamShows, workflow.ShowsSeconds)
	if err == nil {
		say, err = say.WithParam(workflow.ParamSeconds, 6)
	}
	box := node(t, 3, workflow.KindShowMessage, id(0))
	if err == nil {
		box, err = box.WithParam(workflow.ParamAway, workflow.AwayClosed)
	}
	shake := node(t, 4, workflow.KindShake, id(0))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := saved.Insert([]workflow.Node{say, box, shake}, nil, nil); err != nil {
		t.Fatal(err)
	}
	data, err := saved.Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := workflow.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	readSay, _ := read.Node(id(2))
	readBox, _ := read.Node(id(3))
	readShake, _ := read.Node(id(4))
	seconds, err := readSay.Seconds()
	if err != nil || seconds != 6 || !readSay.UsesSeconds() || readBox.UsesSeconds() || !readShake.UsesSeconds() {
		t.Fatalf("a Say for 6 seconds uses them, a box closed by the player doesn't, a shake does: %d %v", seconds, err)
	}
	if away, err := readBox.Choice(workflow.ParamAway); err != nil || away != workflow.AwayClosed {
		t.Fatalf("the box waits to be closed: %q %v", away, err)
	}
	tooLong, err := shake.WithParam(workflow.ParamSeconds, workflow.MaxSeconds+1)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.CheckParams(tooLong); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("a shake of more than MaxSeconds is refused: %v", err)
	}
}
