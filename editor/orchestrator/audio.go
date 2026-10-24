package orchestrator

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	beradaudio "github.com/bradhannah/berad-sdk/modules/berad-audio"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// The project's audio keeps berad-audio's own formats rather than the
// document envelope (D11), so the terminal audio editor can open it too:
// audio/project.json, a version-1 descriptor, names the library and the
// cues' categories; without it, audio/library.json is read alone. Only
// berad-audio's authoring package reads them, and the editor never writes
// them.
const (
	audioDir        = "audio"
	audioDescriptor = "audio/project.json"
	audioLibrary    = "audio/library.json"
)

// Problems with the project's audio, as windows show them.
const (
	problemNoAudio = "This project has no audio library yet. Add audio/library.json, for example with " +
		"the terminal audio editor."
	problemAudioOutside  = "The audio project points outside the project folder, so the editor cannot read it."
	problemAudioReadFail = "The audio library could not be read: "
)

// errNotPlayable reports a recording a window cannot be sent.
var errNotPlayable = errors.New("this recording cannot be sent")

// projectAudio answers windows' questions about the project's audio. It
// reads the files again for every question, so changes made in the terminal
// audio editor show up at once.
type projectAudio struct {
	root *os.Root
	dir  string
}

// loadedAudio is the audio library as last read: its cues for windows, and
// where each recording's file is, relative to the project.
type loadedAudio struct {
	library protocol.AudioLibrary
	files   map[string]string
}

func (a *projectAudio) load() loadedAudio {
	loaded := loadedAudio{
		library: protocol.AudioLibrary{Problem: "", Cues: []protocol.AudioCue{}}, files: map[string]string{},
	}
	libraryPath, assetRoot, targets, problem := a.locate()
	if problem == "" {
		problem = a.read(libraryPath, assetRoot, targets, &loaded)
	}
	if problem != "" {
		loaded.library = protocol.AudioLibrary{Problem: problem, Cues: nil}
	}

	return loaded
}

// locate finds the library, the folder its recordings' paths start from,
// and the consumer's targets, all relative to the project.
func (a *projectAudio) locate() (string, string, authoring.AuthoringTargets, string) {
	var targets authoring.AuthoringTargets
	if _, err := a.root.Stat(audioDescriptor); errors.Is(err, fs.ErrNotExist) {
		return audioLibrary, audioDir, targets, ""
	}
	descriptorPath := filepath.Join(a.dir, filepath.FromSlash(audioDescriptor))
	descriptor, err := authoring.ReadProjectDescriptor(descriptorPath)
	if err != nil {
		return "", "", targets, problemAudioReadFail + err.Error()
	}
	libraryPath, inside := a.relative(descriptor.LibraryPath(descriptorPath))
	if !inside {
		return "", "", targets, problemAudioOutside
	}
	assetRoot := path.Dir(libraryPath)
	if descriptor.AssetRoot != "" {
		if assetRoot, inside = a.relative(filepath.Join(filepath.Dir(filepath.Join(a.dir, libraryPath)),
			filepath.FromSlash(descriptor.AssetRoot))); !inside {
			return "", "", targets, problemAudioOutside
		}
	}

	return libraryPath, assetRoot, descriptor.Targets, ""
}

// relative returns an absolute path relative to the project, with forward
// slashes, and whether it is inside the project.
func (a *projectAudio) relative(absolute string) (string, bool) {
	inside, err := filepath.Rel(a.dir, absolute)
	if err != nil || !filepath.IsLocal(inside) {
		return "", false
	}

	return filepath.ToSlash(inside), true
}

// read parses the library and lists its cues, ordered by ID, each with its
// category and recordings. It returns a problem when there is nothing to
// play.
func (a *projectAudio) read(
	libraryPath, assetRoot string, targets authoring.AuthoringTargets, loaded *loadedAudio,
) string {
	data, err := a.root.ReadFile(libraryPath)
	if errors.Is(err, fs.ErrNotExist) {
		return problemNoAudio
	}
	if err != nil {
		return problemAudioReadFail + err.Error()
	}
	document, err := authoring.ParseDocument(data, nil)
	if err != nil {
		return problemAudioReadFail + err.Error()
	}
	definition, err := authoring.PlaybackDefinition(document)
	if err != nil {
		return problemAudioReadFail + err.Error()
	}
	for _, id := range slices.Sorted(maps.Keys(document.Cues)) {
		source := document.Cues[id]
		played := definition.Cues[beradaudio.CueID(id)]
		cue := protocol.AudioCue{
			ID: id, Name: source.Name, Category: authoring.CueCategory(targets, document.Routing, id), Gain: played.Gain,
			SearchTerms: source.SearchTerms, Recordings: make([]protocol.AudioRecording, 0, len(played.Recordings)),
		}
		for _, recordingID := range played.Recordings {
			recording := definition.Recordings[recordingID]
			sent := protocol.AudioRecording{
				ID: string(recordingID), Extension: "", Version: "", Size: 0, CalibrationDB: recording.CalibrationDB,
				StartMs: 0, EndMs: 0, Status: protocol.RecordingNoFile,
			}
			if recording.Window != nil {
				sent.StartMs, sent.EndMs = recording.Window.Start.Milliseconds(), recording.Window.End.Milliseconds()
			}
			if recording.File != "" {
				file := path.Join(assetRoot, recording.File)
				loaded.files[sent.ID] = file
				sent.Extension = strings.ToLower(path.Ext(recording.File))
				a.describe(file, &sent)
			}
			cue.Recordings = append(cue.Recordings, sent)
		}
		loaded.library.Cues = append(loaded.library.Cues, cue)
	}

	return ""
}

// describe fills in a recording's status, size and version from its file.
// The version is the file's size and modification time, which change
// whenever the file does.
func (a *projectAudio) describe(file string, sent *protocol.AudioRecording) {
	info, err := a.root.Stat(file)
	switch {
	case err != nil || !info.Mode().IsRegular():
		sent.Status = protocol.RecordingMissing
	case info.Size() > protocol.MaxRecording:
		sent.Status = protocol.RecordingTooLarge
	default:
		sent.Status, sent.Size = protocol.RecordingReady, info.Size()
		sent.Version = strconv.FormatInt(info.Size(), 36) + "-" + strconv.FormatInt(info.ModTime().UnixNano(), 36)
	}
}

// recording reads part of a ready recording's bytes from an offset, at most
// MaxRecordingPart of them.
func (a *projectAudio) recording(asked protocol.Recording) (protocol.Recording, error) {
	loaded := a.load()
	var found *protocol.AudioRecording
	for _, cue := range loaded.library.Cues {
		for index := range cue.Recordings {
			if cue.Recordings[index].ID == asked.ID {
				found = &cue.Recordings[index]
			}
		}
	}
	if found == nil || found.Status != protocol.RecordingReady {
		return protocol.Recording{}, fmt.Errorf("%w: %q is not a ready recording of the audio library", errNotPlayable,
			asked.ID)
	}
	if asked.Offset < 0 || asked.Offset > found.Size {
		return protocol.Recording{}, fmt.Errorf("%w: offset %d is outside its %d bytes", errNotPlayable, asked.Offset,
			found.Size)
	}
	file, err := a.root.Open(loaded.files[asked.ID])
	if err != nil {
		return protocol.Recording{}, fmt.Errorf("open recording: %w", err)
	}
	data := make([]byte, min(found.Size-asked.Offset, protocol.MaxRecordingPart))
	_, readErr := file.ReadAt(data, asked.Offset)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if err := errors.Join(readErr, file.Close()); err != nil {
		return protocol.Recording{}, fmt.Errorf("read recording: %w", err)
	}

	return protocol.Recording{ID: asked.ID, Offset: asked.Offset, Version: found.Version, Size: found.Size, Data: data},
		nil
}

// handleAudio answers a window's audio library or recording question.
func (o *Orchestrator) handleAudio(from *window, message protocol.Message) {
	if message.Kind == protocol.KindAudioLibrary {
		o.sendAudioLibrary(from, message)

		return
	}
	o.sendRecording(from, message)
}

// sendAudioLibrary answers a window's question about the audio library.
func (o *Orchestrator) sendAudioLibrary(from *window, message protocol.Message) {
	o.send(from, mustMessage(protocol.KindAudioLibrary, message.Request, o.audio.load().library))
}

// sendRecording answers a window's question for part of a recording.
func (o *Orchestrator) sendRecording(from *window, message protocol.Message) {
	asked, err := protocol.DecodePayload[protocol.Recording](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	part, err := o.audio.recording(asked)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	o.send(from, mustMessage(protocol.KindRecording, message.Request, part))
}
