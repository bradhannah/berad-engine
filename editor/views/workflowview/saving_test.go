package workflowview_test

import (
	"image"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// typeOnly types into a field without saving.
func (f *fixture) typeOnly(field, text string) {
	f.t.Helper()
	f.click("ui:" + field)
	f.keys("Cmd+A")
	input := ui.NewInput(f.at("ui:" + field))
	input.Text = text
	f.frame(input)
}

// TestOneSaveForSeveralFields types an option's name and share, then saves
// both with one click: one command, so one undo step. A draft survives the
// form being rebuilt when an option is added.
func TestOneSaveForSeveralFields(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindRandom)
	before := len(f.requests.Of(workflowplugin.ReplaceEdit))
	f.typeOnly("options-1-label", "left")
	f.typeOnly("options-1-share", "3")
	if f.state.Unsaved(f.docs) != 2 {
		t.Fatalf("unsaved %d", f.state.Unsaved(f.docs))
	}
	f.click("ui:save")
	if replaces := len(f.requests.Of(workflowplugin.ReplaceEdit)); replaces != before+1 {
		t.Fatalf("one Save sends one command, not %d", replaces-before)
	}
	options, _ := f.nth(flow.KindRandom, 1).Shares("options")
	if options[0].Label != "left" || options[0].Share != 3 {
		t.Fatalf("options %+v", options)
	}
	f.typeOnly("options-2-label", "right")
	f.click("ui:add-option")
	if f.state.Unsaved(f.docs) != 1 {
		t.Fatal("adding an option keeps the typed name")
	}
	f.state.Save(f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	options, _ = f.nth(flow.KindRandom, 1).Shares("options")
	if len(options) != 3 || options[1].Label != "right" {
		t.Fatalf("options after adding one %+v", options)
	}
}

// TestABadFieldSavesNothing saves a good share beside a share that is not a
// number: nothing is sent, and the status says why.
func TestABadFieldSavesNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindRandom)
	before := len(f.requests.Of(workflowplugin.ReplaceEdit))
	f.typeOnly("options-1-share", "2")
	f.typeOnly("options-2-share", "lots")
	if f.state.Save(f.ctx) || len(f.requests.Of(workflowplugin.ReplaceEdit)) != before || f.status() == "" {
		t.Fatalf("a bad field stops the save: %q", f.status())
	}
}

// TestSelectingAnotherNodeAsks types into a node, then clicks another:
// Cancel stays, and Don't save moves on without saving.
func TestSelectingAnotherNodeAsks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindWait)
	before := len(f.requests.Of(workflowplugin.ReplaceEdit))
	f.typeOnly("param-duration", "9")
	f.click("node:start:1")
	f.click("ui:cancel")
	if !f.has("selected:"+flow.KindWait) || f.state.Unsaved(f.docs) != 1 {
		t.Fatal("Cancel keeps the Wait selected with its change")
	}
	f.click("node:start:1")
	f.click("ui:discard-changes")
	if !f.has("selected:"+flow.KindStart) || len(f.requests.Of(workflowplugin.ReplaceEdit)) != before {
		t.Fatal("Don't save selects the start and saves nothing")
	}
}
