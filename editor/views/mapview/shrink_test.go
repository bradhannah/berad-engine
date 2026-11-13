package mapview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/game"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// TestAShrinkListsAllThatGoes removes the ground floor's right column, which
// holds an NPC, an ambient sound and stairs, and lies under stairs on the
// floor above that lead down into it: the question lists each before
// anything changes (increment 58).
func TestAShrinkListsAllThatGoes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ground := groundFloor(t)
	if err := ground.Floor.InsertPlacement(0, floor.Placement{ID: frametest.ID(70), NPC: frametest.ID(71), Col: 11, Row: 5}); err != nil {
		t.Fatal(err)
	}
	if err := ground.Floor.InsertSound(0, floor.Sound{ID: frametest.ID(72), Cue: "waves", Col: 11, Row: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := ground.Floor.SetTop(11, 3, "stairs.up"); err != nil {
		t.Fatal(err)
	}
	upper, err := tilefloor.NewDocument(12, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := upper.Floor.SetTop(11, 4, "stairs.down"); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(groundID, "Ground", ground)
	f.docs.Put(upperID, "Upper", upper)
	f.keys("R")
	f.click("ui:shrink-right")
	if len(resizes(t, f.requests)) != 0 {
		t.Fatal("nothing changes before the question is answered")
	}
	lines := strings.Join(f.state.DialogLines(), "\n")
	for _, want := range []string{
		"This takes 1 NPC off the map", "This removes the ambient sound: waves.", "This removes the stairs at 11,3.",
		"It leaves the stairs on Upper at 11,4, which lead down into them, leading nowhere.",
	} {
		if !strings.Contains(lines, want) {
			t.Fatalf("the question says %q:\n%s", want, lines)
		}
	}
	f.click("ui:resize")
	if got := resizes(t, f.requests); len(got) != 1 || got[0].Bounds.Width != 11 {
		t.Fatalf("resize %+v", got)
	}
}

// TestAShrinkKeepsThePlayerStart refuses a resize that would leave the
// player start off the floor, until the start is moved (increment 58).
func TestAShrinkKeepsThePlayerStart(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: zoneID, Floor: groundID, Col: 11, Row: 7}
	f.docs.Put(game.SettingsID(), game.SettingsName, &gameplugin.SettingsDocument{Settings: settings})
	snapshot := game.SettingsID()
	f.state.Changed(client.Update{Snapshot: &snapshot}, f.ctx)
	f.idle(2)
	f.keys("R")
	f.click("ui:shrink-right")
	if len(resizes(t, f.requests)) != 0 || f.state.DialogLines() != nil {
		t.Fatal("the resize is refused, not asked")
	}
	if got := f.status(); !strings.Contains(got, "The player start, 11,7, would be off the floor. Move it first") {
		t.Fatalf("the refusal says why: %q", got)
	}
}
