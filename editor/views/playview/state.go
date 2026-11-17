package playview

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/bradhannah/berad-engine/engine/player"
)

// hasState answers input scripts waiting on the game:
//
//   - "playing" once it runs, and "problem:<start>" when it cannot;
//   - "scale:N.NN" when the screen is drawn that many times its size, and
//     "launches:N" after N launches;
//   - and the states of the engine's player, which a game's own window
//     answers too: "player:C,R", "floor:N", "sprite:<what>", "heard:<cue>",
//     "layer:<cue>", "ground:C,R:<id>" and "box:<text>".
func (s *State) hasState(state string) bool {
	kind, value, _ := strings.Cut(state, ":")
	switch kind {
	case "playing":
		return s.game != nil
	case "problem":
		return s.problem != "" && strings.HasPrefix(s.problem, value)
	case "scale":
		return strconv.FormatFloat(s.screen.Scale()/pen.Scale(), 'f', 2, 64) == value
	case "launches":
		return strconv.Itoa(s.launches) == value
	}
	if s.game == nil {
		return s.stateWithNoGame(kind, value)
	}
	in, _ := player.State(state, s.game, s.screen, s.sound.rules, s.floors)

	return in
}

// stateWithNoGame answers for what outlasts a game that stopped: the sprites
// it drew, and what was heard.
func (s *State) stateWithNoGame(kind, value string) bool {
	switch kind {
	case "sprite":
		_, shown := s.screen.Art().Image(value)

		return shown
	case "heard":
		return s.sound.rules.Heard(value) > 0
	case "layer":
		return s.sound.rules.Playing(value)
	}

	return false
}
