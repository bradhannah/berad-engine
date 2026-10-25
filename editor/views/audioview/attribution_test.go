package audioview_test

import (
	"image"
	"reflect"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
)

// TestAttributionInThePanel shows the attribution of the recording in focus
// in the panel, clears it for a commercial game from its checkbox, keeping
// the rest, and edits the rest in the dialog; an empty slot has none.
func TestAttributionInThePanel(t *testing.T) {
	t.Parallel()
	library := testLibrary()
	slot := recording("slot", false)
	slot.Status, slot.File = protocol.RecordingNoFile, ""
	library.Cues = append(library.Cues, cue("sfx/empty", "Empty", protocol.CategorySFX, slot))
	f := newFixtureWith(t, library)
	f.click("cue:sfx/door")
	if !f.has("ui:cleared") || !f.has("ui:edit-attribution") {
		t.Fatal("the recording in focus shows its attribution")
	}
	f.click("ui:cleared")
	var change protocol.AttributionChange
	if err := f.edit(protocol.AudioSetAttribution).Decode(&change); err != nil {
		t.Fatal(err)
	}
	want := recording("door_a", true).Attribution
	want.Commercial = false
	if change.Recording != "door_a" || !reflect.DeepEqual(change.Attribution, want) {
		t.Fatalf("unticking it keeps the rest: %+v", change)
	}
	f.click("ui:edit-attribution")
	if !f.has("state:dialog") || !f.has("ui:source-type") || !f.has("ui:add-step") {
		t.Fatal("Edit attribution opens the dialog")
	}
	f.click("ui:" + ui.DialogCancel)
	f.click("cue:sfx/empty")
	if f.has("ui:cleared") {
		t.Fatal("an empty slot has no attribution")
	}
}

// TestPanelScrolls scrolls a panel taller than the window with the wheel.
func TestPanelScrolls(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.ctx.Size = image.Pt(1200, 560)
	f.click("cue:sfx/door")
	before := f.at("ui:cleared")
	wheel := ui.NewInput(image.Pt(600, 300))
	wheel.WheelY = -3
	f.frame(wheel)
	if after := f.at("ui:cleared"); after.Y != before.Y-3*ui.RowHeight {
		t.Fatalf("the wheel scrolls the panel: %v to %v", before, after)
	}
	wheel.WheelY = -100
	f.frame(wheel)
	bottom := f.at("ui:cleared")
	f.frame(wheel)
	if f.at("ui:cleared") != bottom {
		t.Fatal("it stops at the end")
	}
}
