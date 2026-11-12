package workflowview_test

import (
	"strings"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestAWorkflowSaysWhichNode says an added node by its kind, a line joined
// or unlinked by its output, and a setting changed by its name
// (increment 51).
func TestAWorkflowSaysWhichNode(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindWait)
	added := f.requests.Of(workflowplugin.AddEdit)
	if says := added[len(added)-1].Command.Says; says != "Added a Wait to Daily routine" {
		t.Fatalf("an added node is said by its kind: %q", says)
	}
	f.typeInto("param-duration", "45")
	replaced := f.requests.Of(workflowplugin.ReplaceEdit)
	if says := replaced[len(replaced)-1].Command.Says; !strings.HasPrefix(says, "Changed Wait's ") {
		t.Fatalf("a setting is said by its name: %q", says)
	}
	f.click("node:start:1")
	f.click("unlink:start:1:" + flow.OutputNext)
	replaced = f.requests.Of(workflowplugin.ReplaceEdit)
	if says := replaced[len(replaced)-1].Command.Says; says != "Unlinked Start · then from Wait" {
		t.Fatalf("an unlinked line is said by its output: %q", says)
	}
}
