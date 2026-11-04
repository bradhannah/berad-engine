package workflowview_test

import (
	"strconv"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestHelpForEveryNode finds a one line and a help page for every kind of
// node each kind of workflow offers.
func TestHelpForEveryNode(t *testing.T) {
	t.Parallel()
	if missing := workflowview.MissingHelp(); len(missing) > 0 {
		t.Fatalf("every node offered explains itself: %v", missing)
	}
}

// search types a search into the palette.
func (f *fixture) search(query string) {
	f.t.Helper()
	f.click("ui:palette-search")
	f.keys("Cmd+A")
	typed := noInput()
	typed.Text = query
	f.frame(typed)
}

// TestSearchingThePalette finds nodes by name, by what they do and by a
// setting; Down and Enter show a result's help; Escape clears the search.
func TestSearchingThePalette(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.search("sound")
	if !f.has("results=1") {
		t.Fatal("sound finds Play sound by its name")
	}
	if _, found := f.state.Locate("result:"+flow.KindPlaySound, f.docs); !found {
		t.Fatal("Play sound is a result")
	}
	f.search("anchor")
	if _, found := f.state.Locate("result:"+flow.KindGoTo, f.docs); !found {
		t.Fatal("anchor finds Go to by what it does")
	}
	f.search("duration")
	if _, found := f.state.Locate("result:"+flow.KindWait, f.docs); !found {
		t.Fatal("duration finds Wait by its setting")
	}
	f.search("value")
	f.keys("Down")
	f.keys("Enter")
	if !f.has("help=" + flow.KindCheckValue) {
		t.Fatal("Down and Enter show the second result's help: Set value, then Check value")
	}
	f.click("ui:palette-search")
	f.keys("Escape")
	if !f.has("results=" + strconv.Itoa(len(flow.KindsFor(flow.PurposeBehaviour))-1)) {
		t.Fatal("Escape clears the search, every entry showing again")
	}
}

// TestHelpAndHover shows a node's help on a click, placing nothing, goes
// back, adds a Wait after the start, puts a Move between them with Add
// after, and shows a tooltip once the pointer rests on an entry.
func TestHelpAndHover(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindCheckValue)
	if !f.has("help="+flow.KindCheckValue) || !f.has("selected:start") || !f.has("nodes:1") {
		t.Fatal("a click shows the help, the start still selected, and places nothing")
	}
	f.click("ui:help-back")
	if f.has("help=" + flow.KindCheckValue) {
		t.Fatal("back closes the help")
	}
	f.add(flow.KindWait)
	f.click("node:start:1")
	f.add(flow.KindMove)
	start, move, wait := f.nth(flow.KindStart, 1), f.nth(flow.KindMove, 1), f.nth(flow.KindWait, 1)
	if start.Next[flow.OutputNext] != move.ID || move.Next[flow.OutputNext] != wait.ID {
		t.Fatalf("Add after puts the move between the start and the wait: %+v", f.workflow().Nodes())
	}
	rest := ui.NewInput(f.at("palette:" + flow.KindLog))
	for range 40 {
		f.frame(rest)
	}
	if !f.has("tooltip=" + flow.KindLog) {
		t.Fatal("resting on Log shows its tooltip")
	}
	f.frame(noInput())
	if f.has("tooltip=" + flow.KindLog) {
		t.Fatal("moving away hides it")
	}
}
