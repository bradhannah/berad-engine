package workflowview_test

import (
	"fmt"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestTogetherLanes adds a Together after the start, with its two lanes;
// adds a third and renames the second; puts a Wait in the first lane and a
// Play sound in the next empty one; and removes the first lane with its
// Wait, in one undo step, until two lanes are left, which stay.
func TestTogetherLanes(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindTogether)
	if !f.has("lanes:" + flow.KindTogether + ":1=2") {
		t.Fatal("a new Together has two lanes")
	}
	f.click("ui:add-lane")
	f.typeInto("lanes-2-label", "Shake")
	lanes, err := f.nth(flow.KindTogether, 1).Lanes()
	if err != nil || len(lanes) != 3 || lanes[1].Label != "Shake" || lanes[2].Label != "Lane 3" {
		t.Fatalf("three lanes, the second renamed: %+v %v", lanes, err)
	}
	f.add(flow.KindWait)
	f.click("node:" + flow.KindTogether + ":1")
	f.add(flow.KindPlaySound)
	together, wait, sound := f.nth(flow.KindTogether, 1), f.nth(flow.KindWait, 1), f.nth(flow.KindPlaySound, 1)
	if together.Next["l1"] != wait.ID || together.Next["l2"] != sound.ID || wait.Parent != together.ID {
		t.Fatalf("each goes in the next empty lane: %v", together.Next)
	}
	if !f.has("title:" + flow.KindPlaySound + ":1=Play sound · no cue") {
		t.Fatal("the sound is drawn in its lane")
	}
	f.click("node:" + flow.KindTogether + ":1")
	commands := len(f.requests.Commands)
	f.click("ui:lanes-1-remove")
	if len(f.workflow().Nodes()) != 3 || !f.has("lanes:"+flow.KindTogether+":1=2") {
		t.Fatalf("the first lane goes with its Wait: %d nodes", len(f.workflow().Nodes()))
	}
	removal := f.requests.Commands[commands:]
	if len(removal) != 2 || removal[0].Command.Gesture == "" || removal[0].Command.Gesture != removal[1].Command.Gesture {
		t.Fatalf("removing a lane is one undo step: %+v", removal)
	}
	f.click("ui:lanes-1-remove")
	if !f.has("lanes:" + flow.KindTogether + ":1=2") {
		t.Fatal("a Together keeps two lanes")
	}
}

// TestDroppingIntoALane drags a Wait from the palette into a Together's
// empty second lane, where it is the lane's first step, and draws where a
// lane's last step goes: lane done.
func TestDroppingIntoALane(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindTogether)
	f.drag(f.at("palette:"+flow.KindWait), f.at("lane:2"))
	together, wait := f.nth(flow.KindTogether, 1), f.nth(flow.KindWait, 1)
	if together.Next["l2"] != wait.ID || wait.Parent != together.ID {
		t.Fatalf("dropped in the second lane, it is its first step: %v", together.Next)
	}
}

// TestLaneFlags flags a Call in a lane, a Wait two lanes reach, and a Go to
// in the second lane when a Move in the first already moves the NPC.
func TestLaneFlags(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	together, err := flow.NewNode(flow.KindTogether)
	if err != nil {
		t.Fatal(err)
	}
	together.ID = frametest.ID(80)
	child := func(seed byte, kind string) flow.Node {
		made, err := flow.NewNode(kind)
		if err != nil {
			t.Fatal(err)
		}
		made.ID, made.Parent = frametest.ID(seed), together.ID

		return made
	}
	move, goTo, wait, call := child(81, flow.KindMove), child(82, flow.KindGoTo), child(83, flow.KindWait),
		child(84, flow.KindCall)
	f.docs.View(workflowID, func(document client.Document) {
		routine := document.Model.(*workflowplugin.Document).Workflow
		if err := routine.Insert([]flow.Node{together, move, goTo, wait, call}, nil, []flow.Link{
			{From: startID, Output: flow.OutputNext, To: together.ID}, {From: together.ID, Output: "l1", To: move.ID},
			{From: together.ID, Output: "l2", To: goTo.ID}, {From: move.ID, Output: flow.OutputNext, To: wait.ID},
			{From: goTo.ID, Output: flow.OutputNext, To: wait.ID}, {From: wait.ID, Output: flow.OutputNext, To: call.ID},
		}); err != nil {
			t.Fatal(err)
		}
	})
	f.docs.Touch(workflowID)
	f.at("node:" + flow.KindTogether + ":1")
	for kind, problem := range map[string]string{
		flow.KindGoTo: fmt.Sprintf("Only one lane can move the NPC, and %s already does. Move it there, or out of "+
			"the Together.", "Lane 1"),
		flow.KindWait: "It is in two lanes, Lane 1 and Lane 2: give each lane steps of its own.",
		flow.KindCall: "A lane holds steps in a line: a Call or a Return can't go in one yet. Put it before or " +
			"after the Together.",
	} {
		if !f.has("problem:" + kind + ":1=" + problem) {
			t.Fatalf("the %s is flagged", kind)
		}
	}
	if f.has("problem:" + flow.KindMove + ":1") {
		t.Fatal("the first lane's Move is fine")
	}
}
