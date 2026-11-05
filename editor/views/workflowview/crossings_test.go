package workflowview_test

import (
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

func TestANodeDroppedOnALineIsLooseAndCrossed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.add("" + flow.KindMove)
	f.drag(f.at("palette:"+flow.KindWait), f.at("line:start:1:next"))
	if !f.has("loose:wait:1") || !f.has("crossed:wait:1") {
		t.Fatal("the dropped wait is not linked, and the start's line runs under it")
	}
	if f.has("loose:move:1") || f.has("crossed:move:1") {
		t.Fatal("the move the line leads to is linked, and the line ends at it")
	}
}
