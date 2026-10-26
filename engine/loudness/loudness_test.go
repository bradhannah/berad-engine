package loudness_test

import (
	"math"
	"testing"

	"github.com/bradhannah/berad-engine/engine/loudness"
)

func TestStepsRunFromQuietestToLoudest(t *testing.T) {
	steps := loudness.Steps()
	for index := 1; index < len(steps); index++ {
		if steps[index].DB <= steps[index-1].DB {
			t.Fatalf("%s is not louder than %s", steps[index].ID, steps[index-1].ID)
		}
	}
	normal, found := loudness.Find(loudness.Normal)
	if !found || normal.DB != 0 {
		t.Fatal("Normal plays at the reference")
	}
	if loudest := steps[len(steps)-1].DB + loudness.MaxCategoryDB; loudest > loudness.MaxBoostDB {
		t.Fatalf("the loudest step and adjustment reach %v dB, past berad-audio's +12", loudest)
	}
}

func TestCueGainsRoundTripThroughTheirStep(t *testing.T) {
	for _, step := range loudness.Steps() {
		for _, adjustment := range []float64{loudness.MinCategoryDB, -3, 0, loudness.MaxCategoryDB} {
			gain := loudness.CueGain(step.DB, adjustment)
			matched, found := loudness.Match(loudness.CueStepDB(gain, adjustment))
			if !found || matched.ID != step.ID {
				t.Fatalf("%s at %v dB came back as %+v", step.ID, adjustment, matched)
			}
		}
	}
	if _, found := loudness.Match(-3); found {
		t.Fatal("a value between steps matched one")
	}
	if math.Abs(loudness.Gain(-6)-0.501) > 0.001 || !math.IsInf(loudness.DB(0), -1) {
		t.Fatal("gains and dB disagree")
	}
}
