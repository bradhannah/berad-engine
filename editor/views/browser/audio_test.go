package browser_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/audioview"
)

// withAudio answers the browser's audio library question with a cue of two
// categories, one with no sound yet.
func withAudio(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	if len(f.requests.AudioAsks) != 1 {
		t.Fatalf("the browser lists the audio library once: %d", len(f.requests.AudioAsks))
	}
	ready := protocol.AudioRecording{ID: "door", Status: protocol.RecordingReady,
		Attribution: protocol.Attribution{Creator: "c", Source: "s", License: "l", Commercial: true}}
	empty := protocol.AudioRecording{ID: "theme", Status: protocol.RecordingNoFile,
		Attribution: protocol.Attribution{Creator: protocol.Unknown, Source: protocol.Unknown,
			License: protocol.Unknown}}
	f.requests.AudioAsks[0](protocol.AudioLibrary{Schema: 4, Cues: []protocol.AudioCue{
		{ID: "sfx/door", Name: "Door opens", Category: protocol.CategorySFX, Gain: 1,
			Recordings: []protocol.AudioRecording{ready}},
		{ID: "music/theme", Name: "Theme", Category: protocol.CategoryMusic, Gain: 1,
			Recordings: []protocol.AudioRecording{empty}},
	}}, nil)
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

// TestAudioSectionListsCuesAndOpensThem lists the cues under their
// categories; opening one shows it in the Audio window, and the section's
// [+] adds a sound there.
func TestAudioSectionListsCuesAndOpensThem(t *testing.T) {
	t.Parallel()
	f := withAudio(t)
	for _, name := range []string{"section:audio", "row:Music", "row:SFX", "row:Door opens", "row:Theme"} {
		if _, ok := f.state.Locate(name, f.ctx.Docs); !ok {
			t.Fatalf("lists %s", name)
		}
	}
	f.click("row:Door opens", true)
	f.click("ui:new-sound", false)
	opened := f.requests.Opened
	if len(opened) != 2 || opened[0].View != audioview.ViewName || opened[0].Item != "sfx/door" ||
		opened[1].Item != audioview.ItemAddSound {
		t.Fatalf("the cue opens, and [+] adds a sound, in the Audio window: %+v", opened)
	}
}

// TestAudioSectionDeletesACue asks where a cue is used, then deletes it from
// the audio library.
func TestAudioSectionDeletesACue(t *testing.T) {
	t.Parallel()
	f := withAudio(t)
	f.click("row:Door opens", false)
	f.requests.Uses = nil
	f.click("ui:delete", false)
	if len(f.requests.Uses) != 1 {
		t.Fatal("deleting asks where the cue is used")
	}
	f.requests.Uses[0]([]protocol.Use{{Name: "Doorman", Label: "a Play sound node"}}, nil)
	f.click("ui:delete", false)
	deletes := f.requests.AudioOf(protocol.AudioDeleteCue)
	var item protocol.AudioItem
	if len(deletes) != 1 || deletes[0].Decode(&item) != nil || item.ID != "sfx/door" {
		t.Fatalf("the cue is deleted: %+v", item)
	}
}
