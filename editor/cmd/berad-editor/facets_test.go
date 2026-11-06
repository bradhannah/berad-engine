package main_test

import (
	"path/filepath"
	"testing"

	viewsplugin "github.com/bradhannah/berad-engine/editor/plugins/views"
)

// TestFacetsNarrowTheProject ticks Workflows in the Project window's
// facets: the list becomes a table of the sample's workflows. It saves the
// tick as a view, takes it away, and applies the view again from the facets
// column. The view is saved with the project.
func TestFacetsNarrowTheProject(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	facetsShot, tableShot := shotPath(t, "project-facets.png"), shotPath(t, "project-table.png")
	browserScript := script(
		must("row:Sample zone", 600), must("state:facets", 60), wait(60), shoot(facetsShot),
		click("facet:Kind/Workflows"), must("state:table", 60), must("state:cell:Clerk's day/Kind=NPC behaviour", 60),
		click("ui:save-view"), wait(5), keys("Cmd+A"), typed("Workflows"), keys("Enter"),
		must("state:view=Workflows", 300), click("chip:Kind: Workflows"), wait(5), click("view:Workflows"),
		must("state:ticked:Kind/Workflows", 60), wait(30), shoot(tableShot), wait(10), quitWhenAlone(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}})
	noCrashes(t, project)
	saved := readEnvelope(t, project, "views/*.json")
	decoded, err := viewsplugin.Decode(saved.Data)
	if err != nil {
		t.Fatal(err)
	}
	if views := decoded.(*viewsplugin.Document).Views; len(views) != 1 || views[0].Name != "Workflows" {
		t.Fatalf("the view was saved with the project: %+v", views)
	}
	t.Logf("screenshots: %s %s", facetsShot, tableShot)
}
