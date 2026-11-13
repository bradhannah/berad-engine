package mapview_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
)

// TestTheMapsPanelFoldsToARail folds the left panel: the floor's canvas
// starts at the rail, the rail picks a tool, and the button unfolds it
// (increment 43).
func TestTheMapsPanelFoldsToARail(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.press("ui:fold-left", false)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if f.ctx.Frame.ShowLeft.On || f.state.Grid().Min.X != mapview.RailWidth {
		t.Fatalf("folded, the canvas starts at the rail: %v", f.state.Grid())
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.docs)
	f.press("ui:rail-"+string(mapview.ToolWalls), false)
	if f.state.Tool() != mapview.ToolWalls {
		t.Fatalf("the rail picks a tool: %v", f.state.Tool())
	}
	f.press("ui:fold-left", false)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.ctx.Frame.ShowLeft.On || f.state.Grid().Min.X != mapview.PanelWidth {
		t.Fatal("the button unfolds it")
	}
}
