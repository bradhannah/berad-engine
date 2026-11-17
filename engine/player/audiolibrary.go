package player

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"

	audio "github.com/bradhannah/berad-sdk/modules/berad-audio"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/backend/ebitengine"

	"github.com/bradhannah/berad-engine/engine/clock"
)

// A project's audio keeps berad-audio's own formats, in its audio folder:
// project.json, when there is one, names the library and the cues'
// categories; without it, library.json is read alone, beside the sound
// files it names.
const (
	audioDir        = "audio"
	audioDescriptor = "audio/project.json"
	audioLibrary    = "audio/library.json"
	// Decoded sound is 16-bit stereo: four bytes a frame.
	bytesPerFrame = 4
	millisecond   = 1000
)

// ErrAudio reports a project's audio that a game cannot read.
var ErrAudio = errors.New("the project's audio cannot be read")

// CueSound is how a cue sounds in a game: the gain it plays at, its
// category, and for an ambient cue how many cells away it is heard, zero
// for any other.
type CueSound struct {
	Gain     float64
	Category string
	Radius   float64
}

// AudioLibrary is a project's sounds as a game plays them, read from its
// audio folder: each cue's recordings, gain, category and distance, as the
// editor's Audio window sets them. A project with no audio folder has a
// library with no cues.
type AudioLibrary struct {
	// files is the folder the recordings' paths start from.
	files      fs.FS
	definition audio.Definition
	cues       map[string]CueSound
	// categoryDB is the adjustment, in dB, each category's cues apply on top
	// of a step of package loudness.
	categoryDB map[string]float64
	// lengths keeps each recording's length, in milliseconds, once measured.
	lengths map[audio.RecordingID]int64
}

// ReadAudio reads a project folder's audio library. A project with none
// reads as a library with no cues; an error is audio that is there and
// cannot be read.
func ReadAudio(project fs.FS) (*AudioLibrary, error) {
	library := &AudioLibrary{
		files:      nil,
		definition: audio.Definition{Recordings: map[audio.RecordingID]audio.Recording{}, Cues: map[audio.CueID]audio.Cue{}},
		cues:       map[string]CueSound{}, categoryDB: map[string]float64{}, lengths: map[audio.RecordingID]int64{},
	}
	libraryPath, assetRoot := audioLibrary, audioDir
	var descriptor authoring.ProjectDescriptor
	described, err := fs.ReadFile(project, audioDescriptor)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("%w: %w", ErrAudio, err)
	default:
		if descriptor, err = authoring.ParseProjectDescriptor(described); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrAudio, err)
		}
		libraryPath = path.Join(audioDir, descriptor.Library)
		assetRoot = path.Join(path.Dir(libraryPath), descriptor.AssetRoot)
		if !fs.ValidPath(libraryPath) || !fs.ValidPath(assetRoot) {
			return nil, fmt.Errorf("%w: the audio project points outside the project folder", ErrAudio)
		}
	}
	data, err := fs.ReadFile(project, libraryPath)
	if errors.Is(err, fs.ErrNotExist) {
		return library, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAudio, err)
	}
	document, err := authoring.ParseDocument(data, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAudio, err)
	}
	if library.definition, err = authoring.PlaybackDefinition(document); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAudio, err)
	}
	if library.files, err = fs.Sub(project, assetRoot); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAudio, err)
	}
	for cueID := range document.Cues {
		sound := CueSound{
			Gain:     library.definition.Cues[audio.CueID(cueID)].Gain,
			Category: authoring.CueCategory(descriptor.Targets, document.Routing, cueID), Radius: 0,
		}
		if ambient, found := document.Routing.Ambient[cueID]; found {
			sound.Radius = ambient.RadiusTiles
		}
		library.cues[cueID] = sound
	}
	maps.Copy(library.categoryDB, descriptor.CategoryGainsDB)

	return library, nil
}

// Cues lists the library's cues, in order.
func (l *AudioLibrary) Cues() []string { return slices.Sorted(maps.Keys(l.cues)) }

// Cue is how a cue sounds, if the library has it.
func (l *AudioLibrary) Cue(cue string) (CueSound, bool) {
	sound, found := l.cues[cue]

	return sound, found
}

// Radius is how many cells away an ambient cue is heard, zero for any
// other.
func (l *AudioLibrary) Radius(cue string) float64 { return l.cues[cue].Radius }

// CategoryDB is the adjustment, in dB, a category's cues apply on top of a
// step of package loudness: none is zero.
func (l *AudioLibrary) CategoryDB(category string) float64 { return l.categoryDB[category] }

// CueTicks is how many ticks of play a cue lasts: its longest recording's
// played part. A cue that loops, music or ambient, or whose lengths are not
// known, lasts none, so a Play sound waiting for it carries on at once.
func (l *AudioLibrary) CueTicks(cue string) int {
	if l.cues[cue].Category != authoring.CategorySFX {
		return 0
	}
	longest := int64(0)
	for _, recording := range l.definition.Cues[audio.CueID(cue)].Recordings {
		longest = max(longest, l.lengthOf(recording))
	}

	return int((longest*clock.TicksPerSecond + millisecond - 1) / millisecond)
}

// lengthOf is how long the part of a recording that plays lasts, in
// milliseconds: its region, or the whole file, decoded to find out; zero
// when it has no file or cannot be decoded.
func (l *AudioLibrary) lengthOf(recordingID audio.RecordingID) int64 {
	recording := l.definition.Recordings[recordingID]
	if window := recording.Window; window != nil && window.End > window.Start {
		return window.End.Milliseconds() - window.Start.Milliseconds()
	}
	if known, found := l.lengths[recordingID]; found {
		return known
	}
	measured := l.decodeLength(recording.File)
	l.lengths[recordingID] = measured

	return measured
}

// decodeLength decodes a file's header far enough to know its length.
func (l *AudioLibrary) decodeLength(file string) int64 {
	if file == "" {
		return 0
	}
	data, err := fs.ReadFile(l.files, file)
	if err != nil {
		return 0
	}
	stream, err := ebitengine.Decode(data, file, authoring.EditorSampleRate)
	if err != nil {
		return 0
	}
	if closer, ok := stream.(io.Closer); ok {
		defer closer.Close() //nolint:errcheck // Nothing more is read from it.
	}

	return stream.Length() * millisecond / (bytesPerFrame * authoring.EditorSampleRate)
}

// playing makes a berad-audio library that plays some of the cues, with
// room for exactly their files.
func (l *AudioLibrary) playing(cues []string) (*audio.Library, error) {
	definition := audio.Definition{Recordings: map[audio.RecordingID]audio.Recording{}, Cues: map[audio.CueID]audio.Cue{}}
	total := int64(1)
	for _, cue := range cues {
		played, found := l.definition.Cues[audio.CueID(cue)]
		if !found {
			return nil, fmt.Errorf("%w: the project has no cue %q", ErrAudio, cue)
		}
		definition.Cues[audio.CueID(cue)] = played
		for _, recordingID := range played.Recordings {
			if _, counted := definition.Recordings[recordingID]; counted {
				continue
			}
			recording := l.definition.Recordings[recordingID]
			definition.Recordings[recordingID] = recording
			if recording.File == "" {
				continue
			}
			info, err := fs.Stat(l.files, recording.File)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrAudio, err)
			}
			total += info.Size()
		}
	}
	library, err := audio.NewLibrary(definition, l.files, total)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAudio, err)
	}

	return library, nil
}
