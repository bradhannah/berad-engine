package orchestrator_test

import (
	"os"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/player"
)

// TestAGameReadsTheAudioTheEditorLists reads one project's audio twice: as
// the orchestrator lists it for the editor's windows, and as the engine's
// player reads it from the folder for a game (increment 62). Each cue has
// the same gain, category, distance and length in both, and each category
// the same adjustment.
func TestAGameReadsTheAudioTheEditorLists(t *testing.T) {
	parallelAudio(t)
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	c := h.dial(t, "a")
	provenance := `"provenance": {"originalFilename": "unknown", "sourceType": "unknown", "sourceUrl": "unknown",
        "sourceCreator": "unknown", "license": "unknown", "importedFrom": "unknown", "processing": []}`
	writeAudio(t, dir, "library.json", `{"schemaVersion": 3, "recordings": {
		"tone": {"file": "sounds/tone.wav", `+provenance+`},
		"cut": {"file": "sounds/tone.wav", "calibrationGainDb": -3, "playbackWindow": {"startMs": 10, "endMs": 260}, `+provenance+`},
		"gone": {"file": "sounds/gone.wav", `+provenance+`},
		"quiet": {"file": null, `+provenance+`}},
	  "cues": {
		"bell/ring": {"name": "Bell", "recordingIds": ["tone", "cut"], "selection": "cycle", "volumeModifier": 0.8,
		  "howToHear": "Ring."},
		"bed/hum": {"name": "Hum", "recordingId": "tone", "volumeModifier": 0.5, "howToHear": "Stand still."},
		"door/open": {"name": "Door", "recordingIds": ["cut", "gone"], "selection": "cycle", "volumeModifier": 1,
		  "howToHear": "Open."},
		"panel/accept": {"name": "Accept", "recordingId": "quiet", "volumeModifier": 1, "howToHear": "Accept."},
		"theme/title": {"name": "Title theme", "recordingId": "tone", "volumeModifier": 0.6, "howToHear": "Start."}},
	  "taxonomy": {"recordingFamilies": {"signal": ["tone", "cut", "gone", "quiet"]},
		"cueProfiles": {"signal": ["bell/ring", "bed/hum", "door/open", "panel/accept", "theme/title"]},
		"profiles": {"signal": {"targetAcousticFamily": "signal", "phase": "start", "outcome": "neutral",
		"mixRole": "interface", "emitters": ["panel"], "contexts": ["menu"]}}},
	  "routing": {"ambient": {"bed/hum": {"radiusTiles": 6, "fadeInMs": 100, "fadeOutMs": 100}}}}`)
	writeAudio(t, dir, "project.json", `{"schemaVersion": 2, "library": "library.json",
		"targets": {"CueCategories": {"theme/title": "music"}}, "categoryGainsDb": {"sfx": -2, "music": -6}}`)
	const halfSecond = 11025
	writeAudio(t, dir, "sounds/tone.wav", string(toneWAV(halfSecond)))
	listed := askAudio(t, c)
	game, err := player.ReadAudio(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	if listed.Problem != "" || len(listed.Cues) != 5 || len(game.Cues()) != 5 {
		t.Fatalf("five cues in both: %+v and %v", listed, game.Cues())
	}
	for _, cue := range listed.Cues {
		sound, found := game.Cue(cue.ID)
		radius := 0.0
		if cue.Ambient != nil {
			radius = cue.Ambient.RadiusTiles
		}
		if !found || sound.Gain != cue.Gain || sound.Category != cue.Category || sound.Radius != radius {
			t.Fatalf("%s: the game reads %+v, the editor lists gain %v, category %s, distance %v", cue.ID, sound, cue.Gain,
				cue.Category, radius)
		}
		if got, want := game.CueTicks(cue.ID), listed.CueTicks(cue.ID); got != want {
			t.Fatalf("%s lasts %d ticks in the game and %d in the editor", cue.ID, got, want)
		}
	}
	if ticks := game.CueTicks("bell/ring"); ticks != 30 {
		t.Fatalf("the bell lasts its longer recording, half a second's ticks: %d", ticks)
	}
	if game.Radius("bed/hum") != 6 || game.Radius("bell/ring") != 0 {
		t.Fatal("only the ambient cue is heard from a distance")
	}
	for _, category := range []string{protocol.CategorySFX, protocol.CategoryMusic, protocol.CategoryAmbient} {
		if got, want := game.CategoryDB(category), listed.CategoryGainsDB[category]; got != want {
			t.Fatalf("%s is adjusted %v dB in the game and %v in the editor", category, got, want)
		}
	}
}
