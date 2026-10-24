package window

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TestQuitDialogNamesUnsavedChanges offers Quit when everything is saved,
// and Save and quit with the unsaved changes named otherwise.
func TestQuitDialogNamesUnsavedChanges(t *testing.T) {
	t.Parallel()
	saved := quitDialog(protocol.QuitAsk{Windows: 3, Unsaved: nil})
	if saved.Buttons[0].ID != quitNow || len(saved.Buttons) != 2 || saved.Lines[0] != "All 3 windows will close." {
		t.Fatalf("saved %+v", saved)
	}
	unsaved := quitDialog(protocol.QuitAsk{Windows: 1, Unsaved: []protocol.UnsavedWindow{
		{Window: ids.ULID{}, View: "browser", Name: "NPC 1", Count: 2},
		{Window: ids.ULID{}, View: "text", Name: "", Count: 1},
	}})
	text := strings.Join(unsaved.Lines, "\n")
	if unsaved.Buttons[0].ID != quitSave || unsaved.Buttons[1].ID != quitDiscard || len(unsaved.Buttons) != 3 ||
		!strings.Contains(text, "NPC 1 · 2 changes") || !strings.Contains(text, "a text window · 1 change") ||
		unsaved.Lines[0] != "This window will close." {
		t.Fatalf("unsaved %q", text)
	}
}
