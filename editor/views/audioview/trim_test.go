package audioview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// withWaveform selects the door cue, whose first recording comes into
// focus, and answers the waveform question that asks.
func withWaveform(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.click("cue:sfx/door")
	if len(f.requests.Analyses) == 0 || f.requests.Analyses[len(f.requests.Analyses)-1].Asked.Recording != "door_a" {
		t.Fatal("the recording in focus is analysed")
	}
	f.answerWaveform()

	return f
}

// answerWaveform answers the latest waveform question: a second of sound
// that fills the middle, with a suggested gain.
func (f *fixture) answerWaveform() {
	f.t.Helper()
	asked := f.requests.Analyses[len(f.requests.Analyses)-1]
	answer := asked.Asked
	answer.Version, answer.DurationMs = "v1", 1000
	answer.Peaks = make([]byte, max(answer.Columns, 1))
	for index := range answer.Peaks {
		if index > len(answer.Peaks)/4 && index < len(answer.Peaks)*3/4 {
			answer.Peaks[index] = 200
		}
	}
	answer.Suggested = &protocol.AudioWindow{StartMs: 240, EndMs: 760}
	answer.Calibration = &protocol.AudioCalibration{
		Method: "active_rms", MeasuredDB: -24, TargetDB: -20, PeakDB: -3, GainDB: 4, Limited: false,
	}
	asked.Done(answer, nil)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestTrimsARecording suggests a region, drags its end, applies it, and
// plays the draft before that.
func TestTrimsARecording(t *testing.T) {
	t.Parallel()
	f := withWaveform(t)
	if !f.has("state:waveform") || !f.has("state:region=0.000-1.000") {
		t.Fatal("the waveform shows all of the recording as its region")
	}
	f.click("ui:suggest-region")
	if !f.has("state:region=0.240-0.760") {
		t.Fatal("the suggestion becomes the draft")
	}
	end := f.at("wave:end")
	press := ui.NewInput(end)
	press.Pressed, press.Down = true, true
	f.frame(press)
	drag := ui.NewInput(image.Pt(end.X+60, end.Y))
	drag.Down = true
	f.frame(drag)
	f.frame(ui.NewInput(image.Pt(end.X+60, end.Y)))
	if f.has("state:region=0.240-0.760") {
		t.Fatal("dragging the end moves it")
	}
	f.click("ui:play-region")
	sounds := f.ctx.Sounds.(*frametest.Sounds)
	if len(sounds.Auditioned) != 1 || sounds.Auditioned[0].Window == nil || sounds.Auditioned[0].Window.StartMs != 240 {
		t.Fatalf("the draft plays: %+v", sounds.Auditioned)
	}
	f.click("ui:apply-region")
	var change protocol.RegionChange
	if err := f.edit(protocol.AudioSetRegion).Decode(&change); err != nil || change.Recording != "door_a" ||
		change.Region == nil || change.Region.StartMs != 240 || change.Region.EndMs <= 760 {
		t.Fatalf("the region is applied: %+v %v", change, err)
	}
}

// TestTypedRegionAndDuplicateRegion drafts a region from typed seconds and
// makes a new recording of it for the cue.
func TestTypedRegionAndDuplicateRegion(t *testing.T) {
	t.Parallel()
	f := withWaveform(t)
	f.typeInto("region-start", "0.1")
	f.keys("Enter")
	f.typeInto("region-end", "0.5")
	f.keys("Enter")
	if !f.has("state:region=0.100-0.500") {
		t.Fatal("typed seconds draft the region")
	}
	if f.Unsaved() != 1 {
		t.Fatal("a drafted region is an unsaved change")
	}
	f.click("ui:duplicate-region")
	var duplicate protocol.DuplicateRegion
	if err := f.edit(protocol.AudioDuplicateRegion).Decode(&duplicate); err != nil || duplicate.Cue != "sfx/door" ||
		duplicate.Region == nil || duplicate.Region.EndMs != 500 {
		t.Fatalf("a new recording of the draft, for the cue: %+v %v", duplicate, err)
	}
}

// TestSetsLevels uses the suggested level, sets one by hand and a target.
func TestSetsLevels(t *testing.T) {
	t.Parallel()
	f := withWaveform(t)
	f.click("ui:use-suggestion")
	var measured protocol.AudioItem
	if err := f.edit(protocol.AudioAcceptCalibration).Decode(&measured); err != nil || measured.ID != "door_a" {
		t.Fatalf("the suggestion measures the recording: %+v %v", measured, err)
	}
	f.click("ui:set-level")
	f.typeInto("level", "-60")
	f.click("ui:set-level")
	if len(f.requests.AudioOf(protocol.AudioSetCalibration)) != 0 {
		t.Fatal("a gain berad-audio refuses is refused in the dialog")
	}
	f.typeInto("level", "−6 dB")
	f.click("ui:set-level")
	var gain protocol.CalibrationChange
	if err := f.edit(protocol.AudioSetCalibration).Decode(&gain); err != nil || gain.GainDB == nil || *gain.GainDB != -6 {
		t.Fatalf("a gain by hand: %+v %v", gain, err)
	}
	f.requests.AudioEdits = nil
	f.click("ui:set-target")
	f.click("ui:family-level")
	var target protocol.CalibrationChange
	if err := f.edit(protocol.AudioSetCalibration).Decode(&target); err != nil || !target.DefaultTarget {
		t.Fatalf("the family's target: %+v %v", target, err)
	}
}

// TestCheckMarksFilesThatCannotPlay shows the files a check could not
// decode as problems.
func TestCheckMarksFilesThatCannotPlay(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:check-files")
	if len(f.requests.Checks) != 1 {
		t.Fatal("checking asks the orchestrator")
	}
	f.requests.Checks[0](protocol.AudioCheck{Broken: []protocol.AudioProblem{{Recording: "door_b", Reason: "r"}}}, nil)
	f.click("ui:filter-problems")
	if !f.has("cue:sfx/door") {
		t.Fatal("a cue playing a file that cannot be decoded has a problem")
	}
}

// Unsaved counts the window's unsaved changes.
func (f *fixture) Unsaved() int { return f.state.Unsaved(f.docs) }

// TestDraftedRegionIsKeptOrSaved asks before another recording comes into
// focus while a region is drafted.
func TestDraftedRegionIsKeptOrSaved(t *testing.T) {
	t.Parallel()
	f := withWaveform(t)
	f.click("ui:suggest-region")
	f.click("plays:door_b")
	if !f.has("ui:" + frame.GuardSave) {
		t.Fatal("choosing another recording asks about the drafted region")
	}
	f.click("ui:" + frame.GuardSave)
	var change protocol.RegionChange
	if err := f.edit(protocol.AudioSetRegion).Decode(&change); err != nil || change.Region == nil {
		t.Fatalf("saving applies the region: %+v %v", change, err)
	}
}

// TestPlayButtonsStop shows Stop on the button whose part is playing; it
// stops, and the other button plays its part instead.
func TestPlayButtonsStop(t *testing.T) {
	t.Parallel()
	f := withWaveform(t)
	sounds := f.ctx.Sounds.(*frametest.Sounds)
	f.click("ui:play-region")
	if !f.has("state:stop=play-region") {
		t.Fatal("the playing part shows Stop")
	}
	f.click("ui:play-whole")
	if len(sounds.Auditioned) != 2 || sounds.Auditioned[1].Window != nil || !f.has("state:stop=play-whole") {
		t.Fatalf("the whole file plays instead: %+v", sounds.Auditioned)
	}
	f.click("ui:play-whole")
	if len(sounds.Auditioned) != 2 || f.has("state:stop=play-whole") {
		t.Fatal("Stop stops it")
	}
}
