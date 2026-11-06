package browser_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestInspectorScrolls scrolls Ada's inspector, taller than the window,
// with the wheel over it. A press on the save bar, where her greeting's
// Write… lies hidden behind it, does not reach it; an input script's click
// on Write… scrolls it into view first; and another row starts at the top.
func TestInspectorScrolls(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withShop()
	f.click("row:Ada Clerk", false)
	if offset, most := f.state.InspectorScroll(); offset != 0 || most == 0 {
		t.Fatalf("her inspector starts at the top and scrolls: %d of %d", offset, most)
	}
	write := "ui:constant-write-npc.shop.greeting"
	hidden, _ := f.state.LocateInPlace(write, f.ctx.Docs)
	save, _ := f.state.LocateInPlace("ui:save", f.ctx.Docs)
	if hidden.Y < save.Y-ui.ButtonH || hidden.Y > save.Y+ui.ButtonH {
		t.Fatalf("Write… lies behind the save bar: %v and %v", hidden, save)
	}
	press := ui.NewInput(hidden)
	press.Pressed, press.Down = true, true
	f.frame(press)
	f.frame(ui.NewInput(hidden))
	if f.has("sheet") {
		t.Fatal("a press on the save bar does not reach Write… behind it")
	}
	wheel := ui.NewInput(hidden)
	wheel.WheelY = -2
	f.frame(wheel)
	if offset, _ := f.state.InspectorScroll(); offset != 2*ui.RowHeight {
		t.Fatalf("the wheel scrolls it two rows: %d", offset)
	}
	wheel.WheelY = 100
	f.frame(wheel)
	if offset, _ := f.state.InspectorScroll(); offset != 0 {
		t.Fatalf("it scrolls no further than its top: %d", offset)
	}
	f.click(write, false)
	if offset, _ := f.state.InspectorScroll(); offset == 0 || !f.has("sheet") {
		t.Fatalf("a script's click scrolls Write… into view and opens the sheet: %d", offset)
	}
	f.press("Escape")
	f.click("row:Town", false)
	if offset, _ := f.state.InspectorScroll(); offset != 0 {
		t.Fatalf("another row starts at the top: %d", offset)
	}
}
