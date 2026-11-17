package playview

import (
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/sound"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/player"
)

// sounds is what a game plays in the Play tab: the engine's player decides
// what plays when, as it does for a game (increment 62), and the window's
// sounds play it, with the recordings the orchestrator sends. The audio
// library tells how far each ambient cue is heard, as the Audio window
// draws it, and how long each cue lasts.
type sounds struct {
	rules   *player.Sound
	library *protocol.AudioLibrary
}

func newSounds() *sounds {
	return &sounds{rules: player.NewSound(), library: nil}
}

// layers is the window's soundscape as the player plays layers on it: each
// again and again.
type layers struct {
	scape frame.Soundscape
}

func (l layers) PlayLayer(layer player.Layer) { l.scape.PlayLayer(looped(layer)) }

func (l layers) SetLayer(layer player.Layer) { l.scape.SetLayer(looped(layer)) }

func (l layers) StopLayer(slot string) { l.scape.StopLayer(slot) }

// looped is a player's layer as the window's soundscape plays it.
func looped(layer player.Layer) sound.Layer {
	return sound.Layer{Slot: layer.Slot, Cue: layer.Cue, Gain: layer.Gain, Loop: true, Again: false, Rest: 0}
}

// launch lists the audio library for the ambient cues' distances, fetches
// what the NPCs' behaviours play, and starts the zone's sounds.
func (s *sounds) launch(ctx *frame.Context, played *game.Game, cues []string) {
	ctx.Requests.AudioLibrary(func(library protocol.AudioLibrary, err error) {
		if err == nil {
			s.library = &library
			played.KnowCues(s.library.CueTicks)
			s.rules.KnowRadius(s.radius)
		}
	})
	s.rules.Start(ctx.Sounds, layers{scape: ctx.Soundscape}, played, cues)
}

// update plays what a tick asked for and follows the player with the
// ambient sounds.
func (s *sounds) update(ctx *frame.Context, played *game.Game, step game.Step) {
	s.rules.Update(ctx.Sounds, layers{scape: ctx.Soundscape}, played, step)
}

// radius is how many cells away a cue is heard: an ambient cue's distance,
// and none for any other.
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
	s.rules.Silence(ctx.Sounds, layers{scape: ctx.Soundscape})
}
