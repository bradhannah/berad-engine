package window

import (
	"image"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// namingView is a pickingView that names the first part a change touched.
type namingView struct{ pickingView }

func (v *namingView) Named(touched protocol.Touch, _ frame.Docs) (string, string, bool) {
	if len(touched.Items) == 0 {
		return "", "", false
	}

	return touched.Items[0], "Wait", true
}

// TestAClickOnAHistoryLineAsksTheViewToShowIt asks the view to show what a
// clicked line names, and says so; a pane whose changes wait for Save keeps
// its selection and says why (increment 45).
func TestAClickOnAHistoryLineAsksTheViewToShowIt(t *testing.T) {
	t.Parallel()
	view := &namingView{pickingView{selected: ""}}
	shown := &host{view: view, frame: frame.NewFrame(time.Now), visible: image.Pt(1200, 800)}
	notes := ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")
	shown.frame.StatusAbout("Added nodes to Notes", protocol.Touching(notes, []string{"node:wait"}))
	shown.frame.UpdateStatus(&ui.Input{}, shown.visible)
	click := func(name string) {
		shown.updateHistory(&ui.Input{})
		point, found := shown.frame.LocateStatus(name)
		if !found {
			t.Fatalf("no %s", name)
		}
		input := ui.NewInput(point)
		input.Pressed, input.Down = true, true
		shown.updateHistory(&input)
		shown.frame.UpdateStatus(&input, shown.visible)
	}
	shown.reported = protocol.Unsaved{Count: 1, Name: "Notes"}
	click("ui:history")
	click("history:Added nodes")
	if message, _ := shown.frame.Message(); view.selected != "" || message != keptForSave {
		t.Fatalf("a selection waiting on Save is kept, and the status bar says so: %q", message)
	}
	shown.reported = protocol.Unsaved{Count: 0, Name: ""}
	click("ui:history")
	click("history:Added nodes")
	if message, _ := shown.frame.Message(); view.selected != "node:wait" || message != "Selected Wait" {
		t.Fatalf("the view shows what the line names: %q, %q", view.selected, message)
	}
}
