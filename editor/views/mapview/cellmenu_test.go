package mapview_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// rightClick presses the right button at a place.
func (f *fixture) rightClick(name string) {
	input := ui.NewInput(f.at(name))
	input.Secondary = true
	f.frame(input)
}

func (f *fixture) menu() string { return strings.Join(f.state.MenuItems(), ", ") }

// TestRightClickMenuOffersWhatIsThere opens the menu on an anchor, a wall,
// an empty cell and a cell beyond the floor bounds, and nowhere on the zoom
// control; Escape and a click outside close it.
func TestRightClickMenuOffersWhatIsThere(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.rightClick("cell:4,4")
	if got := f.menu(); got != "rename-anchor, delete-anchor, add-anchor (disabled), place-npc (disabled), preview-here" {
		t.Fatalf("anchor menu: %s", got)
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.keys("Escape")
	if f.state.MenuItems() != nil {
		t.Fatal("Escape keeps the menu open")
	}
	f.rightClick("cell:2,2")
	want := "select-walls, add-anchor, place-npc (disabled), clear-top, preview-here (disabled)"
	if got := f.menu(); got != want {
		t.Fatalf("wall menu: %s", got)
	}
	f.click("ui:zoom-fit")
	if f.state.MenuItems() != nil {
		t.Fatal("a click outside keeps the menu open")
	}
	f.rightClick("cell:14,3")
	if got := f.menu(); got != "extend-floor" {
		t.Fatalf("outside menu: %s", got)
	}
	f.keys("Escape")
	f.rightClick("ui:zoom-fit")
	if f.state.MenuItems() != nil {
		t.Fatal("a right-click on the zoom control opened the menu")
	}
}

// TestRightClickMenuActions adds an anchor, renames and deletes one, clears
// a wall, selects walls, extends the floor and starts a preview, each from
// the menu.
func TestRightClickMenuActions(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.rightClick("cell:5,5")
	f.click("menu:add-anchor")
	if added := f.requests.Of(tilefloor.AddAnchorEdit); len(added) != 1 {
		t.Fatalf("add anchor commands %d", len(added))
	}
	f.rightClick("cell:4,4")
	f.click("menu:rename-anchor")
	if f.state.SelectedAnchor() != deskID || f.state.Focused() == nil {
		t.Fatal("rename selects the anchor and focuses its name")
	}
	f.keys("Escape")
	f.rightClick("cell:4,4")
	f.click("menu:delete-anchor")
	if len(f.requests.Uses) != 1 {
		t.Fatal("deleting asks where the anchor is used")
	}
	f.requests.Uses[0](nil, nil)
	if removed := f.requests.Of(tilefloor.RemoveAnchorEdit); len(removed) != 1 {
		t.Fatalf("remove anchor commands %d", len(removed))
	}
	f.rightClick("cell:3,2")
	f.click("menu:clear-top")
	var paint tilefloor.PaintArgs
	if painted := f.requests.Of(tilefloor.PaintEdit); len(painted) != 1 || painted[0].Decode(&paint) != nil ||
		!slices.Equal(paint.Cells, []tilefloor.Cell{{Col: 3, Row: 2, Layer: tilefloor.LayerTop, Tile: ""}}) {
		t.Fatalf("clear sent %+v", paint)
	}
	f.rightClick("cell:2,4")
	f.click("menu:select-walls")
	if f.state.Tool() != mapview.ToolSelect || len(f.state.Selected()) == 0 {
		t.Fatalf("select walls: tool %q, %d selected", f.state.Tool(), len(f.state.Selected()))
	}
	f.rightClick("cell:14,-1")
	f.click("menu:extend-floor")
	var resize tilefloor.ResizeArgs
	if resized := f.requests.Of(tilefloor.ResizeEdit); len(resized) != 1 || resized[0].Decode(&resize) != nil ||
		resize.Bounds != tilefloor.BoundsOf(image.Rect(0, -1, 15, 10)) {
		t.Fatalf("extend sent %+v", resize.Bounds)
	}
	f.rightClick("cell:9,8")
	f.click("menu:preview-here")
	if len(f.requests.Previews) != 1 {
		t.Fatal("Preview from here asks to start a preview")
	}
}

// TestRightClickOnAnNPC selects the placement, opens its behaviour once the
// NPC has loaded, and removes it.
func TestRightClickOnAnNPC(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withClerk(f)
	ground := groundFloor(t)
	if err := ground.Floor.InsertPlacement(0, floor.Placement{ID: placedID, NPC: clerkID, Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(groundID, "Ground", ground)
	f.rightClick("cell:9,8")
	want := "select-npc, open-behaviour, remove-npc, add-anchor, place-npc (disabled), preview-here"
	if got := f.menu(); got != want {
		t.Fatalf("NPC menu: %s", got)
	}
	f.click("menu:open-behaviour")
	f.idle(2)
	if opened := f.requests.Opened; len(opened) != 1 || opened[0].Target != routineID {
		t.Fatalf("opened %+v", opened)
	}
	if f.state.SelectedPlacement() != placedID {
		t.Fatal("the placement is selected")
	}
	f.rightClick("cell:9,8")
	f.click("menu:remove-npc")
	if removed := f.requests.Of(tilefloor.RemovePlacementEdit); len(removed) != 1 {
		t.Fatalf("remove commands %d", len(removed))
	}
}
