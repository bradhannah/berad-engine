package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
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
	if !f.has("local=value:number:0") {
		t.Fatalf("a new local is a number named value: %+v", f.workflow().Locals().Values())
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
