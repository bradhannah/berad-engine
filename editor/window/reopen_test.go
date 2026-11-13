package window

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TestTheWindowReopensAsItWasLeft plans the kept tabs, leaving out the
// Project window and Play, follows each pane as it opens or is not found,
// and puts the split, the tab in front and what could not be found back
// (increment 55).
func TestTheWindowReopensAsItWasLeft(t *testing.T) {
	t.Parallel()
	town, notes := ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC"), ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHD")
	project, _ := pickingPane("Project")
	project.view = "browser"
	shown := &shell{tabs: []*tab{{panes: []*pane{project}}}}
	shown.reopenAsLeft(protocol.Arrangement{
		Kinds: nil, Frame: protocol.WindowFrame{X: 0, Y: 0, Width: 0, Height: 0}, Active: 1,
		Tabs: []protocol.TabLayout{
			{Panes: []protocol.PaneLayout{{View: "browser", Target: ids.ULID{}}}, Split: 0},
			{Panes: []protocol.PaneLayout{{View: "map", Target: town}, {View: "watch", Target: town}}, Split: 0.4},
			{Panes: []protocol.PaneLayout{{View: "workflow", Target: notes}}, Split: 0},
			{Panes: []protocol.PaneLayout{{View: "play", Target: ids.ULID{}}}, Split: 0},
		},
	})
	plan := shown.reopen
	if plan == nil || len(plan.steps) != 3 || plan.steps[0].pane.View != "map" || plan.steps[1].first {
		t.Fatalf("the Map, Watch beside it and the Workflow, but not the Project or Play: %+v", plan)
	}
	townMap, _ := pickingPane("Map: Town")
	townMap.view, townMap.target = "map", town
	townWatch, _ := pickingPane("Watch: Town")
	townWatch.view, townWatch.target = "watch", town
	split := &tab{panes: []*pane{townMap, townWatch}, focus: 0, split: defaultSplit, tint: 0}
	shown.tabs = append(shown.tabs, split)
	plan.waiting = true
	if !shown.reopenSettled(plan) || plan.tabs[1] != split || plan.at != 1 {
		t.Fatal("the Map is found once open")
	}
	plan.waiting = true
	shown.reopenSettled(plan)
	plan.waiting, plan.failed = true, true
	if !shown.reopenSettled(plan) || len(plan.missing) != 1 || plan.missing[0] != "a Workflow" {
		t.Fatalf("a Workflow that could not be opened is noted: %v", plan.missing)
	}
	shown.finishReopening(plan)
	if shown.active != 1 || split.split != 0.4 || shown.reopen != nil {
		t.Fatalf("the split and the tab in front are put back: tab %d, share %v", shown.active, split.split)
	}
	if text, _ := townMap.host.frame.Message(); !strings.Contains(text, "Could not find a Workflow") {
		t.Fatalf("what could not be found is said: %q", text)
	}
}
