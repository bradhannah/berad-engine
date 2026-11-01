package main_test

import (
	"path/filepath"
	"testing"

	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestWorldStateDeclaresValues opens the World state window from the
// project browser, declares an NPC value, names it and makes it a list of
// choices: the saved world state holds npc.mood.
func TestWorldStateDeclaresValues(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	shot := shotPath(t, "world-state.png")
	browserScript := script(must("row:World state", 600), wait(5), doubleClick("row:World state"), quitWhenAlone())
	stateScript := script(
		must("state:loaded", 600), wait(5), click("scope:npc"), wait(5), click("ui:add-value"),
		must("state:selected=npc.value", 300), wait(5), click("ui:value-name"), keys("Cmd+A", "Backspace"),
		typed("mood"), keys("Enter"), must("state:selected=npc.mood", 300), wait(5),
		click("ui:value-type-choice"), must("state:value=npc.mood:choice:first", 300), wait(30),
		shoot(shot), wait(10), quit(),
	)
	runEditor(t, binary, project, scripts{"browser": {browserScript}, "world-state": {stateScript}})
	noCrashes(t, project)
	saved := readEnvelope(t, project, "world-states/*.json")
	decoded, err := stateplugin.Decode(saved.Data)
	if err != nil {
		t.Fatal(err)
	}
	values := decoded.(*stateplugin.Document).State.Values.Values()
	if saved.ID != worldstate.ID() || len(values) != 1 || values[0].Path() != "npc.mood" ||
		values[0].Type != worldstate.TypeChoice || len(values[0].Choices) != 2 {
		t.Fatalf("the world state was saved as %+v", values)
	}
	t.Logf("screenshot: %s", shot)
}
