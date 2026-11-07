package workflowview_test

import (
	"fmt"
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

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
