package player_test

import (
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/player"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

func id(number byte) ids.ULID {
	var made ids.ULID
	made[len(made)-1] = number

	return made
}

// speaker records what a game's sound asks to be played.
type speaker struct {
	prepared []string
	played   []string
	stopped  int
	layers   map[string]player.Layer
}

func (s *speaker) Prepare(cues []string)        { s.prepared = append(s.prepared, cues...) }
func (s *speaker) Play(cue string)              { s.played = append(s.played, cue) }
func (s *speaker) PlayAt(cue, step string)      { s.played = append(s.played, cue+"@"+step) }
func (s *speaker) Stop()                        { s.stopped++ }
func (s *speaker) PlayLayer(layer player.Layer) { s.layers[layer.Slot] = layer }
func (s *speaker) SetLayer(layer player.Layer)  { s.layers[layer.Slot] = layer }
func (s *speaker) StopLayer(slot string)        { delete(s.layers, slot) }

// town starts a game in a zone that plays music throughout, on a 12 by 10
// floor with a fountain at 8,5 and Ada Clerk, whose day rings a bell then waits an hour, with
// the player at 5,5.
func town(t *testing.T) *game.Game {
	t.Helper()
	ground, err := tilefloor.New(12, 10, "grass")
	if err != nil {
		t.Fatal(err)
	}
	if err := ground.InsertSound(0, tilefloor.Sound{ID: id(20), Cue: "town/fountain", Col: 8, Row: 5}); err != nil {
		t.Fatal(err)
	}
	if err := ground.InsertPlacement(0, tilefloor.Placement{ID: id(21), NPC: id(4), Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	zoned := zone.New()
	if err := zoned.Insert(0, id(2)); err != nil {
		t.Fatal(err)
	}
	if err := zoned.InsertSound(0, "music/town"); err != nil {
		t.Fatal(err)
	}
	day, err := flow.New(id(30))
	if err != nil {
		t.Fatal(err)
	}
	ring, err := flow.NewNode(flow.KindPlaySound)
	if err != nil {
		t.Fatal(err)
	}
	ring.ID = id(31)
	if ring, err = ring.WithParam("cue", "bell"); err != nil {
		t.Fatal(err)
	}
	wait, err := flow.NewNode(flow.KindWait)
	if err != nil {
		t.Fatal(err)
	}
	wait.ID = id(32)
	if wait, err = wait.WithParam("duration", flow.Duration{Amount: 1, Unit: flow.UnitHours, Clock: flow.ClockGame}); err != nil {
		t.Fatal(err)
	}
	if err := day.Insert([]flow.Node{ring, wait}, nil, []flow.Link{
		{From: id(30), Output: flow.OutputNext, To: ring.ID}, {From: ring.ID, Output: flow.OutputNext, To: wait.ID},
	}); err != nil {
		t.Fatal(err)
	}
	clerk := npc.New()
	clerk.Behaviour = id(5)
	settings := game.DefaultSettings()
	settings.Start = game.Start{Zone: id(1), Floor: id(2), Col: 5, Row: 5}
	played, err := game.New(game.Project{
		Settings: settings, Zone: zoned, Floors: map[ids.ULID]*tilefloor.Floor{id(2): ground},
		NPCs: map[ids.ULID]npc.NPC{id(4): clerk}, Workflows: map[ids.ULID]*flow.Workflow{id(5): day},
		Values: nil, Seconds: 8 * 3600, Seed: 1, Names: nil, Signals: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	return played
}

// TestSoundPlaysTheZoneTheCuesAndWhatIsNear starts the zone's sounds and
// readies the workflows' cues when the game starts; plays what NPCs ask
// for; plays an ambient sound only once its distance is known and the
// player is within it, louder closer; and stops everything when told.
func TestSoundPlaysTheZoneTheCuesAndWhatIsNear(t *testing.T) {
	t.Parallel()
	played, heard, sound := town(t), &speaker{prepared: nil, played: nil, stopped: 0, layers: map[string]player.Layer{}}, player.NewSound()
	sound.Start(heard, heard, played, []string{"bell"})
	if !slices.Equal(heard.prepared, []string{"bell"}) || heard.layers["zone:music/town"].Gain != 1 || !sound.Playing("music/town") {
		t.Fatalf("the zone's music plays and the bell is ready: %+v", heard)
	}
	fountain := "ambient:" + id(20).String()
	for range 30 {
		sound.Update(heard, heard, played, played.Step(play.Input{Held: 0, Pressed: 0}))
	}
	if !slices.Contains(heard.played, "bell") || sound.Heard("bell") == 0 {
		t.Fatalf("the clerk's bell is played: %v", heard.played)
	}
	if _, playing := heard.layers[fountain]; playing {
		t.Fatal("no ambient sound plays before its distance is known")
	}
	sound.KnowRadius(func(cue string) float64 {
		if cue == "town/fountain" {
			return 5
		}

		return 0
	})
	sound.Update(heard, heard, played, played.Step(play.Input{Held: 0, Pressed: 0}))
	far := heard.layers[fountain].Gain
	if far <= 0 || far >= 1 || !sound.Playing("town/fountain") {
		t.Fatalf("three cells away, within five, the fountain plays quietly: %v", far)
	}
	for range 40 {
		sound.Update(heard, heard, played, played.Step(play.Input{Held: play.East, Pressed: 0}))
	}
	if near := heard.layers[fountain].Gain; near <= far {
		t.Fatalf("closer, the fountain is louder: %v then %v", far, near)
	}
	for range 200 {
		sound.Update(heard, heard, played, played.Step(play.Input{Held: play.West, Pressed: 0}))
	}
	if _, playing := heard.layers[fountain]; playing || sound.Playing("town/fountain") {
		t.Fatal("out of its distance, the fountain stops")
	}
	sound.Silence(heard, heard)
	if len(heard.layers) != 0 || heard.stopped != 1 || sound.Playing("music/town") {
		t.Fatalf("everything stops: %+v", heard)
	}
}
