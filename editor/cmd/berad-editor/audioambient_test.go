package main_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
)

// TestAudioWindowAmbientAndSoundscape opens the sample's hum in Audio, sets how far away it is heard, offers the profiles, plays it with
// the tune and the chime in the soundscape, and reads the saved distance
// back.
func TestAudioWindowAmbientAndSoundscape(t *testing.T) {
	requireGUI(t)
	binary := buildEditor(t)
	project := filepath.Join(t.TempDir(), "project")
	ambientShot, scapeShot := shotPath(t, "audio-ambient.png"), shotPath(t, "audio-soundscape.png")
	profileShot := shotPath(t, "audio-profiles.png")
	audioScript := script(
		must("state:selected=cue:sample/hum", 900), wait(10),
		click("ui:radius"), keys("Cmd+A"), typed("6"), keys("Enter"), wait(40),
		shoot(ambientShot),
		click("ui:profile"), wait(5), shoot(profileShot), keys("Escape"), wait(3),
		click("ui:add-layer"), wait(20), click("cue:sample/tune"), wait(10), click("ui:add-layer"), wait(20),
		click("cue:sample/chime"), wait(10), click("ui:add-layer"), wait(20),
		must("state:layer=music:sample/tune", 120), click("ui:layer-solo-music"), wait(5),
		click("ui:monitor-quieter-ambient"), wait(5), shoot(scapeShot), quit(),
	)
	browserScript := script(must("section:audio", 600), wait(5), doubleClick("row:Hum"), wait(30), quitWhenAlone())
	runEditorWith(t, binary, project, scripts{
		"browser": {browserScript}, "audio": {audioScript},
	})
	noCrashes(t, project)

	data, err := os.ReadFile(filepath.Join(project, "audio", "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	library, err := authoring.ParseDocument(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hum := library.Routing.Ambient["sample/hum"]; hum.RadiusTiles != 6 || hum.FadeInMs != 200 {
		t.Fatalf("heard from 6 tiles away, its fades kept: %+v", hum)
	}
	t.Logf("screenshots: %s %s %s", ambientShot, profileShot, scapeShot)
}
