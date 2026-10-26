package orchestrator

import (
	"context"
	"slices"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// Trimming and levels: edits that measure a recording's file, so each loads
// it first. berad-audio checks the file is still the one measured before it
// changes the library.

// loadSource decodes a recording's file for an edit that measures it.
func (s *audioSession) loadSource(recording string) (*authoring.RecordingAudio, error) {
	path, err := s.library.RecordingPath(recording)
	if err != nil {
		return nil, errNoRecordingFile
	}
	ctx, cancel := context.WithTimeout(context.Background(), analysisTimeout)
	defer cancel()
	clip, err := s.library.LoadRecording(ctx, path)
	if err != nil {
		return nil, authoringError(err)
	}

	return clip, nil
}

// setRegion sets the part of a recording that plays, and measures it again
// when its gain is measured.
func (s *audioSession) setRegion(change protocol.RegionChange) (string, error) {
	draft, err := s.regionDraft(change.Recording)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), analysisTimeout)
	defer cancel()

	return change.Recording, authoringError(s.library.ApplyRecordingRegion(ctx, draft, playbackWindow(change.Region)))
}

func (s *audioSession) regionDraft(recording string) (*authoring.RecordingRegionDraft, error) {
	if !slices.Contains(s.library.RecordingIDs(), recording) {
		return nil, errNoRecording
	}
	source, err := s.loadSource(recording)
	if err != nil {
		return nil, err
	}
	draft, err := s.library.BeginRecordingRegion(recording, source)
	if err != nil {
		return nil, authoringError(err)
	}

	return draft, nil
}

// duplicateRegion makes a new recording of part of one, played in turn by a
// cue when one is named.
func (s *audioSession) duplicateRegion(duplicate protocol.DuplicateRegion) (string, error) {
	if duplicate.Cue != "" && !slices.Contains(s.library.CueIDs(), duplicate.Cue) {
		return "", errNoCue
	}
	draft, err := s.regionDraft(duplicate.Recording)
	if err != nil {
		return "", err
	}
	made, err := s.library.NextRecordingID(duplicate.Recording)
	if err != nil {
		return "", authoringError(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), analysisTimeout)
	defer cancel()
	err = s.library.DuplicateRecordingRegion(ctx, draft, made, playbackWindow(duplicate.Region), duplicate.Cue)
	if err != nil {
		return "", authoringError(err)
	}

	return made, s.measureNew(made)
}

// acceptCalibration measures a recording and sets its gain from that, to be
// measured again whenever its region or family changes.
func (s *audioSession) acceptCalibration(item protocol.AudioItem) (string, error) {
	if !slices.Contains(s.library.RecordingIDs(), item.ID) {
		return "", errNoRecording
	}
	source, err := s.loadSource(item.ID)
	if err != nil {
		return "", err
	}
	draft, err := s.library.BeginCalibration(item.ID, source)
	if err != nil {
		return "", authoringError(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), analysisTimeout)
	defer cancel()

	return item.ID, authoringError(s.library.ApplyCalibration(ctx, draft))
}

// setCalibration sets a recording's gain by hand, which keeps it through
// later changes, or the loudness its measurement aims at, measuring a
// measured recording again.
func (s *audioSession) setCalibration(change protocol.CalibrationChange) (string, error) {
	if !slices.Contains(s.library.RecordingIDs(), change.Recording) {
		return "", errNoRecording
	}
	recording := s.library.Recording(change.Recording)
	if change.GainDB != nil {
		gain := *change.GainDB
		recording.CalibrationGainDB, recording.CalibrationPolicy = &gain, authoring.CalibrationManual
	}
	switch {
	case change.DefaultTarget:
		recording.CalibrationTargetDB = nil
	case change.TargetDB != nil:
		target := *change.TargetDB
		recording.CalibrationTargetDB = &target
	}
	if err := s.library.SetRecording(change.Recording, recording); err != nil {
		return "", authoringError(err)
	}
	retarget := change.DefaultTarget || change.TargetDB != nil
	if change.GainDB == nil && retarget && recording.CalibrationMode() == authoring.CalibrationAuto {
		return s.acceptCalibration(protocol.AudioItem{ID: change.Recording})
	}

	return change.Recording, nil
}
