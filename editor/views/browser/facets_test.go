package browser_test

import (
	"image"
	"slices"
	"testing"

	viewsplugin "github.com/bradhannah/berad-engine/editor/plugins/views"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// TestFacetsNarrowThePlaces ticks a kind from the bar: Matches lists the two
// workflows, the places column keeps only the places holding them, each
// saying how many of all it holds, and a value not ticked still counts
// what ticking it would add. A folder shows only what is kept in it, and
// Clear puts everything back.
func TestFacetsNarrowThePlaces(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	if f.has("narrowed") || !f.has("count:Kind/Workflows=2") || !f.has("count:Kind/NPCs=1") {
		t.Fatal("nothing narrows yet, and each value says how many things have it")
	}
	f.click("ui:facet-kind", false)
	if _, listed := f.state.Locate("facet:Kind/Workflows", f.ctx.Docs); !listed {
		t.Fatal("the Kind button opens its values")
	}
	f.click("facet:Kind/Workflows", false)
	if !f.has("narrowed") || !f.has("showing=2 of 5") || !f.has("ticked:Kind/Workflows") || !f.has("place=Matches") {
		t.Fatal("ticking Workflows shows Matches, two workflows of five things")
	}
	if got := f.matchLabels(); !slices.Equal(got, []string{"Clerk day", "Night watch"}) {
		t.Fatalf("Matches lists the workflows: %v", got)
	}
	if !f.has("count:Kind/NPCs=1") {
		t.Fatal("NPCs still counts the NPC ticking it would add")
	}
	places := map[string]string{}
	for _, row := range f.state.Places(f.ctx.Docs) {
		places[row.Label] = row.Detail
	}
	if places["Workflows"] != "2 of 2" || places["Market"] != "2 of 2" || places["Guards"] != "1 of 1" {
		t.Fatalf("the places holding them say how many: %v", places)
	}
	if _, shown := places["Wilds"]; shown {
		t.Fatal("a folder holding nothing kept is not listed")
	}
	if _, shown := places["NPCs"]; shown {
		t.Fatal("a kind holding nothing kept is not listed")
	}
	f.press("Escape")
	f.click("place:Market", false)
	if got := f.labels(); !slices.Equal(got, []string{"Guards", "Clerk day"}) {
		t.Fatalf("Market shows what is kept in it: %v", got)
	}
	f.click("ui:clear", false)
	if f.has("narrowed") || !slices.Contains(f.placeLabels(), "Wilds") {
		t.Fatal("Clear puts every place back")
	}
}

// TestSearchingShowsMatches types a search: Matches lists what it finds,
// Settings among them when its words match.
func TestSearchingShowsMatches(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.typeInto("search", "night")
	if !f.has("place=Matches") || !f.has("showing=1 of 5") || !slices.Equal(f.matchLabels(), []string{"Night watch"}) {
		t.Fatalf("night finds Night watch: %v", f.matchLabels())
	}
	f.press("Escape")
	f.press("Escape")
	if f.has("narrowed") || !f.has("place=Home") {
		t.Fatal("clearing the search shows the place shown before")
	}
	f.typeInto("search", "tile")
	if _, found := f.state.Locate("row:Settings", f.ctx.Docs); !found {
		t.Fatal("tile finds the Settings")
	}
}

// TestSavingAView saves the ticks under a name, finds them a saved view,
// applies it again from the places column, and deletes it.
func TestSavingAView(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	docs := f.ctx.Docs.(*frametest.Docs)
	docs.Put(viewsplugin.ID(), viewsplugin.Name, &viewsplugin.Document{Views: nil})
	f.click("ui:facet-kind", false)
	f.click("facet:Kind/Workflows", false)
	f.press("Escape")
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
	f.click("ui:clear", false)
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

// placeLabels lists the places column by name.
func (f *fixture) placeLabels() []string {
	var labels []string
	for _, row := range f.state.Places(f.ctx.Docs) {
		labels = append(labels, row.Label)
	}

	return labels
}
