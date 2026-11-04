package stateview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/npc"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// signals reads the world state's signals.
func (f *fixture) signals() []worldstate.Signal {
	var declared []worldstate.Signal
	f.docs.View(worldstate.ID(), func(document client.Document) {
		declared = document.Model.(*stateplugin.Document).State.Signals
	})

	return declared
}

// withSender gives the project the baker's day, which sends a signal.
func (f *fixture) withSender(signal worldstate.Signal) {
	f.t.Helper()
	day, err := flow.New(frametest.ID(70))
	if err != nil {
		f.t.Fatal(err)
	}
	send, err := flow.NewNode(flow.KindSendSignal)
	if err == nil {
		send, err = send.WithParam(flow.ParamSignal, signal.ID.String())
	}
	if err != nil {
		f.t.Fatal(err)
	}
	send.ID = frametest.ID(71)
	if err := day.Insert([]flow.Node{send}, nil, nil); err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(frametest.ID(72), "Baker's day", &workflowplugin.Document{Workflow: day})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: georgeID, Name: "George", Format: npc.FormatName}, {ID: guardID, Name: "Guard 1", Format: npc.FormatName},
		{ID: marketID, Name: "Market", Format: zone.FormatName},
		{ID: frametest.ID(72), Name: "Baker's day", Format: flow.FormatName},
	}))
}

// TestSignalsAreAddedRenamedAndDeleted adds a signal under SIGNALS, names
// it, and deletes it: once it is used, only after asking, listing where.
func TestSignalsAreAddedRenamedAndDeleted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:add-signal")
	if !f.has("signals=1") || !f.has("selected=signal") {
		t.Fatal("Add signal declares one and selects it")
	}
	f.typeInto("signal-name", "bread_called")
	declared := f.signals()
	if len(declared) != 1 || declared[0].Name != "bread_called" {
		t.Fatalf("renamed: %+v", declared)
	}
	f.withSender(declared[0])
	f.click("signal:bread_called")
	f.click("ui:delete-signal")
	if !f.has("says=Delete bread_called?") || !f.has("says=Baker's day · Send signal") {
		t.Fatal("deleting a signal in use asks first, saying where it is used")
	}
	f.click("dialog:delete")
	if !f.has("signals=0") {
		t.Fatalf("deleted: %+v", f.signals())
	}
	f.click("ui:add-signal")
	f.click("ui:delete-signal")
	if !f.has("signals=0") {
		t.Fatal("an unused signal is deleted at once")
	}
}
