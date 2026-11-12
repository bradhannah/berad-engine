package browser_test

import (
	"encoding/json"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
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

// withKinds gives the fixture a behaviour running in Town, one running in
// any zone, one running in Harbour and a zone loop running in Town, with
// Ada Clerk living in Town.
func (f *fixture) withKinds() map[string]ids.ULID {
	f.t.Helper()
	made := map[string]ids.ULID{
		"Clerk day": dayID, "Roam": frametest.ID(50), "Harbour patrol": frametest.ID(51), "Market day": frametest.ID(52),
		"Harbour": frametest.ID(53),
	}
	entries := []project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone", Folder: "", Tags: nil, Kind: ""},
		{ID: made["Harbour"], Name: "Harbour", Format: "zone", Folder: "", Tags: nil, Kind: ""},
		{ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName, Folder: "", Tags: nil, Kind: ""},
	}
	for name, runs := range map[string]ids.ULID{
		"Clerk day": zoneID, "Roam": {}, "Harbour patrol": made["Harbour"], "Market day": zoneID,
	} {
		purpose := flow.PurposeBehaviour
		if name == "Market day" {
			purpose = flow.PurposeZoneLoop
		}
		workflow, err := flow.NewFor(frametest.ID(60), purpose)
		if err != nil {
			f.t.Fatal(err)
		}
		workflow.SetRunsIn(runs)
		f.docs().Put(made[name], name, &workflowplugin.Document{Workflow: workflow})
		entries = append(entries, project.Entry{
			ID: made[name], Name: name, Format: flow.FormatName, Folder: "", Tags: nil, Kind: string(purpose),
		})
	}
	f.docs().Put(made["Harbour"], "Harbour", zoneplugin.NewDocument())
	clerk := npcplugin.NewDocument()
	clerk.NPC.Zone = zoneID
	f.docs().Put(clerkID, "Ada Clerk", clerk)
	f.docs().Put(project.IndexID(), "Project", project.NewIndex(entries))
	f.press("Shift")

	return made
}

// TestChoosingWhatFits lists an NPC's behaviours by where they run, refuses
// one running where it does not live, and offers a zone the zone loops
// that run there.
func TestChoosingWhatFits(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	made := f.withKinds()
	f.click("row:Ada Clerk", false)
	f.click("ui:choose-behaviour", false)
	for _, row := range []string{
		"Run in Town, where Ada Clerk lives", "Clerk day", "Run in any zone", "Roam",
		"Run in other zones: Ada Clerk can't follow them", "Harbour patrol",
	} {
		if !f.has("dialog-row=" + row) {
			t.Fatalf("the behaviours list %q", row)
		}
	}
	if f.has("dialog-row=Market day") {
		t.Fatal("a zone loop is no NPC's behaviour")
	}
	f.click("row:Harbour patrol", false)
	f.click("ui:use", false)
	if got := f.status(); len(f.requests.Of(npcplugin.SetBehaviourEdit)) != 0 ||
		got != "Refused: Harbour patrol runs in Harbour, and Ada Clerk lives in Town. "+
			"Choose one that runs there or in any zone." {
		t.Fatalf("a behaviour running elsewhere is refused, saying why: %q", got)
	}
	f.click("row:Town", false)
	f.click("ui:choose-zone-loop", false)
	if !f.has("dialog-row=Run in Town") || !f.has("dialog-row=Market day") || f.has("dialog-row=Roam") {
		t.Fatal("a zone is offered the zone loops that run there")
	}
	f.click("row:Market day", false)
	f.click("ui:use", false)
	set := f.requests.Of(zoneplugin.SetLoopEdit)
	var args zoneplugin.SetLoopArgs
	if len(set) != 1 || set[0].Decode(&args) != nil || args.Loop != made["Market day"] ||
		set[0].Command.Document != zoneID {
		t.Fatalf("Town runs Market day: %+v", set)
	}
}
