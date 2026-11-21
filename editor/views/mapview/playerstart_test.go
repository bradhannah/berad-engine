package mapview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/playview"
	"github.com/bradhannah/berad-engine/engine/game"
)

// TestSettingThePlayerStart sets the player start on a cell of the ground
// floor from the right-click menu, which then says the player starts there;
// the marker shows on that floor and not the one above. A wall is no place
// to start.
func TestSettingThePlayerStart(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.docs.Put(game.SettingsID(), game.SettingsName, &gameplugin.SettingsDocument{Settings: game.DefaultSettings()})
	snapshot := game.SettingsID()
	f.state.Changed(client.Update{Snapshot: &snapshot}, f.ctx)
	f.rightClick("cell:3,3")
	f.click("menu:set-start")
	set := f.requests.Of(gameplugin.StartEdit)
	var args gameplugin.StartArgs
	want := game.Start{Zone: zoneID, Floor: groundID, Col: 3, Row: 3}
	if len(set) != 1 || set[0].Command.Document != game.SettingsID() || set[0].Decode(&args) != nil ||
		args.Start != want {
		t.Fatalf("set start %d %+v", len(set), args)
	}
	f.apply(game.SettingsID(), gameplugin.SetStart{}, args)
	f.state.Changed(client.Update{Snapshot: &snapshot}, f.ctx)
	if !f.has("player-start:3,3") || f.has("player-start:4,3") {
		t.Fatal("the marker shows on the start's cell")
	}
	f.rightClick("cell:3,3")
	if got := f.menu(); !strings.Contains(got, "set-start (disabled)") {
		t.Fatalf("the menu says the player starts here: %s", got)
	}
	f.keys("Escape")
	f.keys("PageUp")
	if f.has("player-start:3,3") {
		t.Fatal("the marker shows only on the start's floor")
	}
	f.keys("PageDown")
	f.rightClick("cell:2,2")
	if got := f.menu(); !strings.Contains(got, "set-start (disabled)") {
		t.Fatalf("a wall is no place to start: %s", got)
	}
}

// TestMoveThePlayerHereDuringAPreview offers Move the player here only
// while this Map runs a preview. Chosen on a cell, it puts the player there
// and the preview goes on, its clock as it was; on a wall it is disabled.
func TestMoveThePlayerHereDuringAPreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.rightClick("cell:5,5")
	if strings.Contains(f.menu(), "move-here") {
		t.Fatalf("with no game running there is no player to move: %s", f.menu())
	}
	f.keys("Escape")
	f.startPreview("cell:3,3")
	f.idle(30)
	f.rightClick("cell:2,2")
	if !strings.Contains(f.menu(), "move-here (disabled)") {
		t.Fatalf("the player cannot be moved onto a wall: %s", f.menu())
	}
	f.keys("Escape")
	f.rightClick("cell:5,5")
	if !strings.Contains(f.menu(), "move-here") || strings.Contains(f.menu(), "move-here (disabled)") {
		t.Fatalf("during a preview the player can be moved: %s", f.menu())
	}
	f.click("menu:move-here")
	if !f.has("player:5,5") || !f.has("previewing") {
		t.Fatal("the player stands on the cell chosen, and the preview goes on")
	}
	if !strings.HasPrefix(f.status(), "Moved the player to cell 5, 5") {
		t.Fatalf("the move is said: %q", f.status())
	}
	f.walk(ui.KeyLeft, 1)
	if !f.has("player:4,5") {
		t.Fatal("the player walks on from there")
	}
}

// TestMoveThePlayerHereReachesLaunchsGame offers Move the player here while
// Launch's game runs in the Play tab, and asks the Play tab to move its
// player to the cell, in this Map's zone; once that game has stopped, the
// item is gone.
func TestMoveThePlayerHereReachesLaunchsGame(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	playing := func(running bool) {
		value := `{"running":false}`
		if running {
			value = `{"running":true}`
		}
		f.state.Changed(client.Update{Session: &protocol.Session{Topic: protocol.TopicPlaying, Value: []byte(value)}}, f.ctx)
	}
	playing(true)
	f.rightClick("cell:5,5")
	if !strings.Contains(f.menu(), "move-here") || strings.Contains(f.menu(), "move-here (disabled)") {
		t.Fatalf("while Launch's game runs its player can be moved: %s", f.menu())
	}
	f.click("menu:move-here")
	want := playview.MoveToItem(game.Start{Zone: zoneID, Floor: groundID, Col: 5, Row: 5})
	if opened := f.requests.Opened; len(opened) != 1 || opened[0].View != playview.ViewName || opened[0].Item != want {
		t.Fatalf("the Play tab is asked to move its player here: %+v", opened)
	}
	playing(false)
	f.rightClick("cell:5,5")
	if strings.Contains(f.menu(), "move-here") {
		t.Fatalf("with the game stopped there is no player to move: %s", f.menu())
	}
}
