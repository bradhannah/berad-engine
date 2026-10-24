package mapview_test

import (
	"encoding/json"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// TestPreviewStartsAtTheClockWindowsTime starts the preview at the time the
// Clock window set for every window, and the inspector's Clock button opens
// the Clock window, before and during a preview.
func TestPreviewStartsAtTheClockWindowsTime(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	value, err := json.Marshal(protocol.ClockStart{Seconds: 17 * 3600})
	if err != nil {
		t.Fatal(err)
	}
	f.state.Changed(client.Update{Session: &protocol.Session{Topic: protocol.TopicClockStart, Value: value}}, f.ctx)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.click("ui:clock")
	if opened := f.requests.Opened; len(opened) != 1 || opened[0].View != "clock" ||
		opened[0].Target != project.IndexID() || opened[0].NewView {
		t.Fatalf("Clock opens the Clock window: %+v", opened)
	}
	f.startPreview("cell:3,3")
	f.idle(2)
	if clock := f.lastState().Clock; clock < 17*3600 || clock > 17*3600+60 {
		t.Fatalf("the preview starts at 17:00: %v", clock)
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.click("ui:clock")
	f.click("ui:watch")
	if opened := f.requests.Opened; len(opened) != 3 || opened[1].View != "clock" || opened[2].View != "watch" {
		t.Fatalf("Clock and Watch during a preview: %+v", opened)
	}
}
