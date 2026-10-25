package sound

import (
	"strings"
	"time"

	audio "github.com/bradhannah/berad-sdk/modules/berad-audio"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// Monitor is how loud a window plays each category while its author
// listens, from 0 to 1, standing in for a player's volume settings. With
// Reference, recordings play at their calibrated level alone, without cue
// gains or these. A change applies to what starts next, and to the
// soundscape at once.
type Monitor struct {
	Music, SFX, Ambient float64
	Reference           bool
}

// FullMonitor plays every category at full volume.
func FullMonitor() Monitor { return Monitor{Music: 1, SFX: 1, Ambient: 1, Reference: false} }

// Layer is one sound of the Audio window's soundscape, which plays several
// cues together so an author hears how they sit together: Slot names it;
// Cue plays at Gain, from 0 to 1, without a break when Loop is set, as an
// ambient cue plays, or once more whenever it ends, Rest later, when Again
// is set. Nothing about it is saved.
type Layer struct {
	Slot  string
	Cue   string
	Gain  float64
	Loop  bool
	Again bool
	Rest  time.Duration
}

// layerPlay is a layer and what plays it: its own runtime, so its volume
// changes while it plays, built from the recordings' versions in built;
// waiting says it waits for its recordings, and again when it plays once
// more.
type layerPlay struct {
	layer   Layer
	runtime *audio.Runtime
	built   string
	voice   audio.Voice
	waiting bool
	again   time.Time
}

// PlayLayer plays a layer from its start, replacing what its slot played,
// once its recordings have arrived.
func (s *Player) PlayLayer(layer Layer) {
	s.StopLayer(layer.Slot)
	s.layers[layer.Slot] = &layerPlay{
		layer: layer, runtime: nil, built: "", voice: audio.Voice{}, waiting: true, again: time.Time{},
	}
	s.wanted[layer.Cue] = true
	if _, known := s.listing[layer.Cue]; !s.listed || !known {
		s.refresh()
	}
	s.startLayers()
}

// SetLayer changes a playing layer's gain and whether it plays again,
// without starting it again.
func (s *Player) SetLayer(layer Layer) {
	playing, found := s.layers[layer.Slot]
	if !found || playing.layer.Cue != layer.Cue {
		return
	}
	playing.layer = layer
	if !layer.Again {
		playing.again = time.Time{}
	}
	s.setLayerVolume(playing)
}

// StopLayer stops a layer.
func (s *Player) StopLayer(slot string) {
	playing, found := s.layers[slot]
	if !found {
		return
	}
	delete(s.layers, slot)
	s.closeLayer(playing)
}

// LayerPlaying reports whether a layer plays, waits for its recordings or
// waits to play again.
func (s *Player) LayerPlaying(slot string) bool {
	playing, found := s.layers[slot]

	return found && (playing.waiting || !playing.again.IsZero() || sounding(playing.voice.Status().State))
}

// SetMonitor sets how loud each category plays. Turning Reference on or off
// plays the soundscape again, at the new levels.
func (s *Player) SetMonitor(monitor Monitor) {
	reference := monitor.Reference != s.monitor.Reference
	s.monitor = monitor
	layers := make([]*layerPlay, 0, len(s.layers))
	for _, playing := range s.layers {
		layers = append(layers, playing)
	}
	for _, playing := range layers {
		if reference {
			s.PlayLayer(playing.layer)

			continue
		}
		s.setLayerVolume(playing)
	}
}

// monitorGain is how loud the monitor plays a cue, by its category; a
// recording played alone goes by the cues that play it, and plays as SFX
// when they differ or there are none.
func (s *Player) monitorGain(cue string) float64 {
	if s.monitor.Reference {
		return 1
	}
	category := s.listing[cue].Category
	if strings.HasPrefix(cue, "/") {
		category = s.recordingCategory(s.listing[cue].Recordings)
	}
	switch category {
	case protocol.CategoryMusic:
		return s.monitor.Music
	case protocol.CategoryAmbient:
		return s.monitor.Ambient
	}

	return s.monitor.SFX
}

// recordingCategory is the category every cue playing a recording shares,
// or SFX.
func (s *Player) recordingCategory(recordings []protocol.AudioRecording) string {
	if len(recordings) == 0 {
		return protocol.CategorySFX
	}
	shared := ""
	for id, cue := range s.listing {
		if strings.HasPrefix(id, "/") {
			continue
		}
		for _, recording := range cue.Recordings {
			if recording.ID != recordings[0].ID {
				continue
			}
			if shared != "" && shared != cue.Category {
				return protocol.CategorySFX
			}
			shared = cue.Category
		}
	}
	if shared == "" {
		return protocol.CategorySFX
	}

	return shared
}

// setLayerVolume plays a layer at its gain times its category's monitor
// level.
func (s *Player) setLayerVolume(playing *layerPlay) {
	if playing.runtime == nil {
		return
	}
	volume := min(max(playing.layer.Gain*s.monitorGain(playing.layer.Cue), 0), 1)
	if err := playing.runtime.SetVolume(volume); err != nil {
		s.tell("layer:"+err.Error(), "Sound: "+err.Error())
	}
}

// startLayers starts each layer waiting for its recordings that can play,
// and each whose time to play again has come.
func (s *Player) startLayers() {
	now := s.now()
	for slot, playing := range s.layers {
		if !playing.waiting && (playing.again.IsZero() || now.Before(playing.again)) {
			continue
		}
		state, problem := s.readinessOf(playing.layer.Cue)
		switch state {
		case cueWaiting:
			if s.listed && !s.asking {
				s.fetchWanted()
			}
		case cueBroken, cueSilent:
			if problem != "" {
				s.tell("cue:"+playing.layer.Cue, problem)
			}
			delete(s.layers, slot)
			s.closeLayer(playing)
		case cueReady:
			s.startLayer(playing)
		}
	}
}

// startLayer plays a ready layer, building its runtime again when its
// recordings or the monitor's Reference changed.
func (s *Player) startLayer(playing *layerPlay) {
	playing.waiting, playing.again = false, time.Time{}
	built := s.versionsOf(playing.layer.Cue)
	if playing.runtime == nil || playing.built != built {
		if !s.open() {
			return
		}
		runtime, err := s.build([]string{playing.layer.Cue})
		if err != nil {
			s.tell("layer:"+playing.layer.Cue, "Can’t play “"+s.listing[playing.layer.Cue].Name+"”: "+err.Error())

			return
		}
		if playing.runtime != nil {
			_ = playing.runtime.Close() //nolint:errcheck // A replaced runtime's failures were reported.
		}
		playing.runtime, playing.built = runtime, built
	}
	s.setLayerVolume(playing)
	options := audio.DefaultPlayOptions()
	options.Loop = playing.layer.Loop
	result, err := playing.runtime.Play(audio.CueID(playing.layer.Cue), options)
	if err != nil {
		s.tell("layer:"+playing.layer.Cue, "Can’t play “"+s.listing[playing.layer.Cue].Name+"”: "+err.Error())

		return
	}
	playing.voice = result.Voice
}

// versionsOf names the recordings a cue plays at their versions, and the
// monitor's Reference, which a runtime is built for.
func (s *Player) versionsOf(cue string) string {
	parts := []string{cue}
	for _, recording := range s.listing[cue].Recordings {
		parts = append(parts, recording.ID+"@"+recording.Version)
	}
	if s.monitor.Reference {
		parts = append(parts, "reference")
	}

	return strings.Join(parts, " ")
}

// updateLayers services the layers' runtimes, times when a layer that has
// ended plays again, forgets one that has ended for good, and starts what
// is due.
func (s *Player) updateLayers() {
	for slot, playing := range s.layers {
		if playing.runtime != nil {
			if err := playing.runtime.Update(); err != nil {
				s.tell(err.Error(), "Sound: "+err.Error())
			}
		}
		if playing.waiting || !playing.again.IsZero() || sounding(playing.voice.Status().State) {
			continue
		}
		if playing.layer.Again {
			playing.again = s.now().Add(playing.layer.Rest)

			continue
		}
		delete(s.layers, slot)
		s.closeLayer(playing)
	}
	s.startLayers()
}

// closeLayer stops a layer's sound and closes its runtime.
func (s *Player) closeLayer(playing *layerPlay) {
	_ = playing.voice.Stop(0) //nolint:errcheck // A zero fade cannot be refused.
	if playing.runtime != nil {
		if err := playing.runtime.Close(); err != nil {
			s.tell("layer:"+err.Error(), "Sound: "+err.Error())
		}
	}
}

// stopLayers stops every layer.
func (s *Player) stopLayers() {
	for slot, playing := range s.layers {
		delete(s.layers, slot)
		s.closeLayer(playing)
	}
}
