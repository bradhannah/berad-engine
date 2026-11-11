package window

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TestQuitDialogNamesUnsavedChanges offers Quit when everything is saved,
// and Save and quit with the unsaved changes named otherwise.
func TestQuitDialogNamesUnsavedChanges(t *testing.T) {
	t.Parallel()
	saved := quitDialog(protocol.QuitAsk{Windows: 3, Unsaved: nil})
	if saved.Buttons[0].ID != quitNow || len(saved.Buttons) != 2 || saved.Lines[0] != "Every tab will close." {
		t.Fatalf("saved %+v", saved)
	}
	unsaved := quitDialog(protocol.QuitAsk{Windows: 1, Unsaved: []protocol.UnsavedWindow{
		{Window: ids.ULID{}, View: "browser", Name: "NPC 1", Count: 2},
		{Window: ids.ULID{}, View: "workflow", Name: "", Count: 1},
	}})
	text := strings.Join(unsaved.Lines, "\n")
	if unsaved.Buttons[0].ID != quitSave || unsaved.Buttons[1].ID != quitDiscard || len(unsaved.Buttons) != 3 ||
		!strings.Contains(text, "NPC 1 · 2 changes") || !strings.Contains(text, "a workflow tab · 1 change") ||
		unsaved.Lines[0] != "Every tab will close." {
		t.Fatalf("unsaved %q", text)
	}
}

// TestQuitDialogIsWhereItIsDrawnWhileTheInspectorIsFolded folds the
// inspector, which widens what the view lays out for past the window's edge,
// and asks to quit with changes unsaved. The dialog is drawn on the part of
// the window that shows, so its buttons are found, and take clicks, there: a
// click on Cancel as drawn cancels, and does not land on Don't save.
func TestQuitDialogIsWhereItIsDrawnWhileTheInspectorIsFolded(t *testing.T) {
	t.Parallel()
	ask := protocol.QuitAsk{Windows: 1, Unsaved: []protocol.UnsavedWindow{
		{Window: ids.ULID{}, View: "browser", Name: "NPC 1", Count: 1},
	}}
	h := &host{frame: frame.NewFrame(time.Now), surface: newPaneSurface(), visible: image.Pt(1000, 800)}
	h.frame.ShowInspector.On = false
	h.ctx.Size = h.viewSize()
	if h.ctx.Size == h.visible {
		t.Fatal("a folded inspector widens what the view lays out for")
	}
	h.quitDialog = quitDialog(ask)
	shows := image.Rectangle{Min: image.Pt(0, 0), Max: h.visible}
	drawn, found := quitDialog(ask).Locate(ui.DialogCancel, shows)
	if !found {
		t.Fatal("no Cancel button")
	}
	if at, located := h.Locate("ui:"+ui.DialogCancel, nil); !located || at != drawn {
		t.Fatalf("Cancel is drawn at %v and found at %v", drawn, at)
	}
	press := ui.NewInput(drawn)
	press.Pressed, press.Down = true, true
	h.updateQuitDialog(&press)
	if h.quitDialog != nil {
		t.Fatal("a click on Cancel as drawn did not cancel")
	}
}
