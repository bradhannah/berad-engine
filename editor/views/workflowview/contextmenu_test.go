package workflowview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/hajimehoshi/ebiten/v2"
)

func (f *fixture) rightClick(name string) {
	f.t.Helper()
	input := ui.NewInput(f.at(name))
	input.Secondary = true
	f.frame(input)
}

func (f *fixture) menuOffers(item string) bool {
	_, ok := f.state.Locate("menu:"+item, f.docs)

	return ok
}

// TestRightClickDeletes right-clicks a node for a menu that deletes it,
// the start node for one that cannot, a line for one that deletes the line,
// and empty canvas for no menu at all.
func TestRightClickDeletes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.add("" + flow.KindWait)
	f.add("" + flow.KindWait)
	f.keys("Escape")
	f.rightClick("node:wait:2")
	if !f.menuOffers("delete-node") || f.state.Selected() != f.nth(flow.KindWait, 2).ID {
		t.Fatal("a right-click selects the node and offers to delete it")
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.click("menu:delete-node")
	if !f.has("nodes:2") || f.menuOffers("delete-node") || !strings.Contains(f.status(), "Node removed") {
		t.Fatalf("Delete node removes it and closes the menu: %q", f.status())
	}
	f.rightClick("node:start:1")
	f.click("menu:delete-node")
	if !f.has("nodes:2") {
		t.Fatal("the start node's Delete node does nothing")
	}
	f.rightClick("line:start:1:next")
	if !f.menuOffers("delete-line") || !f.has("line-selected") {
		t.Fatal("a right-click on a line selects it and offers to delete it")
	}
	f.click("menu:delete-line")
	if len(f.nth(flow.KindStart, 1).Next) != 0 || !strings.Contains(f.status(), "Line removed") {
		t.Fatalf("Delete line removes it: %+v", f.nth(flow.KindStart, 1).Next)
	}
	f.rightClick("canvas:900,700")
	if f.menuOffers("delete-node") || f.menuOffers("delete-line") || !f.state.Selected().IsZero() {
		t.Fatal("a right-click on empty canvas selects nothing and opens no menu")
	}
}
