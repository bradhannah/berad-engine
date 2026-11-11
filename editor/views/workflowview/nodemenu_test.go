package workflowview_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestLinesDoOnlyWhatTheirAuthorDid holds a line from the start, which leads
// to one Wait, over another Wait: the old line is marked as the one letting
// go would replace, and not over the Wait it leads to. Tab and a
// double-click on empty canvas open the node menu, joined to nothing; Add
// after names the output it joins from, or says the node goes beside
// (increment 40).
func TestLinesDoOnlyWhatTheirAuthorDid(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:400,300"))
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:400,420"))
	f.drag(f.at("port:start:1:next"), f.at("node:wait:1"))
	port, other, current := f.at("port:start:1:next"), f.at("node:wait:2"), f.at("node:wait:1")
	f.press(port)
	f.hold(port.Add(other).Div(2))
	f.hold(other)
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	if !f.has("replacing") {
		t.Fatal("held over another node, the line it would replace is marked")
	}
	f.hold(current)
	if f.has("replacing") {
		t.Fatal("held over the node it leads to, nothing would be replaced")
	}
	f.release(current)
	f.keys("Tab")
	if !f.menuOffers("node-menu") {
		t.Fatal("Tab opens the node menu")
	}
	f.keys("Escape")
	empty := f.at("canvas:700,150")
	f.press(empty)
	f.release(empty)
	double := ui.NewInput(empty)
	double.Pressed, double.Down, double.DoubleClick = true, true, true
	f.frame(double)
	if !f.menuOffers("node-menu") {
		t.Fatal("a double-click on empty canvas opens the node menu")
	}
	f.keys("Escape")
	if len(f.nth(flow.KindWait, 2).Next) != 0 {
		t.Fatal("nothing was joined")
	}
	f.click("node:wait:1")
	f.click("palette:" + flow.KindWait)
	if point, ok := f.state.Locate("ui:help-add", f.docs); !ok || point.X == 0 {
		t.Fatal("the help page offers Add after")
	}
	f.click("ui:help-add")
	if f.nth(flow.KindWait, 1).Next[flow.OutputNext] != f.nth(flow.KindWait, 3).ID {
		t.Fatal("Add after joins from Wait's free output")
	}
}
