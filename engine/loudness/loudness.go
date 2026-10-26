// Package loudness names how loud a sound is meant to play. Every recording is
// evened out to one reference loudness; a cue then plays at a step from Much
// quieter to Much louder around that reference, and a Play sound node can ask
// for another step for one moment. Steps are in dB above the reference, below
// it when negative; Gain turns dB into the linear gain berad-audio plays.
package loudness

import "math"

// Step IDs, quietest first, as cues and Play sound nodes store them.
const (
	MuchQuieter = "much-quieter"
	Quieter     = "quieter"
	Normal      = "normal"
	Louder      = "louder"
	MuchLouder  = "much-louder"
)

// Limits of a category's adjustment, in dB. A cue's gain is its step plus its
// category's adjustment, and berad-audio plays at most +12 dB, so the loudest
// step, +8 dB, leaves +4 dB for the adjustment.
const (
	MinCategoryDB = -12.0
	MaxCategoryDB = 4.0
	// MaxBoostDB is the most a gain may add: berad-audio's +12 dB.
	MaxBoostDB = 12.0
)

// How loud each step plays, in dB from the reference.
const (
	muchQuieterDB = -12.0
	quieterDB     = -6.0
	louderDB      = 4.0
	muchLouderDB  = 8.0
)

const (
	// tolerance is how close, in dB, a value must be to a step to be it.
	tolerance  = 0.05
	decibels   = 20.0
	decibelLog = 10.0
)

// Step is one named loudness: its ID, its label and how many dB it plays
// above the reference.
type Step struct {
	ID    string
	Label string
	DB    float64
}

// Steps returns the steps, quietest first.
func Steps() []Step {
	return []Step{
		{ID: MuchQuieter, Label: "Much quieter", DB: muchQuieterDB},
		{ID: Quieter, Label: "Quieter", DB: quieterDB},
		{ID: Normal, Label: "Normal", DB: 0},
		{ID: Louder, Label: "Louder", DB: louderDB},
		{ID: MuchLouder, Label: "Much louder", DB: muchLouderDB},
	}
}

// Find returns the step with an ID.
func Find(id string) (Step, bool) {
	for _, step := range Steps() {
		if step.ID == id {
			return step, true
		}
	}

	return Step{ID: "", Label: "", DB: 0}, false
}

// Match returns the step a value in dB is, when it is within a twentieth of
// a dB of one.
func Match(db float64) (Step, bool) {
	for _, step := range Steps() {
		if math.Abs(step.DB-db) <= tolerance {
			return step, true
		}
	}

	return Step{ID: "", Label: "", DB: 0}, false
}

// Gain turns dB into a linear gain.
func Gain(db float64) float64 { return math.Pow(decibelLog, db/decibels) }

// DB turns a linear gain into dB; zero is minus infinity.
func DB(gain float64) float64 { return decibels * math.Log10(gain) }

// CueGain is the gain a cue at a step plays at in a category with an
// adjustment, both in dB.
func CueGain(stepDB, categoryDB float64) float64 { return Gain(stepDB + categoryDB) }

// CueStepDB is how many dB a cue playing at gain sits above the reference,
// its category's adjustment taken out.
func CueStepDB(gain, categoryDB float64) float64 { return DB(gain) - categoryDB }
