package watchview_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// TestFollowIsTheAuthorsToTurnOn starts a preview: Watch does not follow
// the NPC until Follow is turned on, and panning the workflow by hand turns
// it off again and says so (increment 41).
func TestFollowIsTheAuthorsToTurnOn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	if f.has("following") {
		t.Fatal("Follow starts off")
	}
	f.click("ui:follow")
	if !f.has("following") {
		t.Fatal("Follow turns on before any NPC is watched")
	}
	f.click("ui:follow")
	f.state.Changed(client.Update{PreviewStart: &protocol.PreviewStart{Zone: zoneID, Run: runID, Window: ids.ULID{}}}, f.ctx)
	f.send(5, false)
	f.state.Draw(ebiten.NewImage(1000, 700), f.docs)
	f.click("ui:follow")
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("following") {
		t.Fatal("Follow turns on when pressed")
	}
	wheel := ui.NewInput(image.Pt(500, 300))
	wheel.WheelY = 3
	f.frame(wheel)
	if f.has("following") {
		t.Fatal("panning by hand turns Follow off")
	}
	if text, _ := f.ctx.Frame.Message(); text == "" || text[:13] != "Follow is off" {
		t.Fatalf("and says so: %q", text)
	}
}
