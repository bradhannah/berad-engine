package window

import (
	"image"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// quickView stands in for the project browser's quick open.
type quickView struct {
	open   bool
	rect   image.Rectangle
	room   bool
	beside *browser.Beside
}

func (v *quickView) Title(frame.Docs) string                       { return "Project" }
func (v *quickView) Update(*ui.Input, *frame.Context)              {}
func (v *quickView) Changed(client.Update, *frame.Context)         {}
func (v *quickView) Draw(*ebiten.Image, frame.Docs)                {}
func (v *quickView) Focused() *ui.TextField                        { return nil }
func (v *quickView) Locate(string, frame.Docs) (image.Point, bool) { return image.Pt(0, 0), false }
func (v *quickView) OpenQuick()                                    { v.open = true }
func (v *quickView) QuickOpen() bool                               { return v.open }
func (v *quickView) SetQuick(rect image.Rectangle, room bool)      { v.rect, v.room = rect, room }
func (v *quickView) DrawQuick(*ebiten.Image, frame.Docs)           {}

func (v *quickView) TakeBeside() (browser.Beside, bool) {
	beside := v.beside
	v.beside = nil
	if beside == nil {
		return browser.Beside{View: "", Target: ids.ULID{}}, false
	}

	return *beside, true
}

// TestQuickOpenShowsOverTheTabInFront opens quick open with Cmd+P from a
// Map's tab: the project browser's pane takes the input and is told where
// the sheet shows. A window asked for beside the Map opens in the Map's tab,
// and one already open in a tab of its own moves there (increment 38).
func TestQuickOpenShowsOverTheTabInFront(t *testing.T) {
	t.Parallel()
	quick := &quickView{open: false, rect: ui.Zero(), room: false, beside: nil}
	project := titled("browser", "Project")
	project.host = &host{view: quick, closeQuits: true, frame: frame.NewFrame(time.Now)}
	town := titled("map", "Map: Town")
	shop := titled("workflow", "Workflow: Shop")
	shop.target = ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")
	shown := &shell{size: image.Pt(1400, 900), tabs: []*tab{
		{panes: []*pane{project}}, {panes: []*pane{town}}, {panes: []*pane{shop}},
	}, active: 1}
	shown.layout()
	input := ui.NewInput(image.Pt(0, 0))
	input.Presses = []ui.Press{ui.ParsePress("Cmd+P")}
	shown.tabKeys(&input, func(string) {})
	if !shown.quickShowing() || shown.target(town) != project {
		t.Fatal("Cmd+P opens the sheet, and the browser's pane takes the input")
	}
	shown.placeQuick()
	if quick.rect != shown.quickRect().Sub(project.rect.Min) || !quick.room {
		t.Fatalf("the browser is told where the sheet shows, and that the Map has room: %v %v", quick.rect, quick.room)
	}
	notes := titled("workflow", "Workflow: Notes")
	notes.target = ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHD")
	quick.beside = &browser.Beside{View: "workflow", Target: notes.target}
	shown.takeBeside()
	shown.place(notes)
	if len(shown.tabs) != 3 || len(shown.tabs[1].panes) != 2 || shown.tabs[1].panes[1] != notes ||
		shown.active != 1 || shown.focused() != notes {
		t.Fatalf("Notes opens beside the Map, focused: %d tabs, front %d", len(shown.tabs), shown.active)
	}
	shown.remove(notes)
	quick.beside = &browser.Beside{View: "workflow", Target: shop.target}
	shown.takeBeside()
	shop.place.attention = true
	shown.attend()
	if len(shown.tabs) != 2 || len(shown.tabs[1].panes) != 2 || shown.tabs[1].panes[1] != shop || shown.active != 1 {
		t.Fatalf("Shop, already open, moves beside the Map: %d tabs", len(shown.tabs))
	}
	quick.open = false
	shown.placeQuick()
	if quick.room {
		t.Fatal("a tab showing two has no room beside it")
	}
}
