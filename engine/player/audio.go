package player

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	audio "github.com/bradhannah/berad-sdk/modules/berad-audio"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/backend/ebitengine"
	ebitenaudio "github.com/hajimehoshi/ebiten/v2/audio"

	"github.com/bradhannah/berad-engine/engine/loudness"
)

// sampleRate is the rate a game's audio device opens at.
const sampleRate = 48000

// Audio plays a project's sounds in a game, straight from its audio folder:
// the cues a game asks for once, and its soundscape's layers, each on a
// berad-audio runtime of its own so its volume changes while it plays. The
// device opens only when something first plays. Every call runs on the
// game's own goroutine.
type Audio struct {
	library *AudioLibrary
	report  func(string)
	backend audio.Backend
	// wanted are the cues to keep ready; current plays them; retiring are
	// runtimes replaced while their sounds still play, closed once those
	// finish.
	wanted   map[string]bool
	current  *cuePlayer
	retiring []*cuePlayer
	layers   map[string]*layerSound
	// told is what has been reported, so each problem is said once.
	told map[string]bool
}

// cuePlayer is a runtime built to play some cues, and the sounds it plays.
type cuePlayer struct {
	runtime *audio.Runtime
	cues    map[string]bool
	voices  []audio.Voice
}

// layerSound is a layer and what plays it.
type layerSound struct {
	layer   Layer
	runtime *audio.Runtime
	voice   audio.Voice
}

// NewAudio plays a library's sounds. report is told, once each, what cannot
// be played and why.
func NewAudio(library *AudioLibrary, report func(string)) *Audio {
	return &Audio{
		library: library, report: report, backend: nil, wanted: map[string]bool{}, current: nil, retiring: nil,
		layers: map[string]*layerSound{}, told: map[string]bool{},
	}
}

// Prepare has cues ready, so they play without waiting later.
func (a *Audio) Prepare(cues []string) {
	for _, cue := range cues {
		a.wanted[cue] = true
	}
	a.rebuild()
}

// Play plays a cue once, alongside whatever else is playing.
func (a *Audio) Play(cue string) { a.PlayAt(cue, "") }

// PlayAt plays a cue once at a step of package loudness in place of its
// own; "" plays it at its own.
func (a *Audio) PlayAt(cue, step string) {
	if a.current == nil || !a.current.cues[cue] {
		a.wanted[cue] = true
		if !a.rebuild() || !a.current.cues[cue] {
			return
		}
	}
	options := audio.DefaultPlayOptions()
	options.Gain = min(a.stepGain(cue, step), audio.MaxGain)
	result, err := a.current.runtime.Play(audio.CueID(cue), options)
	if err != nil {
		a.tell("play:"+cue, "Can’t play “"+cue+"”: "+err.Error())

		return
	}
	if result.Outcome == audio.Admitted {
		a.current.voices = append(a.current.voices, result.Voice)
	}
}

// stepGain is what a cue's own gain is multiplied by to play it at a step
// of package loudness in its category: 1 for its own, or for a step that is
// not known, or a cue that plays at no gain.
func (a *Audio) stepGain(cue, step string) float64 {
	found, known := loudness.Find(step)
	sound, _ := a.library.Cue(cue)
	if !known || sound.Gain <= 0 {
		return 1
	}

	return loudness.CueGain(found.DB, a.library.CategoryDB(sound.Category)) / sound.Gain
}

// Stop stops every sound but the soundscape.
func (a *Audio) Stop() {
	for _, playing := range a.players() {
		for _, voice := range playing.voices {
			_ = voice.Stop(0) //nolint:errcheck // A zero fade cannot be refused.
		}
	}
}

func (a *Audio) players() []*cuePlayer {
	all := slices.Clone(a.retiring)
	if a.current != nil {
		all = append(all, a.current)
	}

	return all
}

// rebuild makes a runtime for every wanted cue that can play, retiring the
// one playing now so its sounds finish. A cue that cannot play is reported
// and no longer wanted, and the rest are built without it.
func (a *Audio) rebuild() bool {
	a.open()
	ready := slices.Sorted(maps.Keys(a.wanted))
	runtime, err := a.build(ready)
	if err != nil {
		ready = slices.DeleteFunc(ready, func(cue string) bool {
			alone, problem := a.build([]string{cue})
			if problem != nil {
				delete(a.wanted, cue)
				a.tell("cue:"+cue, "Can’t play “"+cue+"”: "+problem.Error())

				return true
			}
			_ = alone.Close() //nolint:errcheck // Built only to find out whether the cue plays.

			return false
		})
		if runtime, err = a.build(ready); err != nil {
			a.tell("build:"+err.Error(), "Sound could not be prepared: "+err.Error())

			return false
		}
	}
	if a.current != nil {
		a.retiring = append(a.retiring, a.current)
	}
	cues := map[string]bool{}
	for _, cue := range ready {
		cues[cue] = true
	}
	a.current = &cuePlayer{runtime: runtime, cues: cues, voices: nil}

	return true
}

// open borrows the process's audio device, opening it the first time
// something plays.
func (a *Audio) open() {
	if a.backend != nil {
		return
	}
	context := ebitenaudio.CurrentContext()
	if context == nil {
		context = ebitenaudio.NewContext(sampleRate)
	}
	a.backend = ebitengine.Backend{Context: context}
}

// build makes a runtime that plays cues.
func (a *Audio) build(cues []string) (*audio.Runtime, error) {
	library, err := a.library.playing(cues)
	if err != nil {
		return nil, err
	}

	runtime, err := audio.NewRuntime(library, audio.DefaultOptions(a.backend, ebitengine.Decode))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAudio, err)
	}

	return runtime, nil
}

// PlayLayer plays a layer from its start, again and again, replacing what
// its slot played.
func (a *Audio) PlayLayer(layer Layer) {
	a.StopLayer(layer.Slot)
	a.open()
	runtime, err := a.build([]string{layer.Cue})
	if err != nil {
		a.tell("layer:"+layer.Cue, "Can’t play “"+layer.Cue+"”: "+err.Error())

		return
	}
	playing := &layerSound{layer: layer, runtime: runtime, voice: audio.Voice{}}
	a.setVolume(playing)
	options := audio.DefaultPlayOptions()
	options.Loop = true
	result, err := runtime.Play(audio.CueID(layer.Cue), options)
	if err != nil {
		a.tell("layer:"+layer.Cue, "Can’t play “"+layer.Cue+"”: "+err.Error())
		_ = runtime.Close() //nolint:errcheck // Nothing plays on it.

		return
	}
	playing.voice = result.Voice
	a.layers[layer.Slot] = playing
}

// SetLayer changes a playing layer's gain.
func (a *Audio) SetLayer(layer Layer) {
	playing, found := a.layers[layer.Slot]
	if !found {
		return
	}
	playing.layer.Gain = layer.Gain
	a.setVolume(playing)
}

func (a *Audio) setVolume(playing *layerSound) {
	if err := playing.runtime.SetVolume(min(max(playing.layer.Gain, 0), 1)); err != nil {
		a.tell("layer:"+err.Error(), "Sound: "+err.Error())
	}
}

// StopLayer stops a layer.
func (a *Audio) StopLayer(slot string) {
	playing, found := a.layers[slot]
	if !found {
		return
	}
	delete(a.layers, slot)
	_ = playing.voice.Stop(0) //nolint:errcheck // A zero fade cannot be refused.
	if err := playing.runtime.Close(); err != nil {
		a.tell("layer:"+err.Error(), "Sound: "+err.Error())
	}
}

// Update services the runtimes and closes replaced ones once their sounds
// have finished. A game calls it every tick.
func (a *Audio) Update() {
	for _, playing := range a.players() {
		if err := playing.runtime.Update(); err != nil {
			a.tell(err.Error(), "Sound: "+err.Error())
		}
		playing.voices = slices.DeleteFunc(playing.voices, func(voice audio.Voice) bool {
			return !sounding(voice.Status().State)
		})
	}
	a.retiring = slices.DeleteFunc(a.retiring, func(playing *cuePlayer) bool {
		if len(playing.voices) > 0 {
			return false
		}
		_ = playing.runtime.Close() //nolint:errcheck // Its failures were reported by Update.

		return true
	})
	for _, playing := range a.layers {
		if err := playing.runtime.Update(); err != nil {
			a.tell(err.Error(), "Sound: "+err.Error())
		}
	}
}

func sounding(state audio.VoiceState) bool {
	return state == audio.VoicePlaying || state == audio.VoiceReleasing || state == audio.VoicePaused
}

// Close stops everything and closes every runtime, when the game ends.
func (a *Audio) Close() error {
	for slot := range a.layers {
		a.StopLayer(slot)
	}
	players := a.players()
	errs := make([]error, 0, len(players))
	for _, playing := range players {
		errs = append(errs, playing.runtime.Close())
	}
	a.current, a.retiring = nil, nil

	return errors.Join(errs...)
}

// tell reports a problem once.
func (a *Audio) tell(key, message string) {
	if a.told[key] || a.report == nil {
		return
	}
	a.told[key] = true
	a.report(message)
}
