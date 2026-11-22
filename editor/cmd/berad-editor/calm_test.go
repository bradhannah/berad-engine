package main_test

import (
	"path/filepath"
	"testing"
)

// TestTheCalmerEditor looks at each place increment 66 restyled, in a real
// window, and photographs it. In the Project window a zone is selected, so
// its inspector shows its one main button; the Kind filter is set to Zones,
// which washes its button and gives it a cross, and the cross clears it.
// In the Map each group's tool is taken in hand by its key, the panel
// showing what that tool paints with. In a new workflow the palette lists
// its nodes as rows. The window bar and the status bar are in every picture.
func TestTheCalmerEditor(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	inspector, filters := shotPath(t, "calm-inspector.png"), shotPath(t, "calm-filters.png")
	selectShot, ground := shotPath(t, "calm-map-select.png"), shotPath(t, "calm-map-ground.png")
	walls, objects := shotPath(t, "calm-map-walls.png"), shotPath(t, "calm-map-objects.png")
	people, palette := shotPath(t, "calm-map-npc.png"), shotPath(t, "calm-palette.png")
	browserScript := script(
		must("row:Sample zone", 600), wait(30), click("row:Sample zone"), wait(30), shoot(inspector),
		click("ui:facet-kind"), click("facet:Kind/Zones"), keys("Escape"), must("state:ticked:Kind/Zones", 60),
		must("state:place=Matches", 60), must("clear:Kind", 60), wait(30), shoot(filters),
		click("clear:Kind"), wait(10), click("place:Home"), must("state:place=Home", 60),
		openZone(), must("state:tabs:2", 600), wait(30),
		click("ui:new-workflow"), wait(30), click("ui:open"), must("state:tabs:3", 600), quitWhenAlone(),
	)
	mapScript := script(
		must("state:loaded", 600), wait(30), keys("S"), wait(20), shoot(selectShot),
		keys("G"), wait(20), shoot(ground), keys("W"), wait(20), shoot(walls),
		keys("O"), wait(20), shoot(objects), keys("N"), wait(20), shoot(people), keys("S"), wait(600), quit(),
	)
	workflowScript := script(must("state:loaded", 600), wait(60), shoot(palette), wait(10), quit())
	runEditor(t, binary, project, scripts{
		"browser": {browserScript}, "map": {mapScript}, "workflow": {workflowScript},
	})
	noCrashes(t, project)
	t.Logf("screenshots: %s %s %s %s %s %s %s %s", inspector, filters, selectShot, ground, walls, objects, people, palette)
}
