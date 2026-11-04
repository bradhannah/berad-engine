package playview

import (
	audio "github.com/bradhannah/berad-sdk/modules/berad-audio"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/sound"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/game"
)

// Slots of the soundscape a game plays: one per cue the zone plays
// throughout, and one per ambient sound within hearing.
const (
	zoneSlot    = "zone:"
	ambientSlot = "ambient:"
)

// sounds is what a game plays: the cues NPCs ask for, once each, and the
// soundscape of the zone's sounds and the ambient sounds near the player,
// each ambient sound's gain on berad-audio's distance curve, as the Audio
// window draws it.
type sounds struct {
	library *protocol.AudioLibrary
	// layers is each playing slot's cue; heard counts the cues NPCs asked
	// for.
	layers map[string]string
	heard  map[string]int
}

func newSounds() *sounds {
	return &sounds{library: nil, layers: map[string]string{}, heard: map[string]int{}}
}

// launch lists the audio library for the ambient cues' distances, fetches
// what the NPCs' behaviours play, and starts the zone's sounds.
func (s *sounds) launch(ctx *frame.Context, played *game.Game, cues []string) {
	clear(s.heard)
	ctx.Requests.AudioLibrary(func(library protocol.AudioLibrary, err error) {
		if err == nil {
			s.library = &library
			played.KnowCues(s.library.CueTicks)
		}
	})
	if len(cues) > 0 {
		ctx.Sounds.Prepare(cues)
	}
	s.startZone(ctx, played)
}

// startZone plays each of the zone's sounds again and again.
func (s *sounds) startZone(ctx *frame.Context, played *game.Game) {
	for _, cue := range played.ZoneSounds() {
		slot := zoneSlot + cue
		if _, playing := s.layers[slot]; playing {
			continue
		}
		ctx.Soundscape.PlayLayer(sound.Layer{Slot: slot, Cue: cue, Gain: 1, Loop: true, Again: false, Rest: 0})
		s.layers[slot] = cue
	}
}

// update plays what a tick asked for and follows the player with the
// ambient sounds: each plays while the player is within its distance, louder
// closer.
func (s *sounds) update(ctx *frame.Context, played *game.Game, step game.Step) {
	for _, asked := range step.Cues {
		s.heard[asked.Cue]++
		if asked.Loudness != "" {
			ctx.Sounds.PlayAt(asked.Cue, asked.Loudness)
		} else {
			ctx.Sounds.Play(asked.Cue)
		}
	}
	s.startZone(ctx, played)
	near := map[string]bool{}
	for _, heard := range played.Ambient() {
		slot := ambientSlot + heard.Sound.String()
		gain := audio.QuadraticDistanceGain(heard.Distance, s.radius(heard.Cue))
		if gain <= 0 {
			continue
		}
		near[slot] = true
		layer := sound.Layer{Slot: slot, Cue: heard.Cue, Gain: gain, Loop: true, Again: false, Rest: 0}
		if _, playing := s.layers[slot]; playing {
			ctx.Soundscape.SetLayer(layer)

			continue
		}
		ctx.Soundscape.PlayLayer(layer)
		s.layers[slot] = heard.Cue
	}
	for slot := range s.layers {
		if len(slot) > len(ambientSlot) && slot[:len(ambientSlot)] == ambientSlot && !near[slot] {
			ctx.Soundscape.StopLayer(slot)
			delete(s.layers, slot)
		}
	}
}

// radius is how many cells away a cue is heard: an ambient cue's distance,
// none until the library is listed, so nothing plays early from everywhere.
func (s *sounds) radius(cueID string) float64 {
	if s.library != nil {
		for _, cue := range s.library.Cues {
			if cue.ID == cueID && cue.Ambient != nil {
				return cue.Ambient.RadiusTiles
			}
		}
	}

	return -1
}

// silence stops everything the game plays.
func (s *sounds) silence(ctx *frame.Context) {
	for slot := range s.layers {
		ctx.Soundscape.StopLayer(slot)
	}
	clear(s.layers)
	ctx.Sounds.Stop()
}

// playing reports whether a cue plays in the soundscape.
func (s *sounds) playing(cue string) bool {
	for _, playing := range s.layers {
		if playing == cue {
			return true
		}
	}

	return false
}
