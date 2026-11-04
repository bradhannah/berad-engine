package window

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestTabsOfAKindAreToldApart gives each open tab of a kind its own tint,
// keeps a tab's tint while it stays open, gives a newly opened tab the
// first tint none of its kind has, and the one fewest have once every tint
// is taken.
func TestTabsOfAKindAreToldApart(t *testing.T) {
	t.Parallel()
	workflow := func() *tab { return &tab{panes: []*pane{{view: "workflow"}}} }
	shown := &shell{tabs: []*tab{{panes: []*pane{{view: "browser"}}}, workflow(), workflow(), workflow()}}
	shown.tintTabs()
	if shown.tabs[1].tint != 1 || shown.tabs[2].tint != 2 || shown.tabs[3].tint != 3 || shown.tabs[0].tint != 1 {
		t.Fatalf("each tab of a kind has its own tint: %d %d %d", shown.tabs[1].tint, shown.tabs[2].tint,
			shown.tabs[3].tint)
	}
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
