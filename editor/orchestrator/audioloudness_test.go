package orchestrator_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/loudness"
)

// near reports whether two values agree to a hundredth.
func near(got, want float64) bool { return math.Abs(got-want) < 0.01 }

func toneCue(t *testing.T, c *client.Client) protocol.AudioCue {
	t.Helper()
	cue, found := cueOf(t, c, "sfx/tone")
	if !found {
		t.Fatal("the tone cue is listed")
	}

	return cue
}

// TestNewSoundsAreEvenedOut measures a sound as it comes in, and starts its
// cue at Normal, the reference loudness.
func TestNewSoundsAreEvenedOut(t *testing.T) {
	t.Parallel()
	_, audio := withPaddedSound(t)
	if tone := toneRecording(t, audio); tone.CalibrationMode != protocol.CalibrationAuto || tone.CalibrationDB == 0 {
		t.Fatalf("a new sound is measured: %+v", tone)
	}
	library := askAudio(t, audio)
	if library.ReferenceDB != -20 || !near(library.MaxCueGain, loudness.Gain(loudness.MaxBoostDB)) {
		t.Fatalf("berad-audio's default reference, and schema 5's reach: %+v", library)
	}
	if cue := toneCue(t, audio); cue.Gain != 1 {
		t.Fatalf("a new cue plays at Normal: %v", cue.Gain)
	}
}

// TestStepsAndCategoryAdjustments sets how loud a cue is meant to sound, then
// its category's adjustment, which moves every cue of the category and is
// kept in the descriptor; a cue changing category keeps its step.
func TestStepsAndCategoryAdjustments(t *testing.T) {
	t.Parallel()
	h, audio := withPaddedSound(t)
	mustAudioEdit(t, audio, protocol.AudioSetLoudness, protocol.LoudnessChange{Cue: "sfx/tone", Step: loudness.Louder})
	if cue := toneCue(t, audio); !near(cue.Gain, loudness.Gain(4)) {
		t.Fatalf("Louder is 4 dB above the reference: %v", cue.Gain)
	}
	mustAudioEdit(t, audio, protocol.AudioSetCategoryGain, protocol.CategoryGainChange{
		Category: protocol.CategorySFX, GainDB: -3,
	})
	if cue := toneCue(t, audio); !near(cue.Gain, loudness.Gain(1)) {
		t.Fatalf("SFX 3 dB down moves the cue with it: %v", cue.Gain)
	}
	if library := askAudio(t, audio); library.CategoryGainsDB[protocol.CategorySFX] != -3 {
		t.Fatalf("the adjustment is listed: %+v", library.CategoryGainsDB)
	}
	data, err := os.ReadFile(filepath.Join(h.dir, "audio", "project.json"))
	if err != nil || !strings.Contains(string(data), `"categoryGainsDb"`) ||
		!strings.Contains(string(data), `"schemaVersion": 2`) {
		t.Fatalf("the descriptor keeps it, as version 2: %s %v", data, err)
	}
	mustAudioEdit(t, audio, protocol.AudioSetCategory, protocol.CategoryChange{
		Cue: "sfx/tone", Category: protocol.CategoryMusic,
	})
	if cue := toneCue(t, audio); !near(cue.Gain, loudness.Gain(4)) {
		t.Fatalf("in music, with no adjustment, it is still Louder: %v", cue.Gain)
	}
	for range 2 {
		if _, err := undo(t, audio); err != nil {
			t.Fatal(err)
		}
	}
	if cue := toneCue(t, audio); !near(cue.Gain, loudness.Gain(4)) || len(askAudio(t, audio).CategoryGainsDB) != 0 {
		t.Fatalf("undo takes back the category and the adjustment: %+v", cue)
	}
	for _, refused := range []struct {
		edit string
		args any
	}{
		{protocol.AudioSetLoudness, protocol.LoudnessChange{Cue: "sfx/tone", Step: "loudest"}},
		{protocol.AudioSetLoudness, protocol.LoudnessChange{Cue: "sfx/none", Step: loudness.Normal}},
		{protocol.AudioSetCategoryGain, protocol.CategoryGainChange{Category: protocol.CategorySFX, GainDB: 5}},
		{protocol.AudioSetCategoryGain, protocol.CategoryGainChange{Category: "voices", GainDB: 1}},
		{protocol.AudioSetReference, protocol.ReferenceChange{TargetDB: -3}},
	} {
		if _, err := audioEdit(t, audio, refused.edit, refused.args); err == nil {
			t.Fatalf("%s %+v is refused", refused.edit, refused.args)
		}
	}
}

// TestReferenceMeasuresAgain moves every measured recording when the
// reference loudness moves, and undo takes both back.
func TestReferenceMeasuresAgain(t *testing.T) {
	t.Parallel()
	_, audio := withPaddedSound(t)
	measured := toneRecording(t, audio).CalibrationDB
	mustAudioEdit(t, audio, protocol.AudioSetReference, protocol.ReferenceChange{TargetDB: -26})
	if quieter := toneRecording(t, audio).CalibrationDB; !near(quieter, measured-6) {
		t.Fatalf("6 dB quieter: %v after %v", quieter, measured)
	}
	if library := askAudio(t, audio); library.ReferenceDB != -26 {
		t.Fatalf("the reference is listed: %v", library.ReferenceDB)
	}
	if _, err := undo(t, audio); err != nil {
		t.Fatal(err)
	}
	if back := toneRecording(t, audio).CalibrationDB; !near(back, measured) || askAudio(t, audio).ReferenceDB != -20 {
		t.Fatalf("undo takes both back: %v", back)
	}
}

// TestMeasureAllAndUseReference measures recordings no level was set for,
// and measures one set by hand again on request.
func TestMeasureAllAndUseReference(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	writeAudio(t, dir, "library.json", strings.Replace(strings.Replace(testLibrary,
		`"calibrationGainDb": -3, `, "", 1), `"sounds/gone.wav"`, `"sounds/chime.wav"`, 1))
	writeAudio(t, dir, "sounds/chime.wav", string(paddedWAV()))
	h := start(t, dir, nil)
	audio := h.dialView(t, "audio", project.IndexID(), "audio", project.IndexID())
	mustAudioEdit(t, audio, protocol.AudioMeasureAll, protocol.AudioItem{ID: ""})
	chime, _ := cueOf(t, audio, "door/open")
	for _, recording := range chime.Recordings {
		if recording.CalibrationMode != protocol.CalibrationAuto || recording.CalibrationDB == 0 {
			t.Fatalf("measured: %+v", recording)
		}
	}
	gain := -6.0
	mustAudioEdit(t, audio, protocol.AudioSetCalibration, protocol.CalibrationChange{
		Recording: "chime", GainDB: &gain, TargetDB: nil, DefaultTarget: false,
	})
	mustAudioEdit(t, audio, protocol.AudioUseReference, protocol.AudioItem{ID: "chime"})
	again, _ := cueOf(t, audio, "door/open")
	if again.Recordings[0].CalibrationMode != protocol.CalibrationAuto ||
		!near(again.Recordings[0].CalibrationDB, chime.Recordings[0].CalibrationDB) {
		t.Fatalf("measured to the reference again: %+v", again.Recordings[0])
	}
}

// TestLoudnessAnswersForEveryRecording measures every recording with a file,
// and says why one cannot be measured.
func TestLoudnessAnswersForEveryRecording(t *testing.T) {
	t.Parallel()
	h, audio := withPaddedSound(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	answer, err := audio.AudioLoudness(ctx)
	if err != nil || len(answer.Recordings) != 1 {
		t.Fatalf("one recording: %+v %v", answer, err)
	}
	tone := answer.Recordings[0]
	if tone.ID != "tone" || tone.Problem != "" || tone.TargetDB != -20 || tone.MeasuredDB >= 0 || tone.PeakDB >= 0 {
		t.Fatalf("measured: %+v", tone)
	}
	recording := toneRecording(t, audio)
	if err := os.Remove(filepath.Join(h.dir, "audio", filepath.FromSlash(recording.File))); err != nil {
		t.Fatal(err)
	}
	answer, err = audio.AudioLoudness(ctx)
	if err != nil || answer.Recordings[0].Problem == "" {
		t.Fatalf("a missing file says why: %+v %v", answer, err)
	}
}

// TestPiecesShareTheirFile finds the sound in a file, makes recordings of
// parts of it into its cue, and changes the attribution of all of them.
func TestPiecesShareTheirFile(t *testing.T) {
	t.Parallel()
	_, audio := withPaddedSound(t)
	analysed := analyse(t, audio, protocol.AudioAnalysis{Recording: "tone", Columns: 10})
	if len(analysed.Sounds) != 1 || analysed.Sounds[0].StartMs < 230 || analysed.Sounds[0].EndMs > 770 {
		t.Fatalf("one sound in the file: %+v", analysed.Sounds)
	}
	made := mustAudioEdit(t, audio, protocol.AudioMakePieces, protocol.MakePieces{
		Recording: "tone", Cue: "sfx/tone",
		Regions: []protocol.AudioWindow{{StartMs: 300, EndMs: 400}, {StartMs: 500, EndMs: 600}},
	})
	cue := toneCue(t, audio)
	if made.Item != "tone_02" || len(cue.Recordings) != 3 || cue.Recordings[2].StartMs != 500 {
		t.Fatalf("two pieces, played in turn: %+v %+v", made, cue.Recordings)
	}
	for _, piece := range cue.Recordings[1:] {
		if piece.File != cue.Recordings[0].File || piece.CalibrationMode != protocol.CalibrationAuto {
			t.Fatalf("a measured piece of the same file: %+v", piece)
		}
	}
	mustAudioEdit(t, audio, protocol.AudioSetFileAttribution, protocol.FileAttributionChange{
		File:        cue.Recordings[0].File,
		Attribution: protocol.Attribution{Creator: "Ada", Source: "", SourceType: "", License: "CC0-1.0", Commercial: true},
	})
	for _, piece := range toneCue(t, audio).Recordings {
		if piece.Attribution.Creator != "Ada" || !piece.Attribution.Commercial {
			t.Fatalf("every piece of the file takes its attribution: %+v", piece)
		}
	}
	if _, err := undo(t, audio); err != nil {
		t.Fatal(err)
	}
	for _, piece := range toneCue(t, audio).Recordings {
		if piece.Attribution.Creator == "Ada" {
			t.Fatalf("one undo takes it back from all: %+v", piece)
		}
	}
	if _, err := audioEdit(t, audio, protocol.AudioSetFileAttribution, protocol.FileAttributionChange{
		File: "nowhere.wav", Attribution: protocol.Attribution{Creator: "", Source: "", License: ""},
	}); err == nil {
		t.Fatal("a file no recording has is refused")
	}
	_ = authoring.LibrarySchemaFive
}
