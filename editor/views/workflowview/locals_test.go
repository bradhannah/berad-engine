package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

func (f *fixture) typeText(text string) {
	f.t.Helper()
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
}

// TestLocalValues declares a local value with nothing selected, names it,
// makes it an NPC and deletes it, each a change to the workflow.
func TestLocalValues(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("Escape")
	f.click("ui:add-local")
	if !f.has("local=value:integer:0") {
		t.Fatalf("a new local is an integer named value: %+v", f.workflow().Locals().Values())
	}
	f.click("ui:value-name")
	f.keys("Cmd+A", "Backspace")
	f.typeText("customer")
	f.keys("Enter")
	f.click("ui:value-type-npc")
	if !f.has("local=customer:npc:none") {
		t.Fatalf("named and made an NPC: %+v", f.workflow().Locals().Values())
	}
	values := f.workflow().Locals().Values()
	if len(values) != 1 || values[0].Scope != worldstate.ScopeLocal {
		t.Fatalf("locals %+v", values)
	}
	f.click("ui:value-delete")
	if f.workflow().Locals().Len() != 0 {
		t.Fatal("deleted")
	}
}

// renameLocalUnsaved adds a local, saves nothing more, and types a new name
// for it without saving.
func (f *fixture) renameLocalUnsaved(name string) {
	f.t.Helper()
	f.keys("Escape")
	f.click("ui:add-local")
	f.click("ui:value-name")
	f.keys("Cmd+A", "Backspace")
	f.typeText(name)
	if f.state.Unsaved(f.docs) != 1 {
		f.t.Fatalf("a typed name waits for Save: %d unsaved", f.state.Unsaved(f.docs))
	}
}

// guardAsks reports whether the Save, Don't save, Cancel question is up.
func (f *fixture) guardAsks() bool {
	_, asking := f.state.Locate("ui:"+frame.GuardSave, f.docs)

	return asking
}

// TestAnUnsavedLocalIsAskedAbout types a new name for a local value, then
// selects a node: the question is asked first, as for a node's own unsaved
// changes. Cancel stays on the local with its name still typed; Don't save
// drops the name and goes to the node; Save saves it and goes to the node.
// A click on empty canvas selects nothing new, and the typed name stays.
func TestAnUnsavedLocalIsAskedAbout(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.renameLocalUnsaved("customer")
	f.click("node:start:1")
	if !f.guardAsks() {
		t.Fatalf("selecting a node asks about the local's typed name: %d unsaved", f.state.Unsaved(f.docs))
	}
	f.click("ui:" + ui.DialogCancel)
	if f.guardAsks() || f.state.Unsaved(f.docs) != 1 || f.has("selected:start") {
		t.Fatalf("Cancel stays on the local, its name still typed: %d unsaved", f.state.Unsaved(f.docs))
	}
	f.click("canvas:700,600")
	if f.guardAsks() || f.state.Unsaved(f.docs) != 1 {
		t.Fatalf("a click on empty canvas keeps the typed name: %d unsaved", f.state.Unsaved(f.docs))
	}
	f.click("node:start:1")
	f.click("ui:" + frame.GuardDiscard)
	if !f.has("selected:start") || f.state.Unsaved(f.docs) != 0 || !f.has("local=value:integer:0") {
		t.Fatalf("Don't save goes to the node and keeps the old name: %+v", f.workflow().Locals().Values())
	}
	f.keys("Escape")
	if f.state.Unsaved(f.docs) != 0 {
		t.Fatal("the dropped name does not come back with the local")
	}

	f.click("ui:value-name")
	f.keys("Cmd+A", "Backspace")
	f.typeText("customer")
	f.click("node:start:1")
	f.click("ui:" + frame.GuardSave)
	if !f.has("selected:start") || !f.has("local=customer:integer:0") {
		t.Fatalf("Save names the local and goes to the node: %+v", f.workflow().Locals().Values())
	}

	f.keys("Escape")
	f.click("ui:value-name")
	f.keys("Cmd+A", "Backspace")
	f.typeText("shopper")
	f.click("ui:add-local")
	if !f.guardAsks() || f.workflow().Locals().Len() != 1 {
		t.Fatalf("Add local asks about the typed name first: %d locals", f.workflow().Locals().Len())
	}
	f.click("ui:" + frame.GuardDiscard)
	if f.workflow().Locals().Len() != 2 || !f.has("local=customer:integer:0") {
		t.Fatalf("Don't save adds the local and keeps the old name: %+v", f.workflow().Locals().Values())
	}
}
