package mapview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
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
