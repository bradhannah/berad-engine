package mapview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// answerAudio answers every audio library question with two ambient cues
// and a sound effect.
func (f *fixture) answerAudio() {
	library := protocol.AudioLibrary{Cues: []protocol.AudioCue{
		{ID: "town/fountain", Name: "Fountain", Category: protocol.CategoryAmbient,
			Ambient: &protocol.AmbientSettings{RadiusTiles: 4, FadeInMs: 0, FadeOutMs: 0}},
		{ID: "town/brook", Name: "Brook", Category: protocol.CategoryAmbient,
			Ambient: &protocol.AmbientSettings{RadiusTiles: 6, FadeInMs: 0, FadeOutMs: 0}},
		{ID: "door/open", Name: "Door opens", Category: protocol.CategorySFX},
	}}
	for _, ask := range f.requests.AudioAsks {
		ask(library, nil)
	}
	f.requests.AudioAsks = nil
}

// TestPlacingAnAmbientSound places the fountain on a cell, choosing among
// the ambient cues only, then gives it the brook's cue, moves it and removes
// it, each one floor edit with undo.
func TestPlacingAnAmbientSound(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.rightClick("cell:9,8")
	f.answerAudio()
	f.keys("Escape")
	f.rightClick("cell:9,8")
	f.answerAudio()
	if !strings.Contains(f.menu(), "place-sound,") {
		t.Fatalf("an ambient sound can be placed: %s", f.menu())
	}
	f.click("menu:place-sound")
	if got := f.menu(); got != "sound-cue:town/brook, sound-cue:town/fountain" {
		t.Fatalf("only ambient cues are offered, by name: %s", got)
	}
	f.click("menu:sound-cue:town/fountain")
	added := f.requests.Of(tilefloor.AddSoundEdit)
	var add tilefloor.AddSoundArgs
	if len(added) != 1 || added[0].Decode(&add) != nil || add.Sound.Cue != "town/fountain" || add.Sound.Col != 9 ||
		add.Sound.Row != 8 {
		t.Fatalf("add %d %+v", len(added), add)
	}
	add.Sound.ID = placedID
	f.apply(groundID, tilefloor.AddSound{}, add)
	if !f.has("ambient:9,8:town/fountain") {
		t.Fatal("the fountain is on the map")
	}
	f.rightClick("cell:9,8")
	f.answerAudio()
	f.click("menu:change-sound")
	f.click("menu:sound-cue:town/brook")
	var changed tilefloor.UpdateSoundArgs
	if updates := f.requests.Of(tilefloor.UpdateSoundEdit); len(updates) != 1 || updates[0].Decode(&changed) != nil ||
		changed.Sound.Cue != "town/brook" || changed.Sound.ID != placedID {
		t.Fatalf("change %+v", changed)
	}
	f.apply(groundID, tilefloor.UpdateSound{}, changed)
	f.rightClick("cell:9,8")
	f.answerAudio()
	f.click("menu:move-sound")
	f.click("cell:10,8")
	var moved tilefloor.UpdateSoundArgs
	if updates := f.requests.Of(tilefloor.UpdateSoundEdit); len(updates) != 2 || updates[1].Decode(&moved) != nil ||
		moved.Sound.Col != 10 || moved.Sound.Cue != "town/brook" {
		t.Fatalf("move %+v", moved)
	}
	f.apply(groundID, tilefloor.UpdateSound{}, moved)
	f.rightClick("cell:10,8")
	f.answerAudio()
	f.click("menu:remove-sound")
	var removed tilefloor.RemoveSoundArgs
	if removes := f.requests.Of(tilefloor.RemoveSoundEdit); len(removes) != 1 || removes[0].Decode(&removed) != nil ||
		removed.ID != placedID {
		t.Fatalf("remove %+v", removed)
	}
}
