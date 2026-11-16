package workflowview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui/pen"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// At 150% for text and controls, the canvas keeps its zoom: a node is as
// large on the screen as at 100%, so a drag of 40 window points, which is
// 60 at 100%, moves it 60 canvas points (increment 56).
//
//nolint:paralleltest // It sets the window's scale.
func TestTheScaleLeavesTheWorkflowsZoomAlone(t *testing.T) {
	t.Cleanup(func() { pen.SetScale(1, 1) })
	pen.SetScale(1, 1.5)
	f := newFixture(t)
	if f.state.ZoomPercent() != 100 {
		t.Fatalf("the scale zoomed the canvas to %d%%", f.state.ZoomPercent())
	}
	f.click("node:start:1")
	f.add("" + flow.KindWait)
	wait := f.nth(flow.KindWait, 1)
	before, _ := f.workflow().Box(wait.ID)
	from := f.at("node:wait:1")
	f.drag(from, from.Add(image.Pt(40, 20)))
	after, _ := f.workflow().Box(wait.ID)
	if after.X != before.X+60 || after.Y != before.Y+30 {
		t.Fatalf("at 150%% a 40×20 drag moved the node from %v to %v", before, after)
	}
}
