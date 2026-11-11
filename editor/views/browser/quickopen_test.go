package browser_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// TestQuickOpenFindsAndOpens opens quick open's sheet, finds the Say in Clerk
// day by its words, and opens it with Enter, leaving the Project's own search
// as it was; Cmd+Enter asks for it beside the tab in front, or says why not
// when that tab has no room; Esc and a click outside close the sheet
// (increment 38).
func TestQuickOpenFindsAndOpens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	day := f.withLine()
	sheet := image.Rect(100, 40, 780, 500)
	f.state.SetQuick(sheet, true)
	rows := len(f.state.Rows(f.ctx.Docs))
	f.state.OpenQuick()
	if !f.state.QuickOpen() || f.state.Focused() == nil || f.state.Focused().ID != "quick-search" {
		t.Fatal("the sheet opens with its search taking the keyboard")
	}
	if _, found := f.state.Locate("quick:Clerk day · Say", f.ctx.Docs); found {
		t.Fatal("nothing is listed before something is typed")
	}
	f.typeText("gold coins")
	if _, found := f.state.Locate("quick:Clerk day · Say", f.ctx.Docs); !found {
		t.Fatal("gold coins finds the Say's line")
	}
	f.state.DrawQuick(ebiten.NewImage(1200, 800), f.ctx.Docs)
	before := len(f.requests.Opened)
	f.press("Enter")
	opened := f.requests.Opened
	if f.state.QuickOpen() || len(opened) != before+1 || opened[before].Target != day ||
		opened[before].Item != "node:"+frametest.ID(70).String() {
		t.Fatalf("Enter opens Clerk day at the Say and closes the sheet: %+v", opened)
	}
	if len(f.state.Rows(f.ctx.Docs)) != rows {
		t.Fatal("the Project's own search is left as it was")
	}
	if _, asked := f.state.TakeBeside(); asked {
		t.Fatal("Enter asks for nothing beside")
	}
	f.state.OpenQuick()
	f.typeText("gold coins")
	f.press("Cmd+Enter")
	if beside, asked := f.state.TakeBeside(); !asked || beside.View != "workflow" || beside.Target != day {
		t.Fatalf("Cmd+Enter asks for Clerk day beside the tab in front: %+v", beside)
	}
	f.state.SetQuick(sheet, false)
	f.state.OpenQuick()
	f.typeText("gold coins")
	before = len(f.requests.Opened)
	f.press("Cmd+Enter")
	if !f.state.QuickOpen() || len(f.requests.Opened) != before {
		t.Fatal("with no room beside the tab in front, Cmd+Enter opens nothing and the sheet stays")
	}
	f.press("Escape")
	if f.state.QuickOpen() {
		t.Fatal("Esc closes the sheet")
	}
	f.state.OpenQuick()
	outside := ui.NewInput(image.Pt(20, 700))
	outside.Pressed, outside.Down = true, true
	f.frame(outside)
	if f.state.QuickOpen() || len(f.requests.Opened) != before {
		t.Fatal("a click outside closes the sheet and opens nothing")
	}
}
