package ui_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestListDragsARowOntoAnother drags a row onto a row CanDrop accepts and
// reports the drop; a short wobble is still a click, and a row CanDrop
// refuses takes nothing.
func TestListDragsARowOntoAnother(t *testing.T) {
	t.Parallel()
	folder := ui.NewRow("folder", "Town")
	folder.Icon = ui.IconFolder
	plan, notes := ui.NewRow("plan", "Plan"), ui.NewRow("notes", "Notes")
	plan.Aside = "#quest"
	list := &ui.List{ID: "tree", Rect: image.Rect(0, 0, 400, 200), Rows: []ui.Row{folder, plan, notes}}
	list.CanDrop = func(dragged, target string) bool { return target == "folder" }
	at := func(id string) image.Point {
		point, _ := list.Locate(id)

		return point
	}
	frame := func(point image.Point, pressed, down bool) ui.ListResult {
		return list.Update(&ui.Input{Cursor: point, Pressed: pressed, Down: down, Released: !down && !pressed})
	}
	frame(at("plan"), true, true)
	frame(at("plan").Add(image.Pt(2, 1)), false, true)
	if result := frame(at("plan"), false, false); result.Dropped != "" || list.Selected != "plan" {
		t.Fatalf("a wobble is a click: %+v", result)
	}
	frame(at("plan"), true, true)
	frame(at("notes"), false, true)
	if !list.Dragging() {
		t.Fatal("moving away drags the row")
	}
	if result := frame(at("notes"), false, false); result.Dropped != "" {
		t.Fatalf("Notes takes nothing: %+v", result)
	}
	frame(at("plan"), true, true)
	frame(at("folder"), false, true)
	list.Draw(ebiten.NewImage(400, 200), theme(t))
	if result := frame(at("folder"), false, false); result.Dragged != "plan" || result.Dropped != "folder" {
		t.Fatalf("dropped on the folder: %+v", result)
	}
	if list.Dragging() {
		t.Fatal("the drag ends on release")
	}
}
