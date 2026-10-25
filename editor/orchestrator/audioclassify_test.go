package orchestrator_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// classifiedLibrary is an empty library with two classification profiles.
const classifiedLibrary = `{
  "schemaVersion": 4,
  "recordings": {},
  "cues": {},
  "taxonomy": {
    "recordingFamilies": {},
    "cueProfiles": {},
    "profiles": {
      "bed": {"targetAcousticFamily":"water","phase":"loop","outcome":"neutral","mixRole":"bed",` +
	`"emitters":["river"],"contexts":["outdoors"]},
      "door": {"targetAcousticFamily":"wood","phase":"start","outcome":"neutral","mixRole":"world",` +
	`"emitters":["door"],"contexts":["house"]}
    }
  },
  "routing": {}
}
`

// presetDescriptor names the library and gives new ambient cues a preset.
const presetDescriptor = `{
  "schemaVersion": 1,
  "library": "library.json",
  "ambientPreset": {"cuePrefix": "bed/", "profile": "bed", "family": "water",
    "howToHear": "Stand near water.", "gain": 0.6},
  "sliderGains": [0, 0.02, 0.05, 0.1, 0.18, 0.28, 0.4, 0.55, 0.7, 0.85, 1]
}
`

// withClassifiedAudio starts a project whose audio has profiles and a preset.
func withClassifiedAudio(t *testing.T) *client.Client {
	t.Helper()
	dir := t.TempDir()
	createProject(t, dir)
	audioDir := filepath.Join(dir, "audio")
	if err := os.MkdirAll(audioDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"library.json": classifiedLibrary, "project.json": presetDescriptor} {
		if err := os.WriteFile(filepath.Join(audioDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	h := start(t, dir, nil)

	return h.dialView(t, "audio", project.IndexID(), "audio", project.IndexID())
}

// TestAmbientCuesProfilesAndFamilies makes an ambient cue from the preset,
// changes how far away it is heard, its profile and its recording's family,
// refuses what berad-audio would not keep, and undoes it all.
func TestAmbientCuesProfilesAndFamilies(t *testing.T) {
	t.Parallel()
	audio := withClassifiedAudio(t)
	library := askAudio(t, audio)
	if len(library.Profiles) != 2 || library.Profiles[0].Name != "bed" || library.Profiles[0].MixRole != "bed" ||
		library.AmbientPreset == nil || library.AmbientPreset.CuePrefix != "bed/" || len(library.SliderGains) != 11 {
		t.Fatalf("the profiles, the preset and the volume levels are listed: %+v", library)
	}
	mustAudioEdit(t, audio, protocol.AudioCreateCue, protocol.NewCue{
		ID: "bed/river", Name: "River", Category: protocol.CategoryAmbient, HowToHear: "",
	})
	river, _ := cueOf(t, audio, "bed/river")
	if river.Category != protocol.CategoryAmbient || river.Profile != "bed" || river.Gain != 0.6 ||
		river.HowToHear != "Stand near water." || river.Recordings[0].Family != "water" ||
		river.Ambient == nil || *river.Ambient != (protocol.AmbientSettings{RadiusTiles: 8, FadeInMs: 1000,
		FadeOutMs: 1000}) {
		t.Fatalf("the preset and a distance: %+v", river)
	}
	mustAudioEdit(t, audio, protocol.AudioSetAmbient, protocol.AmbientChange{Cue: "bed/river",
		Ambient: protocol.AmbientSettings{RadiusTiles: 12.5, FadeInMs: 500, FadeOutMs: 2000}})
	mustAudioEdit(t, audio, protocol.AudioSetProfile, protocol.ProfileChange{Cue: "bed/river", Profile: "door"})
	mustAudioEdit(t, audio, protocol.AudioSetFamily, protocol.FamilyChange{Recording: river.Recordings[0].ID,
		Family: "stream"})
	changed, _ := cueOf(t, audio, "bed/river")
	if changed.Ambient.RadiusTiles != 12.5 || changed.Ambient.FadeOutMs != 2000 || changed.Profile != "door" ||
		changed.Recordings[0].Family != "stream" || !slices.Contains(askAudio(t, audio).Families, "stream") {
		t.Fatalf("changed: %+v", changed)
	}
	for edit, args := range map[string]any{
		protocol.AudioSetAmbient: protocol.AmbientChange{Cue: "bed/river",
			Ambient: protocol.AmbientSettings{RadiusTiles: -1, FadeInMs: 0, FadeOutMs: 0}},
		protocol.AudioSetProfile: protocol.ProfileChange{Cue: "bed/river", Profile: "none"},
		protocol.AudioSetFamily:  protocol.FamilyChange{Recording: river.Recordings[0].ID, Family: "Big Room"},
	} {
		if _, err := audioEdit(t, audio, edit, args); err == nil {
			t.Fatalf("%s is refused: %+v", edit, args)
		}
	}
	for range 3 {
		if _, err := undo(t, audio); err != nil {
			t.Fatal(err)
		}
	}
	undone, _ := cueOf(t, audio, "bed/river")
	if undone.Ambient.RadiusTiles != 8 || undone.Profile != "bed" || undone.Recordings[0].Family != "water" {
		t.Fatalf("undo puts them back: %+v", undone)
	}
}

// TestAmbientCuePlaysOneRecording refuses a distance for a cue that plays
// several recordings in turn.
func TestAmbientCuePlaysOneRecording(t *testing.T) {
	t.Parallel()
	audio := withClassifiedAudio(t)
	attribution := protocol.Attribution{Creator: "", Source: "", License: "", Commercial: false}
	mustAudioEdit(t, audio, protocol.AudioAddSound, protocol.AddSound{Cue: "", NewCue: &protocol.NewCue{
		ID: "door/knock", Name: "Knock", Category: protocol.CategorySFX, HowToHear: "Knock.",
	}, Path: soundFile(t, "knock.wav"), Attribution: attribution})
	knock, _ := cueOf(t, audio, "door/knock")
	if knock.Ambient != nil || knock.Profile != "unclassified" {
		t.Fatalf("an SFX cue has no distance, and the automatic profile: %+v", knock)
	}
	mustAudioEdit(t, audio, protocol.AudioAddSound, protocol.AddSound{Cue: "door/knock", NewCue: nil,
		Path: soundFile(t, "knock2.wav"), Attribution: attribution})
	_, err := audioEdit(t, audio, protocol.AudioSetAmbient, protocol.AmbientChange{Cue: "door/knock",
		Ambient: protocol.AmbientSettings{RadiusTiles: 4, FadeInMs: 0, FadeOutMs: 0}})
	if err == nil {
		t.Fatal("a cue playing recordings in turn has no distance")
	}
}
