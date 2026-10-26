package audioview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/engine/loudness"
)

// piecesLibrary has the door cue playing two pieces of one file, the first
// half and the second, and a spare recording of another file.
func piecesLibrary() protocol.AudioLibrary {
	library := testLibrary()
	first, second := recording("door_a", true), recording("door_b", true)
	first.File, second.File = "imported/door.wav", "imported/door.wav"
	first.StartMs, first.EndMs, second.StartMs, second.EndMs = 0, 400, 400, 800
	library.Cues[1].Recordings = []protocol.AudioRecording{first, second}

	return library
}

// TestRecordingsAreListedByFile groups recordings under the file they were
// cut from, and a file opens its own screen.
func TestRecordingsAreListedByFile(t *testing.T) {
	t.Parallel()
	f := newFixtureWith(t, piecesLibrary())
	f.click("ui:tab-recordings")
	if !f.has("file:imported/door.wav") || !f.has("rec:door_a") || !f.has("rec:door_b") || !f.has("rec:spare") {
		t.Fatal("the door's two pieces under its file, and the spare under its own")
	}
	f.click("file:imported/door.wav")
	if !f.has("ui:play-file") || !f.has("ui:find-sounds") || !f.has("ui:rec:door_b") || !f.has("ui:cleared") {
		t.Fatal("the file shows its pieces and its attribution")
	}
	f.click("ui:rec:door_b")
	if !f.has("state:selected=rec:door_b") || !f.has("ui:open-file") {
		t.Fatal("a piece opens, and leads back to its file")
	}
}

// TestFindSoundsMakesTheRest finds the sounds no piece covers and makes a
// recording of each, played by the one cue that plays the file's pieces.
func TestFindSoundsMakesTheRest(t *testing.T) {
	t.Parallel()
	f := newFixtureWith(t, piecesLibrary())
	f.click("ui:tab-recordings")
	f.click("file:imported/door.wav")
	asked := f.requests.Analyses[len(f.requests.Analyses)-1]
	answer := asked.Asked
	answer.Version, answer.DurationMs, answer.Peaks = "v1", 1200, make([]byte, max(answer.Columns, 1))
	answer.Sounds = []protocol.AudioWindow{{StartMs: 50, EndMs: 350}, {StartMs: 900, EndMs: 1100}}
	asked.Done(answer, nil)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.click("ui:find-sounds")
	f.click("ui:make-pieces")
	var made protocol.MakePieces
	if err := f.edit(protocol.AudioMakePieces).Decode(&made); err != nil || made.Recording != "door_a" ||
		len(made.Regions) != 1 || made.Regions[0].StartMs != 900 || made.Cue != "sfx/door" {
		t.Fatalf("one sound no piece covers, for the door: %+v %v", made, err)
	}
}

// TestPiecesOfAFileThatDisagreeAreAProblem shows a file whose pieces carry
// different attribution under Problems.
func TestPiecesOfAFileThatDisagreeAreAProblem(t *testing.T) {
	t.Parallel()
	library := piecesLibrary()
	library.Cues[1].Recordings[1].Attribution.Creator = "Someone else"
	f := newFixtureWith(t, library)
	f.click("ui:tab-recordings")
	f.click("ui:filter-problems")
	if !f.has("rec:door_a") || !f.has("rec:door_b") || f.has("rec:spare") {
		t.Fatal("both pieces of the file disagree")
	}
}

// TestMeantToSoundSteps shows the step a cue plays at, its category's
// adjustment taken out, and steps it louder.
func TestMeantToSoundSteps(t *testing.T) {
	t.Parallel()
	library := testLibrary()
	library.CategoryGainsDB = map[string]float64{protocol.CategorySFX: -3}
	library.Cues[1].Gain = loudness.Gain(-3)
	f := newFixtureWith(t, library)
	f.click("cue:sfx/door")
	f.click("ui:louder")
	var louder protocol.LoudnessChange
	if err := f.edit(protocol.AudioSetLoudness).Decode(&louder); err != nil || louder.Step != loudness.Louder {
		t.Fatalf("from Normal, Louder: %+v %v", louder, err)
	}
}

// TestLoudnessView sets the reference and a category's adjustment, and
// measures every recording, from the Loudness view.
func TestLoudnessView(t *testing.T) {
	t.Parallel()
	library := testLibrary()
	library.ReferenceDB = -20
	f := newFixtureWith(t, library)
	f.click("ui:loudness")
	if !f.has("ui:measure-all") || len(f.requests.Loudness) == 0 {
		t.Fatal("the view shows, and asks how loud the recordings are")
	}
	f.requests.Loudness[len(f.requests.Loudness)-1](protocol.AudioLoudness{Recordings: []protocol.RecordingLoudness{
		{ID: "door_a", MeasuredDB: -26, PeakDB: -9, TargetDB: -20, Problem: ""},
	}}, nil)
	f.click("ui:reference-louder")
	var reference protocol.ReferenceChange
	if err := f.edit(protocol.AudioSetReference).Decode(&reference); err != nil || reference.TargetDB != -19 {
		t.Fatalf("a dB louder: %+v %v", reference, err)
	}
	f.click("ui:adjust-quieter-" + protocol.CategorySFX)
	var adjustment protocol.CategoryGainChange
	if err := f.edit(protocol.AudioSetCategoryGain).Decode(&adjustment); err != nil ||
		adjustment.Category != protocol.CategorySFX || adjustment.GainDB != -1 {
		t.Fatalf("SFX a dB down: %+v %v", adjustment, err)
	}
	f.click("ui:measure-all")
	if len(f.requests.AudioOf(protocol.AudioMeasureAll)) != 1 {
		t.Fatal("Measure all asks once")
	}
	f.click("cue:sfx/door")
	if f.has("ui:measure-all") {
		t.Fatal("choosing a cue shows it in place of the view")
	}
}
