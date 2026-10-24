package orchestrator_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// testLibrary is a schema-3 library with a music cue (by the descriptor), an
// ambient cue (by its routing), an SFX cue cycling a ready recording and a
// missing file, and a cue whose recording is a silent slot.
const testLibrary = `{
  "schemaVersion": 3,
  "recordings": {
    "chime": {"file": "sounds/chime.wav", "calibrationGainDb": -3, "playbackWindow": {"startMs": 10, "endMs": 400},
      "provenance": {"originalFilename": "unknown", "sourceType": "unknown", "sourceUrl": "unknown",
        "sourceCreator": "unknown", "license": "unknown", "importedFrom": "unknown", "processing": []}},
    "gone": {"file": "sounds/gone.wav",
      "provenance": {"originalFilename": "unknown", "sourceType": "unknown", "sourceUrl": "unknown",
        "sourceCreator": "unknown", "license": "unknown", "importedFrom": "unknown", "processing": []}},
    "quiet": {"file": null,
      "provenance": {"originalFilename": "unknown", "sourceType": "unknown", "sourceUrl": "unknown",
        "sourceCreator": "unknown", "license": "unknown", "importedFrom": "unknown", "processing": []}}
  },
  "cues": {
    "bed/hum": {"name": "Hum", "recordingId": "chime", "volumeModifier": 0.5, "howToHear": "Stand still."},
    "door/open": {"name": "Door opens", "recordingIds": ["chime", "gone"], "selection": "cycle",
      "volumeModifier": 0.8, "howToHear": "Open a door.", "searchTerms": ["hinge"]},
    "panel/accept": {"name": "Accept", "recordingId": "quiet", "volumeModifier": 1, "howToHear": "Accept."},
    "theme/title": {"name": "Title theme", "recordingId": "chime", "volumeModifier": 1, "howToHear": "Start."}
  },
  "taxonomy": {
    "recordingFamilies": {"signal": ["chime", "gone", "quiet"]},
    "cueProfiles": {"signal": ["bed/hum", "door/open", "panel/accept", "theme/title"]},
    "profiles": {"signal": {"targetAcousticFamily": "signal", "phase": "start", "outcome": "neutral",
      "mixRole": "interface", "emitters": ["panel"], "contexts": ["menu"]}}
  },
  "routing": {"ambient": {"bed/hum": {"radiusTiles": 0, "fadeInMs": 100, "fadeOutMs": 100}}}
}`

func writeAudio(t *testing.T, dir, name, text string) {
	t.Helper()
	path := filepath.Join(dir, "audio", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func askAudio(t *testing.T, c *client.Client) protocol.AudioLibrary {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	library, err := c.AudioLibrary(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return library
}

func askRecording(t *testing.T, c *client.Client, recording string, offset int64) (protocol.Recording, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return c.Recording(ctx, recording, offset)
}

// TestAudioLibraryListsCuesWithCategories lists every cue with its category
// from the descriptor or its routing, and each recording's status, and says
// why when the project has no library.
func TestAudioLibraryListsCuesWithCategories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	c := h.dial(t, "a")
	if library := askAudio(t, c); library.Problem == "" || len(library.Cues) != 0 {
		t.Fatalf("no audio library yet: %+v", library)
	}
	writeAudio(t, dir, "library.json", testLibrary)
	writeAudio(t, dir, "sounds/chime.wav", "RIFF-chime")
	library := askAudio(t, c)
	if library.Problem != "" || len(library.Cues) != 4 {
		t.Fatalf("four cues: %+v", library)
	}
	categories := map[string]string{}
	for _, cue := range library.Cues {
		categories[cue.ID] = cue.Category
	}
	if categories["bed/hum"] != protocol.CategoryAmbient || categories["theme/title"] != protocol.CategorySFX {
		t.Fatalf("without a descriptor: ambient by routing, else SFX: %v", categories)
	}
	door := library.Cues[1]
	if door.ID != "door/open" || door.Name != "Door opens" || door.Gain != 0.8 || door.SearchTerms[0] != "hinge" {
		t.Fatalf("door cue: %+v", door)
	}
	chime, gone, quiet := door.Recordings[0], door.Recordings[1], library.Cues[2].Recordings[0]
	if chime.Status != protocol.RecordingReady || chime.Size != 10 || chime.Version == "" || chime.Extension != ".wav" ||
		chime.CalibrationDB != -3 || chime.StartMs != 10 || chime.EndMs != 400 {
		t.Fatalf("a ready recording, in cycle order: %+v", chime)
	}
	if quiet.Status != protocol.RecordingNoFile || gone.Status != protocol.RecordingMissing {
		t.Fatalf("a silent slot and a missing file: %+v %+v", quiet, gone)
	}
	writeAudio(t, dir, "project.json", `{"schemaVersion": 1, "library": "library.json",
		"targets": {"CueCategories": {"theme/title": "music"}}}`)
	if cue := askAudio(t, c).Cues[3]; cue.ID != "theme/title" || cue.Category != protocol.CategoryMusic {
		t.Fatalf("the descriptor names the music: %+v", cue)
	}
	writeAudio(t, dir, "project.json", `{"schemaVersion": 1, "library": "../../elsewhere/library.json"}`)
	if library := askAudio(t, c); library.Problem == "" {
		t.Fatal("a descriptor pointing outside the project is refused")
	}
}

// TestRecordingIsSentInParts sends a large recording's bytes in parts,
// refuses one that is not ready or an offset outside it, and changes its
// version when the file changes.
func TestRecordingIsSentInParts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	writeAudio(t, dir, "library.json", testLibrary)
	whole := bytes.Repeat([]byte("0123456789abcdef"), protocol.MaxRecordingPart/16+3)
	writeAudio(t, dir, "sounds/chime.wav", string(whole))
	h := start(t, dir, nil)
	c := h.dial(t, "a")
	first, err := askRecording(t, c, "chime", 0)
	if err != nil || len(first.Data) != protocol.MaxRecordingPart || first.Size != int64(len(whole)) {
		t.Fatalf("the first part: %d of %d, %v", len(first.Data), first.Size, err)
	}
	rest, err := askRecording(t, c, "chime", int64(len(first.Data)))
	if err != nil || rest.Version != first.Version || !bytes.Equal(append(first.Data, rest.Data...), whole) {
		t.Fatalf("the rest completes the file: %d bytes, %v", len(rest.Data), err)
	}
	for name, offset := range map[string]int64{"quiet": 0, "gone": 0, "unknown": 0, "chime": int64(len(whole)) + 1} {
		if _, err := askRecording(t, c, name, offset); !errors.Is(err, client.ErrRejected) {
			t.Fatalf("%s from %d is refused: %v", name, offset, err)
		}
	}
	writeAudio(t, dir, "sounds/chime.wav", "changed")
	changed, err := askRecording(t, c, "chime", 0)
	if err != nil || changed.Version == first.Version || string(changed.Data) != "changed" {
		t.Fatalf("a changed file has a new version: %+v %v", changed.Version, err)
	}
}
