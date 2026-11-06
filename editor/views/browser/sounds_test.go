package browser_test

import (
	"image"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// TestSoundsShowLoudnessAndPlay shows a category's sounds as rows: each
// asks for its waveform, how it stands against the reference loudness is
// said as the Audio window says it, and its play button plays it in place.
func TestSoundsShowLoudnessAndPlay(t *testing.T) {
	t.Parallel()
	f := withAudio(t)
	f.click("place:SFX", false)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if len(f.requests.Loudness) != 1 {
		t.Fatalf("the browser asks how loud the sounds are once: %d", len(f.requests.Loudness))
	}
	f.requests.Loudness[0](protocol.AudioLoudness{Recordings: []protocol.RecordingLoudness{
		{ID: "door", MeasuredDB: -12, PeakDB: -3, TargetDB: -20, Problem: ""},
	}}, nil)
	if !f.has("sound:Door opens=Not measured yet") {
		t.Fatal("a sound never evened out says so, as the Audio window does")
	}
	if len(f.requests.Analyses) != 1 || f.requests.Analyses[0].Asked.Recording != "door" {
		t.Fatalf("the browser asks for the sound's waveform: %+v", f.requests.Analyses)
	}
	f.click("play:Door opens", false)
	sounds, ok := f.ctx.Sounds.(*frametest.Sounds)
	if !ok || !slices.Contains(sounds.Played, "sfx/door") {
		t.Fatal("the play button plays the sound")
	}
}
