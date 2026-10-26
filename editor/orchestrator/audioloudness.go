package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/backend/ebitengine"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/loudness"
)

// Loudness: every recording is evened out to one reference loudness, the
// descriptor's calibration target, whatever its acoustic family; a family
// only says how to measure it. A cue's gain is its step of package loudness
// plus its category's adjustment, which the descriptor keeps; changing an
// adjustment changes the gain of every cue in the category, so playback, which
// reads only the library, hears it.

// Why a loudness edit is refused.
var (
	errStep       = errors.New("choose one of the loudness steps, from Much quieter to Much louder")
	errAdjustment = fmt.Errorf("a category's adjustment is from %.0f to +%.0f dB",
		loudness.MinCategoryDB, loudness.MaxCategoryDB)
	errReference = errors.New("the reference loudness is from -60 to -6 dB")
	errNoRegions = errors.New("choose at least one part of the file")
	errNoFileUse = errors.New("no recording has that file")
)

// Limits of the reference loudness, in dB, as berad-audio's targets.
const (
	minReferenceDB = -60.0
	maxReferenceDB = -6.0
)

// referenceOf is the loudness a descriptor evens recordings out to: its
// calibration target, or berad-audio's default.
func referenceOf(descriptor authoring.ProjectDescriptor) float64 {
	if descriptor.Calibration != nil {
		return descriptor.Calibration.TargetDB
	}

	return authoring.DefaultCalibrationOptions().TargetDB
}

// evenedOptions are a descriptor's editor options, or the defaults without
// one, measuring each family its own way but aiming every recording at the
// reference, asked for at each measurement so a changed one applies at once.
// A target its author set on one recording still wins, as berad-audio does.
func evenedOptions(descriptor *authoring.ProjectDescriptor, reference func() float64) authoring.EditorOptions {
	options := authoring.DefaultEditorOptions(ebitengine.Decode)
	if descriptor != nil {
		options = descriptor.EditorOptions(ebitengine.Decode)
	}
	measured := options.Calibration
	options.Calibration = func(family string) authoring.CalibrationOptions {
		policy := measured(family)
		policy.TargetDB = reference()

		return policy
	}

	return options
}

// applyLoudness carries out an edit of how loud cues are meant to sound, of
// what recordings are evened out to, or of a file's recordings.
func (s *audioSession) applyLoudness(edit protocol.AudioEdit) (string, error) {
	switch edit.Edit {
	case protocol.AudioSetLoudness:
		return withArgs(edit.Args, s.setLoudness)
	case protocol.AudioSetCategoryGain:
		return withArgs(edit.Args, s.setCategoryGain)
	case protocol.AudioSetReference:
		return withArgs(edit.Args, s.setReference)
	case protocol.AudioMeasureAll:
		return withArgs(edit.Args, s.measureUnmeasured)
	case protocol.AudioUseReference:
		return withArgs(edit.Args, s.useReference)
	case protocol.AudioSetFileAttribution:
		return withArgs(edit.Args, s.setFileAttribution)
	case protocol.AudioMakePieces:
		return withArgs(edit.Args, s.makePieces)
	}

	return "", fmt.Errorf("%w: %s", ErrUnknownEdit, edit.Edit)
}

// categoryOf is a cue's category, as the descriptor names it or its routing
// implies.
func (s *audioSession) categoryOf(cueID string) string {
	targets := authoring.AuthoringTargets{
		Tiles: nil, Groups: nil, Families: nil, CueCategories: s.categories,
	}

	return authoring.CueCategory(targets, s.library.Document().Routing, cueID)
}

// maxCueGain is the largest gain a cue may have in the library's schema.
func (s *audioSession) maxCueGain() float64 { return authoring.MaxCueGain(s.library.SchemaVersion()) }

// normalGain is the gain of a cue at Normal in a category.
func (s *audioSession) normalGain(category string) float64 {
	return min(loudness.CueGain(0, s.gains[category]), s.maxCueGain())
}

// setLoudness sets how loud a cue is meant to sound: its step, plus its
// category's adjustment.
func (s *audioSession) setLoudness(change protocol.LoudnessChange) (string, error) {
	if !slices.Contains(s.library.CueIDs(), change.Cue) {
		return "", errNoCue
	}
	step, found := loudness.Find(change.Step)
	if !found {
		return "", errStep
	}
	cue := s.library.Cue(change.Cue)
	cue.VolumeModifier = min(loudness.CueGain(step.DB, s.gains[s.categoryOf(change.Cue)]), s.maxCueGain())

	return change.Cue, authoringError(s.library.SetCue(change.Cue, cue))
}

// setCategoryGain sets a category's adjustment, and moves every cue in it by
// the difference, so each keeps its step.
func (s *audioSession) setCategoryGain(change protocol.CategoryGainChange) (string, error) {
	switch {
	case !validCategory(change.Category):
		return "", errCategory
	case math.IsNaN(change.GainDB) || change.GainDB < loudness.MinCategoryDB || change.GainDB > loudness.MaxCategoryDB:
		return "", errAdjustment
	}
	if err := s.rescale(change.Category, change.GainDB-s.gains[change.Category], ""); err != nil {
		return "", err
	}
	if change.GainDB == 0 {
		delete(s.gains, change.Category)
	} else {
		s.gains[change.Category] = change.GainDB
	}

	return "", nil
}

// rescale moves the gain of every cue in a category by a number of dB, or of
// one cue when only is set.
func (s *audioSession) rescale(category string, byDB float64, only string) error {
	if byDB == 0 {
		return nil
	}
	for _, cueID := range s.library.CueIDs() {
		if only != "" && cueID != only || s.categoryOf(cueID) != category {
			continue
		}
		cue := s.library.Cue(cueID)
		cue.VolumeModifier = min(cue.VolumeModifier*loudness.Gain(byDB), s.maxCueGain())
		if err := s.library.SetCue(cueID, cue); err != nil {
			return authoringError(err)
		}
	}

	return nil
}

// setReference sets the loudness recordings are evened out to, and measures
// every measured recording again.
func (s *audioSession) setReference(change protocol.ReferenceChange) (string, error) {
	if math.IsNaN(change.TargetDB) || change.TargetDB < minReferenceDB || change.TargetDB > maxReferenceDB {
		return "", errReference
	}
	s.reference = change.TargetDB

	return "", s.measureAll(func(recording authoring.Recording) bool {
		return recording.CalibrationMode() == authoring.CalibrationAuto
	})
}

// measureUnmeasured measures every recording no level was set for yet.
func (s *audioSession) measureUnmeasured(protocol.AudioItem) (string, error) {
	return "", s.measureAll(func(recording authoring.Recording) bool { return recording.CalibrationGainDB == nil })
}

// measureAll measures every recording with a file that chosen picks. One
// that cannot be measured, such as a missing file, is passed over: the
// Loudness view shows it.
func (s *audioSession) measureAll(chosen func(authoring.Recording) bool) error {
	for _, id := range s.library.RecordingIDs() {
		recording := s.library.Recording(id)
		if recording.File == nil || !chosen(recording) {
			continue
		}
		if err := s.measure(id); err != nil && !errors.Is(err, authoring.ErrLibrary) &&
			!errors.Is(err, errNoRecordingFile) {
			return err
		}
	}

	return nil
}

// measure evens a recording out to the reference, unless its level was set by
// hand. A silent one has nothing to even out and is left as it is.
func (s *audioSession) measure(recordingID string) error {
	recording := s.library.Recording(recordingID)
	if recording.File == nil || recording.CalibrationMode() == authoring.CalibrationManual {
		return nil
	}
	source, err := s.loadSource(recordingID)
	if err != nil {
		return err
	}
	options, err := s.library.CalibrationOptionsFor(recordingID)
	if err != nil {
		return authoringError(err)
	}
	if _, err := source.SuggestCalibration(recording.PlaybackWindow, options); err != nil {
		return nil // Silence has no level to set; the recording plays as recorded.
	}
	draft, err := s.library.BeginCalibration(recordingID, source)
	if err != nil {
		return authoringError(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), analysisTimeout)
	defer cancel()

	return authoringError(s.library.ApplyCalibration(ctx, draft))
}

// measureNew measures a recording no level was set for yet, such as a sound
// just added.
func (s *audioSession) measureNew(id string) error {
	if s.library.Recording(id).CalibrationGainDB != nil {
		return nil
	}

	return s.measure(id)
}

// useReference drops a level or target set by hand on a recording and
// measures it again, to the reference.
func (s *audioSession) useReference(item protocol.AudioItem) (string, error) {
	if !slices.Contains(s.library.RecordingIDs(), item.ID) {
		return "", errNoRecording
	}
	recording := s.library.Recording(item.ID)
	if recording.CalibrationTargetDB != nil {
		recording.CalibrationTargetDB = nil
		if err := s.library.SetRecording(item.ID, recording); err != nil {
			return "", authoringError(err)
		}
	}

	return s.acceptCalibration(item)
}

// setFileAttribution sets the attribution of every recording of a file, so
// the pieces of one file agree.
func (s *audioSession) setFileAttribution(change protocol.FileAttributionChange) (string, error) {
	var changed []string
	for _, id := range s.library.RecordingIDs() {
		if file := s.library.Recording(id).File; file != nil && *file == change.File {
			if err := s.attribute(id, change.Attribution); err != nil {
				return "", err
			}
			changed = append(changed, id)
		}
	}
	if len(changed) == 0 {
		return "", errNoFileUse
	}

	return changed[0], nil
}

// makePieces makes a new recording of each region of a recording's file,
// played in turn by a cue when one is named, and measures each.
func (s *audioSession) makePieces(pieces protocol.MakePieces) (string, error) {
	switch {
	case !slices.Contains(s.library.RecordingIDs(), pieces.Recording):
		return "", errNoRecording
	case len(pieces.Regions) == 0:
		return "", errNoRegions
	case pieces.Cue != "" && !slices.Contains(s.library.CueIDs(), pieces.Cue):
		return "", errNoCue
	}
	source, err := s.loadSource(pieces.Recording)
	if err != nil {
		return "", err
	}
	first := ""
	for _, region := range pieces.Regions {
		draft, err := s.library.BeginRecordingRegion(pieces.Recording, source)
		if err != nil {
			return "", authoringError(err)
		}
		made, err := s.library.NextRecordingID(pieces.Recording)
		if err != nil {
			return "", authoringError(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), analysisTimeout)
		err = s.library.DuplicateRecordingRegion(ctx, draft, made, playbackWindow(&region), pieces.Cue)
		cancel()
		if err != nil {
			return "", authoringError(err)
		}
		if err := s.measureNew(made); err != nil {
			return "", err
		}
		if first == "" {
			first = made
		}
	}

	return first, nil
}
