package window

import (
	"errors"
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TestSafelyTurnsAPanicIntoAnError catches a pane's panic, so the shell can
// close that pane alone, and passes its other results through.
func TestSafelyTurnsAPanicIntoAnError(t *testing.T) {
	t.Parallel()
	if err := safely(func() error { panic("boom") }); !errors.Is(err, errPaneCrashed) {
		t.Fatalf("panic gave %v", err)
	}
	if err := safely(func() error { return ebiten.Termination }); !errors.Is(err, ebiten.Termination) {
		t.Fatalf("termination gave %v", err)
	}
	if err := safely(func() error { return nil }); err != nil {
		t.Fatalf("a quiet tick gave %v", err)
	}
}

// TestRemovingATabKeepsTheTabInFront takes away a tab's only pane: the tab
// goes, and the tab in front stays in front, or the one before it comes
// forward when it was the one that went.
func TestRemovingATabKeepsTheTabInFront(t *testing.T) {
	t.Parallel()
	first, second, third := &pane{view: "browser"}, &pane{view: "map"}, &pane{view: "text"}
	shown := &shell{tabs: []*tab{{panes: []*pane{first}}, {panes: []*pane{second}}, {panes: []*pane{third}}}, active: 2}
	shown.remove(second)
	if len(shown.tabs) != 2 || shown.focused() != third {
		t.Fatalf("after removing a tab behind: %d tabs, front %v", len(shown.tabs), shown.focused())
	}
	shown.remove(third)
	if len(shown.tabs) != 1 || shown.focused() != first {
		t.Fatalf("after removing the tab in front: %d tabs, front %v", len(shown.tabs), shown.focused())
	}
	shown.remove(first)
	if len(shown.tabs) != 0 || shown.focused() != nil {
		t.Fatalf("after removing the last tab: %d tabs", len(shown.tabs))
	}
}

// TestWatchOpensBesideAMapOfItsZone puts Watch in the tab of a Map of its
// zone, which comes to the front with the Map still focused, and Watch of a
// zone with no Map open in a tab of its own.
func TestWatchOpensBesideAMapOfItsZone(t *testing.T) {
	t.Parallel()
	town, harbour := ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC"), ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHD")
	project, townMap := &pane{view: "browser"}, &pane{view: "map", target: town}
	shown := &shell{tabs: []*tab{{panes: []*pane{project}}, {panes: []*pane{townMap}}}, active: 0}
	townWatch := &pane{view: "watch", target: town}
	shown.place(townWatch)
	if len(shown.tabs) != 2 || shown.active != 1 || len(shown.tabs[1].panes) != 2 || shown.focused() != townMap {
		t.Fatalf("watch beside its map: %d tabs, front %d, focused %v", len(shown.tabs), shown.active, shown.focused())
	}
	shown.place(&pane{view: "watch", target: harbour})
	if len(shown.tabs) != 3 || shown.active != 2 {
		t.Fatalf("watch of another zone: %d tabs, front %d", len(shown.tabs), shown.active)
	}
	shown.remove(townWatch)
	if len(shown.tabs[1].panes) != 1 || shown.tabs[1].panes[0] != townMap {
		t.Fatal("closing watch leaves its map")
	}
}

// TestSplitDividerDragsAndFocuses lays a split tab out either side of its
// divider, drags the divider within its limits, and focuses the pane a press
// lands in.
func TestSplitDividerDragsAndFocuses(t *testing.T) {
	t.Parallel()
	left, right := &pane{view: "map"}, &pane{view: "watch"}
	shown := &shell{tabs: []*tab{{panes: []*pane{left, right}, split: 0.5}}, size: image.Pt(1000, 832)}
	shown.layout()
	if left.rect.Max.X != 496 || right.rect.Min.X != 504 || left.rect.Min.Y != rowHeight || right.rect.Max.X != 1000 {
		t.Fatalf("split rects %v %v", left.rect, right.rect)
	}
	press := ui.NewInput(image.Pt(500, 400))
	press.Pressed, press.Down = true, true
	shown.updateSplit(&press)
	drag := ui.NewInput(image.Pt(950, 400))
	drag.Down = true
	shown.updateSplit(&drag)
	if split := shown.tabs[0].split; split != maxSplit || !drag.Cursor.Eq(outside()) {
		t.Fatalf("dragged to %v, the panes got %v", split, drag.Cursor)
	}
	shown.updateSplit(&ui.Input{Cursor: image.Pt(950, 400)})
	shown.layout()
	click := ui.NewInput(image.Pt(900, 400))
	click.Pressed, click.Down = true, true
	shown.updateSplit(&click)
	if shown.focused() != right {
		t.Fatal("a press in the right pane focuses it")
	}
}

// titled is a pane of a view whose window has a title.
func titled(view, title string) *pane {
	place := newPaneSurface()
	place.title = title

	return &pane{view: view, place: place}
}

// TestTabsGroupByKind files tabs under their kinds in bar order, names a kind
// after its one window or counts several, shows the second row only for a
// kind in front with several, comes back to a kind's last tab, and tells two
// views of one map apart by their IDs.
func TestTabsGroupByKind(t *testing.T) {
	t.Parallel()
	project := titled("browser", "Project")
	town := titled("map", "Map: Town — Ground  (view aaaaaa)")
	notes := titled("text", "Text: Notes  (window bbbbbb)")
	harbour := titled("map", "Map: Harbour  (view cccccc)")
	townAgain := titled("map", "Map: Town — Ground  (view dddddd)")
	shown := &shell{size: image.Pt(1200, 832), tabs: []*tab{
		{panes: []*pane{project}}, {panes: []*pane{town}}, {panes: []*pane{notes}},
		{panes: []*pane{harbour}}, {panes: []*pane{townAgain}},
	}, active: 2}
	kinds := shown.kinds()
	if len(kinds) != 3 || kinds[0].view != "browser" || kinds[1].view != "map" || len(kinds[1].tabs) != 3 ||
		kinds[2].view != "text" {
		t.Fatalf("kinds %+v", kinds)
	}
	if shown.documentRow() || shown.barHeight() != rowHeight {
		t.Fatal("one text document in front needs no second row")
	}
	tabs := shown.barTabs()
	if len(tabs) != 3 || tabs[1].label != "Maps" || tabs[1].count != 3 || tabs[2].label != "Notes" || !tabs[2].front {
		t.Fatalf("top row %+v", tabs)
	}
	shown.active = 3
	shown.remember()
	shown.active = 2
	if got := shown.chooseKind(kinds[1]); got != 3 {
		t.Fatalf("choosing Maps brings back the last Map in front, tab %d", got)
	}
	shown.active = 3
	if !shown.documentRow() || shown.barHeight() != rowHeight*2 {
		t.Fatal("a Map in front, of three, shows the second row")
	}
	labels := shown.documentLabels(kinds[1])
	if labels[0] != "Town — Ground  (view aaaaaa)" || labels[1] != "Harbour" || labels[2] != "Town — Ground  (view dddddd)" {
		t.Fatalf("document labels %q", labels)
	}
}
