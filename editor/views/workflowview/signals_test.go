package workflowview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// withSignals declares bread_called and flour_low.
func (f *fixture) withSignals() {
	f.docs.View(worldstate.ID(), func(document client.Document) {
		document.Model.(*stateplugin.Document).State.Signals = []worldstate.Signal{
			{ID: frametest.ID(50), Name: "bread_called"}, {ID: frametest.ID(51), Name: "flour_low"},
		}
	})
	f.docs.Touch(worldstate.ID())
}

// chooseSignal chooses the selected node's signal in the signal picker.
func (f *fixture) chooseSignal(path string) {
	f.t.Helper()
	f.click("ui:pick-signal")
	f.click("signalrow:" + path)
}

// TestTheSignalPicker opens the picker from a Send signal's box, finds a
// signal by part of its name, and chooses it with Enter; opened again, the
// signal this workflow sends now comes first.
func TestTheSignalPicker(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withSignals()
	f.click("node:start:1")
	f.add(flow.KindSendSignal)
	f.click("ui:pick-signal")
	if !f.has("signal-picker") {
		t.Fatal("the box opens the picker")
	}
	f.typeText("flour")
	if _, found := f.state.Locate("signalrow:bread_called", f.ctx.Docs); found {
		t.Fatal("typing flour leaves bread_called out")
	}
	f.keys("Enter")
	if chosen, ok := f.nth(flow.KindSendSignal, 1).Signal(); f.has("signal-picker") || !ok || chosen != frametest.ID(51) {
		t.Fatalf("Enter chooses flour_low and closes the picker: %v", f.nth(flow.KindSendSignal, 1).Params)
	}
	f.click("ui:pick-signal")
	if _, found := f.state.Locate("signalrow:flour_low", f.ctx.Docs); !found || !f.has("signal-picker") {
		t.Fatal("the picker opens again, listing every signal")
	}
	f.keys("Escape")
	if f.has("signal-picker") {
		t.Fatal("Escape closes it")
	}
}

// TestSignalProblems sends bread_called to Ada, whose behaviour is this
// workflow: a broken signal call until it has an On signal for it, which a
// Resume ends; a Resume off any receiver's path is a problem, and a
// receiver nothing sends to a warning.
func TestSignalProblems(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withSignals()
	f.click("node:start:1")
	f.add(flow.KindSendSignal)
	if !f.has("problem:" + flow.KindSendSignal + ":1=Choose the signal.") {
		t.Fatal("a new Send signal has no signal")
	}
	f.chooseSignal("bread_called")
	f.click("ui:npc-Ada Clerk")
	broken := "Broken signal call: Ada Clerk's behaviour doesn't answer bread_called. Give it an On signal for it."
	if !f.has("problem:" + flow.KindSendSignal + ":1=" + broken) {
		t.Fatal("Ada doesn't answer it yet")
	}
	if !f.has("title:" + flow.KindSendSignal + ":1=Send signal · bread_called → Ada Clerk") {
		t.Fatal("the node names its signal and NPC")
	}
	f.add(flow.KindOnSignal)
	f.chooseSignal("bread_called")
	f.add(flow.KindResume)
	if f.has("problem:"+flow.KindSendSignal+":1") || f.has("problem:"+flow.KindOnSignal+":1") ||
		f.has("problem:"+flow.KindResume+":1") || f.has("warning:"+flow.KindOnSignal+":1") {
		t.Fatal("Ada answers it now, and the receiver is sent to")
	}
	receiver := f.nth(flow.KindOnSignal, 1)
	if resume := f.nth(flow.KindResume, 1); receiver.Next[flow.OutputNext] != resume.ID {
		t.Fatalf("Add after puts the Resume on the receiver's path: %v", receiver.Next)
	}
	f.add(flow.KindOnSignal)
	f.chooseSignal("flour_low")
	if !f.has("warning:" + flow.KindOnSignal + ":2=Nothing sends flour_low yet, so this receiver never runs.") {
		t.Fatal("a receiver nothing sends to is warned of")
	}
	f.click("node:start:1")
	f.add(flow.KindResume)
	lone := "Resume goes back to where a signal interrupted the NPC, so it belongs on a receiver's path."
	if !f.has("problem:" + flow.KindResume + ":2=" + lone) {
		t.Fatal("a Resume off any receiver's path is a problem")
	}
}

// TestReceiversArrangeUnderTheFlow lays a receiver's path out in a row of
// its own, under the flow from the start, starting at the left.
func TestReceiversArrangeUnderTheFlow(t *testing.T) {
	t.Parallel()
	workflow, err := flow.New(frametest.ID(1))
	if err != nil {
		t.Fatal(err)
	}
	made := func(seed byte, kind string) flow.Node {
		node, err := flow.NewNode(kind)
		if err != nil {
			t.Fatal(err)
		}
		node.ID = frametest.ID(seed)

		return node
	}
	if err := workflow.Insert([]flow.Node{
		made(2, flow.KindWait), made(3, flow.KindWait), made(4, flow.KindOnSignal), made(5, flow.KindResume),
	}, nil, []flow.Link{
		{From: frametest.ID(1), Output: flow.OutputNext, To: frametest.ID(2)},
		{From: frametest.ID(2), Output: flow.OutputNext, To: frametest.ID(3)},
		{From: frametest.ID(4), Output: flow.OutputNext, To: frametest.ID(5)},
	}); err != nil {
		t.Fatal(err)
	}
	boxes := workflowview.Arrange(workflow, func(flow.Node, int) int { return 46 })
	start, last, receiver := boxes[frametest.ID(1)], boxes[frametest.ID(3)], boxes[frametest.ID(4)]
	resume := boxes[frametest.ID(5)]
	if receiver.X != start.X || receiver.Y <= last.Y || resume.Y <= last.Y || resume.X <= receiver.X {
		t.Fatalf("the receiver's path is a row under the flow: %+v %+v %+v", start, last, receiver)
	}
}
