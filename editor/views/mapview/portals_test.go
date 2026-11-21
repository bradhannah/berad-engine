package mapview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

var (
	cellarID      = frametest.ID(60)
	cellarFloorID = frametest.ID(61)
	footID        = frametest.ID(62)
	portalID      = frametest.ID(63)
	otherMapID    = frametest.ID(64)
)

// withCellar adds a second zone to the fixture's project: the Cellar, one
// floor with the anchor Foot at 2,2.
func (f *fixture) withCellar() {
	f.t.Helper()
	below, err := tilefloor.NewDocument(8, 8, "stone")
	if err != nil {
		f.t.Fatal(err)
	}
	if err := below.Floor.InsertAnchor(0, floor.Anchor{ID: footID, Name: "Foot", Col: 2, Row: 2}); err != nil {
		f.t.Fatal(err)
	}
	cellar := zoneplugin.NewDocument()
	if err := cellar.Zone.Insert(0, cellarFloorID); err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(cellarID, "Cellar", cellar)
	f.docs.Put(cellarFloorID, "Cellar floor", below)
	f.docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: zoneID, Name: "Town", Format: "zone"}, {ID: groundID, Name: "Ground", Format: floor.FormatName},
		{ID: upperID, Name: "Upper", Format: floor.FormatName}, {ID: cellarID, Name: "Cellar", Format: "zone"},
		{ID: cellarFloorID, Name: "Cellar floor", Format: floor.FormatName},
	}))
	index := project.IndexID()
	f.state.Changed(client.Update{Snapshot: &index}, f.ctx)
	f.keys()
}

// leadTo chooses a zone in the dialog that asks where a portal leads, says
// which Maps are open, and answers the pick that follows with an anchor.
func (f *fixture) leadTo(zone ids.ULID, open []protocol.PickMap, anchorFloor, anchor ids.ULID, name string) protocol.PickStart {
	f.t.Helper()
	f.click("ui:" + zone.String())
	f.click("ui:lead")
	if len(f.requests.PickAsks) == 0 {
		f.t.Fatal("the Map asks which Maps are open")
	}
	f.requests.PickAsks[len(f.requests.PickAsks)-1](open, nil)
	if len(f.requests.PickStarts) == 0 {
		f.t.Fatal("a pick starts")
	}
	start := f.requests.PickStarts[len(f.requests.PickStarts)-1]
	answer := protocol.PickAnswer{Pick: start.Pick, Floor: anchorFloor, Item: anchor.String(), Name: name}
	f.state.Changed(client.Update{PickAnswer: &answer}, f.ctx)

	return start
}

// TestAddingAPortal adds a portal from a cell's menu: the zone is chosen
// first, the anchor picked in that zone's Map, opened for the pick when none
// shows it. The portal cuts until told to fade; it opens where it leads, is
// moved, led somewhere else and removed, each one floor edit with undo.
func TestAddingAPortal(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withCellar()
	f.rightClick("cell:9,8")
	if !strings.Contains(f.menu(), "add-portal") {
		t.Fatalf("a portal can be added: %s", f.menu())
	}
	f.click("menu:add-portal")
	start := f.leadTo(cellarID, nil, cellarFloorID, footID, "Foot")
	if start.Zone != cellarID || !start.Window.IsZero() || !strings.Contains(start.Label, "the portal at 9,8 of Ground") {
		t.Fatalf("with no Map of the Cellar open, one is opened for the pick: %+v", start)
	}
	added := f.requests.Of(tilefloor.AddPortalEdit)
	var add tilefloor.AddPortalArgs
	want := floor.AnchorRef{Floor: cellarFloorID, Anchor: footID}
	if len(added) != 1 || added[0].Decode(&add) != nil || added[0].Command.Document != groundID || add.Portal.Col != 9 ||
		add.Portal.Row != 8 || add.Portal.To != want || add.Portal.Change != floor.ChangeCut {
		t.Fatalf("add %d %+v", len(added), add)
	}
	add.Portal.ID = portalID
	f.apply(groundID, tilefloor.AddPortal{}, add)
	f.keys()
	if !f.has("portal:9,8") || !f.has("portal:9,8:cut") || !f.has("portal:9,8:to:Foot") || f.has("portal:9,8:nowhere") {
		t.Fatal("the portal is on the map, cuts, and leads to Foot")
	}
	f.rightClick("cell:9,8")
	if got := f.menu(); !strings.Contains(got, "open-portal, lead-portal, portal-change:fade, move-portal, remove-portal") {
		t.Fatalf("the portal's choices: %s", got)
	}
	f.click("menu:open-portal")
	opened := f.requests.Opened[len(f.requests.Opened)-1]
	if opened.View != mapview.ViewName || opened.Target != cellarID ||
		opened.Item != mapview.AnchorItem+cellarFloorID.String()+":"+footID.String() {
		t.Fatalf("the Cellar's Map opens on Foot: %+v", opened)
	}
	f.rightClick("cell:9,8")
	f.click("menu:portal-change:fade")
	var fades tilefloor.UpdatePortalArgs
	if updates := f.requests.Of(tilefloor.UpdatePortalEdit); len(updates) != 1 || updates[0].Decode(&fades) != nil ||
		fades.Portal.Change != floor.ChangeFade || fades.Portal.ID != portalID {
		t.Fatalf("fade %+v", fades)
	}
	f.apply(groundID, tilefloor.UpdatePortal{}, fades)
	f.rightClick("cell:9,8")
	if !f.has("portal:9,8:fade") || !strings.Contains(f.menu(), "portal-change:cut") {
		t.Fatalf("it fades now, and can be told to cut: %s", f.menu())
	}
	f.click("menu:move-portal")
	f.click("cell:10,8")
	var moved tilefloor.UpdatePortalArgs
	if updates := f.requests.Of(tilefloor.UpdatePortalEdit); len(updates) != 2 || updates[1].Decode(&moved) != nil ||
		moved.Portal.Col != 10 || moved.Portal.Change != floor.ChangeFade {
		t.Fatalf("move %+v", moved)
	}
	f.apply(groundID, tilefloor.UpdatePortal{}, moved)
	f.rightClick("cell:10,8")
	f.click("menu:lead-portal")
	here := []protocol.PickMap{
		{Window: otherMapID, Zone: zoneID, Shows: true}, {Window: frametest.ID(99), Zone: zoneID, Shows: true},
		{Window: frametest.ID(98), Zone: cellarID, Shows: true},
	}
	start = f.leadTo(zoneID, here, groundID, deskID, "Desk")
	if start.Window != frametest.ID(99) || start.Now.Item != footID.String() || start.Now.Name != "Foot" {
		t.Fatalf("a portal into this zone is picked in this Map, with where it leads now marked: %+v", start)
	}
	var led tilefloor.UpdatePortalArgs
	desk := floor.AnchorRef{Floor: groundID, Anchor: deskID}
	if updates := f.requests.Of(tilefloor.UpdatePortalEdit); len(updates) != 3 || updates[2].Decode(&led) != nil ||
		led.Portal.To != desk || led.Portal.Col != 10 {
		t.Fatalf("lead %+v", led)
	}
	f.apply(groundID, tilefloor.UpdatePortal{}, led)
	f.keys()
	if !f.has("portal:10,8:to:Desk") {
		t.Fatal("the portal leads to the Desk, in its own zone")
	}
	f.rightClick("cell:10,8")
	f.click("menu:remove-portal")
	var removed tilefloor.RemovePortalArgs
	if removes := f.requests.Of(tilefloor.RemovePortalEdit); len(removes) != 1 || removes[0].Decode(&removed) != nil ||
		removed.ID != portalID {
		t.Fatalf("remove %+v", removed)
	}
}

// TestAPortalIsPickedInAMapAlreadyOpen picks a portal's anchor in a Map that
// already shows the zone chosen, opening no other.
func TestAPortalIsPickedInAMapAlreadyOpen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withCellar()
	f.rightClick("cell:9,8")
	f.click("menu:add-portal")
	start := f.leadTo(cellarID, []protocol.PickMap{
		{Window: frametest.ID(99), Zone: zoneID, Shows: true}, {Window: otherMapID, Zone: cellarID, Shows: true},
	}, cellarFloorID, footID, "Foot")
	if start.Window != otherMapID {
		t.Fatalf("the pick is in the Cellar's open Map: %+v", start)
	}
	f.rightClick("cell:8,8")
	f.click("menu:add-portal")
	f.click("ui:" + cellarID.String())
	f.click("ui:cancel")
	if len(f.requests.PickAsks) != 1 || len(f.requests.Of(tilefloor.AddPortalEdit)) != 1 {
		t.Fatal("cancelling the question of which zone adds nothing")
	}
}

// TestAPortalThatLeadsNowhereSaysSo flags a portal whose anchor is gone,
// and offers no opening of where it leads; and deleting an anchor names the
// portal of its own floor that leads to it.
func TestAPortalThatLeadsNowhereSaysSo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withCellar()
	for index, portal := range []floor.Portal{
		{ID: portalID, Col: 9, Row: 8, To: floor.AnchorRef{Floor: cellarFloorID, Anchor: frametest.ID(77)}, Change: floor.ChangeCut},
		{ID: frametest.ID(65), Col: 10, Row: 8, To: floor.AnchorRef{Floor: groundID, Anchor: deskID}, Change: floor.ChangeFade},
	} {
		f.apply(groundID, tilefloor.AddPortal{}, tilefloor.AddPortalArgs{Portal: portal, Index: index})
	}
	f.keys()
	if !f.has("portal:9,8:nowhere") || f.has("portal:10,8:nowhere") || !f.has("portal:10,8:to:Desk") {
		t.Fatal("the portal to an anchor that is gone leads nowhere; the other leads to the Desk")
	}
	f.rightClick("cell:9,8")
	if !strings.Contains(f.menu(), "open-portal (disabled), lead-portal") {
		t.Fatalf("where it leads cannot be opened, and it can be led somewhere else: %s", f.menu())
	}
	f.keys("Escape")
	f.rightClick("cell:4,4")
	f.click("menu:delete-anchor")
	if len(f.requests.Uses) != 1 {
		t.Fatal("deleting the anchor asks where it is used")
	}
	f.requests.Uses[0](nil, nil)
	lines := strings.Join(f.state.DialogLines(), "\n")
	if !strings.Contains(lines, "used in 1 place") || !strings.Contains(lines, "Ground (a portal at 10, 8)") {
		t.Fatalf("the portal on the anchor's own floor is named: %s", lines)
	}
}
