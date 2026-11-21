package playview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/playview"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/play"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

var (
	jettyID = frametest.ID(62)
	wellID  = frametest.ID(63)
)

// edit changes a floor the tab's project holds, as an applied edit does.
func (f *fixture) edit(floorID [16]byte, change func(*floor.Floor) error) {
	f.t.Helper()
	f.docs.View(floorID, func(document client.Document) {
		if err := change(document.Model.(*tilefloor.Document).Floor); err != nil {
			f.t.Fatal(err)
		}
	})
	f.docs.Touch(floorID)
}

// addPortals gives the Harbour's Quay the anchor Jetty at 1,2 and the town
// the anchor Well at 2,3, then portals: in the town one at 5,6 that fades to
// the Jetty, one at 4,5 that cuts to the Well, in its own zone, and one at
// 5,4 to an anchor that is gone; on the Quay one at 3,2 that cuts back to
// the Well. The game is launched again, so it plays them.
func (f *fixture) addPortals() {
	f.t.Helper()
	f.addHarbour()
	leads := func(floorID, anchor [16]byte) floor.AnchorRef { return floor.AnchorRef{Floor: floorID, Anchor: anchor} }
	f.edit(quayID, func(quay *floor.Floor) error {
		if err := quay.InsertAnchor(0, floor.Anchor{ID: jettyID, Name: "Jetty", Col: 1, Row: 2}); err != nil {
			return err
		}

		return quay.InsertPortal(0, floor.Portal{
			ID: frametest.ID(70), Col: 3, Row: 2, To: leads(groundID, wellID), Change: floor.ChangeCut,
		})
	})
	f.edit(groundID, func(ground *floor.Floor) error {
		if err := ground.InsertAnchor(0, floor.Anchor{ID: wellID, Name: "Well", Col: 2, Row: 3}); err != nil {
			return err
		}
		for index, portal := range []floor.Portal{
			{ID: frametest.ID(71), Col: 5, Row: 6, To: leads(quayID, jettyID), Change: floor.ChangeFade},
			{ID: frametest.ID(72), Col: 4, Row: 5, To: leads(groundID, wellID), Change: floor.ChangeCut},
			{ID: frametest.ID(73), Col: 5, Row: 4, To: leads(quayID, frametest.ID(99)), Change: floor.ChangeCut},
		} {
			if err := ground.InsertPortal(index, portal); err != nil {
				return err
			}
		}

		return nil
	})
	f.state.Changed(client.Update{Attention: true, Item: playview.LaunchItem}, f.ctx)
	f.frame(nil)
	if !f.has("launches:2") || !f.has("player:5,5") {
		f.t.Fatal("the game is launched again with its portals")
	}
}

// frames runs frames with keys held, or none.
func (f *fixture) frames(count int, held ...ui.Key) {
	for range count {
		f.frame(held)
	}
}

// TestLaunchTravelsByAPortalThatFades walks Launch's game onto a portal
// that fades into another zone: the screen goes black, the game enters the
// zone, and fades in with the player on the portal's anchor. The same game
// goes on, with the new zone's music; a portal there cuts back.
func TestLaunchTravelsByAPortalThatFades(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	f.addPortals()
	f.frames(1, ui.KeyDown)
	if !f.has("leaving") || f.has("dark") || !f.has("zone:Town") {
		t.Fatal("stepping onto the portal starts the player leaving, in the town still")
	}
	f.frames(game.FadeTicks/2, ui.KeyDown)
	if f.has("dark") || f.has("light") || !f.has("zone:Town") {
		t.Fatal("half way, the screen is neither light nor black")
	}
	f.frames(game.FadeTicks, ui.KeyDown)
	if !f.has("zone:Harbour") || !f.has("launches:2") || f.has("leaving") {
		t.Fatal("once black, the same game enters the Harbour")
	}
	if !strings.Contains(f.message(), "The portal led to Harbour") {
		t.Fatalf("the status bar says where the portal led: %q", f.message())
	}
	f.frames(game.FadeTicks + 1)
	if !f.has("light") || !f.has("player:1,2") || !f.has("ground:1,2:sand") || !f.has("layer:music/harbour") ||
		f.has("layer:music/town") {
		t.Fatal("the Harbour fades in around the player on the Jetty, with its music")
	}
	f.frames(play.StepTicks+1, ui.KeyRight)
	f.frames(play.StepTicks)
	if !f.has("zone:Town") || !f.has("player:2,3") || !f.has("light") || !f.has("layer:music/town") {
		t.Fatal("two cells on, a portal cuts back to the Well in the town")
	}
}

// TestLaunchTravelsWithinItsZone steps onto a portal that leads to an
// anchor of the zone being played: the player is there at once and nothing
// starts again. A portal whose anchor is gone leaves the player on it, and
// the status bar says why.
func TestLaunchTravelsWithinItsZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	f.addPortals()
	f.frames(1, ui.KeyLeft)
	f.frames(play.StepTicks)
	if !f.has("player:2,3") || !f.has("zone:Town") || !f.has("launches:2") || f.has("leaving") {
		t.Fatal("the portal takes the player to the Well, in the same zone")
	}
	f.state.Changed(client.Update{Attention: true, Item: playview.LaunchItem}, f.ctx)
	f.frame(nil)
	f.frames(1, ui.KeyUp)
	f.frames(play.StepTicks)
	if !f.has("player:5,4") || f.has("leaving") || !f.has("zone:Town") {
		t.Fatal("a portal that leads nowhere leaves the player on it")
	}
	if !strings.Contains(f.message(), "The portal at 5,4 cannot take the player: its anchor is gone.") {
		t.Fatalf("and the status bar says why: %q", f.message())
	}
}

// TestLaunchWaitsOnAPortalForItsZone holds the player on a portal while a
// document of the zone it leads to has not arrived, and travels when it
// has.
func TestLaunchWaitsOnAPortalForItsZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	f.addPortals()
	var quay *tilefloor.Document
	f.docs.View(quayID, func(document client.Document) { quay, _ = document.Model.(*tilefloor.Document) })
	f.docs.Remove(quayID)
	f.frames(3*game.FadeTicks, ui.KeyDown)
	if !f.has("leaving") || !f.has("dark") || !f.has("zone:Town") {
		t.Fatal("the screen stays black, and the player on the portal, until the zone has arrived")
	}
	f.docs.Put(quayID, "Quay", quay)
	f.frames(2)
	if !f.has("zone:Harbour") || f.has("leaving") {
		t.Fatal("the game enters the Harbour once its floor arrives")
	}
}
