package mapview_test

import (
	"errors"
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/play"
)

var runID = frametest.ID(7)

// walk holds a key for some frames, then lets go for long enough that the
// player comes to rest.
func (f *fixture) walk(key ui.Key, frames int) {
	for range frames {
		input := ui.NewInput(image.Pt(0, 0))
		input.Keys = []ui.Key{key}
		f.frame(input)
	}
	f.idle(play.StepTicks)
}

func (f *fixture) idle(frames int) {
	for range frames {
		f.frame(ui.NewInput(image.Pt(0, 0)))
	}
}

func (f *fixture) has(state string) bool {
	_, ok := f.state.Locate("state:"+state, f.ctx.Docs)

	return ok
}

func (f *fixture) status() string {
	message, _ := f.ctx.Frame.Message()

	return message
}

// startPreview presses P, clicks a cell and agrees to the start.
func (f *fixture) startPreview(cell string) {
	f.t.Helper()
	f.keys("P")
	f.click(cell)
	if len(f.requests.Previews) != 1 {
		f.t.Fatalf("preview starts asked for: %d", len(f.requests.Previews))
	}
	f.requests.Previews[0](protocol.Result{OK: true, Reason: "", Revision: 0, Created: runID}, nil)
	if !f.has("previewing") {
		f.t.Fatal("the preview did not start")
	}
}

// D15: P, then a click on the start cell, starts a preview; the arrow keys
// walk the player, walls stop them, the state goes to the other views and P
// stops it.
func TestPreviewWalksThePlayer(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.startPreview("cell:3,3")
	f.walk(ui.KeyRight, 1)
	if !f.has("player:4,3") {
		t.Fatalf("a tap walks one cell: %s", f.status())
	}
	f.walk(ui.KeyRight, 2*play.StepTicks)
	f.walk(ui.KeyRight, 1)
	if !f.has("player:6,3") {
		t.Fatal("the wall at 7,3 stops the player at 6,3")
	}
	states := f.requests.PreviewStates
	if len(states) == 0 {
		t.Fatal("no state sent to the other views")
	}
	last := states[len(states)-1]
	if last.Run != runID || last.Entities[0].X != 6 || last.Entities[0].Y != 3 || last.Entities[0].Floor != groundID {
		t.Fatalf("last state %+v", last)
	}
	for index := 1; index < len(states); index++ {
		if states[index].Tick < states[index-1].Tick+2 {
			t.Fatalf("states are sent at most every second tick: %d then %d", states[index-1].Tick, states[index].Tick)
		}
	}
	if len(f.requests.Commands) != 0 {
		t.Fatal("a preview never changes the project")
	}
	f.keys("P")
	if f.has("previewing") || len(f.requests.PreviewStops) != 1 || f.requests.PreviewStops[0].Run != runID {
		t.Fatalf("P stops the preview: %+v", f.requests.PreviewStops)
	}
}

// D15: editing during a preview applies at once.
func TestPreviewSeesEdits(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.startPreview("cell:3,3")
	f.docs.View(groundID, func(document client.Document) {
		if _, err := document.Model.(*tilefloor.Document).Floor.SetTop(3, 4, "wall.b"); err != nil {
			t.Fatal(err)
		}
	})
	f.walk(ui.KeyDown, 1)
	if !f.has("player:3,3") {
		t.Fatal("the new wall blocks the player")
	}
	f.walk(ui.KeyRight, 1)
	if !f.has("player:4,3") {
		t.Fatal("editing tools keep working, and so does walking")
	}
}

// Stairs take the player to the same cell of the floor above, and the view
// follows them there.
func TestPreviewStairsSwitchTheFloor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.docs.View(groundID, func(document client.Document) {
		if _, err := document.Model.(*tilefloor.Document).Floor.SetTop(5, 4, "stairs.up"); err != nil {
			t.Fatal(err)
		}
	})
	f.startPreview("cell:5,3")
	f.walk(ui.KeyDown, 1)
	if !f.has("floor:2") || !f.has("player:5,4") {
		t.Fatalf("the view follows the player upstairs: %s", f.status())
	}
	if !strings.Contains(f.status(), "Upper") {
		t.Fatalf("status %q", f.status())
	}
}

// The start cell must be one the player can stand on; Escape gives up.
func TestPreviewStartCell(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("P")
	f.click("cell:2,2")
	if len(f.requests.Previews) != 0 || !strings.Contains(f.status(), "cannot stand") {
		t.Fatalf("a wall is no place to start: %q", f.status())
	}
	f.keys("Escape")
	f.click("cell:3,3")
	if len(f.requests.Previews) != 0 {
		t.Fatal("Escape stops choosing a start cell")
	}
	f.keys("P")
	f.click("cell:3,3")
	f.requests.Previews[0](protocol.Result{OK: false, Reason: "a preview is already running", Revision: 0, Created: runID},
		errors.New("refused"))
	if f.has("previewing") || !strings.Contains(f.status(), "already running") {
		t.Fatalf("a refused start says why: %q", f.status())
	}
}

// Arrow keys before a preview runs say what is needed: P, then a click on the
// start cell.
func TestArrowKeysExplainHowToStart(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("Right")
	if !strings.Contains(f.status(), "press P, then click") {
		t.Fatalf("no preview yet: %q", f.status())
	}
	f.keys("P")
	f.keys("Up")
	if !strings.Contains(f.status(), "First click the cell") {
		t.Fatalf("choosing the start cell: %q", f.status())
	}
	if len(f.requests.Previews) != 0 {
		t.Fatal("arrow keys do not start a preview")
	}
}

// D13: every view of the zone shows the player of a preview another window
// runs, ignoring states older than the one it holds.
func TestAnotherViewsPreviewIsShown(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: upperID}}, f.ctx)
	state := func(tick uint64, x float64) *protocol.PreviewState {
		return &protocol.PreviewState{Run: runID, Tick: tick, Entities: []protocol.PreviewEntity{
			{ID: protocol.EntityPlayer, Floor: groundID, X: x, Y: 3, Face: [2]int{0, 1}},
		}}
	}
	f.state.Changed(client.Update{PreviewState: state(4, 3)}, f.ctx)
	f.state.Changed(client.Update{PreviewState: state(2, 9)}, f.ctx)
	if !f.has("remote-player:3,3") {
		t.Fatal("the latest state shows")
	}
	if _, ok := f.state.Locate("player", f.ctx.Docs); !ok {
		t.Fatal("the player can be found on screen")
	}
	f.keys("P")
	if !strings.Contains(f.status(), "another view") {
		t.Fatalf("one preview at a time: %q", f.status())
	}
	f.state.Changed(client.Update{PreviewStop: &protocol.PreviewStop{Run: runID, Reason: "stopped"}}, f.ctx)
	if f.has("remote-player:3,3") {
		t.Fatal("the player goes when the preview stops")
	}
}
