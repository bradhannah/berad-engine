package browser_test

import (
	"image"
	"slices"
	"testing"

	viewsplugin "github.com/bradhannah/berad-engine/editor/plugins/views"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// TestFacetsNarrowTheListToATable ticks a kind, then a folder: the list
// becomes a table of what has them, a value not ticked still counts what
// ticking it would add, and taking the ticks away brings the tree back.
func TestFacetsNarrowTheListToATable(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	if !f.has("facets") || f.has("table") {
		t.Fatal("the facets stand beside the tree")
	}
	if !f.has("count:Kind/Workflows=2") || !f.has("count:Kind/NPCs=1") || !f.has("count:Folder/Market=2") {
		t.Fatal("each value says how many things have it")
	}
	f.click("facet:Kind/Workflows", false)
	if !f.has("table") || !f.has("showing=2 of 5") || !f.has("ticked:Kind/Workflows") {
		t.Fatal("ticking Workflows shows the two workflows of five things")
	}
	if !f.has("count:Kind/NPCs=1") || !f.has("cell:Night watch/Kind=NPC behaviour") {
		t.Fatal("NPCs still counts the NPC ticking it would add, and a workflow says its kind")
	}
	f.click("facet:Folder/Market / Guards", false)
	if !f.has("showing=1 of 5") || !f.has("cell:Night watch/Folder=Market / Guards") {
		t.Fatal("ticking Market / Guards leaves Night watch")
	}
	if _, listed := f.state.Locate("facet:Tags/#market", f.ctx.Docs); listed {
		t.Fatal("no tag that would leave nothing is offered")
	}
	f.click("chip:Kind: Workflows", false)
	f.click("chip:Folder: Market / Guards", false)
	if f.has("table") {
		t.Fatal("with nothing ticked, the tree is back")
	}
}

// TestSearchingShowsATable types a search: the list becomes a table of
// what matches, Settings among them when its words match.
func TestSearchingShowsATable(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.typeInto("search", "night")
	if !f.has("table") || !f.has("showing=1 of 5") {
		t.Fatal("night finds Night watch")
	}
	f.press("Escape")
	f.press("Escape")
	f.typeInto("search", "tile")
	if _, found := f.state.Locate("row:Settings", f.ctx.Docs); !found {
		t.Fatal("tile finds the Settings")
	}
}

// TestSavingAView saves the ticks under a name, finds them a saved view,
// applies it again from the facets column, and deletes it.
func TestSavingAView(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	docs := f.ctx.Docs.(*frametest.Docs)
	docs.Put(viewsplugin.ID(), viewsplugin.Name, &viewsplugin.Document{Views: nil})
	f.click("facet:Kind/Workflows", false)
	f.click("ui:save-view", false)
	f.press("Cmd+A")
	f.typeText("All workflows")
	f.press("Enter")
	var saved viewsplugin.SetArgs
	if sets := f.requests.Of(viewsplugin.SetEdit); len(sets) != 1 || sets[0].Decode(&saved) != nil ||
		len(saved.Views) != 1 || saved.Views[0].Name != "All workflows" ||
		!slices.Equal(saved.Views[0].Ticks["kind"], []string{"workflow"}) {
		t.Fatalf("saved: %+v", saved)
	}
	docs.Put(viewsplugin.ID(), viewsplugin.Name, &viewsplugin.Document{Views: saved.Views})
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("view=All workflows") {
		t.Fatal("the ticks are the saved view's")
	}
	f.click("chip:Kind: Workflows", false)
	f.click("view:All workflows", false)
	if !f.has("showing=2 of 5") {
		t.Fatal("the view ticks Workflows again")
	}
	f.click("ui:delete-view", false)
	var deleted viewsplugin.SetArgs
	if sets := f.requests.Of(viewsplugin.SetEdit); len(sets) != 2 || sets[1].Decode(&deleted) != nil ||
		len(deleted.Views) != 0 {
		t.Fatalf("deleted: %+v", deleted)
	}
}
