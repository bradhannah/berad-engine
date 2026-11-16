package window

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TestWatchOpensInItsOwnTabWithoutRoom opens Watch for a Map's zone in a
// window 800 points wide, as a 1200 × 832 window is at 150%: beside the Map
// both would need more than the window has, so Watch opens in a tab of its
// own and says why and what makes room. In a window wide enough it opens
// beside the Map (increment 61).
func TestWatchOpensInItsOwnTabWithoutRoom(t *testing.T) {
	t.Parallel()
	townMap, _ := panelPane("map")
	townWatch, _ := panelPane("watch")
	town := ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")
	townMap.target, townWatch.target = town, town
	shown := &shell{tabs: []*tab{{panes: []*pane{townMap}}}, active: 0, size: image.Pt(800, 555), textScale: 1.5}
	shown.place(townWatch)
	if len(shown.tabs) != 2 || len(shown.tabs[0].panes) != 1 || shown.active != 1 {
		t.Fatalf("Watch opened beside a Map without room: %d tabs", len(shown.tabs))
	}
	refusal := townWatch.host.frame.Refusal()
	if !strings.Contains(refusal, "a window 2142 wide, and this one is 1200") || !strings.Contains(refusal, "Cmd+I") {
		t.Fatalf("Watch says why it is in its own tab, in the screen's points: %q", refusal)
	}
	wide := &shell{tabs: []*tab{{panes: []*pane{townMap}}}, active: 0, size: image.Pt(1500, 900), textScale: 1}
	besideWatch, _ := panelPane("watch")
	besideWatch.target = townMap.target
	wide.place(besideWatch)
	if len(wide.tabs) != 1 || len(wide.tabs[0].panes) != 2 {
		t.Fatalf("Watch did not open beside its Map in a wide window: %d tabs", len(wide.tabs))
	}
}

// TestTheDividerKeepsEachPanesRoom drags a split's divider far to the left:
// it stops where the left pane keeps its panel, inspector and the canvas's
// room (increment 61).
func TestTheDividerKeepsEachPanesRoom(t *testing.T) {
	t.Parallel()
	townMap, _ := panelPane("map")
	townWatch, _ := panelPane("watch")
	each := &tab{panes: []*pane{townMap, townWatch}, focus: 0, split: minSplit, tint: 0}
	area := image.Rect(0, 32, 1500, 832)
	layoutTab(each, area)
	if got, want := townMap.rect.Dx(), paneNeed(townMap); got < want {
		t.Fatalf("the left pane is %d wide, less than the %d it needs", got, want)
	}
	if got, want := townWatch.rect.Dx(), paneNeed(townWatch); got < want {
		t.Fatalf("the right pane is %d wide, less than the %d it needs", got, want)
	}
}

// TestALargerTextStepIsRefusedWithoutRoom asks for 175% in a window 1200
// of the screen's points wide at 125%, where a Map's panels and canvas need
// 710 of the window's points: at 175% the window would have 685, so the
// step is refused, saying why and what makes room, and the scale stays; at
// 150% it has 800, and the step is taken (increment 61).
func TestALargerTextStepIsRefusedWithoutRoom(t *testing.T) {
	t.Parallel()
	townMap, _ := panelPane("map")
	shown := &shell{
		tabs: []*tab{{panes: []*pane{townMap}}}, folded: map[string]bool{}, leftFolded: map[string]bool{},
		widths: map[string]kindWidths{}, screen: "1512x982", known: map[string]bool{}, textScale: 1.25,
		size: image.Pt(960, 800),
	}
	townMap.host.frame.AskTextScale(1.75)
	shown.followTextScale()
	refusal := townMap.host.frame.Refusal()
	if shown.textScale != 1.25 || !strings.HasPrefix(refusal, "Text and controls stay at 125%: at 175%") ||
		!strings.Contains(refusal, "Cmd+I") {
		t.Fatalf("a step past the room was taken: %v, %q", shown.textScale, refusal)
	}
	townMap.host.frame.AskTextScale(1.5)
	shown.followTextScale()
	if shown.textScale != 1.5 {
		t.Fatalf("a step with room was refused: %v", shown.textScale)
	}
}

// canvasOnly is a view with no inspector, as Watch is.
type canvasOnly struct{ pickingView }

func (*canvasOnly) HasInspector() bool { return false }

// TestWatchFitsBesideItsMapAt100 opens Watch, which has no inspector, beside
// a Map in a window 1200 points wide, as at 100%: the two need the Map's
// panels and two canvases' room, which the window has. In a window 800 wide
// they do not fit, and Watch opens in a tab of its own (increment 61).
func TestWatchFitsBesideItsMapAt100(t *testing.T) {
	t.Parallel()
	for _, wide := range []bool{true, false} {
		townMap, _ := panelPane("map")
		townWatch := titled("watch", "Watch: Town")
		townWatch.host = &host{view: &canvasOnly{pickingView: pickingView{selected: ""}}, frame: frame.NewFrame(time.Now)}
		town := ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")
		townMap.target, townWatch.target = town, town
		size := image.Pt(1200, 800)
		if !wide {
			size = image.Pt(800, 555)
		}
		shown := &shell{tabs: []*tab{{panes: []*pane{townMap}}}, active: 0, size: size, textScale: 1}
		shown.place(townWatch)
		if beside := len(shown.tabs) == 1; beside != wide {
			t.Fatalf("in a window %v wide, Watch beside its Map: %v", size.X, beside)
		}
	}
}
