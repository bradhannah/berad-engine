package mapview_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/views/mapview"
)

var layout = mapview.Layout{Width: 1200, Height: 800}

func cellCenter(state *mapview.State, col, row int) (int, int) {
	rect := state.CellRect(col, row, layout)
	return rect.Min.X + rect.Dx()/2, rect.Min.Y + rect.Dy()/2
}

func TestStrokePaintsEachCellOnceAsOneGesture(t *testing.T) {
	t.Parallel()
	state := mapview.New("w1")
	state.SetFloorSize(32, 24)
	state.Update(mapview.Input{TileKey: 3}, layout) // wall

	x, y := cellCenter(state, 2, 2)
	actions := state.Update(mapview.Input{CursorX: x, CursorY: y, Down: true, Pressed: true}, layout)
	if len(actions.Paints) != 1 || actions.Paints[0].Tile != "wall" {
		t.Fatalf("press: %+v", actions.Paints)
	}
	gesture := actions.Paints[0].Gesture

	actions = state.Update(mapview.Input{CursorX: x + 1, CursorY: y, Down: true}, layout)
	if len(actions.Paints) != 0 {
		t.Fatal("same cell painted twice")
	}
	x, y = cellCenter(state, 3, 2)
	actions = state.Update(mapview.Input{CursorX: x, CursorY: y, Down: true}, layout)
	if len(actions.Paints) != 1 || actions.Paints[0].Col != 3 || actions.Paints[0].Gesture != gesture {
		t.Fatalf("drag: %+v", actions.Paints)
	}
	state.Update(mapview.Input{CursorX: x, CursorY: y, Released: true}, layout)
	actions = state.Update(mapview.Input{CursorX: x, CursorY: y, Down: true, Pressed: true}, layout)
	if actions.Paints[0].Gesture == gesture {
		t.Fatal("a new stroke needs a new gesture")
	}
}

func TestPaletteClickSelectsWithoutPainting(t *testing.T) {
	t.Parallel()
	state := mapview.New("w1")
	state.SetFloorSize(8, 8)
	rect := mapview.PaletteRect(4)
	actions := state.Update(mapview.Input{CursorX: rect.Min.X + 2, CursorY: rect.Min.Y + 2, Down: true, Pressed: true}, layout)
	if len(actions.Paints) != 0 || state.Tile().Name != "water" {
		t.Fatalf("tile %q paints %v", state.Tile().Name, actions.Paints)
	}
}

func TestOutsideFloorDoesNotPaint(t *testing.T) {
	t.Parallel()
	state := mapview.New("w1")
	state.SetFloorSize(2, 2)
	x, y := cellCenter(state, 5, 5)
	actions := state.Update(mapview.Input{CursorX: x, CursorY: y, Down: true, Pressed: true}, layout)
	if len(actions.Paints) != 0 || state.Hover().OK {
		t.Fatal("painted outside the floor")
	}
}

func TestKeysPassThrough(t *testing.T) {
	t.Parallel()
	state := mapview.New("w1")
	actions := state.Update(mapview.Input{Undo: true, NewView: true}, layout)
	if !actions.Undo || !actions.NewView || actions.Redo {
		t.Fatalf("actions %+v", actions)
	}
}
