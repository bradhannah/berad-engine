package window

import (
	"errors"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
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
