package workflowview_test

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// emptyCanvas is a canvas point no node is near.
const emptyCanvas = "canvas:520,480"

// cmdClick clicks a named part of the window with Cmd held.
func (f *fixture) cmdClick(name string) {
	f.t.Helper()
	at := f.at(name)
	input := ui.NewInput(at)
	input.Pressed, input.Down, input.Held = true, true, ui.Cmd()
	f.frame(input)
	f.release(at)
}

// withThree adds a Wait, a Say and a Log after Start, one after another.
func withThree(t *testing.T) *fixture {
	t.Helper()
	f := withNode(t, flow.KindWait)
	f.add(flow.KindSay)
	f.add(flow.KindLog)

	return f
}

// TestCmdClickSelectsSeveral adds and takes out nodes with Cmd+click, and
// goes back to one node or none with a click, a click on empty canvas or
// Escape.
func TestCmdClickSelectsSeveral(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	f.click("node:" + flow.KindWait + ":1")
	f.cmdClick("node:" + flow.KindSay + ":1")
	if !f.has("selection=2") || !f.has("in-selection:"+flow.KindWait+":1") ||
		!f.has("in-selection:"+flow.KindSay+":1") || f.has("selected:"+flow.KindWait) {
		t.Fatal("Cmd+click adds a node to the selection, and no one node is selected alone")
	}
	f.cmdClick("node:" + flow.KindLog + ":1")
	if !f.has("selection=3") {
		t.Fatal("Cmd+click adds a third")
	}
	f.cmdClick("node:" + flow.KindSay + ":1")
	if !f.has("selection=2") || f.has("in-selection:"+flow.KindSay+":1") {
		t.Fatal("Cmd+click takes a selected node out")
	}
	f.cmdClick("node:" + flow.KindLog + ":1")
	if !f.has("selection=1") || !f.has("selected:"+flow.KindWait) {
		t.Fatal("a group of one is one node selected, with its inspector")
	}
	f.cmdClick("node:" + flow.KindSay + ":1")
	f.click("node:start:1")
	if !f.has("selection=1") || !f.has("selected:start") {
		t.Fatal("a click selects that node alone")
	}
	f.cmdClick("node:" + flow.KindSay + ":1")
	f.click(emptyCanvas)
	if !f.has("selection=0") {
		t.Fatal("a click on empty canvas selects nothing")
	}
	f.click("node:" + flow.KindSay + ":1")
	f.cmdClick("node:" + flow.KindLog + ":1")
	f.keys("Escape")
	if !f.has("selection=0") {
		t.Fatal("Escape selects nothing")
	}
}

// TestSeveralSelectedInspector finds the inspector's count, with no node's
// fields and no locals, while several nodes are selected.
func TestSeveralSelectedInspector(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	f.cmdClick("node:" + flow.KindWait + ":1")
	if !f.has("selection=2") {
		t.Fatal("two selected")
	}
	if _, found := f.state.Locate("ui:add-local", f.docs); found {
		t.Fatal("the locals are not shown while several nodes are selected")
	}
	if !f.state.Selected().IsZero() {
		t.Fatal("no one node is selected, so no node's fields show")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
}

// TestCmdClickAsksBeforeLeaving types a Wait's minutes without saving, then
// Cmd+clicks the start: the window asks, and Don't save goes on to both.
func TestCmdClickAsksBeforeLeaving(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindWait)
	f.typeOnly("param-duration", "45")
	f.cmdClick("node:start:1")
	f.click("ui:discard-changes")
	if !f.has("selection=2") || !f.has("in-selection:start:1") {
		t.Fatal("Don't save goes on to the two selected")
	}
}

// cmdDrag drags from one point to another with Cmd held.
func (f *fixture) cmdDrag(from, to image.Point) {
	f.t.Helper()
	for index, at := range []image.Point{from, from.Add(to).Div(2), to} {
		input := ui.NewInput(at)
		input.Down, input.Pressed, input.Held = true, index == 0, ui.Cmd()
		f.frame(input)
	}
	f.release(to)
}

// TestABoxSelects drags a box over a Wait and a Say, then with Cmd over the
// Log too.
func TestABoxSelects(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	f.drag(f.at("canvas:200,170"), f.at("canvas:660,30"))
	if !f.has("selection=2") || !f.has("in-selection:"+flow.KindWait+":1") ||
		!f.has("in-selection:"+flow.KindSay+":1") {
		t.Fatal("a box selects the nodes it touches")
	}
	f.cmdDrag(f.at("canvas:700,170"), f.at("canvas:660,30"))
	if !f.has("selection=3") || !f.has("in-selection:"+flow.KindLog+":1") {
		t.Fatal("a box dragged with Cmd adds to the selection")
	}
	f.drag(f.at("canvas:700,170"), f.at("canvas:660,30"))
	if !f.has("selection=1") || !f.has("selected:"+flow.KindLog) {
		t.Fatal("a box without Cmd starts afresh")
	}
}

// TestABoxTakesAContainerOnlyWhole drags a box across a container's edge,
// which leaves it, then around all of it.
func TestABoxTakesAContainerOnlyWhole(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindContainer)
	box, _ := f.workflow().Box(f.nth(flow.KindContainer, 1).ID)
	if box.W == 0 || box.H == 0 {
		t.Fatalf("a container has a size: %+v", box)
	}
	canvas := func(x, y int) image.Point { return f.at(fmt.Sprintf("canvas:%d,%d", x, y)) }
	f.drag(canvas(box.X-5, box.Y+box.H/2), canvas(box.X+box.W/2, box.Y-5))
	if !f.has("selection=0") {
		t.Fatal("a box across a container's edge leaves it")
	}
	f.drag(canvas(box.X-5, box.Y+box.H+5), canvas(box.X+box.W+5, box.Y-5))
	if !f.has("selection=1") || !f.has("selected:"+flow.KindContainer) {
		t.Fatal("a box around a container takes it")
	}
}

// TestABoxIsDrawn draws the window while a box is dragged.
func TestABoxIsDrawn(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	from, to := f.at("canvas:200,170"), f.at("canvas:660,30")
	f.press(from)
	f.hold(to)
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.release(to)
	if !f.has("selection=2") {
		t.Fatal("the box selected as it was drawn")
	}
}

// boxOf is where a node sits on the canvas.
func (f *fixture) boxOf(kind string) image.Point {
	f.t.Helper()
	box, _ := f.workflow().Box(f.nth(kind, 1).ID)

	return image.Pt(box.X, box.Y)
}

// gesturesOf counts the gestures of an edit's commands from the nth on.
func (f *fixture) gesturesOf(edit string, from int) int {
	gestures := map[string]bool{}
	for _, command := range f.requests.Of(edit)[from:] {
		gestures[command.Command.Gesture] = true
	}

	return len(gestures)
}

// TestTheSelectionMovesTogether drags a Wait with the Say selected beside
// it: both move the same way, as one undo step, and the Log stays. A click
// on one of them without moving selects it alone.
func TestTheSelectionMovesTogether(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	f.drag(f.at("canvas:200,170"), f.at("canvas:660,30"))
	wait, say, log := f.boxOf(flow.KindWait), f.boxOf(flow.KindSay), f.boxOf(flow.KindLog)
	before := len(f.requests.Of(workflowplugin.MoveEdit))
	from := f.at("node:" + flow.KindWait + ":1")
	f.drag(from, from.Add(image.Pt(40, 120)))
	moved := f.boxOf(flow.KindWait).Sub(wait)
	if moved == image.Pt(0, 0) || f.boxOf(flow.KindSay).Sub(say) != moved || f.boxOf(flow.KindLog) != log {
		t.Fatalf("the Wait and the Say move together, and the Log stays: %v", moved)
	}
	if f.gesturesOf(workflowplugin.MoveEdit, before) != 1 || !f.has("selection=2") {
		t.Fatal("a drag is one undo step, and keeps the selection")
	}
	f.click("node:" + flow.KindSay + ":1")
	if !f.has("selection=1") || !f.has("selected:"+flow.KindSay) {
		t.Fatal("a click without moving selects that node alone")
	}
}

// TestArrowKeysNudge nudges two nodes three times right and once down with
// Shift: one undo step for the run.
func TestArrowKeysNudge(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	f.drag(f.at("canvas:200,170"), f.at("canvas:660,30"))
	wait, say := f.boxOf(flow.KindWait), f.boxOf(flow.KindSay)
	before := len(f.requests.Of(workflowplugin.MoveEdit))
	f.keys("Right")
	f.keys("Right")
	f.keys("Right")
	f.keys("Shift+Down")
	want := image.Pt(24, 40)
	if f.boxOf(flow.KindWait).Sub(wait) != want || f.boxOf(flow.KindSay).Sub(say) != want {
		t.Fatalf("nudged by %v, wanted %v", f.boxOf(flow.KindWait).Sub(wait), want)
	}
	if f.gesturesOf(workflowplugin.MoveEdit, before) != 1 {
		t.Fatal("a run of nudges is one undo step")
	}
}

// TestASelectionStopsAtAContainersWall drags two nodes in a container far
// to the left: they stop at its wall.
func TestASelectionStopsAtAContainersWall(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindContainer)
	f.add(flow.KindWait)
	f.add(flow.KindSay)
	container := f.nth(flow.KindContainer, 1).ID
	if f.nth(flow.KindWait, 1).Parent != container || f.nth(flow.KindSay, 1).Parent != container {
		t.Fatal("the Wait and the Say are in the container")
	}
	f.cmdClick("node:" + flow.KindWait + ":1")
	box, _ := f.workflow().Box(container)
	from := f.at("node:" + flow.KindWait + ":1")
	f.drag(from, from.Add(image.Pt(-600, 0)))
	if f.boxOf(flow.KindWait).X < box.X || f.boxOf(flow.KindSay).X < box.X {
		t.Fatalf("the nodes stop at the container's wall: %v %v, wall %d", f.boxOf(flow.KindWait),
			f.boxOf(flow.KindSay), box.X)
	}
	if moved, _ := f.workflow().Box(container); moved.X != box.X {
		t.Fatal("the container stays")
	}
}

// TestTheSelectionIsColouredAndDeleted colours a Wait and a Say at once,
// then deletes them with the start, which stays, each as one command.
func TestTheSelectionIsColouredAndDeleted(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	f.drag(f.at("canvas:200,170"), f.at("canvas:660,30"))
	f.click("ui:swatch:group:green")
	if !f.has("tint:"+flow.KindWait+":1=green") || !f.has("tint:"+flow.KindSay+":1=green") ||
		f.has("tint:"+flow.KindLog+":1=green") || len(f.requests.Of(workflowplugin.TintEdit)) != 1 {
		t.Fatal("one swatch colours both, in one edit")
	}
	f.cmdClick("node:start:1")
	if !f.has("selection=3") {
		t.Fatal("the start joins the selection")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.click("ui:delete-group")
	if !f.has("nodes:2") || !f.has("selection=0") || len(f.requests.Of(workflowplugin.RemoveEdit)) != 1 ||
		!strings.Contains(f.status(), "2 nodes removed") {
		t.Fatalf("Delete 2 nodes removes the Wait and the Say, not the start: %q", f.status())
	}
}

// TestDeleteKeyAndMenuRemoveTheSelection removes a selection with the
// Delete key, and another from the right-click menu on one of its nodes.
func TestDeleteKeyAndMenuRemoveTheSelection(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	f.drag(f.at("canvas:200,170"), f.at("canvas:660,30"))
	f.keys("Delete")
	if !f.has("nodes:2") {
		t.Fatal("the Delete key removes both")
	}
	f.add(flow.KindWait)
	f.cmdClick("node:" + flow.KindLog + ":1")
	f.rightClick("node:" + flow.KindLog + ":1")
	if !f.menuOffers("delete-group") || !f.has("selection=2") {
		t.Fatal("a right-click on a selected node keeps the selection and offers to delete it")
	}
	f.click("menu:delete-group")
	if !f.has("nodes:1") {
		t.Fatal("Delete 2 nodes removes them")
	}
}

// TestCmdDDuplicates duplicates a green Wait leading to a Say: the copies
// sit a little below and right, keep the line between them and the tint,
// lose the line out to the Log, and are selected.
func TestCmdDDuplicates(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	f.drag(f.at("canvas:200,170"), f.at("canvas:660,30"))
	f.click("ui:swatch:group:green")
	wait := f.boxOf(flow.KindWait)
	f.keys("Cmd+D")
	if !f.has("nodes:6") || len(f.requests.Of(workflowplugin.AddEdit)) != 4 {
		t.Fatal("Cmd+D adds two nodes in one edit")
	}
	waitCopy, sayCopy := f.nth(flow.KindWait, 2), f.nth(flow.KindSay, 2)
	if waitCopy.Next[flow.OutputNext] != sayCopy.ID || len(sayCopy.Next) != 0 {
		t.Fatalf("the copies keep the line between them, not the one out: %v %v", waitCopy.Next, sayCopy.Next)
	}
	if box, _ := f.workflow().Box(waitCopy.ID); image.Pt(box.X, box.Y) != wait.Add(image.Pt(24, 24)) {
		t.Fatalf("the copy sits below and right: %+v", box)
	}
	if !f.has("tint:"+flow.KindWait+":2=green") || !f.has("entries:"+flow.KindWait+":2=0") {
		t.Fatal("the copy keeps its tint, and nothing leads into it")
	}
	if !f.has("selection=2") || !f.has("in-selection:"+flow.KindWait+":2") ||
		!f.has("in-selection:"+flow.KindSay+":2") || !strings.Contains(f.status(), "Duplicated 2 nodes") {
		t.Fatalf("the copies are selected: %q", f.status())
	}
}

// TestCmdDDuplicatesAContainer duplicates a container with a Wait in it:
// the copy holds a copy of the Wait.
func TestCmdDDuplicatesAContainer(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindContainer)
	f.add(flow.KindWait)
	f.click("node:" + flow.KindContainer + ":1")
	f.keys("Cmd+D")
	container, wait := f.nth(flow.KindContainer, 2), f.nth(flow.KindWait, 2)
	if container.ID.IsZero() || wait.Parent != container.ID || !f.has("selected:"+flow.KindContainer) {
		t.Fatalf("the container's copy holds the Wait's: %+v %+v", container, wait)
	}
}
