package orchestrator

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// How far away an ambient cue can be heard, in tiles, and how long it can
// fade, in milliseconds; and what a new ambient cue starts with, until its
// author changes it.
const (
	maxRadiusTiles = 1000
	maxFadeMs      = 60_000
	newRadiusTiles = 8
	newFadeMs      = 1000
)

// Why an ambient, profile or family edit is refused.
var (
	errAmbientRadius = errors.New("an ambient cue is heard from 0 to 1000 tiles away")
	errAmbientFade   = errors.New("a fade lasts from 0 to 60 seconds")
	errAmbientCycle  = errors.New("an ambient cue heard from a distance plays one recording; take out the " +
		"others first")
	errNoProfile = errors.New("the audio library has no such profile")
	errFamily    = errors.New("a new family's name uses lowercase letters, digits, _ and -")
)

// applyClassification carries out an edit that sets how far away an ambient
// cue is heard, a cue's profile or a recording's family.
func (s *audioSession) applyClassification(edit protocol.AudioEdit) (string, error) {
	switch edit.Edit {
	case protocol.AudioSetAmbient:
		return withArgs(edit.Args, s.setAmbient)
	case protocol.AudioSetProfile:
		return withArgs(edit.Args, s.setProfile)
	case protocol.AudioSetFamily:
		return withArgs(edit.Args, s.setFamily)
	}

	return "", fmt.Errorf("%w: %s", ErrUnknownEdit, edit.Edit)
}

// setAmbient sets an ambient cue's distance and fades, which berad-audio
// keeps in the library's routing.
func (s *audioSession) setAmbient(change protocol.AmbientChange) (string, error) {
	settings := change.Ambient
	switch {
	case !slices.Contains(s.library.CueIDs(), change.Cue):
		return "", errNoCue
	case math.IsNaN(settings.RadiusTiles) || settings.RadiusTiles < 0 || settings.RadiusTiles > maxRadiusTiles:
		return "", errAmbientRadius
	case settings.FadeInMs < 0 || settings.FadeInMs > maxFadeMs || settings.FadeOutMs < 0 ||
		settings.FadeOutMs > maxFadeMs:
		return "", errAmbientFade
	case s.library.Cue(change.Cue).RecordingID == "":
		return "", errAmbientCycle
	}
	routing := s.library.Routing()
	if routing.Ambient == nil {
		routing.Ambient = map[string]authoring.AmbientProfile{}
	}
	routing.Ambient[change.Cue] = authoring.AmbientProfile{
		RadiusTiles: settings.RadiusTiles, FadeInMs: settings.FadeInMs, FadeOutMs: settings.FadeOutMs,
	}

	return change.Cue, authoringError(s.library.SetRouting(routing))
}

// setProfile gives a cue one of the library's classification profiles.
func (s *audioSession) setProfile(change protocol.ProfileChange) (string, error) {
	switch {
	case !slices.Contains(s.library.CueIDs(), change.Cue):
		return "", errNoCue
	case !slices.Contains(s.library.ClassificationProfiles(), change.Profile):
		return "", errNoProfile
	}

	return change.Cue, authoringError(s.library.SetCueProfile(change.Cue, change.Profile))
}

// setFamily puts a recording in an acoustic family; a family no recording is
// in yet is named like an ID's part. berad-audio measures a measured
// recording again for its new family's target.
func (s *audioSession) setFamily(change protocol.FamilyChange) (string, error) {
	if !slices.Contains(s.library.RecordingIDs(), change.Recording) {
		return "", errNoRecording
	}
	known := s.library.AcousticFamilies()
	if !slices.Contains(known, change.Family) &&
		(change.Family == "" || strings.Trim(change.Family, "abcdefghijklmnopqrstuvwxyz0123456789_-") != "") {
		return "", errFamily
	}

	return change.Recording, authoringError(s.library.SetRecordingFamily(change.Recording, change.Family))
}

// newAmbient gives a new ambient cue what the audio project's preset says,
// when it has one: its profile, its gain and how to hear it.
func (s *audioSession) newAmbient(cue *authoring.Cue, profile *string) {
	preset := s.preset
	if preset == nil {
		return
	}
	cue.VolumeModifier = preset.Gain
	if cue.HowToHear == defaultHowToHear && preset.HowToHear != "" {
		cue.HowToHear = preset.HowToHear
	}
	if slices.Contains(s.library.ClassificationProfiles(), preset.Profile) {
		*profile = preset.Profile
	}
}

// heardFromAfar makes a new ambient cue heard from newRadiusTiles tiles away,
// fading in and out over newFadeMs, and puts its first recording in the
// preset's family.
func (s *audioSession) heardFromAfar(cueID, slot string) error {
	if s.preset != nil && s.preset.Family != "" {
		if err := s.library.SetRecordingFamily(slot, s.preset.Family); err != nil {
			return authoringError(err)
		}
	}
	_, err := s.setAmbient(protocol.AmbientChange{Cue: cueID, Ambient: protocol.AmbientSettings{
		RadiusTiles: newRadiusTiles, FadeInMs: newFadeMs, FadeOutMs: newFadeMs,
	}})

	return err
}
