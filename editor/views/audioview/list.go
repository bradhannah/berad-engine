package audioview

import (
	"path"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
)

// Problems with a recording, as the list and the panels name them.
const (
	problemMissing    = "file missing"
	problemTooLarge   = "file too large"
	problemUnknown    = "attribution incomplete"
	problemUncleared  = "not cleared for a commercial game"
	problemBroken     = "cannot be decoded"
	stateNoSound      = "no sound yet"
	sectionUsed       = sectionPrefix + "used"
	sectionUnused     = sectionPrefix + "unused"
	categorySectionOf = sectionPrefix + "category:"
)

// categories are the cue categories in the order the window lists them.
func categories() []string {
	return []string{protocol.CategoryMusic, protocol.CategorySFX, protocol.CategoryAmbient}
}

// categoryLabel names a category for authors.
func categoryLabel(category string) string {
	switch category {
	case protocol.CategoryMusic:
		return "Music"
	case protocol.CategoryAmbient:
		return "Ambient"
	}

	return "SFX"
}

// problems lists what is wrong with a recording that has a file: it is
// missing or too large, its creator or licence is unknown, or it is not
// cleared for a commercial game. A recording with no file has none.
func fileProblems(recording protocol.AudioRecording) []string {
	var found []string
	switch recording.Status {
	case protocol.RecordingNoFile:
		return nil
	case protocol.RecordingMissing:
		found = append(found, problemMissing)
	case protocol.RecordingTooLarge:
		found = append(found, problemTooLarge)
	}
	attribution := recording.Attribution
	if unknown(attribution.Creator) || unknown(attribution.License) {
		found = append(found, problemUnknown)
	}
	if !attribution.Commercial {
		found = append(found, problemUncleared)
	}

	return found
}

func unknown(value string) bool {
	return strings.TrimSpace(value) == "" || strings.EqualFold(strings.TrimSpace(value), protocol.Unknown)
}

// problems lists what is wrong with a recording, starting with a file the
// last check could not decode, while the file is the one checked.
func (s *State) problems(recording protocol.AudioRecording) []string {
	found := fileProblems(recording)
	if broken, checked := s.broken[recording.ID]; checked && broken == recording.Version {
		found = append([]string{problemBroken}, found...)
	}

	return found
}

// cueProblems lists the problems of a cue's recordings, each once.
func (s *State) cueProblems(cue protocol.AudioCue) []string {
	var found []string
	for _, recording := range cue.Recordings {
		for _, problem := range s.problems(recording) {
			if !contains(found, problem) {
				found = append(found, problem)
			}
		}
	}

	return found
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}

	return false
}

// shows reports whether the filter shows something with these problems.
func (s *State) shows(found []string) bool {
	switch s.filter {
	case filterProblems:
		return contains(found, problemMissing) || contains(found, problemTooLarge) || contains(found, problemUnknown) ||
			contains(found, problemBroken)
	case filterUncleared:
		return contains(found, problemUncleared)
	}

	return true
}

// matches reports whether every word of the search is in one of the texts.
func (s *State) matches(texts ...string) bool {
	words := strings.Fields(strings.ToLower(s.search.Field.Value()))
	joined := strings.ToLower(strings.Join(texts, "\n"))
	for _, word := range words {
		if !strings.Contains(joined, word) {
			return false
		}
	}

	return true
}

func (s *State) searching() bool { return strings.TrimSpace(s.search.Field.Value()) != "" }

// rows lists the library in the window's mode.
func (s *State) rows() []ui.Row {
	if s.library == nil || s.library.Problem != "" {
		return nil
	}
	if s.mode == modeRecordings {
		return s.recordingRows()
	}

	return s.cueRows()
}

// cueRows lists the cues under their categories, each with its ID beside its
// name and its state at the right: a problem, no sound yet, or how many
// recordings it plays in turn.
func (s *State) cueRows() []ui.Row {
	var rows []ui.Row
	for _, category := range categories() {
		var shown []protocol.AudioCue
		for _, cue := range s.library.Cues {
			if cue.Category == category && s.shows(s.cueProblems(cue)) && s.matchesCue(cue) {
				shown = append(shown, cue)
			}
		}
		section := categorySectionOf + category
		rows = append(rows, s.section(section, categoryLabel(category), len(shown), true))
		if s.closed[section] && !s.searching() {
			continue
		}
		for _, cue := range shown {
			row := ui.NewRow(cuePrefix+cue.ID, cue.Name)
			row.Icon, row.Aside, row.Detail = ui.IconSound, cue.ID, s.cueState(cue)
			rows = append(rows, row)
		}
	}

	return rows
}

func (s *State) matchesCue(cue protocol.AudioCue) bool {
	texts := append([]string{cue.ID, cue.Name, cue.HowToHear, cue.Profile}, cue.SearchTerms...)
	if profile, found := s.profileOf(cue.Profile); found {
		texts = append(append(append(texts, profile.Phase, profile.Outcome, profile.MixRole), profile.Emitters...),
			append(profile.Contexts, profile.SearchTerms...)...)
	}
	for _, recording := range cue.Recordings {
		texts = append(texts, recordingTexts(recording)...)
	}

	return s.matches(texts...)
}

func recordingTexts(recording protocol.AudioRecording) []string {
	attribution := recording.Attribution

	return append([]string{
		recording.ID, recording.File, recording.OriginalFile, recording.Note, attribution.Creator,
		attribution.Source, attribution.License, recording.Family,
	}, recording.SearchTerms...)
}

// cueState says, for a cue's row, its first problem, that it has no sound
// yet, or how many recordings it plays in turn.
func (s *State) cueState(cue protocol.AudioCue) string {
	if found := s.cueProblems(cue); len(found) > 0 {
		return found[0]
	}
	if len(cue.Recordings) == 1 && cue.Recordings[0].Status == protocol.RecordingNoFile {
		return stateNoSound
	}
	if len(cue.Recordings) > 1 {
		return strconv.Itoa(len(cue.Recordings)) + " in turn"
	}

	return ""
}

// recordingRows lists the recordings cues play, then those no cue plays,
// each with its file beside it and its first problem at the right.
func (s *State) recordingRows() []ui.Row {
	var used []protocol.AudioRecording
	seen := map[string]bool{}
	for _, cue := range s.library.Cues {
		for _, recording := range cue.Recordings {
			if !seen[recording.ID] {
				seen[recording.ID] = true
				used = append(used, recording)
			}
		}
	}
	rows := s.recordingSection(sectionUsed, "Played by cues", used)

	return append(rows, s.recordingSection(sectionUnused, "Not used", s.library.Unused)...)
}

func (s *State) recordingSection(rowID, label string, recordings []protocol.AudioRecording) []ui.Row {
	var shown []protocol.AudioRecording
	for _, recording := range recordings {
		if s.shows(s.problems(recording)) && s.matches(recordingTexts(recording)...) {
			shown = append(shown, recording)
		}
	}
	rows := []ui.Row{s.section(rowID, label, len(shown), false)}
	if s.closed[rowID] && !s.searching() {
		return rows
	}
	for _, recording := range shown {
		row := ui.NewRow(recordingPrefix+recording.ID, recording.ID)
		row.Icon, row.Aside = ui.IconSound, path.Base(recording.File)
		if recording.File == "" {
			row.Aside = ""
		}
		if found := s.problems(recording); len(found) > 0 {
			row.Detail = found[0]
		} else if recording.Status == protocol.RecordingNoFile {
			row.Detail = stateNoSound
		}
		rows = append(rows, row)
	}

	return rows
}

// section returns a section heading with its count, open or closed; a
// search opens every section.
func (s *State) section(rowID, label string, count int, add bool) ui.Row {
	row := ui.NewHeading(rowID, label)
	row.Section, row.Add = true, add
	row.Detail = strconv.Itoa(count)
	row.Toggle = ui.DisclosureOpen
	if s.closed[rowID] && !s.searching() {
		row.Toggle = ui.DisclosureClosed
	}

	return row
}

// count is the number of cues or recordings the library lists.
func (s *State) count(mode string) int {
	if s.library == nil {
		return 0
	}
	if mode == modeCues {
		return len(s.library.Cues)
	}
	seen := map[string]bool{}
	for _, cue := range s.library.Cues {
		for _, recording := range cue.Recordings {
			seen[recording.ID] = true
		}
	}

	return len(seen) + len(s.library.Unused)
}
