package mapview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
)

// At 150% for text and controls, a cell takes as much of the screen as at
// 100%, two thirds as many of the window's points, and a click paints the
// cell under it (increment 56).
//
//nolint:paralleltest // It sets the window's scale.
func TestTheScaleLeavesTheMapsCellsAlone(t *testing.T) {
	t.Cleanup(func() { pen.SetScale(1, 1) })
	at100 := newFixture(t)
	width := at100.at("cell:9,8").X - at100.at("cell:8,8").X
	pen.SetScale(1, 1.5)
	f := newFixture(t)
	if _, ok := f.state.Locate("state:zoom:100", f.ctx.Docs); !ok {
		t.Fatal("the scale changed the Map's zoom")
	}
	if got := f.at("cell:11,8").X - f.at("cell:8,8").X; got != 3*width*2/3 {
		t.Fatalf("three cells are %d of the window's points at 150%%, want %d", got, 3*width*2/3)
	}
	f.keys("3")
	f.press("cell:8,8", false)
	f.hold("cell:11,8")
	f.release("cell:11,8")
	cells := paints(t, f.requests)
	if len(cells) != 4 || cells[0].Layer != tilefloor.LayerGround || cells[0].Col != 8 || cells[3].Col != 11 {
		t.Fatalf("painted %+v", cells)
	}
}
