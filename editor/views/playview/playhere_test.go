package playview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/views/playview"
	"github.com/bradhannah/berad-engine/engine/game"
)

// TestPlayFromHereLaunchesOnceFromACell launches with the player on the cell
// Play from here gives, even with no player start, and says so; Launch then
// plays from the player start again (increment 47).
func TestPlayFromHereLaunchesOnceFromACell(t *testing.T) {
	t.Parallel()
	f := newFixture(t, false)
	start := game.Start{Zone: zoneID, Floor: groundID, Col: 2, Row: 3}
	f.state.Changed(client.Update{Attention: true, Item: playview.LaunchFromItem(start)}, f.ctx)
	f.frame(nil)
	if !f.has("playing") || !f.has("player:2,3") {
		t.Fatal("Play from here puts the player on the cell")
	}
	if text, _ := f.ctx.Frame.Message(); !strings.HasPrefix(text, "Playing from 2,3 for this run") {
		t.Fatalf("and says it is for this run: %q", text)
	}
	f.state.Changed(client.Update{Attention: true, Item: playview.LaunchItem}, f.ctx)
	f.frame(nil)
	if f.has("playing") || !f.has("problem:No player start yet") {
		t.Fatal("Launch plays from the player start again, and there is none")
	}
}
