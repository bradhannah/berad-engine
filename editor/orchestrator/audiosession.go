package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/backend/ebitengine"
)

// Permissions of the audio folder when an edit makes it.
const audioDirPerm = 0o750

// errNoLibrary refuses an edit that needs a library the project does not have.
var errNoLibrary = errors.New("this project has no sounds yet; add a sound to start its library")

// errAudioChanged refuses an undo whose values changed since.
var errAudioChanged = errors.New(reasonAudioChanged)

// audioSession is one job's view of the audio files: the library, opened
// afresh, and the cues' categories from the descriptor.
type audioSession struct {
	files   audioFiles
	library *authoring.LibraryEditor
	// descriptor is the descriptor's bytes as last read or written, nil when
	// the project has none; saved are the categories it held when the job
	// began and categories those the job leaves.
	descriptor []byte
	saved      map[string]string
	categories map[string]string
	// preset is what the audio project gives a new ambient cue, nil for
	// nothing.
	preset *authoring.AmbientPreset
	// createdLibrary and createdDescriptor say the job made those files, so
	// a failed job removes them again.
	createdLibrary    bool
	createdDescriptor bool
}

// openAudioSession reads the descriptor and opens the library, upgrading it
// to schema 4 in memory; the job's save writes the upgrade. With create, a
// project without a library gets an empty one.
func openAudioSession(files audioFiles, create bool) (*audioSession, error) {
	session := &audioSession{
		files: files, library: nil, descriptor: nil, saved: map[string]string{}, categories: nil, preset: nil,
		createdLibrary: false, createdDescriptor: false,
	}
	options := authoring.DefaultEditorOptions(ebitengine.Decode)
	data, err := os.ReadFile(files.descriptor)
	switch {
	case err == nil:
		descriptor, err := authoring.ParseProjectDescriptor(data)
		if err != nil {
			return nil, fmt.Errorf("read the audio project: %w", err)
		}
		options = descriptor.EditorOptions(ebitengine.Decode)
		session.descriptor, session.preset = data, descriptor.Ambient
		if descriptor.Targets.CueCategories != nil {
			session.saved = maps.Clone(descriptor.Targets.CueCategories)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("read the audio project: %w", err)
	}
	session.categories = maps.Clone(session.saved)
	options.SchemaVersion = authoring.LibrarySchemaFour
	if err := session.ensureLibrary(create); err != nil {
		return nil, err
	}
	library, err := authoring.OpenLibraryProject(files.library, options)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open the audio library: %w", err), session.discard())
	}
	session.library = library

	return session, nil
}

// ensureLibrary makes an empty library when the project has none and the
// job may start one.
func (s *audioSession) ensureLibrary(create bool) error {
	_, err := os.Stat(s.files.library)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("find the audio library: %w", err)
	case !create:
		return errNoLibrary
	}
	if err := os.MkdirAll(filepath.Dir(s.files.library), audioDirPerm); err != nil {
		return fmt.Errorf("make the audio folder: %w", err)
	}
	if err := authoring.CreateLibrary(s.files.library); err != nil {
		return fmt.Errorf("start the audio library: %w", err)
	}
	s.createdLibrary = true

	return nil
}

// save writes the categories that changed into the descriptor, then the
// library. If the library cannot be saved, the descriptor is put back, so the
// job changes both files or neither.
func (s *audioSession) save() error {
	before := maps.Clone(s.saved)
	if err := s.writeCategories(before, s.categories); err != nil {
		return err
	}
	err := s.library.SaveChanged()
	if err == nil || errors.Is(err, authoring.ErrAudioPublicationSync) {
		return nil
	}
	restore := s.writeCategories(s.categories, before)

	return errors.Join(fmt.Errorf("save the audio library: %w", err), restore)
}

// writeCategories changes the descriptor's categories from one set to
// another, making a descriptor first when the project has none.
func (s *audioSession) writeCategories(from, target map[string]string) error {
	var changed []string
	for _, cue := range slices.Sorted(maps.Keys(mapsUnion(from, target))) {
		if from[cue] != target[cue] {
			changed = append(changed, cue)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	if s.descriptor == nil {
		if err := s.createDescriptor(); err != nil {
			return err
		}
	}
	for _, cue := range changed {
		data, err := authoring.SetDescriptorCueCategory(s.files.descriptor, s.descriptor, cue, target[cue])
		if err != nil {
			return fmt.Errorf("save the category of %s: %w", cue, err)
		}
		s.descriptor = data
	}

	return nil
}

// createDescriptor makes a descriptor naming the library, to hold categories.
func (s *audioSession) createDescriptor() error {
	library, err := filepath.Rel(filepath.Dir(s.files.descriptor), s.files.library)
	if err != nil {
		return fmt.Errorf("name the audio library: %w", err)
	}
	if err := authoring.CreateProjectDescriptor(s.files.descriptor, filepath.ToSlash(library)); err != nil {
		return fmt.Errorf("start the audio project: %w", err)
	}
	s.createdDescriptor = true
	data, err := os.ReadFile(s.files.descriptor)
	if err != nil {
		return fmt.Errorf("read the audio project: %w", err)
	}
	s.descriptor = data

	return nil
}

// discard removes the files a failed job made.
func (s *audioSession) discard() error {
	return errors.Join(removeCreated(s.files.library, s.createdLibrary),
		removeCreated(s.files.descriptor, s.createdDescriptor))
}

func mapsUnion(first, second map[string]string) map[string]string {
	union := maps.Clone(first)
	if union == nil {
		union = map[string]string{}
	}
	maps.Copy(union, second)

	return union
}

// Kinds of entry an undo step can change, as its keys name them: "cue:" and
// a cue's ID, and so on.
const (
	entryRecording = "recording"
	entryCue       = "cue"
	entryFamily    = "family"
	entryProfileOf = "profile-of"
	entryProfile   = "profile"
	entryAmbient   = "ambient"
	entryCategory  = "category"
)

// entries lists every value an audio edit can change, encoded, by key: each
// recording and cue, the family of each recording and the profile of each
// cue, each profile, each cue's ambient settings and its category.
func (s *audioSession) entries() map[string]string {
	document := s.library.Document()
	entries := map[string]string{}
	add := func(kind, name string, value any) { entries[kind+":"+name] = encodeValue(value) }
	for id, recording := range document.Recordings {
		add(entryRecording, id, recording)
	}
	for id, cue := range document.Cues {
		add(entryCue, id, cue)
	}
	for family, members := range document.Taxonomy.RecordingFamilies {
		for _, id := range members {
			add(entryFamily, id, family)
		}
	}
	for profile, members := range document.Taxonomy.CueProfiles {
		for _, id := range members {
			add(entryProfileOf, id, profile)
		}
	}
	for name, profile := range document.Taxonomy.Profiles {
		add(entryProfile, name, profile)
	}
	for cue, ambient := range document.Routing.Ambient {
		add(entryAmbient, cue, ambient)
	}
	for cue, category := range s.categories {
		add(entryCategory, cue, category)
	}

	return entries
}

// audioPatch sets entries to encoded values, or removes them where nil.
type audioPatch map[string]*string

// audioChange is what an edit changed: the patches that redo and undo it, and
// the keys it touched.
type audioChange struct {
	forward json.RawMessage
	back    json.RawMessage
	touched []string
}

func noChange() audioChange { return audioChange{forward: nil, back: nil, touched: nil} }

// diffEntries compares the entries before and after an edit.
func diffEntries(before, after map[string]string) audioChange {
	forward, back := audioPatch{}, audioPatch{}
	var touched []string
	keys := slices.Sorted(maps.Keys(mapsUnion(before, after)))
	for _, key := range keys {
		was, had := before[key]
		now, has := after[key]
		if had == has && was == now {
			continue
		}
		forward[key], back[key] = presentValue(now, has), presentValue(was, had)
		touched = append(touched, key)
	}

	return audioChange{
		forward: json.RawMessage(encodeValue(forward)), back: json.RawMessage(encodeValue(back)), touched: touched,
	}
}

func presentValue(value string, present bool) *string {
	if !present {
		return nil
	}

	return &value
}

// replay undoes or redoes an audio step: each action's values must still be
// those it left, and are then set back, or again.
func (s *audioSession) replay(applied step, undo bool) error {
	actions := slices.Clone(applied.actions)
	if undo {
		slices.Reverse(actions)
	}
	for _, reapplied := range actions {
		var forward, back audioPatch
		err := errors.Join(json.Unmarshal(reapplied.forward, &forward), json.Unmarshal(reapplied.back, &back))
		if err != nil {
			return fmt.Errorf("read the undo step: %w", err)
		}
		expected, target := forward, back
		if !undo {
			expected, target = back, forward
		}
		current := s.entries()
		for key, value := range expected {
			now, present := current[key]
			if (value == nil) == present || value != nil && *value != now {
				return errAudioChanged
			}
		}
		if err := s.applyPatch(target); err != nil {
			return err
		}
	}

	return nil
}

// applyPatch sets the patch's entries as one edit of the library and the
// categories.
func (s *audioSession) applyPatch(patch audioPatch) error {
	document := s.library.Document()
	if document.Routing.Ambient == nil {
		document.Routing.Ambient = map[string]authoring.AmbientProfile{}
	}
	for _, key := range slices.Sorted(maps.Keys(patch)) {
		if err := s.setEntry(&document, key, patch[key]); err != nil {
			return err
		}
	}
	if err := s.library.ApplyDocument(s.library.Revision(), document); err != nil {
		return fmt.Errorf("undo the sound change: %w", err)
	}

	return nil
}

// setEntry sets one entry of a document, or of the categories.
func (s *audioSession) setEntry(document *authoring.LibraryDocument, key string, value *string) error {
	kind, name, _ := strings.Cut(key, ":")
	switch kind {
	case entryRecording:
		return setValue(document.Recordings, name, value)
	case entryCue:
		return setValue(document.Cues, name, value)
	case entryFamily:
		return setMember(document.Taxonomy.RecordingFamilies, name, value)
	case entryProfileOf:
		return setMember(document.Taxonomy.CueProfiles, name, value)
	case entryProfile:
		return setValue(document.Taxonomy.Profiles, name, value)
	case entryAmbient:
		return setValue(document.Routing.Ambient, name, value)
	case entryCategory:
		return setValue(s.categories, name, value)
	}

	return fmt.Errorf("%w: undo step entry %q", ErrUnknownEdit, key)
}

func setValue[T any](values map[string]T, name string, value *string) error {
	if value == nil {
		delete(values, name)

		return nil
	}
	var decoded T
	if err := json.Unmarshal([]byte(*value), &decoded); err != nil {
		return fmt.Errorf("read the undo step: %w", err)
	}
	values[name] = decoded

	return nil
}

// setMember moves an identity into the group a value names, or out of every
// group where nil.
func setMember(groups map[string][]string, identity string, value *string) error {
	for group, members := range groups {
		groups[group] = slices.DeleteFunc(members, func(member string) bool { return member == identity })
		if len(groups[group]) == 0 {
			delete(groups, group)
		}
	}
	if value == nil {
		return nil
	}
	var group string
	if err := json.Unmarshal([]byte(*value), &group); err != nil {
		return fmt.Errorf("read the undo step: %w", err)
	}
	groups[group] = append(groups[group], identity)
	slices.Sort(groups[group])

	return nil
}
