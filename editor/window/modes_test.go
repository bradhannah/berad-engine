package window

import (
	"image"
	"testing"
)

// previewingView is a panelView that previews, as a Map does.
type previewingView struct {
	panelView

	previewing bool
}

func (v *previewingView) Previewing() bool { return v.previewing }

// TestAMapKeepsAnArrangementForPreviewing folds a Map's inspector while it
// previews: it unfolds as it was when the preview stops, and folds again
// when the next one starts (increment 54).
func TestAMapKeepsAnArrangementForPreviewing(t *testing.T) {
	t.Parallel()
	shown, _ := panelPane("map")
	view := &previewingView{panelView: panelView{pickingView: pickingView{selected: ""}, left: 0}, previewing: false}
	shown.host.view = view
	window := &shell{
		tabs: []*tab{{panes: []*pane{shown}}}, folded: map[string]bool{}, leftFolded: map[string]bool{},
		widths: map[string]kindWidths{"map": {inspector: 300, left: 0}}, known: map[string]bool{"map": true},
		keptArrived: true,
	}
	window.followInspector()
	view.previewing = true
	window.followInspector()
	shown.host.frame.ShowInspector.On = false
	window.followInspector()
	view.previewing = false
	window.followInspector()
	if !shown.host.frame.ShowInspector.On || shown.host.inspectorWidth != 300 {
		t.Fatal("stopping the preview puts the editing arrangement back")
	}
	view.previewing = true
	window.followInspector()
	if shown.host.frame.ShowInspector.On {
		t.Fatal("the next preview puts the previewing arrangement back")
	}
}

// TestTheScreenChoosesStartingWidths starts panels narrower on a small
// screen, wider on a large one, and at the views' own widths between
// (increment 54).
func TestTheScreenChoosesStartingWidths(t *testing.T) {
	t.Parallel()
	if got := startingWidths(image.Pt(1440, 900)); got != (kindWidths{inspector: narrowInspector, left: narrowLeft}) {
		t.Fatalf("a small screen: %+v", got)
	}
	if got := startingWidths(image.Pt(1920, 1080)); got != (kindWidths{inspector: 0, left: 0}) {
		t.Fatalf("a middling screen: %+v", got)
	}
	if got := startingWidths(image.Pt(2560, 1440)); got != (kindWidths{inspector: wideInspector, left: wideLeft}) {
		t.Fatalf("a large screen: %+v", got)
	}
}

// TestTheWindowStopsNarrowing needs room for each pane's panels and canvas,
// and the folded strip for a folded inspector (increment 54).
func TestTheWindowStopsNarrowing(t *testing.T) {
	t.Parallel()
	shown, _ := panelPane("map")
	shown.host.inspectorWidth = 300
	front := &tab{panes: []*pane{shown}}
	if need := neededWidth(front); need != 190+300+minCanvas {
		t.Fatalf("the left panel, the inspector and the canvas: %d", need)
	}
	shown.host.frame.ShowInspector.On = false
	if need := neededWidth(front); need != 190+24+minCanvas {
		t.Fatalf("a folded inspector leaves its strip: %d", need)
	}
}
