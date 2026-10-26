package orchestrator_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// paddedWAV is a second of mono 16-bit sound: a quarter second of silence, a
// tone, and silence again, so its sound fills only the middle.
func paddedWAV() []byte {
	const rate = 22050
	var pcm bytes.Buffer
	for index := range rate {
		value := int16(0)
		if index > rate/4 && index < rate*3/4 {
			value = int16((index%50)*600 - 15000)
		}
		_ = binary.Write(&pcm, binary.LittleEndian, value)
	}
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(36+pcm.Len()))
	wav.WriteString("WAVEfmt ")
	for _, value := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(rate * 2), uint16(2), uint16(16)} {
		_ = binary.Write(&wav, binary.LittleEndian, value)
	}
	wav.WriteString("data")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(pcm.Len()))
	wav.Write(pcm.Bytes())

	return wav.Bytes()
}

// withPaddedSound starts a project with one cue playing the padded sound.
func withPaddedSound(t *testing.T) (*harness, *client.Client) {
	t.Helper()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	audio := h.dialView(t, "audio", project.IndexID(), "audio", project.IndexID())
	source := filepath.Join(t.TempDir(), "tone.wav")
	if err := os.WriteFile(source, paddedWAV(), 0o600); err != nil {
		t.Fatal(err)
	}
	mustAudioEdit(t, audio, protocol.AudioAddSound, protocol.AddSound{
		Cue: "", NewCue: &protocol.NewCue{ID: "sfx/tone", Name: "Tone", Category: "", HowToHear: ""}, Path: source,
		Attribution: protocol.Attribution{Creator: "", Source: "", License: "", Commercial: false},
	})

	return h, audio
}

func analyse(t *testing.T, c *client.Client, asked protocol.AudioAnalysis) protocol.AudioAnalysis {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	answer, err := c.AudioAnalysis(ctx, asked)
	if err != nil {
		t.Fatal(err)
	}

	return answer
}

func toneRecording(t *testing.T, c *client.Client) protocol.AudioRecording {
	t.Helper()
	cue, found := cueOf(t, c, "sfx/tone")
	if !found {
		t.Fatal("the tone cue is listed")
	}

	return cue.Recordings[0]
}

// TestAnalysisDrawsAndSuggests sends a recording's waveform, the region its
// sound fills and a calibration suggestion, for all of it or a part.
func TestAnalysisDrawsAndSuggests(t *testing.T) {
	t.Parallel()
	_, audio := withPaddedSound(t)
	answer := analyse(t, audio, protocol.AudioAnalysis{Recording: "tone", Columns: 40})
	if answer.Problem != "" || answer.DurationMs < 990 || len(answer.Peaks) != 40 || answer.Version == "" {
		t.Fatalf("the waveform of a second: %+v", answer)
	}
	if answer.Peaks[0] != 0 || answer.Peaks[20] < 200 || answer.Peaks[39] != 0 {
		t.Fatalf("silent at the ends, loud in the middle: %v", answer.Peaks)
	}
	if answer.Suggested == nil || answer.Suggested.StartMs < 230 || answer.Suggested.EndMs > 770 {
		t.Fatalf("the suggested region is the sound: %+v", answer.Suggested)
	}
	if answer.Calibration == nil || answer.Calibration.TargetDB != -20 {
		t.Fatalf("a suggestion for the default target: %+v", answer.Calibration)
	}
	zoomed := analyse(t, audio, protocol.AudioAnalysis{
		Recording: "tone", From: 0, To: 250, Columns: 10, Region: &protocol.AudioWindow{StartMs: 300, EndMs: 700},
	})
	if len(zoomed.Peaks) != 10 || zoomed.Peaks[0] != 0 || zoomed.Peaks[7] != 0 || zoomed.Calibration == nil {
		t.Fatalf("the first quarter second is silent: %+v", zoomed)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := audio.AudioAnalysis(ctx, protocol.AudioAnalysis{Recording: "none", Columns: 4}); err == nil {
		t.Fatal("a recording the library lacks is refused")
	}
}

// TestTrimDuplicateAndLevel trims a recording, duplicates part of it into
// its cue, measures its gain, sets one by hand and aims at another target.
func TestTrimDuplicateAndLevel(t *testing.T) {
	t.Parallel()
	_, audio := withPaddedSound(t)
	region := &protocol.AudioWindow{StartMs: 240, EndMs: 760}
	mustAudioEdit(t, audio, protocol.AudioSetRegion, protocol.RegionChange{Recording: "tone", Region: region})
	if trimmed := toneRecording(t, audio); trimmed.StartMs != 240 || trimmed.EndMs != 760 {
		t.Fatalf("trimmed: %+v", trimmed)
	}
	made := mustAudioEdit(t, audio, protocol.AudioDuplicateRegion, protocol.DuplicateRegion{
		Recording: "tone", Region: &protocol.AudioWindow{StartMs: 300, EndMs: 400}, Cue: "sfx/tone",
	})
	cue, _ := cueOf(t, audio, "sfx/tone")
	if made.Item != "tone_02" || len(cue.Recordings) != 2 || cue.Recordings[1].StartMs != 300 ||
		cue.Recordings[1].File != cue.Recordings[0].File {
		t.Fatalf("a new recording of the same file, played in turn: %+v %+v", made, cue)
	}
	mustAudioEdit(t, audio, protocol.AudioAcceptCalibration, protocol.AudioItem{ID: "tone"})
	measured := toneRecording(t, audio)
	if measured.CalibrationMode != protocol.CalibrationAuto || measured.CalibrationDB == 0 {
		t.Fatalf("measured: %+v", measured)
	}
	target := -30.0
	mustAudioEdit(t, audio, protocol.AudioSetCalibration, protocol.CalibrationChange{
		Recording: "tone", GainDB: nil, TargetDB: &target, DefaultTarget: false,
	})
	quieter := toneRecording(t, audio)
	if quieter.TargetDB != -30 || quieter.CalibrationMode != protocol.CalibrationAuto ||
		quieter.CalibrationDB >= measured.CalibrationDB-9 {
		t.Fatalf("a target 10 dB lower is measured again: %+v after %+v", quieter, measured)
	}
	gain := -6.0
	mustAudioEdit(t, audio, protocol.AudioSetCalibration, protocol.CalibrationChange{
		Recording: "tone", GainDB: &gain, TargetDB: nil, DefaultTarget: true,
	})
	if manual := toneRecording(t, audio); manual.CalibrationMode != protocol.CalibrationManual ||
		manual.CalibrationDB != -6 || manual.TargetDB != 0 {
		t.Fatalf("set by hand, with the family's target: %+v", manual)
	}
	for range 3 {
		if _, err := undo(t, audio); err != nil {
			t.Fatal(err)
		}
	}
	if undone := toneRecording(t, audio); undone.CalibrationMode != protocol.CalibrationAuto ||
		!near(undone.CalibrationDB, measured.CalibrationDB) || undone.StartMs != 240 {
		t.Fatalf("undo takes the levels back to measured when it came in: %+v", undone)
	}
}

// TestCheckFindsFilesThatCannotPlay decodes every recording and lists those
// that cannot play.
func TestCheckFindsFilesThatCannotPlay(t *testing.T) {
	t.Parallel()
	h, audio := withPaddedSound(t)
	recording := toneRecording(t, audio)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if result, err := audio.AudioCheck(ctx); err != nil || len(result.Broken) != 0 {
		t.Fatalf("every file plays: %+v %v", result, err)
	}
	broken := filepath.Join(h.dir, "audio", filepath.FromSlash(recording.File))
	if err := os.WriteFile(broken, []byte("RIFF, but not a sound"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := audio.AudioCheck(ctx)
	if err != nil || len(result.Broken) != 1 || result.Broken[0].Recording != "tone" || result.Broken[0].Reason == "" {
		t.Fatalf("the broken file is found: %+v %v", result, err)
	}
}
