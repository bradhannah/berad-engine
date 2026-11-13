package pixelview

import (
	"encoding/json"
	"maps"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
)

// A sprite opens with the colour last used in it (increment 60), kept for
// this person beside the window's arrangement. One never painted opens with
// none, and the Pencil and Fill refuse until a colour is chosen: the editor
// remembers, and never guesses a first colour.

// choose takes a colour to paint with, to be kept as this sprite's.
func (s *State) choose(ref art.ColorRef) {
	s.color, s.chosen, s.keeping = ref, true, true
}

// keepColour keeps the colour chosen as this sprite's, once.
func (s *State) keepColour(ctx *frame.Context) {
	if !s.keeping {
		return
	}
	s.keeping = false
	encoded, err := json.Marshal(s.color)
	if err != nil {
		return
	}
	kept := protocol.PixelColours{Sprites: maps.Clone(s.kept.Sprites)}
	if kept.Sprites == nil {
		kept.Sprites = map[string]json.RawMessage{}
	}
	kept.Sprites[s.sprite.String()] = encoded
	s.kept = kept
	ctx.Requests.Publish(protocol.TopicPixelColours, kept)
}

// takeKept takes the colours kept, and the one last used in this sprite
// while none is chosen yet.
func (s *State) takeKept(update client.Update) {
	if update.Session == nil || update.Session.Topic != protocol.TopicPixelColours {
		return
	}
	var kept protocol.PixelColours
	if json.Unmarshal(update.Session.Value, &kept) != nil {
		return
	}
	s.kept = kept
	raw, has := kept.Sprites[s.sprite.String()]
	if s.chosen || !has {
		return
	}
	var ref art.ColorRef
	if json.Unmarshal(raw, &ref) == nil {
		s.color, s.chosen = ref, true
	}
}

// refuseNoColour refuses the Pencil and Fill while no colour is chosen. It
// reports whether it refused.
func (s *State) refuseNoColour(ctx *frame.Context) bool {
	if s.chosen || (s.tool != ToolPencil && s.tool != ToolFill) {
		return false
	}
	ctx.Frame.Refuse("Choose a colour to paint with")

	return true
}
