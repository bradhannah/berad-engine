package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// defaultHowToHear is how to hear a new cue until its author says.
const defaultHowToHear = "Not described yet."

// Why an audio edit is refused.
var (
	errCueID = errors.New("a cue ID uses lowercase letters, digits, _ and -, in parts joined by /, " +
		"such as door/open")
	errNoName       = errors.New("a cue needs a name")
	errNoHowToHear  = errors.New("say how to hear the cue in the game")
	errNoFile       = errors.New("choose a sound file")
	errNoCue        = errors.New("the audio library has no such cue")
	errNoRecording  = errors.New("the audio library has no such recording")
	errCategory     = errors.New("a category is music, sfx or ambient")
	errNoRecordings = errors.New("a cue plays at least one recording")
	errGain         = errors.New("a cue's gain is between 0 and 1")
)

// apply carries out one audio edit and returns the cue or recording it made.
func (s *audioSession) apply(edit protocol.AudioEdit) (string, error) {
	switch edit.Edit {
	case protocol.AudioCreateCue:
		return withArgs(edit.Args, s.createCue)
	case protocol.AudioAddSound:
		return withArgs(edit.Args, s.addSound)
	case protocol.AudioReplaceFile:
		return withArgs(edit.Args, s.replaceFile)
	case protocol.AudioSetCue:
		return withArgs(edit.Args, s.setCue)
	case protocol.AudioSetCategory:
		return withArgs(edit.Args, s.setCategory)
	case protocol.AudioSetRecordings:
		return withArgs(edit.Args, s.setRecordings)
	case protocol.AudioSetRecording:
		return withArgs(edit.Args, s.setRecording)
	case protocol.AudioSetAttribution:
		return withArgs(edit.Args, func(change protocol.AttributionChange) (string, error) {
			return change.Recording, s.attribute(change.Recording, change.Attribution)
		})
	case protocol.AudioDeleteCue:
		return withArgs(edit.Args, s.deleteCue)
	case protocol.AudioDeleteRecording:
		return withArgs(edit.Args, func(item protocol.AudioItem) (string, error) {
			return "", authoringError(s.library.DeleteRecording(item.ID))
		})
	}

	return "", fmt.Errorf("%w: %s", ErrUnknownEdit, edit.Edit)
}

func withArgs[T any](args json.RawMessage, apply func(T) (string, error)) (string, error) {
	var decoded T
	if err := json.Unmarshal(args, &decoded); err != nil {
		return "", fmt.Errorf("%w: %w", protocol.ErrPayload, err)
	}

	return apply(decoded)
}

// authoringError keeps berad-audio's reason and drops its prefix, which says
// only that the input was invalid.
func authoringError(err error) error {
	if err == nil {
		return nil
	}
	reason := strings.TrimPrefix(err.Error(), authoring.ErrLibrary.Error()+": ")

	return fmt.Errorf("%w: %s", authoring.ErrLibrary, reason)
}

// validCueID reports whether a new cue's ID is lowercase letters, digits, _
// and -, in parts joined by /.
func validCueID(id string) bool {
	if id == "" {
		return false
	}
	for part := range strings.SplitSeq(id, "/") {
		if part == "" || strings.Trim(part, "abcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
			return false
		}
	}

	return true
}

// createCue makes a cue with an empty slot for its first recording.
func (s *audioSession) createCue(cue protocol.NewCue) (string, error) {
	name, how := strings.TrimSpace(cue.Name), strings.TrimSpace(cue.HowToHear)
	switch {
	case !validCueID(cue.ID):
		return "", errCueID
	case name == "":
		return "", errNoName
	case cue.Category != "" && !validCategory(cue.Category):
		return "", errCategory
	}
	if how == "" {
		how = defaultHowToHear
	}
	slot := s.newRecordingID(cue.ID)
	if err := s.library.AddRecording(slot); err != nil {
		return "", authoringError(err)
	}
	err := s.library.CreateCue(cue.ID, authoring.Cue{
		Name: name, RecordingID: slot, RecordingIDs: nil, Selection: "", VolumeModifier: 1, HowToHear: how,
		SearchTerms: nil,
	}, "")
	if err != nil {
		return "", authoringError(err)
	}
	if cue.Category != "" {
		s.categories[cue.ID] = cue.Category
	}

	return cue.ID, nil
}

func validCategory(category string) bool {
	return category == protocol.CategoryMusic || category == protocol.CategorySFX ||
		category == protocol.CategoryAmbient
}

// newRecordingID names a new recording after its cue's last part, made
// lowercase snake_case, numbered when the name is taken.
func (s *audioSession) newRecordingID(cueID string) string {
	base := cueID[strings.LastIndex(cueID, "/")+1:]
	base = strings.ReplaceAll(base, "-", "_")
	if base == "" {
		base = "recording"
	}
	taken := s.library.RecordingIDs()
	candidate := base
	for number := 2; slices.Contains(taken, candidate); number++ {
		candidate = base + "_" + strconv.Itoa(number)
	}

	return candidate
}

// addSound copies a file in as a new recording: into a new cue, into an
// existing cue's empty slot, or as another recording it plays in turn.
func (s *audioSession) addSound(sound protocol.AddSound) (string, error) {
	if strings.TrimSpace(sound.Path) == "" {
		return "", errNoFile
	}
	cueID := sound.Cue
	if sound.NewCue != nil {
		created, err := s.createCue(*sound.NewCue)
		if err != nil {
			return "", err
		}
		cueID = created
	}
	if !slices.Contains(s.library.CueIDs(), cueID) {
		return "", errNoCue
	}
	cue := s.library.Cue(cueID)
	recordings := cue.Recordings()
	var target string
	if len(recordings) == 1 && s.library.Recording(recordings[0]).File == nil {
		target = recordings[0]
	} else {
		target = s.newRecordingID(cueID)
		if err := s.library.AddRecording(target); err != nil {
			return "", authoringError(err)
		}
	}
	if err := s.copyIn(target, sound.Path, sound.Attribution); err != nil {
		return "", err
	}
	if !slices.Contains(recordings, target) {
		if err := s.library.SetCue(cueID, cue.WithRecordings(append(recordings, target))); err != nil {
			return "", authoringError(err)
		}
	}

	return cueID, nil
}

// replaceFile copies a new file in for a recording.
func (s *audioSession) replaceFile(replace protocol.ReplaceFile) (string, error) {
	if !slices.Contains(s.library.RecordingIDs(), replace.Recording) {
		return "", errNoRecording
	}
	if strings.TrimSpace(replace.Path) == "" {
		return "", errNoFile
	}

	return replace.Recording, s.copyIn(replace.Recording, replace.Path, replace.Attribution)
}

// copyIn copies the author's file into the project for a recording, then
// records its attribution. berad-audio decodes the file first, so one that
// cannot play is refused before anything is copied.
func (s *audioSession) copyIn(recording, path string, attribution protocol.Attribution) error {
	if err := s.library.ImportRecording(recording, path); err != nil {
		return fmt.Errorf("add the sound: %w", authoringError(err))
	}

	return s.attribute(recording, attribution)
}

// attribute records who made a recording, where it came from, its licence and
// whether it is cleared for a commercial game. A blank answer is unknown.
func (s *audioSession) attribute(recording string, attribution protocol.Attribution) error {
	if !slices.Contains(s.library.RecordingIDs(), recording) {
		return errNoRecording
	}
	provenance := s.library.Recording(recording).Provenance
	provenance.SourceCreator = known(attribution.Creator)
	provenance.SourceURL = known(attribution.Source)
	provenance.License = known(attribution.License)
	cleared := attribution.Commercial
	provenance.CommercialUse = &cleared

	return authoringError(s.library.SetRecordingProvenance(recording, provenance))
}

func known(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}

	return protocol.Unknown
}

// setCue changes a cue's name, gain, how to hear it or its search terms.
func (s *audioSession) setCue(change protocol.CueChange) (string, error) {
	if !slices.Contains(s.library.CueIDs(), change.Cue) {
		return "", errNoCue
	}
	cue := s.library.Cue(change.Cue)
	if change.Name != nil {
		if cue.Name = strings.TrimSpace(*change.Name); cue.Name == "" {
			return "", errNoName
		}
	}
	if change.HowToHear != nil {
		if cue.HowToHear = strings.TrimSpace(*change.HowToHear); cue.HowToHear == "" {
			return "", errNoHowToHear
		}
	}
	if change.Gain != nil {
		if *change.Gain < 0 || *change.Gain > 1 {
			return "", errGain
		}
		cue.VolumeModifier = *change.Gain
	}
	if change.SearchTerms != nil {
		cue.SearchTerms = terms(*change.SearchTerms)
	}

	return change.Cue, authoringError(s.library.SetCue(change.Cue, cue))
}

// terms trims search terms and drops empty ones; none is nil, as the schema
// leaves them out.
func terms(values []string) []string {
	var kept []string
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}

	return kept
}

// setCategory names a cue's category in the descriptor, or returns it to the
// default when empty.
func (s *audioSession) setCategory(change protocol.CategoryChange) (string, error) {
	if !slices.Contains(s.library.CueIDs(), change.Cue) {
		return "", errNoCue
	}
	if change.Category == "" {
		delete(s.categories, change.Cue)

		return change.Cue, nil
	}
	if !validCategory(change.Category) {
		return "", errCategory
	}
	s.categories[change.Cue] = change.Category

	return change.Cue, nil
}

// setRecordings sets the recordings a cue plays, in turn when there are
// several; one taken out stays in the library.
func (s *audioSession) setRecordings(change protocol.CueRecordings) (string, error) {
	if !slices.Contains(s.library.CueIDs(), change.Cue) {
		return "", errNoCue
	}
	if len(change.Recordings) == 0 {
		return "", errNoRecordings
	}
	cue := s.library.Cue(change.Cue).WithRecordings(change.Recordings)

	return change.Cue, authoringError(s.library.SetCue(change.Cue, cue))
}

// setRecording changes a recording's note or search terms.
func (s *audioSession) setRecording(change protocol.RecordingChange) (string, error) {
	if !slices.Contains(s.library.RecordingIDs(), change.Recording) {
		return "", errNoRecording
	}
	recording := s.library.Recording(change.Recording)
	if change.Note != nil {
		recording.Note = strings.TrimSpace(*change.Note)
	}
	if change.SearchTerms != nil {
		recording.SearchTerms = terms(*change.SearchTerms)
	}

	return change.Recording, authoringError(s.library.SetRecording(change.Recording, recording))
}

// deleteCue deletes a cue and its category; its recordings stay.
func (s *audioSession) deleteCue(item protocol.AudioItem) (string, error) {
	if err := s.library.DeleteCue(item.ID); err != nil {
		return "", authoringError(err)
	}
	delete(s.categories, item.ID)

	return "", nil
}
