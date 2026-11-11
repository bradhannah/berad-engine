package window

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestOpenDocumentsAreToldApart gives each open document's tab its own
// tint, whatever its kind, and none to the project browser's; keeps a tab's
// tint while it stays open; gives a newly opened tab the first tint no open
// tab has, and the one fewest have once every tint is taken.
func TestOpenDocumentsAreToldApart(t *testing.T) {
	t.Parallel()
	workflow := func() *tab { return &tab{panes: []*pane{{view: "workflow"}}} }
	browser := &tab{panes: []*pane{{view: "browser", host: &host{closeQuits: true}}}}
	shown := &shell{tabs: []*tab{browser, workflow(), workflow(), workflow()}}
	shown.tintTabs()
	if shown.tabs[1].tint != 1 || shown.tabs[2].tint != 2 || shown.tabs[3].tint != 3 || shown.tabs[0].tint != 0 {
		t.Fatalf("each document's tab has its own tint, the browser's none: %d %d %d %d", shown.tabs[0].tint,
			shown.tabs[1].tint, shown.tabs[2].tint, shown.tabs[3].tint)
	}
	shown.tabs = append(shown.tabs, &tab{panes: []*pane{{view: "map"}}})
	shown.tintTabs()
	if shown.tabs[4].tint != 4 {
		t.Fatalf("a map takes a tint no workflow has: %d", shown.tabs[4].tint)
	}
	shown.tabs = shown.tabs[:4]
	shown.tabs = append(shown.tabs[:2], shown.tabs[3:]...)
	shown.tabs = append(shown.tabs, workflow())
	shown.tintTabs()
	if shown.tabs[1].tint != 1 || shown.tabs[2].tint != 3 || shown.tabs[3].tint != 2 {
		t.Fatalf("a new tab takes the tint a closed one left: %d %d %d", shown.tabs[1].tint, shown.tabs[2].tint,
			shown.tabs[3].tint)
	}
	for range len(ui.Tints()) {
		shown.tabs = append(shown.tabs, workflow())
	}
	shown.tintTabs()
	if last := shown.tabs[len(shown.tabs)-1].tint; last < 1 || last > len(ui.Tints()) {
		t.Fatalf("with every tint taken a tab still has one: %d", last)
	}
	if tint, tinted := shown.tintOf(1); !tinted || tint.Name != ui.Tints()[0].Name {
		t.Fatalf("a tab's tint by number: %+v", tint)
	}
}
