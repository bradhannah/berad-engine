package main_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
)

// knockWAV is a short generated mono tone for the test to add.
func knockWAV() []byte {
	const rate, samples = 22050, 6000
	var pcm bytes.Buffer
	for index := range samples {
		_ = binary.Write(&pcm, binary.LittleEndian, int16((index%40)*300-6000))
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

// runEditorWith is runEditor with more environment, such as the file a
// window chooses instead of showing the Open dialog.
func runEditorWith(t *testing.T, binary, project string, windows scripts, env ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "open", "--project", project, "--sample")
	cmd.Env = append(append(os.Environ(), windows.env(t)), env...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("editor did not quit after its windows closed\n%s", output)
	}
	if err != nil {
		t.Fatalf("editor: %v\n%s", err, output)
	}
}

// TestAudioWindowAddsAndEditsSounds adds a sound from the project browser's
// Audio section: the Audio window opens on the Open dialog's file, asks what
// it is and who made it, and copies it in as a new cue. It plays the cue,
// lowers its gain, changes its category and undoes that, then lists the
// recordings.
func TestAudioWindowAddsAndEditsSounds(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	source := filepath.Join(t.TempDir(), "door_knock.wav")
	if err := os.WriteFile(source, knockWAV(), 0o600); err != nil {
		t.Fatal(err)
	}
	browserShot, dialogShot := shotPath(t, "audio-browser.png"), shotPath(t, "audio-dialog.png")
	cueShot, recordingsShot := shotPath(t, "audio-cue.png"), shotPath(t, "audio-recordings.png")
	browserScript := script(
		must("row:Notes", 600), must("section:audio", 600), wait(5), click("ui:new-sound"),
		must("row:Door knock", 1800), wait(5), click("row:Door knock"), wait(10),
		step{Repeat: 2, Screenshot: browserShot}, quit(),
	)
	audioScript := script(
		must("state:dialog", 900), wait(5), click("ui:creator"), typed("Test recordist"),
		click("ui:license"), typed("CC0-1.0"), click("ui:commercial"), wait(3),
		step{Repeat: 2, Screenshot: dialogShot}, click("ui:dialog-ok"),
		must("state:selected=cue:sfx/door_knock", 900), wait(10), click("ui:play"), wait(30),
		click("ui:quieter"), wait(30), click("ui:category-music"), wait(30), keys("Cmd+Z"), wait(30),
		step{Repeat: 2, Screenshot: cueShot}, click("ui:tab-recordings"), wait(10),
		click("rec:door_knock"), wait(10), step{Repeat: 2, Screenshot: recordingsShot}, quit(),
	)
	mapScript := script(must("state:loaded", 600), wait(5), quit())
	runEditorWith(t, binary, project, scripts{
		"browser": {browserScript}, "audio": {audioScript}, "map": {mapScript},
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
	cue, found := library.Cues["sfx/door_knock"]
	if !found || cue.Name != "Door knock" || cue.VolumeModifier != 0.9 {
		t.Fatalf("the cue, a little quieter: %+v", cue)
	}
	recording := library.Recordings[cue.RecordingID]
	provenance := recording.Provenance
	if recording.File == nil || !strings.HasPrefix(*recording.File, "imported/") ||
		provenance.SourceCreator != "Test recordist" || provenance.License != "CC0-1.0" ||
		!provenance.CommercialUseAllowed() || provenance.OriginalFilename != "door_knock.wav" {
		t.Fatalf("the copied sound and its attribution: %+v", recording)
	}
	descriptor, err := authoring.ReadProjectDescriptor(filepath.Join(project, "audio", "project.json"))
	if err != nil || descriptor.Targets.CueCategories["sfx/door_knock"] != "sfx" {
		t.Fatalf("undo put the category back: %+v %v", descriptor.Targets.CueCategories, err)
	}
	t.Logf("screenshots: %s %s %s %s", browserShot, dialogShot, cueShot, recordingsShot)
}
