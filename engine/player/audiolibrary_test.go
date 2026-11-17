package player_test

import (
	"errors"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bradhannah/berad-engine/engine/player"
)

func audioFile(text string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(text), Mode: 0o600, ModTime: time.Time{}, Sys: nil}
}

const provenance = `"provenance": {"originalFilename": "unknown", "sourceType": "unknown", "sourceUrl": "unknown",
	"sourceCreator": "unknown", "license": "unknown", "importedFrom": "unknown", "processing": []}`

const library = `{"schemaVersion": 3, "recordings": {
	"cut": {"file": "sounds/tone.wav", "playbackWindow": {"startMs": 0, "endMs": 250}, ` + provenance + `}},
  "cues": {
	"bell/ring": {"name": "Bell", "recordingId": "cut", "volumeModifier": 0.8, "howToHear": "Ring."},
	"bed/hum": {"name": "Hum", "recordingId": "cut", "volumeModifier": 0.5, "howToHear": "Stand still."}},
  "taxonomy": {"recordingFamilies": {"signal": ["cut"]}, "cueProfiles": {"signal": ["bell/ring", "bed/hum"]},
	"profiles": {"signal": {"targetAcousticFamily": "signal", "phase": "start", "outcome": "neutral",
	"mixRole": "interface", "emitters": ["panel"], "contexts": ["menu"]}}},
  "routing": {"ambient": {"bed/hum": {"radiusTiles": 4, "fadeInMs": 100, "fadeOutMs": 100}}}}`

// TestReadAudioWithNoAudio reads a project with no audio folder as a
// library with no cues.
func TestReadAudioWithNoAudio(t *testing.T) {
	t.Parallel()
	read, err := player.ReadAudio(fstest.MapFS{"zones/town.json": audioFile("{}")})
	if err != nil || len(read.Cues()) != 0 || read.CueTicks("bell/ring") != 0 || read.Radius("bed/hum") != 0 {
		t.Fatalf("no audio, no cues, no error: %v", err)
	}
}

// TestReadAudioReadsCues reads each cue's gain, its category, by routing
// without an audio project, and its distance and length.
func TestReadAudioReadsCues(t *testing.T) {
	t.Parallel()
	read, err := player.ReadAudio(fstest.MapFS{"audio/library.json": audioFile(library)})
	if err != nil {
		t.Fatal(err)
	}
	bell, found := read.Cue("bell/ring")
	if !found || bell.Gain != 0.8 || bell.Category != "sfx" || bell.Radius != 0 {
		t.Fatalf("the bell: %+v", bell)
	}
	hum, found := read.Cue("bed/hum")
	if !found || hum.Category != "ambient" || hum.Radius != 4 {
		t.Fatalf("the hum: %+v", hum)
	}
	if ticks := read.CueTicks("bell/ring"); ticks != 15 {
		t.Fatalf("a quarter of a second is 15 ticks: %d", ticks)
	}
	if ticks := read.CueTicks("bed/hum"); ticks != 0 {
		t.Fatalf("an ambient cue loops, so it lasts no ticks: %d", ticks)
	}
}

// TestReadAudioRefusesWhatItCannotRead refuses an audio project that points
// outside the project folder, and a library that is not one.
func TestReadAudioRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	outside := fstest.MapFS{
		"audio/project.json": audioFile(`{"schemaVersion": 1, "library": "../../elsewhere/library.json"}`),
	}
	if _, err := player.ReadAudio(outside); !errors.Is(err, player.ErrAudio) {
		t.Fatalf("a library outside the project is refused: %v", err)
	}
	broken := fstest.MapFS{"audio/library.json": audioFile("{")}
	if _, err := player.ReadAudio(broken); !errors.Is(err, player.ErrAudio) {
		t.Fatalf("a broken library is refused: %v", err)
	}
	moved := fstest.MapFS{
		"audio/project.json":     audioFile(`{"schemaVersion": 2, "library": "mix/library.json", "categoryGainsDb": {"sfx": -2}}`),
		"audio/mix/library.json": audioFile(library),
	}
	read, err := player.ReadAudio(moved)
	if err != nil || len(read.Cues()) != 2 || read.CategoryDB("sfx") != -2 {
		t.Fatalf("an audio project names where its library is: %v", err)
	}
}
