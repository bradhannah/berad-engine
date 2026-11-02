package workflowview_test

import (
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestGoToAPlaceValue makes a Go to go to a place value, which it needs
// chosen, and picks a place local for it; the canvas reads Go to and the
// local's path, and the anchor's row comes back when it goes to a place on
// a map again.
func TestGoToAPlaceValue(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("Escape")
	f.click("ui:add-local")
	f.click("ui:value-type-place")
	f.click("node:start:1")
	f.click("palette:" + flow.KindGoTo)
	f.click("ui:goes-value")
	if _, shown := f.state.Locate("ui:pick", f.docs); shown {
		t.Fatal("going to a place value, it shows no anchor to pick")
	}
	if !f.has("problem:go-to:1=No place value chosen yet. Choose a place local, or pick a place on a map.") {
		t.Fatal("a Go to to a place value needs one chosen")
	}
	f.click("ui:pick-place")
	if !f.shown("local.value") || !f.has("picker-hidden=0") {
		t.Fatal("the picker offers the place local only")
	}
	f.click("valuerow:local.value")
	goTo := f.nth(flow.KindGoTo, 1)
	if ref, err := goTo.Ref(flow.ParamPlace); err != nil || ref.Scope != worldstate.ScopeLocal {
		t.Fatalf("it goes to local.value: %v %v", ref, err)
	}
	if !f.has("title:go-to:1=Go to · local.value") || f.has("problem:go-to:1") {
		t.Fatal("the canvas reads Go to local.value, with nothing wrong")
	}
	f.click("ui:goes-map")
	if _, shown := f.state.Locate("ui:pick", f.docs); !shown {
		t.Fatal("going to a place on a map, its anchor shows again")
	}
}
