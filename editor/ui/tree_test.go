package ui_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

func clickAt(point image.Point) *ui.Input {
	return &ui.Input{Cursor: point, Pressed: true, Down: true}
}

// TestTreeRowsToggleAddAndSelect clicks a section, its [+], a zone's
// triangle and a floor: the section toggles, the [+] adds, the triangle
// toggles without selecting, and the floor is selected.
func TestTreeRowsToggleAddAndSelect(t *testing.T) {
	t.Parallel()
	section := ui.NewHeading("section:zones", "Zones")
	section.Section, section.Add, section.Toggle, section.Detail = true, true, ui.DisclosureOpen, "1"
	town := ui.NewRow("town", "Town")
	town.Icon, town.Strong, town.Toggle = ui.IconZone, true, ui.DisclosureOpen
	ground := ui.NewRow("ground", "Ground")
	ground.Icon, ground.Indent, ground.Guide, ground.Detail = ui.IconFloor, 1, ui.GuideLast, "top"
	list := &ui.List{ID: "tree", Rect: image.Rect(0, 0, 400, 200), Rows: []ui.Row{section, town, ground}}

	add, _ := list.LocateAdd("section:zones")
	if result := list.Update(clickAt(add)); result.Added != "section:zones" || result.Toggled != "" {
		t.Fatalf("[+]: %+v", result)
	}
	heading, _ := list.Locate("section:zones")
	if result := list.Update(clickAt(heading)); result.Toggled != "section:zones" || result.Changed {
		t.Fatalf("section heading: %+v", result)
	}
	triangle, _ := list.LocateToggle("town")
	if result := list.Update(clickAt(triangle)); result.Toggled != "town" || list.Selected != "" {
		t.Fatalf("triangle: %+v, selected %q", result, list.Selected)
	}
	floor, _ := list.Locate("ground")
	if result := list.Update(clickAt(floor)); !result.Changed || list.Selected != "ground" {
		t.Fatalf("floor: %+v", result)
	}
	list.Draw(ebiten.NewImage(400, 200), theme(t))
	for _, icon := range []ui.Icon{ui.IconZone, ui.IconFloor, ui.IconWorkflow, ui.IconNPC, ui.IconText} {
		if ui.IconColor(icon) == ui.ColorSelect() || ui.IconColor(icon) == ui.ColorLive() {
			t.Fatalf("icon %d uses selection blue or live orange", icon)
		}
	}
}

// TestMenuChoosesAndCloses opens a menu, ignores a disabled item, closes on
// Escape, and returns a chosen item.
func TestMenuChoosesAndCloses(t *testing.T) {
	t.Parallel()
	menu := ui.Menu{Items: []ui.MenuItem{
		{ID: "zone", Label: "Zone", Icon: ui.IconZone, Disabled: false, Danger: false, Rule: false},
		{ID: "floor", Label: "Floor", Icon: ui.IconFloor, Disabled: true, Danger: false, Rule: false},
	}, Width: 0}
	window := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(600, 10, 700, 36)
	menu.Open(anchor, window)
	floor, _ := menu.Locate("floor")
	if chosen := menu.Update(clickAt(floor)); chosen != "" || menu.IsOpen() {
		t.Fatalf("a disabled item is not chosen: %q", chosen)
	}
	menu.Open(anchor, window)
	if chosen := menu.Update(frame("Escape")); chosen != "" || menu.IsOpen() {
		t.Fatal("Escape closes the menu")
	}
	menu.Open(anchor, window)
	menu.Draw(ebiten.NewImage(800, 600), theme(t))
	zone, _ := menu.Locate("zone")
	if chosen := menu.Update(clickAt(zone)); chosen != "zone" {
		t.Fatalf("chosen %q", chosen)
	}
}

// TestMenuOpensAtTheCursor opens a right-click menu inside the window, wide
// enough for its longest label, and chooses with the right button.
func TestMenuOpensAtTheCursor(t *testing.T) {
	t.Parallel()
	items := []ui.MenuItem{
		{ID: "add", Label: "Add anchor here", Icon: ui.IconNone, Disabled: false, Danger: false, Rule: false},
		{
			ID: "delete", Label: "Delete anchor and everything that refers to it in every workflow", Icon: ui.IconNone,
			Disabled: false, Danger: true, Rule: true,
		},
	}
	menu := ui.Menu{Items: items, Width: ui.MenuWidth(items, theme(t))}
	if menu.Width <= 200 {
		t.Fatalf("width %d does not fit the long label", menu.Width)
	}
	window := image.Rect(0, 0, 800, 600)
	menu.OpenAt(image.Pt(790, 590), window)
	add, _ := menu.Locate("add")
	if !add.In(window) || add.X > 790 || add.Y > 590 {
		t.Fatalf("the menu left the window: %v", add)
	}
	menu.Draw(ebiten.NewImage(800, 600), theme(t))
	if chosen := menu.Update(&ui.Input{Cursor: add, Secondary: true}); chosen != "add" {
		t.Fatalf("right button chose %q", chosen)
	}
}

// TestSaveBarStates greys out its buttons with nothing unsaved, and names
// the unsaved changes otherwise; a narrow bar has two rows.
func TestSaveBarStates(t *testing.T) {
	t.Parallel()
	bar := ui.NewSaveBar()
	bar.Rect = image.Rect(0, 0, 260, ui.SaveBarHeight(260))
	save, _ := bar.Locate(ui.SaveID)
	if clicked := bar.Update(clickAt(save)); clicked != "" || bar.Message() != "All changes saved" {
		t.Fatalf("nothing to save: %q %q", clicked, bar.Message())
	}
	bar.Unsaved = 2
	if clicked := bar.Update(clickAt(save)); clicked != ui.SaveID || bar.Message() != "2 unsaved changes" {
		t.Fatalf("save: %q %q", clicked, bar.Message())
	}
	revert, _ := bar.Locate(ui.RevertID)
	if clicked := bar.Update(clickAt(revert)); clicked != ui.RevertID {
		t.Fatalf("revert: %q", clicked)
	}
	bar.Draw(ebiten.NewImage(300, 100), theme(t))
	if ui.SaveBarHeight(260) <= ui.SaveBarHeight(600) {
		t.Fatal("a narrow bar has two rows")
	}
}

// TestWheelPanIsShared turns scrolling into the same pan in every canvas,
// moving the content the way the fingers move.
func TestWheelPanIsShared(t *testing.T) {
	t.Parallel()
	scroll := &ui.Input{WheelX: -1, WheelY: 1}
	if pan := scroll.WheelPan(); pan.X >= 0 || pan.Y <= 0 {
		t.Fatalf("pan %v", pan)
	}
}
