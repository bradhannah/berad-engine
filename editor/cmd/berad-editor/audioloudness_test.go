package main_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
)

// twoKnocksWAV is 1.2 seconds of generated sound: a knock from 0.1 to 0.3
// seconds and another from 0.7 to 0.9, with silence round them, so the file
// can be cut into two recordings.
func twoKnocksWAV() []byte {
	const rate = 22050
	var pcm bytes.Buffer
	for index := range rate * 6 / 5 {
		value := int16(0)
		if index > rate/10 && index < rate*3/10 || index > rate*7/10 && index < rate*9/10 {
			value = int16((index%40)*500 - 10000)
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

// TestAudioWindowLoudnessAndFiles adds a file of two knocks, trims its
// recording to the first, opens the file, finds the second and makes a
// recording of it, which joins the cue; then measures every recording in the
// Loudness view and makes the SFX category a dB quieter.
func TestAudioWindowLoudnessAndFiles(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	source := filepath.Join(t.TempDir(), "two_knocks.wav")
	if err := os.WriteFile(source, twoKnocksWAV(), 0o600); err != nil {
		t.Fatal(err)
	}
	fileShot, cueShot := shotPath(t, "audio-file.png"), shotPath(t, "audio-cue-loudness.png")
	recordingShot, loudnessShot := shotPath(t, "audio-recording.png"), shotPath(t, "audio-loudness.png")
	playedByShot := shotPath(t, "audio-played-by.png")
	browserScript := script(must("section:audio", 600), wait(5), click("ui:new-sound"),
		must("row:Two knocks", 1800), wait(5), quitWhenAlone())
	audioScript := script(
		must("state:dialog", 900), wait(5), click("ui:dialog-ok"),
		must("state:selected=cue:sfx/two_knocks", 900), must("state:waveform", 900), wait(10),
		click("ui:region-end"), keys("Cmd+A"), typed("0.35"), keys("Enter"), wait(5),
		click("ui:apply-region"), wait(40),
		click("ui:tab-recordings"), wait(10), click("rec:two_knocks"), wait(10), click("ui:open-file"), wait(60),
		click("ui:find-sounds"), wait(5), shoot(fileShot),
		click("ui:make-pieces"), wait(60), click("rec:two_knocks_02"), must("state:waveform", 900), wait(30),
		shoot(recordingShot),
		step{Wheel: &[2]float64{0, -20}, Repeat: 2, At: "ui:part-whole"}, wait(5),
		shoot(playedByShot),
		click("ui:tab-cues"), wait(10), click("cue:sfx/two_knocks"), wait(40),
		step{Wheel: &[2]float64{0, -20}, Repeat: 2, At: "ui:play"}, wait(5), shoot(cueShot),
		click("ui:loudness"), wait(60), click("ui:measure-all"), wait(120),
		click("ui:adjust-quieter-sfx"), wait(60), shoot(loudnessShot), quit(),
	)
	runEditorWith(t, binary, project, scripts{
		"browser": {browserScript}, "audio": {audioScript},
	}, "BERAD_TEST_SOUND_FILE="+source)
	noCrashes(t, project)

	data, err := os.ReadFile(filepath.Join(project, "audio", "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	library, err := authoring.ParseDocument(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, second := library.Recordings["two_knocks"], library.Recordings["two_knocks_02"]
	if first.PlaybackWindow == nil || first.PlaybackWindow.EndMs != 350 || second.PlaybackWindow == nil ||
		second.PlaybackWindow.StartMs < 600 || *first.File != *second.File {
		t.Fatalf("two pieces of one file: %+v %+v", first, second)
	}
	for id, recording := range library.Recordings {
		if recording.File != nil && recording.CalibrationMode() != authoring.CalibrationAuto {
			t.Fatalf("%s is measured: %+v", id, recording)
		}
	}
	if cue := library.Cues["sfx/two_knocks"]; len(cue.Recordings()) != 2 {
		t.Fatalf("the new piece joins the cue: %+v", cue)
	}
	descriptor, err := authoring.ReadProjectDescriptor(filepath.Join(project, "audio", "project.json"))
	if err != nil || descriptor.CategoryGainDB(authoring.CategorySFX) != -1 {
		t.Fatalf("SFX a dB quieter: %+v %v", descriptor.CategoryGainsDB, err)
	}
	t.Logf("screenshots: %s %s %s %s", fileShot, recordingShot, cueShot, loudnessShot)
}
