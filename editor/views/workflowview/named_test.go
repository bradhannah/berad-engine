package workflowview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestAHistoryLineNamesANode finds the node a change to the workflow
// touched while it is still there, and nothing for another workflow or a
// node taken out (increment 45).
func TestAHistoryLineNamesANode(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindLog)
	logged := f.nth(flow.KindLog, 1)
	touched := protocol.Touching(workflowID, []string{workflow.BoxItem(logged.ID), workflow.NodeItem(logged.ID)})
	item, name, named := f.state.Named(touched, f.ctx.Docs)
	if !named || item != workflowview.NodeItem+logged.ID.String() || name != "Log" {
		t.Fatalf("a line names the node its change touched: %q %q %v", item, name, named)
	}
	if _, _, other := f.state.Named(protocol.Touching(frametest.ID(7), touched.Items), f.ctx.Docs); other {
		t.Fatal("a change to another workflow names nothing here")
	}
	f.click("node:" + flow.KindLog + ":1")
	f.keys("Delete")
	if f.has("nodes:2") || !f.has("nodes:1") {
		t.Fatal("the Log is taken out")
	}
	if _, _, gone := f.state.Named(touched, f.ctx.Docs); gone {
		t.Fatal("a node taken out is not named")
	}
}
