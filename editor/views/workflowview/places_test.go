package workflowview_test

import (
	"encoding/json"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

var shelfID = frametest.ID(88)

// withTown puts the office in a zone, Town, with a shelf beside its desk,
// so the list of places has both.
func (f *fixture) withTown() {
	f.t.Helper()
	town := zoneplugin.NewDocument()
	if err := town.Zone.Insert(0, floorID); err != nil {
		f.t.Fatal(err)
	}
	f.docs.Put(frametest.ID(87), "Town", town)
	f.docs.View(floorID, func(document client.Document) {
		office := document.Model.(*tilefloor.Document)
		if err := office.Floor.InsertAnchor(1, floor.Anchor{ID: shelfID, Name: "Shelf", Col: 1, Row: 1}); err != nil {
			f.t.Fatal(err)
		}
	})
	var entries []project.Entry
	f.docs.View(project.IndexID(), func(document client.Document) {
		entries = document.Model.(*project.Index).Entries()
	})
	entries = append(entries, project.Entry{ID: frametest.ID(87), Name: "Town", Format: zone.FormatName})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(entries))
}

// TestChoosingAPlaceByName chooses a Call's place input, a place local's
// start and a Go to's place from the list each box opens, the status naming
// the place and what Undo puts back; the list's last row picks on a map.
func TestChoosingAPlaceByName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withCall()
	f.withTown()
	f.click("ui:input-place-place")
	f.click("place:Shelf")
	var given worldstate.Place
	if err := json.Unmarshal(f.nth(flow.KindCall, 1).Inputs()[placeIn], &given); err != nil ||
		given != (worldstate.Place{Floor: floorID, Anchor: shelfID}) {
		t.Fatalf("the input is given the shelf: %s", f.nth(flow.KindCall, 1).Inputs()[placeIn])
	}
	if got := f.status(); got != "Chose “Shelf” for place." {
		t.Fatalf("the status names the place chosen: %q", got)
	}
	f.click("ui:input-place-place")
	f.click("place:Office desk 3")
	if got := f.status(); got != "Chose “Office desk 3” for place. Undo puts “Shelf” back." {
		t.Fatalf("the status says what Undo puts back: %q", got)
	}
	f.keys("Escape")
	f.click("ui:add-local")
	f.click("ui:value-type-place")
	f.click("ui:value-place")
	f.click("place:Shelf")
	if start := f.local().Zero().Place; start != (worldstate.Place{Floor: floorID, Anchor: shelfID}) {
		t.Fatalf("the local starts at the shelf: %+v", start)
	}
	asks := len(f.requests.PickAsks)
	f.click("ui:value-place")
	f.click("ui:" + "pick-on-map")
	if len(f.requests.PickAsks) != asks+1 {
		t.Fatal("the list's last row picks on a map")
	}
	f.click("ui:value-clear-place")
	if start := f.local().Zero().Place; !start.IsZero() {
		t.Fatalf("Clear leaves no place: %+v, %q", start, f.status())
	}
}

// TestGoToChoosesAPlace chooses a Go to's place from its box's list.
func TestGoToChoosesAPlace(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withTown()
	f.click("node:start:1")
	f.click("palette:" + flow.KindGoTo)
	f.click("ui:place")
	f.click("place:Shelf")
	destination, err := f.nth(flow.KindGoTo, 1).Anchor("destination")
	if err != nil || destination != (floor.AnchorRef{Floor: floorID, Anchor: shelfID}) {
		t.Fatalf("Go to goes to the shelf: %+v %v", destination, err)
	}
}
