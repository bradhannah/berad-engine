package browser_test

import (
	"encoding/json"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestMakingEachKind makes a zone loop from New ▾ and an NPC behaviour from
// the Workflows section's [+], then finds the zone loop's row marked as one
// and counting the zones it is the loop of.
func TestMakingEachKind(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:new", false)
	f.click("menu:new-zone-loop", false)
	f.click("ui:new-workflow", false)
	creates := f.requests.Of(project.CreateEdit)
	if len(creates) != 2 {
		t.Fatalf("two workflows made: %d", len(creates))
	}
	for index, want := range []struct {
		name    string
		purpose flow.Purpose
	}{{"Zone loop 1", flow.PurposeZoneLoop}, {"Workflow 1", flow.PurposeBehaviour}} {
		var args project.DocumentArgs
		var data struct {
			Purpose flow.Purpose `json:"purpose"`
		}
		if creates[index].Decode(&args) != nil || json.Unmarshal(args.Data, &data) != nil || args.Name != want.name ||
			args.Format != flow.FormatName || data.Purpose != want.purpose {
			t.Fatalf("made %q as %q, wanting %+v", args.Name, data.Purpose, want)
		}
	}
	loopID := frametest.ID(40)
	loop, err := flow.NewFor(frametest.ID(41), flow.PurposeZoneLoop)
	if err != nil {
		t.Fatal(err)
	}
	f.docs().Put(loopID, "Market day", &workflowplugin.Document{Workflow: loop})
	town := zoneplugin.NewDocument()
	if err := town.Zone.Insert(0, floorID); err != nil {
		t.Fatal(err)
	}
	town.Zone.Loop = loopID
	f.docs().Put(zoneID, "Town", town)
	f.docs().Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone", Folder: "", Tags: nil, Kind: ""},
		{ID: dayID, Name: "Clerk day", Format: flow.FormatName, Folder: "", Tags: nil, Kind: string(flow.PurposeBehaviour)},
		{ID: loopID, Name: "Market day", Format: flow.FormatName, Folder: "", Tags: nil, Kind: string(flow.PurposeZoneLoop)},
	}))
	f.press("Shift")
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	if !f.has("row-loop:Market day") || !f.has("row-detail:Market day=loop of 1 zone") || f.has("row-loop:Clerk day") {
		t.Fatal("a zone loop's row shows its icon and the zones it is the loop of")
	}
}
