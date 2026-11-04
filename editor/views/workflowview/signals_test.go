package workflowview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
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
	f.click("ui:signal-bread_called")
	f.click("ui:npc-Ada Clerk")
	broken := "Broken signal call: Ada Clerk's behaviour doesn't answer bread_called. Give it an On signal for it."
	if !f.has("problem:" + flow.KindSendSignal + ":1=" + broken) {
		t.Fatal("Ada doesn't answer it yet")
	}
	if !f.has("title:" + flow.KindSendSignal + ":1=Send signal · bread_called → Ada Clerk") {
		t.Fatal("the node names its signal and NPC")
	}
	f.add(flow.KindOnSignal)
	f.click("ui:signal-bread_called")
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
	f.click("ui:signal-flour_low")
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
