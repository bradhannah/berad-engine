package workflowview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestTheStatusBarNamesWhatIsInHand names the workflow, the node selected
// and what the pointer is over, or that Tab adds a node on empty canvas
// (increment 44).
func TestTheStatusBarNamesWhatIsInHand(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.drag(f.at("palette:"+flow.KindWait), f.at("canvas:400,300"))
	f.click("node:start:1")
	f.frame(ui.NewInput(f.at("node:wait:1")))
	for _, words := range []string{"Start selected", "over Wait"} {
		if _, said := f.ctx.Frame.LocateStatus("state:hand:" + words); !said {
			t.Fatalf("the status bar says %q", words)
		}
	}
	f.frame(ui.NewInput(f.at("canvas:700,150")))
	if _, said := f.ctx.Frame.LocateStatus("state:hand:Tab adds a node here"); !said {
		t.Fatal("over empty canvas, it says Tab adds a node")
	}
}
