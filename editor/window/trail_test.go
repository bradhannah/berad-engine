package window

import (
	"image"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// pickingView is a view whose selection Back and Forward can read and put
// back, as a Workflow's node.
type pickingView struct{ selected string }

func (v *pickingView) Title(frame.Docs) string                       { return "" }
func (v *pickingView) Update(*ui.Input, *frame.Context)              {}
func (v *pickingView) Draw(*ebiten.Image, frame.Docs)                {}
func (v *pickingView) Focused() *ui.TextField                        { return nil }
func (v *pickingView) Locate(string, frame.Docs) (image.Point, bool) { return image.Pt(0, 0), false }
func (v *pickingView) Selection() string                             { return v.selected }

func (v *pickingView) Changed(update client.Update, _ *frame.Context) {
	if update.Attention {
		v.selected = update.Item
	}
}

// pickingPane is a pane of a pickingView.
func pickingPane(title string) (*pane, *pickingView) {
	view := &pickingView{selected: ""}
	shown := titled("workflow", title)
	shown.host = &host{view: view, frame: frame.NewFrame(time.Now)}

	return shown, view
}

// TestBackAndForwardStepThroughWhereTheAuthorHasBeen keeps a trail of tabs
// and of what was selected in them, steps back and forward along it, puts
// a selection back only where nothing waits for Save, and forgets closed
// tabs (increment 38).
func TestBackAndForwardStepThroughWhereTheAuthorHasBeen(t *testing.T) {
	t.Parallel()
	notes, notesView := pickingPane("Workflow: Notes")
	shop, shopView := pickingPane("Workflow: Shop")
	project := titled("browser", "Project")
	shown := &shell{tabs: []*tab{{panes: []*pane{project}}, {panes: []*pane{notes}}, {panes: []*pane{shop}}}}
	var said string
	key := func(name string) {
		input := ui.NewInput(image.Pt(0, 0))
		input.Presses = []ui.Press{ui.ParsePress(name)}
		shown.tabKeys(&input, func(message string) { said = message })
	}
	shown.noteTrail()
	shown.active = 1
	shown.noteTrail()
	notesView.selected = "node:a"
	shown.noteTrail()
	notesView.selected = ""
	shown.noteTrail()
	notesView.selected = "node:b"
	shown.noteTrail()
	shown.active = 2
	shown.noteTrail()
	if len(shown.trail.places) != 4 {
		t.Fatalf("project, node a, node b, shop: %+v", shown.trail.places)
	}
	key("Cmd+[")
	shown.noteTrail()
	if shown.active != 1 || notesView.selected != "node:b" {
		t.Fatalf("back to node b in Notes: tab %d, %q", shown.active, notesView.selected)
	}
	key("Cmd+[")
	shown.noteTrail()
	if notesView.selected != "node:a" {
		t.Fatalf("back again selects node a: %q", notesView.selected)
	}
	key("Cmd+]")
	shown.noteTrail()
	if notesView.selected != "node:b" || len(shown.trail.places) != 4 {
		t.Fatalf("forward to node b keeps the trail: %q %d", notesView.selected, len(shown.trail.places))
	}
	notes.host.reported = protocol.Unsaved{Count: 1, Name: "Notes"}
	key("Cmd+[")
	shown.noteTrail()
	if notesView.selected != "node:b" || shown.trail.at != 1 {
		t.Fatalf("a selection waiting on Save is kept: %q", notesView.selected)
	}
	notes.host.reported = protocol.Unsaved{Count: 0, Name: ""}
	key("Cmd+[")
	key("Cmd+[")
	if shown.active != 0 || said != "Nothing to go back to" {
		t.Fatalf("back past the start: tab %d, %q", shown.active, said)
	}
	shown.remove(notes)
	shown.noteTrail()
	if len(shown.trail.places) != 2 || shown.trail.places[1].tab.panes[0] != shop || shopView.selected != "" {
		t.Fatalf("a closed tab leaves the trail: %+v", shown.trail.places)
	}
}
