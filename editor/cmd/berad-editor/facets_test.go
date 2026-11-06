package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	viewsplugin "github.com/bradhannah/berad-engine/editor/plugins/views"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// TestTheProjectByFolders opens the Project window on Home, listing what the
// sample holds as changed today, and pins Clerk's day there. Sprites shows
// as pictures and opens one large in Preview. Ticking Workflows in the bar
// shows Matches; the tick is saved as a view, cleared, and applied again
// from the places column. The view is saved with the project, and the pin
// and Sprites' view in this person's own state.
func TestTheProjectByFolders(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	homeShot, picturesShot := shotPath(t, "project-home.png"), shotPath(t, "project-pictures.png")
	previewShot, matchesShot := shotPath(t, "project-preview.png"), shotPath(t, "project-matches.png")
	browserScript := script(
		must("place:Home", 600), must("state:place=Home", 60), must("row:Clerk's day", 300), wait(30),
		rightClick("row:Clerk's day"), must("menu:context-pin", 60), click("menu:context-pin"), wait(10),
		must("state:row-detail:Clerk's day=pinned", 60), shoot(homeShot),
		click("place:Sprites"), must("state:pane=pictures", 60), wait(60), shoot(picturesShot),
		click("ui:view-preview"), must("state:pane=preview", 60), keys("Right"), wait(60), shoot(previewShot),
		click("ui:facet-kind"), click("facet:Kind/Workflows"), keys("Escape"), must("state:place=Matches", 60),
		click("ui:save-view"), wait(5), keys("Cmd+A"), typed("Workflows"), keys("Enter"),
		must("state:view=Workflows", 300), click("ui:clear"), wait(5), click("view:Workflows"),
		must("state:ticked:Kind/Workflows", 60), wait(30), shoot(matchesShot), wait(10), quitWhenAlone(),
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
	checkHomeKept(t, project)
	t.Logf("screenshots: %s %s %s %s", homeShot, picturesShot, previewShot, matchesShot)
}

// checkHomeKept reads this person's state: the pin and Sprites' view, kept
// apart from the project and out of version control.
func checkHomeKept(t *testing.T, project string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project, ".berad", "home.json"))
	if err != nil {
		t.Fatal(err)
	}
	var home struct {
		Kept protocol.HomeKept `json:"kept"`
	}
	if err := json.Unmarshal(data, &home); err != nil {
		t.Fatal(err)
	}
	if len(home.Kept.Pins) != 1 || home.Kept.Folders["section:sprites"].View != "preview" {
		t.Fatalf("Home kept the pin and Sprites' view: %+v", home.Kept)
	}
	ignore, err := os.ReadFile(filepath.Join(project, ".berad", ".gitignore"))
	if err != nil || !slices.Contains(strings.Fields(string(ignore)), "*") {
		t.Fatalf("the state folder keeps itself out of version control: %q %v", ignore, err)
	}
}
