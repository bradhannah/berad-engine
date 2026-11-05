package stateview_test

import (
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
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
	f := signalsFixture(t)
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

// TestSignalGroups adds a group of signals from the SIGNALS heading's
// menu, which offers no template, and a signal inside it from the group's;
// moves a signal into the group by its menu and out again by dragging it
// onto the heading; refuses a move beside another of its name; and deletes
// the group with what it holds.
func TestSignalGroups(t *testing.T) {
	t.Parallel()
	f := signalsFixture(t)
	f.rightClick("section:signals")
	if _, template := f.state.Locate("menu:add-template", f.ctx.Docs); template {
		t.Fatal("the SIGNALS heading's menu adds signals, not templates")
	}
	f.click("menu:add-signal-group")
	f.typeInto("signal-name", "shop")
	f.addBy("signal:shop", "add-signal", "")
	f.typeInto("signal-name", "bell_rung")
	if !f.has("selected=shop.bell_rung") || !f.has("adds=adds in shop") {
		t.Fatalf("a signal added inside its group, named by its path: %s", f.status())
	}
	f.addBy("section:signals", "add-signal", "")
	f.typeInto("signal-name", "door_open")
	f.moveBy("door_open", "into:shop")
	if !f.has("signals=2") || !f.located("signal:shop.door_open") {
		t.Fatalf("moved into the group: %+v", f.signals())
	}
	f.drag("signal:shop.door_open", "section:signals", 2)
	if !f.located("signal:door_open") {
		t.Fatalf("dragged back to the top: %+v", f.signals())
	}
	f.addBy("section:signals", "add-signal", "")
	f.typeInto("signal-name", "bell_rung")
	f.moveBy("bell_rung", "into:shop")
	if !strings.Contains(f.status(), "There is a bell_rung in shop already") || !f.located("signal:bell_rung") {
		t.Fatalf("a move beside another of its name is refused: %s", f.status())
	}
	f.click("signal:shop")
	f.click("ui:delete-signal")
	if !f.has("signals=2") || f.located("signal:shop") {
		t.Fatalf("the group goes with what it holds: %+v", f.signals())
	}
}

// located reports whether a named part of the window can be found.
func (f *fixture) located(name string) bool {
	_, found := f.state.Locate(name, f.ctx.Docs)

	return found
}

// TestValuesAndSignalsAreApart shows the values in the Values window and
// the signals in the Signals window, each with its own adds, and only
// Values has Find a value.
func TestValuesAndSignalsAreApart(t *testing.T) {
	t.Parallel()
	values, signals := newFixture(t), signalsFixture(t)
	for _, found := range []struct {
		f     *fixture
		name  string
		shown bool
	}{
		{values, "section:world", true}, {values, "section:signals", false}, {values, "ui:add-value", true},
		{values, "ui:add-signal", false}, {values, "ui:find-value", true},
		{signals, "section:signals", true}, {signals, "section:world", false}, {signals, "ui:add-signal", true},
		{signals, "ui:add-value", false}, {signals, "ui:find-value", false},
	} {
		if _, ok := found.f.state.Locate(found.name, found.f.ctx.Docs); ok != found.shown {
			t.Errorf("%s in %s: located %v", found.name, found.f.state.Title(found.f.ctx.Docs), ok)
		}
	}
	if values.state.Title(values.ctx.Docs) != "World state: Values" ||
		signals.state.Title(signals.ctx.Docs) != "World state: Signals" {
		t.Fatal("each window's title names its part")
	}
}

// TestTheInspectorStandsBesideTheList shows a selected signal's inspector
// right after the list on a wide window, and at the right edge, past the
// window, while the inspector is folded.
func TestTheInspectorStandsBesideTheList(t *testing.T) {
	t.Parallel()
	f := signalsFixture(t)
	f.click("ui:add-signal")
	beside := f.at("ui:signal-name")
	f.ctx.InspectorFolded = true
	f.frame(ui.NewInput(image.Pt(0, 0)))
	folded := f.at("ui:signal-name")
	if beside.X > f.ctx.Size.X/2+frame.InspectorWidth || folded.X < f.ctx.Size.X-frame.InspectorWidth {
		t.Fatalf("the name field is beside the list at %v, and at the edge when folded at %v", beside, folded)
	}
}
