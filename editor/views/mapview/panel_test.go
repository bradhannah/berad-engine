package mapview_test

import (
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/views/mapview"
)

// TestToolsAreGroupedByJob lists the tools under Select, Paint, Add and
// Floor, each with its key as a cap, and the tool in hand as the list's one
// filled row (increment 66). A tool is still taken in hand by a click or by
// its key, and the headings take no click.
func TestToolsAreGroupedByJob(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	rows, inHand := f.state.ToolRows()
	want := []string{
		"# SELECT", "select=Select [S]",
		"# PAINT", "ground=Ground [G]", "walls=Walls [W]", "objects=Doors, stairs, pickups [O]", "erase=Erase [E]",
		"# ADD", "anchor=Anchor [A]", "npc=NPC [N]",
		"# FLOOR", "resize=Resize floor [R]",
	}
	if !slices.Equal(rows, want) || !inHand {
		t.Fatalf("the tools, grouped, with the one in hand filled: %v %v", rows, inHand)
	}
	f.click("tool:walls")
	if f.state.Tool() != mapview.ToolWalls {
		t.Fatal("a click takes a tool in hand")
	}
	f.keys("N")
	if f.state.Tool() != mapview.ToolNPC {
		t.Fatal("a tool's key takes it in hand")
	}
	if label, key := f.state.PreviewButton(); label != "Preview" || key != "P" {
		t.Fatalf("Preview shows its key as a cap: %q %q", label, key)
	}
}

// widePanel is a left panel dragged wide enough for two of the ground's
// tiles side by side.
const widePanel = 240

// TestWhatAToolPaintsWithIsChips lays the ground's tiles out as chips: one
// to a line in a panel as wide as it starts, which is as wide as it was
// before, and two to a line once the panel is dragged wide enough for every
// name. Then the doors, stairs and pickups are still one to a line, since
// "Medium pickup" does not fit beside another, and the wall families two
// to a line under shorter names. Select offers none until walls are
// selected, and the NPCs to place stay a list.
func TestWhatAToolPaintsWithIsChips(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("G")
	f.keys()
	if columns, names := f.state.TileChips(); columns != 1 || !slices.Contains(names, "Unassigned") {
		t.Fatalf("in a panel as wide as it starts, one to a line, their names whole: %d %v", columns, names)
	}
	f.state.SetLeftWidth(widePanel)
	f.keys()
	if columns, names := f.state.TileChips(); columns != 2 || !slices.Contains(names, "Grass") {
		t.Fatalf("the ground's tiles, two to a line: %d %v", columns, names)
	}
	f.keys("3")
	f.press("cell:9,8", false)
	f.release("cell:9,8")
	if cells := paints(t, f.requests); len(cells) != 1 || cells[0].Tile != "road" {
		t.Fatalf("a number key still chooses a tile: %+v", cells)
	}
	f.keys("O")
	f.keys()
	if columns, names := f.state.TileChips(); columns != 1 || !slices.Contains(names, "Medium pickup") {
		t.Fatalf("doors, stairs and pickups, one to a line, their names whole: %d %v", columns, names)
	}
	f.keys("W")
	f.keys()
	if columns, names := f.state.TileChips(); columns != 2 || !slices.Contains(names, "Family A") ||
		!slices.Contains(names, "No family") {
		t.Fatalf("the wall families, two to a line, without the word their heading says: %d %v", columns, names)
	}
	f.keys("S")
	f.keys()
	if columns, names := f.state.TileChips(); columns != 0 || len(names) != 0 {
		t.Fatalf("Select offers nothing to paint with until walls are selected: %d %v", columns, names)
	}
	f.click("cell:2,4")
	f.keys()
	if columns, names := f.state.TileChips(); columns != 2 || len(names) == 0 {
		t.Fatalf("with walls selected, the families to change them to: %d %v", columns, names)
	}
	f.keys("N")
	f.keys()
	if columns, _ := f.state.TileChips(); columns != 0 {
		t.Fatalf("the NPCs to place stay a list: %d", columns)
	}
}
