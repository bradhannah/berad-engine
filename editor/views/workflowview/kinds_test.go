package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// asZoneLoop makes the fixture's workflow a zone loop, as if it had been
// made as one.
func (f *fixture) asZoneLoop() {
	f.t.Helper()
	loop, err := flow.NewFor(startID, flow.PurposeZoneLoop)
	if err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(workflowID, "Market day", &workflowplugin.Document{Workflow: loop})
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestEachKindsPalette offers an NPC behaviour every node, and a zone loop
// none that moves a body: no Go to, Move or Find NPC nearby.
func TestEachKindsPalette(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, kind := range []string{flow.KindGoTo, flow.KindMove, flow.KindFindNPC, flow.KindWait} {
		if _, found := f.state.Locate("palette:"+kind, f.ctx.Docs); !found {
			t.Fatalf("an NPC behaviour offers %s", kind)
		}
	}
	f.asZoneLoop()
	var purpose flow.Purpose
	f.docs.View(workflowID, func(document client.Document) {
		purpose = document.Model.(*workflowplugin.Document).Workflow.Purpose()
	})
	if purpose != flow.PurposeZoneLoop {
		t.Fatal("the fixture's workflow is a zone loop now")
	}
	for _, kind := range []string{flow.KindGoTo, flow.KindMove, flow.KindFindNPC} {
		if _, found := f.state.Locate("palette:"+kind, f.ctx.Docs); found {
			t.Fatalf("a zone loop offers no %s", kind)
		}
	}
	if _, found := f.state.Locate("palette:"+flow.KindWait, f.ctx.Docs); !found {
		t.Fatal("a zone loop waits")
	}
}
