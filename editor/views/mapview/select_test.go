package mapview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/engine/game"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// drag presses on one place, moves to another with the button held, and
// releases there.
func (f *fixture) drag(from, to string) {
	f.t.Helper()
	f.frame(pointer(f.at(from), true, true, false))
	f.frame(pointer(f.at(to), false, true, false))
	f.frame(pointer(f.at(to), false, false, true))
}

// crowded puts on the ground floor, beside the Desk anchor at 4,4: Ada
// Clerk at 9,8; a portal and an ambient sound sharing 10,3, with an anchor,
// Bench; a portal alone at 10,5; a sound alone at 10,7; and the player start
// at 9,5.
func crowded(f *fixture) {
	f.t.Helper()
	withClerk(f)
	ground := groundFloor(f.t)
	leads := floor.AnchorRef{Floor: groundID, Anchor: deskID}
	for _, err := range []error{
		ground.Floor.InsertPlacement(0, floor.Placement{ID: placedID, NPC: clerkID, Col: 9, Row: 8}),
		ground.Floor.InsertAnchor(1, floor.Anchor{ID: frametest.ID(80), Name: "Bench", Col: 10, Row: 3}),
		ground.Floor.InsertPortal(0, floor.Portal{ID: frametest.ID(81), Col: 10, Row: 3, To: leads, Change: floor.ChangeFade}),
		ground.Floor.InsertPortal(1, floor.Portal{ID: frametest.ID(82), Col: 10, Row: 5, To: leads, Change: floor.ChangeFade}),
		ground.Floor.InsertSound(0, floor.Sound{ID: frametest.ID(83), Cue: "town/fountain", Col: 10, Row: 3}),
		ground.Floor.InsertSound(1, floor.Sound{ID: frametest.ID(84), Cue: "town/brook", Col: 10, Row: 7}),
	} {
		if err != nil {
			f.t.Fatal(err)
		}
	}
	f.docs.Put(groundID, "Ground", ground)
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: zoneID, Floor: groundID, Col: 9, Row: 5}
	f.docs.Put(game.SettingsID(), game.SettingsName, &gameplugin.SettingsDocument{Settings: settings})
	snapshot := game.SettingsID()
	f.state.Changed(client.Update{Snapshot: &snapshot}, f.ctx)
	f.keys()
}

// TestSelectSelectsOneThing selects each kind of thing with a click of the
// Select tool, one at a time: an anchor, an NPC, a portal, an ambient
// sound, the player start, and a wall's run. A click on a cell with nothing
// to select, and Escape, select nothing; and a click never changes the
// project.
func TestSelectSelectsOneThing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	crowded(f)
	f.keys("S")
	if f.state.Tool() != mapview.ToolSelect || !f.has("selected:none") {
		t.Fatal("S takes Select in hand, with nothing selected")
	}
	for cell, kind := range map[string]string{
		"4,4": "anchor", "9,8": "npc", "10,5": "portal", "10,7": "sound", "9,5": "start",
	} {
		f.click("cell:" + cell)
		if !f.has("selected:" + kind + ":" + cell) {
			t.Fatalf("a click on %s selects the %s there", cell, kind)
		}
		for other, otherKind := range map[string]string{
			"4,4": "anchor", "9,8": "npc", "10,5": "portal", "10,7": "sound", "9,5": "start",
		} {
			if other != cell && f.has("selected:"+otherKind+":"+other) {
				t.Fatalf("with the %s selected, the %s is not", kind, otherKind)
			}
		}
	}
	f.click("cell:2,4")
	if len(f.state.Selected()) == 0 || f.has("selected:start:9,5") {
		t.Fatal("a click on a wall selects its run, and nothing else")
	}
	f.click("cell:9,2")
	if !f.has("selected:none") {
		t.Fatal("a click on a cell with nothing to select selects nothing")
	}
	f.click("cell:10,5")
	f.keys("Escape")
	if !f.has("selected:none") {
		t.Fatal("Escape selects nothing")
	}
	if got := len(f.requests.Commands); got != 0 {
		t.Fatalf("selecting changed the project: %d commands", got)
	}
}

// TestSelectAsksWhichOfSeveral clicks a cell that holds an anchor, a portal
// and an ambient sound: a menu at the cell lists the three, and the one
// chosen is selected. Escape selects nothing. Clicking the cell again asks
// again, so another of them can be chosen.
func TestSelectAsksWhichOfSeveral(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	crowded(f)
	f.keys("S")
	f.click("cell:10,3")
	if got := f.menu(); got != "select:anchor, select:portal, select:sound" {
		t.Fatalf("the menu lists what is on the cell: %s", got)
	}
	if !f.has("selected:none") {
		t.Fatal("nothing is selected until one is chosen")
	}
	f.keys("Escape")
	if f.state.MenuItems() != nil || !f.has("selected:none") {
		t.Fatal("Escape closes the menu and selects nothing")
	}
	f.click("cell:10,3")
	f.click("menu:select:portal")
	if !f.has("selected:portal:10,3") || f.has("selected:anchor:10,3") || f.has("selected:sound:10,3") {
		t.Fatal("the portal chosen is selected, alone")
	}
	if !strings.Contains(f.status(), "Selected portal to Desk") {
		t.Fatalf("the status bar says what was selected: %q", f.status())
	}
	f.click("cell:10,3")
	if got := f.menu(); got != "select:anchor, select:portal, select:sound" {
		t.Fatalf("a click on the cell asks again: %s", got)
	}
	f.click("menu:select:sound")
	if !f.has("selected:sound:10,3") || f.has("selected:portal:10,3") {
		t.Fatal("the sound chosen is selected in the portal's place")
	}
}

// TestSelectDragsToMove drags each kind of thing to another cell with the
// Select tool: one edit each. On a cell with several things the drag moves
// the one selected. A cell that already holds one of the same kind refuses,
// and the player start refuses a cell nothing can stand on.
func TestSelectDragsToMove(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	crowded(f)
	f.keys("S")
	f.drag("cell:4,4", "cell:5,4")
	var anchor tilefloor.UpdateAnchorArgs
	if moves := f.requests.Of(tilefloor.UpdateAnchorEdit); len(moves) != 1 || moves[0].Decode(&anchor) != nil ||
		anchor.Anchor.ID != deskID || anchor.Anchor.Col != 5 {
		t.Fatalf("the anchor moves: %+v", anchor)
	}
	f.drag("cell:10,5", "cell:11,5")
	var portal tilefloor.UpdatePortalArgs
	if moves := f.requests.Of(tilefloor.UpdatePortalEdit); len(moves) != 1 || moves[0].Decode(&portal) != nil ||
		portal.Portal.ID != frametest.ID(82) || portal.Portal.Col != 11 {
		t.Fatalf("the portal moves: %+v", portal)
	}
	f.drag("cell:10,7", "cell:11,7")
	var sound tilefloor.UpdateSoundArgs
	if moves := f.requests.Of(tilefloor.UpdateSoundEdit); len(moves) != 1 || moves[0].Decode(&sound) != nil ||
		sound.Sound.ID != frametest.ID(84) || sound.Sound.Col != 11 {
		t.Fatalf("the sound moves: %+v", sound)
	}
	f.drag("cell:9,5", "cell:9,6")
	var start gameplugin.StartArgs
	if moves := f.requests.Of(gameplugin.StartEdit); len(moves) != 1 || moves[0].Decode(&start) != nil ||
		start.Start.Row != 6 || start.Start.Floor != groundID {
		t.Fatalf("the player start moves: %+v", start)
	}
	f.drag("cell:9,5", "cell:2,2")
	if len(f.requests.Of(gameplugin.StartEdit)) != 1 || !strings.Contains(f.status(), "The player cannot start there") {
		t.Fatalf("the start refuses a wall: %q", f.status())
	}
	f.drag("cell:10,5", "cell:10,3")
	if len(f.requests.Of(tilefloor.UpdatePortalEdit)) != 1 || !strings.Contains(f.status(), "Another portal is already there") {
		t.Fatalf("a cell with a portal refuses another: %q", f.status())
	}
	f.drag("cell:10,3", "cell:11,3")
	if got := len(f.requests.Commands); got != 4 {
		t.Fatalf("with none of a crowded cell's things selected, a drag moves nothing: %d commands", got)
	}
	f.click("cell:10,3")
	f.click("menu:select:sound")
	f.drag("cell:10,3", "cell:11,3")
	if moves := f.requests.Of(tilefloor.UpdateSoundEdit); len(moves) != 2 || moves[1].Decode(&sound) != nil ||
		sound.Sound.ID != frametest.ID(83) || sound.Sound.Col != 11 {
		t.Fatalf("the drag moves the one selected, the sound: %+v", sound)
	}
}

// TestTheAddToolsOnlyAdd clicks the Anchors tool on a cell with an anchor:
// nothing is added or selected, and the status bar says how to change it.
func TestTheAddToolsOnlyAdd(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.keys("A")
	f.click("cell:4,4")
	if len(f.requests.Commands) != 0 || !f.state.SelectedAnchor().IsZero() ||
		!strings.Contains(f.status(), "An anchor is already here. Select (S) selects it") {
		t.Fatalf("the Anchors tool only adds: %q", f.status())
	}
	f.drag("cell:4,4", "cell:5,4")
	if len(f.requests.Of(tilefloor.UpdateAnchorEdit)) != 0 {
		t.Fatal("the Anchors tool does not move an anchor")
	}
	f.click("cell:9,8")
	if len(f.requests.Of(tilefloor.AddAnchorEdit)) != 1 {
		t.Fatal("on a cell with no anchor it adds one")
	}
}
