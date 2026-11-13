package mapview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestTheStatusBarNamesTheToolInHand names the floor, the tool and its tile,
// and the cell under the pointer, in the status bar; a tool that paints
// nothing names no tile (increment 39).
func TestTheStatusBarNamesTheToolInHand(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.frame(ui.NewInput(f.at("cell:2,3")))
	f.frame(ui.NewInput(f.at("cell:2,3")))
	for _, words := range []string{"Ground · Ground · Grass", "cell 2, 3"} {
		if _, said := f.ctx.Frame.LocateStatus("state:hand:" + words); !said {
			t.Fatalf("the status bar names %q", words)
		}
	}
	f.keys("A")
	f.frame(ui.NewInput(f.at("cell:2,3")))
	if _, said := f.ctx.Frame.LocateStatus("state:hand:Ground · Anchors · cell"); !said {
		t.Fatal("Anchors paints with no tile")
	}
}
