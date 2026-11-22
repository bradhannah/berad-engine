package ui_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestAButtonHasThreeStrengths gives a panel's one main button the
// selection's blue, leaves a plain one white with an ink outline, and draws
// a quiet one in the page's tone with a hairline's colour, so not every
// button shouts alike (increment 66). A quiet button that is on is washed
// blue, one that deletes stays red, and one that cannot be pressed goes
// pale where it stands.
func TestAButtonHasThreeStrengths(t *testing.T) {
	t.Parallel()
	main, plain, quiet := ui.NewButton("open", "Open map"), ui.NewButton("add", "Add"), ui.NewQuietButton("move", "Move to…")
	main.Primary = true
	if fill, ink, _ := main.Look(); fill != ui.ColorSelect() || ink != ui.ColorReverse() {
		t.Fatalf("the main button is filled blue: %v %v", fill, ink)
	}
	if fill, ink, edge := plain.Look(); fill != ui.ColorField() || ink != ui.ColorInk() || edge != ui.ColorInk() {
		t.Fatalf("a plain button is white with an ink outline: %v %v %v", fill, ink, edge)
	}
	if fill, ink, edge := quiet.Look(); fill != ui.ColorPage() || ink != ui.ColorInk() || edge != ui.ColorRule() {
		t.Fatalf("a quiet button is page tone with a hairline's colour: %v %v %v", fill, ink, edge)
	}
	quiet.On = true
	if fill, _, edge := quiet.Look(); fill != ui.ColorSelectWash() || edge != ui.ColorSelect() {
		t.Fatalf("a quiet button that is on is washed blue: %v %v", fill, edge)
	}
	quiet.On, quiet.Danger = false, true
	if fill, ink, edge := quiet.Look(); fill != ui.ColorPage() || ink != ui.ColorWarn() || edge != ui.ColorWarn() {
		t.Fatalf("a quiet button that deletes is red on the page: %v %v %v", fill, ink, edge)
	}
	quiet.Danger, quiet.Disabled = false, true
	if fill, ink, edge := quiet.Look(); fill != ui.ColorPage() || ink != ui.ColorSubdued() || edge != ui.ColorRuleSoft() {
		t.Fatalf("a quiet button that cannot be pressed goes pale: %v %v %v", fill, ink, edge)
	}
	screen := ebiten.NewImage(400, 200)
	for index, button := range []*ui.Button{main, plain, quiet} {
		button.Rect, button.Key = image.Rect(10, 10+index*40, 200, 36+index*40), "P"
		button.Draw(screen, theme(t))
	}
	if width := ui.CapWidth(theme(t), "P"); width < 16 || ui.CapWidth(theme(t), "Cmd+R") <= width {
		t.Fatalf("a cap is at least a square, and wider for a longer key: %d", width)
	}
}

// TestAGridLaysRowsOutAsChips lays five rows out in two columns: three
// lines of chips, each clicked where it is drawn, and the list as tall as
// its lines.
func TestAGridLaysRowsOutAsChips(t *testing.T) {
	t.Parallel()
	list := ui.NewList("tiles", "grass")
	list.Rect, list.Grid = image.Rect(0, 0, 240, 200), 2
	for _, name := range []string{"grass", "floor", "road", "water", "unassigned"} {
		row := ui.NewRow(name, name)
		swatch := ui.ColorZone()
		row.Swatch, row.Key = &swatch, "1"
		list.Rows = append(list.Rows, row)
	}
	first, second, third := list.RowRect(0), list.RowRect(1), list.RowRect(2)
	if first.Min.Y != second.Min.Y || second.Min.X <= first.Max.X || third.Min.Y <= first.Max.Y || third.Min.X != first.Min.X {
		t.Fatalf("two chips a line, the third under the first: %v %v %v", first, second, third)
	}
	if last := list.RowRect(4); list.Height() != last.Max.Y-first.Min.Y {
		t.Fatalf("the list is as tall as its three lines: %d, %v", list.Height(), last)
	}
	water, found := list.Locate("water")
	if !found || !water.In(list.RowRect(3)) {
		t.Fatalf("a chip is found where it is drawn: %v %v", water, found)
	}
	if result := list.Update(clickAt(water)); !result.Changed || list.Selected != "water" {
		t.Fatalf("a click on a chip chooses it: %+v %q", result, list.Selected)
	}
	gap := image.Pt(first.Max.X+2, first.Min.Y+2)
	if result := list.Update(clickAt(gap)); result.Changed || list.Selected != "water" {
		t.Fatalf("a click between chips chooses nothing: %+v", result)
	}
	list.Draw(ebiten.NewImage(240, 200), theme(t))
}

// TestRowsCarryCapsAndGlyphs draws a list whose selected row is the one in
// hand, with a glyph before each plain row and its key as a cap, a tree
// list whose counts are caps, joined buttons, and every tool icon.
func TestRowsCarryCapsAndGlyphs(t *testing.T) {
	t.Parallel()
	screen := ebiten.NewImage(400, 300)
	tools := ui.NewList("tools", "ground")
	tools.Rect, tools.InHand = image.Rect(0, 0, 200, 120), true
	for index, icon := range []ui.Icon{
		ui.IconSelect, ui.IconGround, ui.IconWalls, ui.IconDoor, ui.IconErase, ui.IconResize, ui.IconGrip, ui.IconCross,
	} {
		if ui.Chipped(icon) || ui.IconColor(icon) != ui.ColorInk() {
			t.Fatalf("tool icon %d is a plain mark in ink", icon)
		}
		row := ui.NewRow([]string{"select", "ground", "walls", "door", "erase", "resize", "grip", "cross"}[index], "Tool")
		row.Glyph, row.Key = icon, "G"
		tools.Rows = append(tools.Rows, row)
	}
	tools.Draw(screen, theme(t))
	places := ui.NewList("places", "zones")
	places.Rect, places.InHand, places.Caps = image.Rect(200, 0, 400, 120), true, true
	zones := ui.NewRow("zones", "Zones")
	zones.Icon, zones.Strong, zones.Detail = ui.IconZone, true, "2"
	people := ui.NewRow("npcs", "NPCs")
	people.Icon, people.Strong, people.Detail = ui.IconNPC, true, "12"
	places.Rows = []ui.Row{zones, people}
	places.Draw(screen, theme(t))
	cards, rows := ui.NewButton("cards", "Cards"), ui.NewButton("rows", "Rows")
	cards.On = true
	cards.Rect, rows.Rect = image.Rect(10, 200, 80, 226), image.Rect(80, 200, 150, 226)
	ui.DrawJoined(screen, theme(t), []*ui.Button{cards, rows})
	ui.DrawJoined(screen, theme(t), nil)
}

// TestARowWithNoIDIsNotUnderThePointerByDefault lists a row whose ID is
// empty, as the Map's unassigned ground is: it is drawn as under the
// pointer only while the pointer is over it, not whenever the pointer is
// over no row at all.
func TestARowWithNoIDIsNotUnderThePointerByDefault(t *testing.T) {
	t.Parallel()
	tiles := ui.NewList("tiles", "grass")
	tiles.Rect, tiles.Grid = image.Rect(0, 0, 200, 120), 1
	tiles.Rows = []ui.Row{ui.NewRow("grass", "Grass"), ui.NewRow("", "Unassigned")}
	away := ui.NewInput(image.Pt(400, 400))
	tiles.Update(&away)
	if tiles.Hovers("") || tiles.Hovers("grass") {
		t.Fatal("with the pointer elsewhere, no row is under it")
	}
	over := ui.NewInput(ui.Center(tiles.RowRect(1)))
	tiles.Update(&over)
	if !tiles.Hovers("") || tiles.Hovers("grass") {
		t.Fatal("with the pointer over the row with no ID, that row is under it")
	}
}
