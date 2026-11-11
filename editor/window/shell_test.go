package window

import (
	"errors"
	"image"
	"strconv"
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
	first, second, third := &pane{view: "browser"}, &pane{view: "map"}, &pane{view: "workflow"}
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

// TestOneRowOfTabs lays every open tab out on one row, in the order they
// opened, each named by its window and two views of one map told apart by
// their IDs (increment 38).
func TestOneRowOfTabs(t *testing.T) {
	t.Parallel()
	project := titled("browser", "Project")
	town := titled("map", "Map: Town — Ground  (view aaaaaa)")
	notes := titled("workflow", "Workflow: Notes  (window bbbbbb)")
	harbour := titled("map", "Map: Harbour  (view cccccc)")
	townAgain := titled("map", "Map: Town — Ground  (view dddddd)")
	shown := &shell{size: image.Pt(1200, 832), tabs: []*tab{
		{panes: []*pane{project}}, {panes: []*pane{town}}, {panes: []*pane{notes}},
		{panes: []*pane{harbour}}, {panes: []*pane{townAgain}},
	}, active: 2}
	if shown.barHeight() != rowHeight {
		t.Fatal("the bar is one row")
	}
	tabs := shown.barTabs()
	want := []string{"Project", "Town — Ground  (view aaaaaa)", "Notes", "Harbour", "Town — Ground  (view dddddd)"}
	if len(tabs) != len(want) {
		t.Fatalf("one tab each: %+v", tabs)
	}
	room := 1200 - readoutWidth - launchWidth
	for index, label := range want {
		if tabs[index].label != label || tabs[index].tab != index || tabs[index].front != (index == 2) {
			t.Fatalf("tab %d: %+v", index, tabs[index])
		}
	}
	if width := tabs[0].rect.Dx(); width != min(tabWidth, room/len(want)) || tabs[4].rect.Max.X > room {
		t.Fatalf("tabs share the room: %d wide, last ends at %d of %d", width, tabs[4].rect.Max.X, room)
	}
	shown.pressBar(ui.Center(tabs[3].rect))
	if shown.active != 3 {
		t.Fatalf("a press on a tab brings it forward: %d", shown.active)
	}
}

// TestTabsThatDoNotFitGoUnderMore shrinks tabs to their least width, then
// puts the last of them under a "more" tab that counts them, is the tab in
// front while one of them is, and lists them (increment 38).
func TestTabsThatDoNotFitGoUnderMore(t *testing.T) {
	t.Parallel()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	shown := &shell{size: image.Pt(1200, 832), theme: theme, active: 8}
	for index := range 10 {
		shown.tabs = append(shown.tabs, &tab{panes: []*pane{titled("workflow", "Workflow: W"+strconv.Itoa(index))}})
	}
	room := 1200 - readoutWidth - launchWidth
	tabs := shown.barTabs()
	more := tabs[len(tabs)-1]
	fit := (room - moreWidth) / leastTab
	if len(tabs) != fit+1 || more.tab != moreTab || more.more != 10-fit || !more.front {
		t.Fatalf("%d fit beside more, which counts %d and is in front: %+v", fit, more.more, more)
	}
	if tabs[0].rect.Dx() < leastTab || more.rect.Max.X > room {
		t.Fatalf("tabs at least %d wide, more inside the room: %v %v", leastTab, tabs[0].rect, more.rect)
	}
	shown.pressBar(ui.Center(more.rect))
	if !shown.more.IsOpen() || len(shown.more.Items) != 10-fit || shown.more.Items[0].Label != "W"+strconv.Itoa(fit) {
		t.Fatalf("more lists the tabs that do not fit: %+v", shown.more.Items)
	}
	shown.size = image.Pt(2400, 832)
	if tabs := shown.barTabs(); len(tabs) != 10 {
		t.Fatalf("on a wider window every tab fits: %d", len(tabs))
	}
}

// TestKeysMoveBetweenTabs chooses tabs by position, the one last used and
// its neighbours, and says why when a key has nowhere to go (increment 38).
func TestKeysMoveBetweenTabs(t *testing.T) {
	t.Parallel()
	shown := &shell{tabs: []*tab{
		{panes: []*pane{titled("browser", "Project")}}, {panes: []*pane{titled("workflow", "Workflow: Notes")}},
		{panes: []*pane{titled("map", "Map: Town")}},
	}, active: 0}
	var said string
	status := func(message string) { said = message }
	press := func(key string) {
		input := ui.NewInput(image.Pt(0, 0))
		input.Presses = []ui.Press{ui.ParsePress(key)}
		shown.tabKeys(&input, status)
		shown.noteUsed()
		if len(input.Presses) != 0 {
			t.Fatalf("%s was left for the view", key)
		}
	}
	shown.noteUsed()
	press("Cmd+3")
	if !shown.frontNamed("Town") {
		t.Fatalf("Cmd+3 brings the third tab forward: %d", shown.active)
	}
	press("Ctrl+Tab")
	if shown.active != 0 {
		t.Fatalf("Ctrl+Tab goes back to the tab last used: %d", shown.active)
	}
	press("Ctrl+Tab")
	if shown.active != 2 {
		t.Fatalf("and again comes back: %d", shown.active)
	}
	press("Cmd+Shift+[")
	if shown.active != 1 {
		t.Fatalf("Cmd+Shift+[ goes to the tab before: %d", shown.active)
	}
	press("Cmd+Shift+]")
	press("Cmd+Shift+]")
	if shown.active != 2 || said != "This is the last tab" {
		t.Fatalf("past the last tab nothing moves and the status says why: %d %q", shown.active, said)
	}
	press("Cmd+7")
	if shown.active != 2 || said != "There is no tab 7: 3 tabs are open" {
		t.Fatalf("a position with no tab: %d %q", shown.active, said)
	}
	input := ui.NewInput(image.Pt(0, 0))
	input.Presses = []ui.Press{ui.ParsePress("Tab"), ui.ParsePress("Cmd+Alt+1")}
	shown.tabKeys(&input, status)
	if len(input.Presses) != 2 || shown.active != 2 {
		t.Fatalf("Tab and Cmd+Alt+1 are the view's: %v", input.Presses)
	}
}
