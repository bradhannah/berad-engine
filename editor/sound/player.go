// Package sound plays the project's cues in an editor window. It asks the
// orchestrator for the audio library and for the recordings a cue needs,
// since windows never read project files (D1, D8), and plays them through a
// berad-audio runtime on the window's own audio device.
package sound

import (
	"bytes"
	"errors"
	"io/fs"
	"maps"
	"slices"
	"time"

	audio "github.com/bradhannah/berad-sdk/modules/berad-audio"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/backend/ebitengine"
	ea "github.com/hajimehoshi/ebiten/v2/audio"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// sampleRate is the rate a window's audio device opens at.
const sampleRate = 48000

// Requests are what a Player asks the orchestrator: the audio library, and a
// recording's bytes, a part at a time from an offset. Answers arrive on the
// window's goroutine.
type Requests interface {
	AudioLibrary(done func(protocol.AudioLibrary, error))
	Recording(recording string, offset int64, done func(protocol.Recording, error))
}

// Player plays the project's cues in one window. It keeps recordings' bytes
// by version, and plays them through a berad-audio runtime built from what
// it holds. The device opens only when something first plays, so a window
// that never plays never opens one. Every call runs on the window's
// goroutine.
type Player struct {
	requests Requests
	report   func(string)
	device   func() (audio.Backend, error)
	decode   audio.Decoder
	backend  audio.Backend

	// listing is the library's cues as last listed; listed says one has
	// arrived, asking that a question is out, and again that another is
	// wanted once it is answered.
	listing map[string]protocol.AudioCue
	listed  bool
	asking  bool
	again   bool
	// held keeps recordings' bytes by recording ID, and fetching marks
	// those on their way.
	held     map[string]heldRecording
	fetching map[string]bool
	// wanted are the cues to keep ready; queued are waiting to play.
	wanted map[string]bool
	queued []string
	// told is what has been reported, so each problem is said once.
	told map[string]bool
	// current plays; retiring are runtimes replaced while their sounds
	// still play, closed once those finish.
	current  *player
	retiring []*player
	// audition is the part of a recording the Audio window plays while its
	// region is a draft, and loops whether it plays again and again.
	audition *protocol.AudioCue
	loops    bool
	// pending is the audition last asked for, kept until its recording is
	// listed.
	pending *audition
}

// auditionCue is the cue a Player makes to play an audition. Its slashes are
// never in a cue ID.
const auditionCue = "//audition"

// audition is part of a recording to play: Window, nil for all of it, at a
// calibration gain in dB, again and again when Loop is set.
type audition struct {
	Recording     string
	Window        *protocol.AudioWindow
	CalibrationDB float64
	Loop          bool
}

// heldRecording is a recording's bytes at a version, complete when it holds
// size bytes.
type heldRecording struct {
	version string
	size    int64
	data    []byte
}

func (h heldRecording) complete(version string) bool {
	return h.version == version && int64(len(h.data)) == h.size
}

// player is a runtime, the cues it can play and the voices it started.
type player struct {
	runtime *audio.Runtime
	cues    map[string]bool
	voices  []audio.Voice
}

// New returns a player that asks requests for what it plays and reports
// what cannot play to report, once each.
func New(requests Requests, report func(string)) *Player {
	return &Player{
		requests: requests, report: report, device: openDevice, decode: ebitengine.Decode, backend: nil,
		listing: map[string]protocol.AudioCue{}, listed: false, asking: false, again: false,
		held: map[string]heldRecording{}, fetching: map[string]bool{}, wanted: map[string]bool{}, queued: nil,
		told: map[string]bool{}, current: nil, retiring: nil, audition: nil, loops: false, pending: nil,
	}
}

// Audition plays part of a recording, nil for all of it, at a calibration
// gain in dB, again and again when loop is set, replacing whatever played
// before, once the recording has arrived.
func (s *Player) Audition(recording string, window *protocol.AudioWindow, calibrationDB float64, loop bool) {
	asked := audition{Recording: recording, Window: window, CalibrationDB: calibrationDB, Loop: loop}
	s.Stop()
	if s.current != nil && s.current.cues[auditionCue] {
		s.retiring, s.current = append(s.retiring, s.current), nil
	}
	s.audition, s.loops = nil, asked.Loop
	if listed, found := s.listing[recordingCue(asked.Recording)]; found {
		s.audition = auditionOf(listed.Recordings[0], asked)
		s.listing[auditionCue] = *s.audition
	}
	s.pending = &asked
	s.Play(auditionCue)
}

// auditionOf is the cue that plays an audition of a listed recording.
func auditionOf(recording protocol.AudioRecording, asked audition) *protocol.AudioCue {
	recording.CalibrationDB, recording.StartMs, recording.EndMs = asked.CalibrationDB, 0, 0
	if asked.Window != nil {
		recording.StartMs, recording.EndMs = asked.Window.StartMs, asked.Window.EndMs
	}

	return &protocol.AudioCue{
		ID: auditionCue, Name: recording.ID, Category: "", CategorySet: false, Gain: 1, HowToHear: "",
		SearchTerms: nil, Recordings: []protocol.AudioRecording{recording},
	}
}

// AuditionAt reports where the audition is, in its recording's milliseconds,
// while it plays.
func (s *Player) AuditionAt() (int64, bool) {
	if s.audition == nil {
		return 0, false
	}
	start := s.audition.Recordings[0].StartMs
	for _, playing := range s.players() {
		for _, voice := range playing.voices {
			if status := voice.Status(); string(status.Cue) == auditionCue && sounding(status.State) {
				return start + status.Position.Milliseconds(), true
			}
		}
	}

	return 0, false
}

// openDevice borrows the process's audio context, opening it the first time.
//
//nolint:ireturn // The runtime takes any backend; this is the native one.
func openDevice() (audio.Backend, error) {
	context := ea.CurrentContext()
	if context == nil {
		context = ea.NewContext(sampleRate)
	}

	return ebitengine.Backend{Context: context}, nil
}

// recordingCue is the cue a Player makes to play one recording on its own,
// at its calibration and unit gain. Its leading slash is never in a cue ID.
func recordingCue(recording string) string { return "/" + recording }

// PlayRecording plays one recording on its own, once it has arrived, as the
// Audio window auditions one of a cue's recordings, or one no cue plays.
func (s *Player) PlayRecording(recording string) { s.Play(recordingCue(recording)) }

// PlayingRecording reports whether a recording played on its own is playing
// or waiting to.
func (s *Player) PlayingRecording(recording string) bool { return s.Playing(recordingCue(recording)) }

// Changed lists the library again after it changed, so what plays next plays
// the library as it is now.
func (s *Player) Changed() {
	if s.listed || len(s.wanted) > 0 {
		s.refresh()
	}
}

// Prepare fetches the library again and the recordings cues need.
func (s *Player) Prepare(cues []string) {
	for _, cue := range cues {
		s.wanted[cue] = true
	}
	s.refresh()
}

// Play plays a cue once its recordings have arrived. A cue already held
// plays at once, without asking the orchestrator again; one the library did
// not have when last listed is looked for in a fresh listing first.
func (s *Player) Play(cue string) {
	s.wanted[cue] = true
	s.queued = append(s.queued, cue)
	if _, known := s.listing[cue]; !s.listed || !known {
		s.refresh()
	}
	s.playQueued()
}

// Stop stops every sound this window plays, and forgets what waits to play.
func (s *Player) Stop() {
	s.queued = nil
	for _, playing := range s.players() {
		for _, voice := range playing.voices {
			_ = voice.Stop(0) //nolint:errcheck // A zero fade cannot be refused.
		}
	}
}

// Playing reports whether a cue is playing or waiting to.
func (s *Player) Playing(cue string) bool {
	if slices.Contains(s.queued, cue) {
		return true
	}
	for _, playing := range s.players() {
		for _, voice := range playing.voices {
			if status := voice.Status(); string(status.Cue) == cue && sounding(status.State) {
				return true
			}
		}
	}

	return false
}

func sounding(state audio.VoiceState) bool {
	return state == audio.VoicePlaying || state == audio.VoiceReleasing || state == audio.VoicePaused
}

func (s *Player) players() []*player {
	all := slices.Clone(s.retiring)
	if s.current != nil {
		all = append(all, s.current)
	}

	return all
}

// Update services the runtimes, closes replaced ones once their sounds have
// finished, and plays what has become ready. The window calls it every frame.
func (s *Player) Update() {
	for _, playing := range s.players() {
		if err := playing.runtime.Update(); err != nil {
			s.tell(err.Error(), "Sound: "+err.Error())
		}
	}
	for _, playing := range s.players() {
		playing.voices = slices.DeleteFunc(playing.voices, func(voice audio.Voice) bool {
			return !sounding(voice.Status().State)
		})
	}
	s.retiring = slices.DeleteFunc(s.retiring, func(playing *player) bool {
		if len(playing.voices) > 0 {
			return false
		}
		_ = playing.runtime.Close() //nolint:errcheck // Its failures were reported by Update.

		return true
	})
	s.playQueued()
}

// Close stops every runtime when the window closes.
func (s *Player) Close() error {
	players := s.players()
	errs := make([]error, 0, len(players))
	for _, playing := range players {
		errs = append(errs, playing.runtime.Close())
	}
	s.current, s.retiring = nil, nil

	return errors.Join(errs...)
}

// refresh asks for the library, or asks again once the question out now is
// answered.
func (s *Player) refresh() {
	if s.asking {
		s.again = true

		return
	}
	s.asking = true
	s.requests.AudioLibrary(func(library protocol.AudioLibrary, err error) {
		s.asking = false
		switch {
		case err != nil:
			s.tell("library:"+err.Error(), "The audio library could not be listed: "+err.Error())
		case library.Problem != "":
			s.listed, s.listing = true, map[string]protocol.AudioCue{}
			if len(s.wanted) > 0 {
				s.tell("library:"+library.Problem, library.Problem)
			}
		default:
			s.listed, s.listing = true, map[string]protocol.AudioCue{}
			s.list(library)
		}
		s.fetchWanted()
		if s.again {
			s.again = false
			s.refresh()
		}
		s.playQueued()
	})
}

// list keeps a listing's cues, a cue for each recording on its own, and the
// audition, which a listing may now be able to play.
func (s *Player) list(library protocol.AudioLibrary) {
	defer s.listAudition()
	alone := func(recording protocol.AudioRecording) {
		s.listing[recordingCue(recording.ID)] = protocol.AudioCue{
			ID: recordingCue(recording.ID), Name: recording.ID, Category: "", CategorySet: false, Gain: 1,
			HowToHear: "", SearchTerms: nil, Recordings: []protocol.AudioRecording{recording},
		}
	}
	for _, cue := range library.Cues {
		s.listing[cue.ID] = cue
		for _, recording := range cue.Recordings {
			alone(recording)
		}
	}
	for _, recording := range library.Unused {
		alone(recording)
	}
}

// listAudition lists the audition waiting for its recording, or keeps the
// one listed.
func (s *Player) listAudition() {
	if s.pending != nil {
		if listed, found := s.listing[recordingCue(s.pending.Recording)]; found {
			s.audition = auditionOf(listed.Recordings[0], *s.pending)
		}
	}
	if s.audition != nil {
		s.listing[auditionCue] = *s.audition
	}
}

// fetchWanted asks for every wanted cue's recordings not held at their
// listed version.
func (s *Player) fetchWanted() {
	for cue := range s.wanted {
		for _, recording := range s.listing[cue].Recordings {
			if recording.Status == protocol.RecordingReady && !s.held[recording.ID].complete(recording.Version) &&
				!s.fetching[recording.ID] {
				s.fetch(recording.ID, 0)
			}
		}
	}
}

// fetch asks for a recording's bytes from an offset, then for the rest,
// starting again if the file changes on the way.
func (s *Player) fetch(recording string, offset int64) {
	s.fetching[recording] = true
	s.requests.Recording(recording, offset, func(part protocol.Recording, err error) {
		if err != nil {
			s.fetching[recording] = false
			s.tell("recording:"+recording, "The recording “"+recording+"” could not be fetched: "+err.Error())

			return
		}
		held := s.held[recording]
		switch {
		case part.Size < 0 || part.Size > protocol.MaxRecording:
			s.fetching[recording] = false
			s.tell("recording:"+recording, "The recording “"+recording+"” is too large to play here")

			return
		case offset == 0:
			held = heldRecording{version: part.Version, size: part.Size, data: make([]byte, 0, part.Size)}
		case part.Version != held.version || offset != int64(len(held.data)):
			// The file changed on the way: start again.
			s.fetch(recording, 0)

			return
		}
		held.data = append(held.data, part.Data...)
		s.held[recording] = held
		if done := int64(len(held.data)); done < held.size && len(part.Data) > 0 {
			s.fetch(recording, done)

			return
		}
		s.fetching[recording] = false
		s.playQueued()
	})
}

// readiness says whether a cue can play now.
type readiness int

const (
	cueWaiting readiness = iota
	cueReady
	cueSilent
	cueBroken
)

// readinessOf says whether a cue can play now, is waiting for the library
// or its recordings, plays nothing on purpose, or cannot play, and why.
func (s *Player) readinessOf(cue string) (readiness, string) {
	if !s.listed {
		return cueWaiting, ""
	}
	listed, found := s.listing[cue]
	switch {
	case !found && s.asking:
		return cueWaiting, ""
	case !found:
		return cueBroken, "Can’t play “" + cue + "”: it is not in the audio library"
	}
	state := cueSilent
	for _, recording := range listed.Recordings {
		switch recording.Status {
		case protocol.RecordingNoFile:
		case protocol.RecordingReady:
			if !s.held[recording.ID].complete(recording.Version) {
				return cueWaiting, ""
			}
			state = cueReady
		default:
			return cueBroken, "Can’t play “" + listed.Name + "”: its recording “" + recording.ID + "” is " +
				recording.Status
		}
	}

	return state, ""
}

// playQueued plays each queued cue that is ready, drops those that cannot
// play, and keeps those still waiting.
func (s *Player) playQueued() {
	waiting := s.queued[:0]
	for _, cue := range s.queued {
		state, problem := s.readinessOf(cue)
		switch state {
		case cueWaiting:
			waiting = append(waiting, cue)
			if s.listed && !s.asking {
				s.fetchWanted()
			}
		case cueBroken:
			delete(s.wanted, cue)
			s.tell("cue:"+cue, problem)
		case cueReady:
			s.start(cue)
		case cueSilent:
		}
	}
	s.queued = waiting
}

// start plays a ready cue, building a runtime that can play it first.
func (s *Player) start(cue string) {
	if s.current == nil || !s.current.cues[cue] || s.stale(s.current) {
		if !s.rebuild() {
			return
		}
	}
	options := audio.DefaultPlayOptions()
	options.Loop = cue == auditionCue && s.loops
	result, err := s.current.runtime.Play(audio.CueID(cue), options)
	if err != nil {
		s.tell("play:"+cue, "Can’t play “"+s.listing[cue].Name+"”: "+err.Error())

		return
	}
	if result.Outcome == audio.Admitted {
		s.current.voices = append(s.current.voices, result.Voice)
	}
}

// stale reports whether a player was built from recordings that have
// changed since.
func (s *Player) stale(playing *player) bool {
	for cue := range playing.cues {
		if state, _ := s.readinessOf(cue); state != cueReady {
			return true
		}
	}

	return false
}

// rebuild makes a runtime for every wanted cue that is ready, retiring the
// one playing now so its sounds finish. A cue the runtime refuses is
// reported and no longer wanted, and the rest are built without it.
func (s *Player) rebuild() bool {
	if s.backend == nil {
		backend, err := s.device()
		if err != nil {
			s.tell("device", "Sound is off: the audio device could not be opened: "+err.Error())

			return false
		}
		s.backend = backend
	}
	var ready []string
	for _, cue := range slices.Sorted(maps.Keys(s.wanted)) {
		if state, _ := s.readinessOf(cue); state == cueReady {
			ready = append(ready, cue)
		}
	}
	runtime, err := s.build(ready)
	if err != nil {
		ready = slices.DeleteFunc(ready, func(cue string) bool {
			if _, alone := s.build([]string{cue}); alone != nil {
				delete(s.wanted, cue)
				s.tell("cue:"+cue, "Can’t play “"+s.listing[cue].Name+"”: "+alone.Error())

				return true
			}

			return false
		})
		if runtime, err = s.build(ready); err != nil {
			s.tell("build:"+err.Error(), "Sound could not be prepared: "+err.Error())

			return false
		}
	}
	if s.current != nil {
		s.retiring = append(s.retiring, s.current)
	}
	cues := map[string]bool{}
	for _, cue := range ready {
		cues[cue] = true
	}
	s.current = &player{runtime: runtime, cues: cues, voices: nil}

	return true
}

// build makes a runtime that plays cues, each ready.
func (s *Player) build(cues []string) (*audio.Runtime, error) {
	definition := audio.Definition{Recordings: map[audio.RecordingID]audio.Recording{}, Cues: map[audio.CueID]audio.Cue{}}
	files, total := map[string][]byte{}, int64(1)
	for _, cue := range cues {
		listed := s.listing[cue]
		played := audio.Cue{Recordings: nil, Gain: listed.Gain}
		for _, recording := range listed.Recordings {
			played.Recordings = append(played.Recordings, audio.RecordingID(recording.ID))
			definition.Recordings[audio.RecordingID(recording.ID)] = s.recordingOf(recording, files, &total)
		}
		definition.Cues[audio.CueID(cue)] = played
	}
	library, err := audio.NewLibrary(definition, memoryFiles(files), total)
	if err != nil {
		return nil, err //nolint:wrapcheck // Reported to the author as it is.
	}

	return audio.NewRuntime(library, audio.DefaultOptions(s.backend, s.decode)) //nolint:wrapcheck // As above.
}

// recordingOf is a recording as the runtime plays it, its bytes served under
// its ID and extension; a silent slot has no file.
func (s *Player) recordingOf(recording protocol.AudioRecording, files map[string][]byte, total *int64) audio.Recording {
	played := audio.Recording{File: "", CalibrationDB: recording.CalibrationDB, Window: nil}
	if recording.EndMs > recording.StartMs {
		played.Window = &audio.Window{
			Start: time.Duration(recording.StartMs) * time.Millisecond, End: time.Duration(recording.EndMs) * time.Millisecond,
		}
	}
	if recording.Status == protocol.RecordingReady {
		played.File = recording.ID + recording.Extension
		files[played.File] = s.held[recording.ID].data
		*total += int64(len(s.held[recording.ID].data))
	}

	return played
}

// tell reports a problem once.
func (s *Player) tell(key, message string) {
	if s.told[key] {
		return
	}
	s.told[key] = true
	s.report(message)
}

// memoryFiles serves recordings' bytes to the runtime by file name.
type memoryFiles map[string][]byte

func (m memoryFiles) Open(name string) (fs.File, error) {
	data, found := m[name]
	if !found {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	return &memoryFile{Reader: bytes.NewReader(data), name: name, size: int64(len(data))}, nil
}

// memoryFile is one recording's bytes, read once.
type memoryFile struct {
	*bytes.Reader

	name string
	size int64
}

func (f *memoryFile) Stat() (fs.FileInfo, error) { return memoryInfo{name: f.name, size: f.size}, nil }

func (f *memoryFile) Close() error { return nil }

// memoryInfo describes a memoryFile.
type memoryInfo struct {
	name string
	size int64
}

func (i memoryInfo) Name() string       { return i.name }
func (i memoryInfo) Size() int64        { return i.size }
func (i memoryInfo) Mode() fs.FileMode  { return readOnly }
func (i memoryInfo) ModTime() time.Time { return time.Time{} }
func (i memoryInfo) IsDir() bool        { return false }
func (i memoryInfo) Sys() any           { return nil }

const readOnly fs.FileMode = 0o444
