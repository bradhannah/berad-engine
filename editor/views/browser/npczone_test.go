package browser_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// has reports whether a state holds in the browser.
func (f *fixture) has(state string) bool {
	_, found := f.state.Locate("state:"+state, f.ctx.Docs)

	return found
}

// docs is the fixture's documents, to change as the orchestrator would.
func (f *fixture) docs() *frametest.Docs {
	docs, _ := f.ctx.Docs.(*frametest.Docs) //nolint:errcheck // The fixture's own documents.

	return docs
}

// TestChoosingAFirstZone gives an NPC that lives nowhere a zone from the
// zone picker, at once; then, placed on its zone's floor, the panel says
// where, and Show on map opens the map there.
func TestChoosingAFirstZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Ada Clerk", false)
	if !f.has("npc-zone=No zone yet") || f.has("npc-placed=Ground, at 3, 4") {
		t.Fatal("a new NPC lives nowhere and is placed nowhere")
	}
	f.click("ui:npc-zone", false)
	if !f.has("zone-picker") || !f.has("zone-look=Town") {
		t.Fatal("the Zone box opens the zone picker")
	}
	f.click("zone:Town", false)
	f.press("Enter")
	set := f.requests.Of(npcplugin.SetZoneEdit)
	var args npcplugin.SetZoneArgs
	if len(set) != 1 || set[0].Decode(&args) != nil || args.Zone != zoneID || f.has("zone-picker") {
		t.Fatalf("choosing a zone for an NPC with none sets it at once: %+v", set)
	}
	set[0].Succeed(frametest.ID(0))
	if got := f.status(); got != "Ada Clerk lives in Town. Undo takes it back." {
		t.Fatalf("status %q", got)
	}
	clerk := npcplugin.NewDocument()
	clerk.NPC.Zone = zoneID
	f.docs().Put(clerkID, "Ada Clerk", clerk)
	ground, err := tilefloor.NewDocument(10, 8, "grass")
	if err != nil {
		t.Fatal(err)
	}
	placement := floor.Placement{ID: frametest.ID(30), NPC: clerkID, Col: 3, Row: 4}
	if err := ground.Floor.InsertPlacement(0, placement); err != nil {
		t.Fatal(err)
	}
	f.docs().Put(floorID, "Ground", ground)
	f.press("Shift")
	if !f.has("npc-zone=Town") || !f.has("npc-placed=Ground, at 3, 4") {
		t.Fatal("the panel names the zone and where the NPC is placed")
	}
	f.click("ui:show-on-map", false)
	opened := f.requests.Opened[len(f.requests.Opened)-1]
	if opened.View != "map" || opened.Target != zoneID ||
		opened.Item != mapview.PlacementItem+floorID.String()+":"+placement.ID.String() {
		t.Fatalf("Show on map opens the zone's map at the placement: %+v", opened)
	}
}

// TestZoneNotes types notes into a zone's inspector and saves them.
func TestZoneNotes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Town", false)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.typeInto("zone-notes", "Busy on market days.")
	if f.state.Unsaved(f.ctx.Docs) != 1 {
		t.Fatal("typed notes are unsaved until saved")
	}
	f.state.Save(f.ctx)
	set := f.requests.Of(zoneplugin.SetNotesEdit)
	var args zoneplugin.SetNotesArgs
	if len(set) != 1 || set[0].Decode(&args) != nil || args.Notes != "Busy on market days." ||
		set[0].Command.Document != zoneID {
		t.Fatalf("Save saves the zone's notes: %+v", set)
	}
}
