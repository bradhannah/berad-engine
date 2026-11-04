package workflow_test

import (
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/workflow"
)

// TestTheSignalNodes saves and reads a receiver, a Send signal and a
// Resume; finds the receivers by signal, the first of two for one; and
// refuses a line into a receiver or a receiver inside a container.
func TestTheSignalNodes(t *testing.T) {
	t.Parallel()
	receiver, err := node(t, 2, workflow.KindOnSignal, id(0)).WithParam(workflow.ParamSignal, id(40).String())
	if err != nil {
		t.Fatal(err)
	}
	second, err := node(t, 5, workflow.KindOnSignal, id(0)).WithParam(workflow.ParamSignal, id(40).String())
	if err != nil {
		t.Fatal(err)
	}
	send := node(t, 3, workflow.KindSendSignal, id(0))
	for name, value := range map[string]any{
		workflow.ParamSignal: id(41).String(), workflow.ParamSendTo: workflow.ToNPC, workflow.ParamNPC: id(42).String(),
	} {
		if send, err = send.WithParam(name, value); err != nil {
			t.Fatal(err)
		}
	}
	made, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := made.Insert([]workflow.Node{receiver, send, node(t, 4, workflow.KindResume, id(0)), second}, nil,
		[]workflow.Link{{From: id(2), Output: workflow.OutputNext, To: id(3)}, {From: id(3), Output: workflow.OutputNext,
			To: id(4)}}); err != nil {
		t.Fatal(err)
	}
	data, err := made.Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := workflow.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	sent, _ := read.Node(id(3))
	signal, chosen := sent.Signal()
	npc, named := sent.Addressee()
	if !chosen || signal != id(41) || !named || npc != id(42) {
		t.Fatalf("the Send names its signal and NPC: %v %v", signal, npc)
	}
	if receivers := read.Receivers(); len(receivers) != 1 || receivers[id(40)] != id(2) {
		t.Fatalf("the first receiver answers its signal: %v", receivers)
	}
	if reach := read.Reach(id(2)); !reach[id(4)] || reach[id(1)] {
		t.Fatalf("the receiver's path reaches the Resume, not the start: %v", reach)
	}
	if err := read.Insert(nil, nil, []workflow.Link{{From: id(1), Output: workflow.OutputNext, To: id(2)}}); !errors.Is(
		err, workflow.ErrNode) {
		t.Fatalf("no line leads into a receiver: %v", err)
	}
	if err := read.Insert([]workflow.Node{
		node(t, 6, workflow.KindContainer, id(0)), node(t, 7, workflow.KindOnSignal, id(6)),
	}, nil, nil); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("a receiver sits at the top level: %v", err)
	}
	bad, err := receiver.WithParam(workflow.ParamSignal, "not an identity")
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.CheckParams(bad); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("a signal is an identity: %v", err)
	}
}

func TestASignalGoesNoFurtherThanThisZone(t *testing.T) {
	t.Parallel()
	send := node(t, 3, workflow.KindSendSignal, id(0))
	for _, to := range []string{workflow.ToNPC, workflow.ToLocal, workflow.ToZone} {
		reached, err := send.WithParam(workflow.ParamSendTo, to)
		if err != nil {
			t.Fatal(err)
		}
		if err := workflow.CheckParams(reached); err != nil {
			t.Fatalf("a signal can go to %s: %v", to, err)
		}
	}
	everyone, err := send.WithParam(workflow.ParamSendTo, "everyone")
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.CheckParams(everyone); !errors.Is(err, workflow.ErrNode) {
		t.Fatalf("only the zone running hears a signal, so it goes to no one beyond it: %v", err)
	}
}
