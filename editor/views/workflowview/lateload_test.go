package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestAModuleLoadedLate shows the ways out of a module chosen before it has
// loaded, once it loads.
func TestAModuleLoadedLate(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: workflowID, Name: "Daily routine", Format: flow.FormatName},
		{ID: moduleID, Name: "Go to work", Format: flow.FormatName},
		{ID: floorID, Name: "Office", Format: floor.FormatName},
		{ID: adaID, Name: "Ada Clerk", Format: npc.FormatName},
	}))
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.keys("Escape")
	f.click("palette:" + flow.KindCall)
	f.click("ui:call-workflow")
	f.click("menu:workflow-named:Go to work")
	f.withModule()
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("output:call-workflow:1:w2=Closed") {
		t.Fatal("its ways out show once it loads")
	}
}
