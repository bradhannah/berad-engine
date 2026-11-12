package workflowview_test

import (
	"strings"
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestTheInspectorWritesOutANodesLinks lists the line joining a Wait from
// the start and where its output goes, each with Unlink; Unlink takes the
// line away and says so, and the output then says the flow goes back to the
// start, as the canvas does (increment 48).
func TestTheInspectorWritesOutANodesLinks(t *testing.T) {
	t.Parallel()
	f := withThree(t)
	f.click("node:" + flow.KindWait + ":1")
	if !f.has("link=From Start") || !f.has("link=then → Say") {
		t.Fatal("the inspector writes out what joins to the Wait and where it goes")
	}
	f.click("unlink:" + flow.KindWait + ":1:" + flow.OutputNext)
	if !f.has("link=then → start") || f.has("link=then → Say") {
		t.Fatal("Unlink takes the line away")
	}
	if !strings.HasPrefix(f.status(), "Unlinked Wait · next from Say") {
		t.Fatalf("and says what from what: %q", f.status())
	}
	f.click("unlink:start:1:" + flow.OutputNext)
	if f.has("link=From Start") {
		t.Fatal("the line from the start is unlinked from the Wait's inspector too")
	}
}
