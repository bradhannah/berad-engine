package window

import (
	"image"
	"testing"
)

// TestASplitHasOneStatusBar gives both panes of a split the tab's whole
// area for the status bar, from each pane's corner, and a pane alone its
// own (increment 52).
func TestASplitHasOneStatusBar(t *testing.T) {
	t.Parallel()
	left, _ := pickingPane("Map: Town")
	right, _ := pickingPane("Watch: Town")
	alone, _ := pickingPane("Workflow: Notes")
	split := &tab{panes: []*pane{left, right}, focus: 1, split: defaultSplit, tint: 0}
	shown := &shell{tabs: []*tab{split, {panes: []*pane{alone}}}, size: image.Pt(1200, 800)}
	shown.layout()
	area := shown.area()
	if left.host.statusArea != area.Sub(left.rect.Min) || right.host.statusArea != area.Sub(right.rect.Min) {
		t.Fatalf("both panes of a split share the tab's status bar: %v %v", left.host.statusArea, right.host.statusArea)
	}
	if right.host.statusArea.Min.X >= 0 || right.host.statusArea.Dx() != area.Dx() {
		t.Fatalf("the right pane's bar starts at the tab's left edge: %v", right.host.statusArea)
	}
	if !alone.host.statusArea.Empty() {
		t.Fatal("a pane alone has its own status bar")
	}
}
