package player

import (
	"slices"
	"strings"

	audio "github.com/bradhannah/berad-sdk/modules/berad-audio"

	"github.com/bradhannah/berad-engine/engine/game"
)

// Slots of the soundscape a game plays: one per cue the zone plays
// throughout, and one per ambient sound within hearing.
const (
	zoneSlot    = "zone:"
	ambientSlot = "ambient:"
)

// Layer is one layer of a game's soundscape: a cue that plays again and
// again in its slot, at a gain from 0 to 1.
type Layer struct {
	Slot string
	Cue  string
	Gain float64
}

// Cues plays a project's cues once each: Prepare has them ready, so they
// play without waiting later; PlayAt plays one at a step of package
// loudness in place of its own; Stop stops them all and what waits to play.
type Cues interface {
	Prepare(cues []string)
	Play(cue string)
	PlayAt(cue, step string)
	Stop()
}

// Layers is a soundscape: PlayLayer plays a layer from its start, replacing
// what its slot played; SetLayer changes a playing layer's gain; StopLayer
// stops one.
type Layers interface {
	PlayLayer(layer Layer)
	SetLayer(layer Layer)
	StopLayer(slot string)
}

// Sound is what a game plays, and when: the cues NPCs ask for, once each,
// and the soundscape of the zone's sounds and the ambient sounds near the
// player, each ambient sound's gain on berad-audio's distance curve. The
// editor's Play tab and a game follow the same rules, each with its own way
// of playing a cue.
type Sound struct {
	// layers is each playing slot's cue; heard counts the cues NPCs asked
	// for.
	layers map[string]string
	heard  map[string]int
	// radius is how many cells away a cue is heard, nil until it is known.
	radius func(cue string) float64
}

// NewSound returns a game's sound with nothing playing.
func NewSound() *Sound {
	return &Sound{layers: map[string]string{}, heard: map[string]int{}, radius: nil}
}

// KnowRadius tells how many cells away an ambient cue is heard, zero or
// less for a cue that is not ambient. Until it is told, no ambient sound
// plays, so nothing plays early from everywhere.
func (s *Sound) KnowRadius(radius func(cue string) float64) { s.radius = radius }

// Start readies the cues a game's workflows play and starts the zone's
// sounds, for a game that is starting.
func (s *Sound) Start(cues Cues, layers Layers, played *game.Game, prepare []string) {
	clear(s.heard)
	if len(prepare) > 0 {
		cues.Prepare(prepare)
	}
	s.startZone(layers, played)
}

// startZone plays each of the zone's sounds again and again, and stops
// those of a zone the player has left (increment 63).
func (s *Sound) startZone(layers Layers, played *game.Game) {
	sounds := played.ZoneSounds()
	for slot, cue := range s.layers {
		if strings.HasPrefix(slot, zoneSlot) && !slices.Contains(sounds, cue) {
			layers.StopLayer(slot)
			delete(s.layers, slot)
		}
	}
	for _, cue := range sounds {
		slot := zoneSlot + cue
		if _, playing := s.layers[slot]; playing {
			continue
		}
		layers.PlayLayer(Layer{Slot: slot, Cue: cue, Gain: 1})
		s.layers[slot] = cue
	}
}

// Update plays what a tick asked for and follows the player with the
// ambient sounds: each plays while the player is within its distance, louder
// closer.
func (s *Sound) Update(cues Cues, layers Layers, played *game.Game, step game.Step) {
	for _, asked := range step.Cues {
		s.heard[asked.Cue]++
		if asked.Loudness != "" {
			cues.PlayAt(asked.Cue, asked.Loudness)
		} else {
			cues.Play(asked.Cue)
		}
	}
	s.startZone(layers, played)
	near := map[string]bool{}
	for _, heard := range played.Ambient() {
		slot := ambientSlot + heard.Sound.String()
		gain := audio.QuadraticDistanceGain(heard.Distance, s.radiusOf(heard.Cue))
		if gain <= 0 {
			continue
		}
		near[slot] = true
		layer := Layer{Slot: slot, Cue: heard.Cue, Gain: gain}
		if _, playing := s.layers[slot]; playing {
			layers.SetLayer(layer)

			continue
		}
		layers.PlayLayer(layer)
		s.layers[slot] = heard.Cue
	}
	for slot := range s.layers {
		if strings.HasPrefix(slot, ambientSlot) && len(slot) > len(ambientSlot) && !near[slot] {
			layers.StopLayer(slot)
			delete(s.layers, slot)
		}
	}
}

// radiusOf is how many cells away a cue is heard: none until it is known.
func (s *Sound) radiusOf(cue string) float64 {
	if s.radius == nil {
		return -1
	}
	if radius := s.radius(cue); radius > 0 {
		return radius
	}

	return -1
}

// Silence stops everything the game plays.
func (s *Sound) Silence(cues Cues, layers Layers) {
	for slot := range s.layers {
		layers.StopLayer(slot)
	}
	clear(s.layers)
	cues.Stop()
}

// Heard is how many times NPCs have asked for a cue since the game started.
func (s *Sound) Heard(cue string) int { return s.heard[cue] }

// Playing reports whether a cue plays in the soundscape.
func (s *Sound) Playing(cue string) bool {
	for _, playing := range s.layers {
		if playing == cue {
			return true
		}
	}

	return false
}
