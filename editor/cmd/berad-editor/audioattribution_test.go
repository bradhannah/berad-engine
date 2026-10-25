package main_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
)

// TestAudioWindowDragsAndAttributes drags the region's start by hand and
// applies it, clears the recording for a commercial game from the panel,
// and records its source type and a processing step in the attribution
// dialog.
func TestAudioWindowDragsAndAttributes(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	source := filepath.Join(t.TempDir(), "padded_knock.wav")
	if err := os.WriteFile(source, paddedKnockWAV(), 0o600); err != nil {
		t.Fatal(err)
	}
	panelShot, dialogShot := shotPath(t, "audio-attribution.png"), shotPath(t, "audio-attribution-dialog.png")
	browserScript := script(must("section:audio", 600), wait(5), click("ui:new-sound"),
		must("row:Padded knock", 1800), wait(5), quit())
	audioScript := script(
		must("state:dialog", 900), wait(5), click("ui:dialog-ok"),
		must("state:selected=cue:sfx/padded_knock", 900), must("state:waveform", 900), wait(10),
		drag("wave:start", "ui:play-whole", "ui:duplicate-region"), wait(5), click("ui:apply-region"), wait(40),
		click("ui:cleared"), wait(40), click("ui:edit-attribution"), must("state:dialog", 60),
		click("ui:source-type"), typed("recorded"), click("ui:add-step"), typed("trimmed"),
		click("ui:step-tool-0"), typed("Audacity"), step{Repeat: 2, Screenshot: dialogShot},
		click("ui:dialog-ok"), wait(40), step{Repeat: 2, Screenshot: panelShot}, quit(),
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
	knock := library.Recordings["padded_knock"]
	provenance := knock.Provenance
	if knock.PlaybackWindow == nil || knock.PlaybackWindow.StartMs < 300 || knock.PlaybackWindow.EndMs != 1000 ||
		!provenance.CommercialUseAllowed() || provenance.SourceType != "recorded" || len(provenance.Processing) != 1 ||
		provenance.Processing[0].Description != "trimmed" || provenance.Processing[0].Tool != "Audacity" {
		t.Fatalf("dragged, cleared and attributed: %+v %+v", knock.PlaybackWindow, provenance)
	}
	t.Logf("screenshots: %s %s", dialogShot, panelShot)
}
