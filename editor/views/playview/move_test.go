package playview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/playview"
	"github.com/bradhannah/berad-engine/engine/game"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

var (
	harbourID = frametest.ID(60)
	quayID    = frametest.ID(61)
)

func (f *fixture) message() string {
	text, _ := f.ctx.Frame.Message()

	return text
}

// move asks the Play tab to move the player, as a Map's Move the player
// here does, and runs a frame.
func (f *fixture) move(start game.Start) {
	f.state.Changed(client.Update{Attention: true, Item: playview.MoveToItem(start)}, f.ctx)
	f.frame(nil)
}

// addHarbour gives the project a second zone, the Harbour, of one floor of
// sand, the Quay, which plays the harbour's music throughout.
func (f *fixture) addHarbour() {
	f.t.Helper()
	quay, err := tilefloor.NewDocument(6, 6, "sand")
	if err != nil {
		f.t.Fatal(err)
	}
	harbour := zone.New()
	if err := harbour.Insert(0, quayID); err != nil {
		f.t.Fatal(err)
	}
	if err := harbour.InsertSound(0, "music/harbour"); err != nil {
		f.t.Fatal(err)
	}
	var entries []project.Entry
	f.docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = index.Entries()
		}
	})
	entries = append(entries, project.Entry{ID: harbourID, Name: "Harbour", Format: zone.FormatName},
		project.Entry{ID: quayID, Name: "Quay", Format: floor.FormatName})
	f.docs.Put(project.IndexID(), "Project", project.NewIndex(entries))
	f.docs.Put(harbourID, "Harbour", &zoneplugin.Document{Zone: harbour})
	f.docs.Put(quayID, "Quay", quay)
}

// TestMoveThePlayerWithinTheZone moves the player of the running game to
// another cell of its zone: the game is not launched again, and goes on. A
// wall refuses the move and the player stays. The Maps are told a game is
// running.
func TestMoveThePlayerWithinTheZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	if !f.has("playing") || !f.has("player:5,5") {
		t.Fatal("the game starts at the player start")
	}
	told := f.requests.Published
	if len(told) != 1 || told[0].Topic != protocol.TopicPlaying || string(told[0].Value) != `{"running":true}` {
		t.Fatalf("the Maps are told a game is running: %+v", told)
	}
	f.move(game.Start{Zone: zoneID, Floor: groundID, Col: 2, Row: 3})
	if !f.has("player:2,3") || !f.has("launches:1") {
		t.Fatal("the player is moved, and the game is the one that was running")
	}
	if !strings.HasPrefix(f.message(), "Moved the player to cell 2, 3") {
		t.Fatalf("the move is said: %q", f.message())
	}
	f.move(game.Start{Zone: zoneID, Floor: groundID, Col: 6, Row: 5})
	if !f.has("player:2,3") || !strings.Contains(f.message(), "cannot be moved there") {
		t.Fatalf("a wall refuses the move, and the player stays: %q", f.message())
	}
}

// TestMoveThePlayerToAnotherZone moves the player into the Harbour: the
// game goes on there, on the Quay, the town's music stops and the harbour's
// plays. Moved back, the town's music plays again.
func TestMoveThePlayerToAnotherZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	f.addHarbour()
	f.frame(nil)
	if !f.has("layer:music/town") {
		t.Fatal("the town's music plays")
	}
	f.move(game.Start{Zone: harbourID, Floor: quayID, Col: 1, Row: 2})
	f.frame(nil)
	if !f.has("playing") || !f.has("player:1,2") || !f.has("floor:1") || !f.has("launches:1") {
		t.Fatal("the same game goes on in the Harbour, with the player on the Quay")
	}
	if !f.has("ground:1,2:sand") || f.has("layer:music/town") || !f.has("layer:music/harbour") {
		t.Fatal("the Quay is under the player, the town's music has stopped and the harbour's plays")
	}
	if !strings.Contains(f.message(), "of Harbour") {
		t.Fatalf("the move names the zone: %q", f.message())
	}
	f.move(game.Start{Zone: zoneID, Floor: groundID, Col: 5, Row: 5})
	f.frame(nil)
	if !f.has("player:5,5") || !f.has("layer:music/town") || f.has("layer:music/harbour") {
		t.Fatal("moved back, the game goes on in the town, with its music")
	}
}

// TestMovingThePlayerWithNoGameLaunchesNothing asks to move a player when
// no game is running: the tab says so, and launches nothing.
func TestMovingThePlayerWithNoGameLaunchesNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t, false)
	if f.has("playing") {
		t.Fatal("with no player start, no game runs")
	}
	f.move(game.Start{Zone: zoneID, Floor: groundID, Col: 2, Row: 3})
	if f.has("playing") || !strings.Contains(f.message(), "No game is running to move the player in") {
		t.Fatalf("no game is launched, and the tab says why: %q", f.message())
	}
	if len(f.requests.Published) != 0 {
		t.Fatalf("the Maps are told nothing, as no game ever ran: %+v", f.requests.Published)
	}
}
