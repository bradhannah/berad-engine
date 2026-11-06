package browser_test

import (
	"encoding/json"
	"image"
	"slices"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	viewsplugin "github.com/bradhannah/berad-engine/editor/plugins/views"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// withHistory dates the fixture's things: Clerk day changed an hour ago,
// Ada Clerk two days ago, and Town opened just now; Clerk day is in the
// folder Shop.
func withHistory(f *fixture) time.Time {
	now := time.Now()
	docs := f.ctx.Docs.(*frametest.Docs)
	docs.Put(project.IndexID(), "Project", project.NewIndexWithFolders([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"}, {ID: floorID, Name: "Ground", Format: floor.FormatName},
		{ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName, Changed: now.Add(-48 * time.Hour)},
		{ID: dayID, Name: "Clerk day", Format: flow.FormatName, Folder: "Shop", Changed: now.Add(-time.Hour)},
	}, []project.Folder{{Format: flow.FormatName, Path: "Shop"}}))
	opened, _ := json.Marshal(protocol.HomeOpened{Opened: []protocol.HomeOpen{{Document: zoneID, At: now}}})
	f.state.Changed(client.Update{Session: &protocol.Session{Topic: protocol.TopicHomeOpened, Value: opened}}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return now
}

// TestHomeListsWhatWasDone lists what was opened and changed lately, newest
// first, under a heading for each day, each saying where it lives. A thing
// pinned is listed first, and Show in folder shows its folder.
func TestHomeListsWhatWasDone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withHistory(f)
	if got := f.labels(); !slices.Equal(got[:3], []string{"Today", "Town", "Clerk day"}) {
		t.Fatalf("today's things, newest first: %v", got)
	}
	if day := f.rowNamed("Clerk day"); day.Aside != "Workflows / Shop" || day.Detail[:len("changed")] != "changed" {
		t.Fatalf("Clerk day says where it lives and that it changed: %+v", day)
	}
	if town := f.rowNamed("Town"); town.Detail[:len("opened")] != "opened" || town.Aside != "Zones" {
		t.Fatalf("Town says it was opened: %+v", town)
	}
	if !slices.Contains(f.labels(), "Earlier this week") || !slices.Contains(f.labels(), "Ada Clerk") {
		t.Fatalf("Ada Clerk is earlier this week: %v", f.labels())
	}
	f.rightClick("row:Ada Clerk")
	f.click("menu:context-pin", false)
	if len(f.requests.Published) != 1 || f.requests.Published[0].Topic != protocol.TopicHomeKept {
		t.Fatalf("pinning tells the orchestrator: %+v", f.requests.Published)
	}
	var kept protocol.HomeKept
	if err := json.Unmarshal(f.requests.Published[0].Value, &kept); err != nil || !slices.Equal(kept.Pins, []ids.ULID{clerkID}) {
		t.Fatalf("Ada Clerk is pinned: %+v %v", kept, err)
	}
	if got := f.labels(); !slices.Equal(got[:2], []string{"Pinned", "Ada Clerk"}) {
		t.Fatalf("a pinned thing comes first, once: %v", got)
	}
	f.rightClick("row:Clerk day")
	f.click("menu:context-reveal", false)
	if !f.has("place=Shop") {
		t.Fatal("Show in folder shows Shop")
	}
	if row, _ := f.state.SelectedRow(f.ctx.Docs); row.Label != "Clerk day" {
		t.Fatalf("Clerk day is selected in Shop: %+v", row)
	}
}

// TestHomeAppliesAView lists the saved views on Home, and a click applies
// one.
func TestHomeAppliesAView(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	docs := f.ctx.Docs.(*frametest.Docs)
	docs.Put(viewsplugin.ID(), viewsplugin.Name, &viewsplugin.Document{Views: []viewsplugin.View{
		{Name: "Workflows", Search: "", Ticks: map[string][]string{"kind": {flow.FormatName}}},
	}})
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !slices.Equal(f.labels(), []string{"Views", "Workflows"}) {
		t.Fatalf("Home lists the saved view: %v", f.labels())
	}
	f.click("row:Workflows", false)
	if !f.has("narrowed") || !f.has("view=Workflows") || !f.has("place=Matches") {
		t.Fatal("a click applies the view")
	}
}
