package workflowview_test

import (
	"slices"
	"strings"
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// wayOf is the nth Return's way out.
func (f *fixture) wayOf(number int) flow.WayOut {
	f.t.Helper()
	way, err := f.nth(flow.KindReturn, number).WayOut()
	if err != nil {
		f.t.Fatal(err)
	}

	return way
}

// TestReturns adds Returns, each a way out of its own, names one as
// another to join it and renames it away again, keeping a lone Return's
// port, refuses done as a name and flags a Return inside a container.
func TestReturns(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindReturn)
	f.drag(f.at("palette:"+flow.KindReturn), f.at("canvas:500,400"))
	if first, second := f.wayOf(1), f.wayOf(2); first != (flow.WayOut{Port: "w1", Label: "finished"}) ||
		second != (flow.WayOut{Port: "w2", Label: "finished 2"}) {
		t.Fatalf("each new Return is a way out of its own: %v %v", first, second)
	}
	f.click("node:return:2")
	f.typeInto("param-name", "finished")
	if way := f.wayOf(2); way.Port != "w1" || len(f.workflow().WaysOut()) != 1 {
		t.Fatalf("a Return named as another joins its way out: %v, %v", way, f.workflow().WaysOut())
	}
	f.typeInto("param-name", "Closed")
	want := []flow.WayOut{{Port: "w1", Label: "finished"}, {Port: "w2", Label: "Closed"}}
	if got := f.workflow().WaysOut(); !slices.Equal(got, want) {
		t.Fatalf("renamed away from a shared way out, it takes a port of its own: %v", got)
	}
	f.click("node:return:1")
	f.typeInto("param-name", "Arrived")
	if way := f.wayOf(1); way != (flow.WayOut{Port: "w1", Label: "Arrived"}) {
		t.Fatalf("a lone Return keeps its port when renamed, so callers keep their lines: %v", way)
	}
	f.typeInto("param-name", "done")
	if !strings.Contains(f.status(), "not done") || f.wayOf(1).Label != "Arrived" {
		t.Fatalf("done is refused as a name: %q", f.status())
	}
	f.click("ui:revert")
	f.drag(f.at("palette:"+flow.KindContainer), f.at("canvas:300,600"))
	f.click("palette:" + flow.KindReturn)
	if !f.has("problem:return:3=A Return goes at the top level. Inside a container it is not a way out " +
		"until containers get named exits.") {
		t.Fatal("a Return inside a container is flagged")
	}
	if len(f.workflow().WaysOut()) != 2 {
		t.Fatalf("a Return inside a container is not a way out: %v", f.workflow().WaysOut())
	}
}
