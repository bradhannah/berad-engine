package playview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/views/showdraw"
	"github.com/bradhannah/berad-engine/engine/show"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// hasState answers input scripts waiting on the game:
//
//   - "playing" once it runs, and "problem:<start>" when it cannot;
//   - "player:C,R" when the player stands still on that cell, and
//     "floor:N" when they are on the Nth floor from the bottom;
//   - "sprite:<what>" when a sprite shows a stand-in, the player or
//     "npc:<id>";
//   - "heard:<cue>" once an NPC asked for a cue, and "layer:<cue>" while a
//     zone or ambient sound plays it;
//   - "scale:N.NN" when the screen is drawn that many times its size, and
//     "launches:N" after N launches;
//   - "ground:C,R:<id>" when the game's floor has that ground there;
//   - "box:<text>" while the game shows that message box.
func (s *State) hasState(state string) bool {
	kind, value, _ := strings.Cut(state, ":")
	switch kind {
	case "playing":
		return s.game != nil
	case "problem":
		return s.problem != "" && strings.HasPrefix(s.problem, value)
	case "sprite":
		_, shown := s.art.image(value)

		return shown
	case "heard":
		return s.sound.heard[value] > 0
	case "layer":
		return s.sound.playing(value)
	case "scale":
		return strconv.FormatFloat(s.scale, 'f', 2, 64) == value
	case "launches":
		return strconv.Itoa(s.launches) == value
	case "box":
		box := showdraw.BoxOf(s.stage())

		return box != nil && box.Text == value
	}

	return s.hasPlace(kind, value)
}

// stage is the game's boxes and shake, or none before it runs.
func (s *State) stage() *show.Stage {
	if s.game == nil {
		return show.NewStage()
	}

	return s.game.Stage()
}

// hasPlace answers "player:", "floor:" and "ground:".
func (s *State) hasPlace(kind, value string) bool {
	if s.game == nil {
		return false
	}
	player := s.game.Player()
	switch kind {
	case "player":
		return !player.Moving() && fmt.Sprintf("%d,%d", player.Cell.X, player.Cell.Y) == value
	case "floor":
		number, err := strconv.Atoi(value)

		return err == nil && number >= 1 && number <= len(s.floors) && s.floors[number-1] == player.Floor
	case "ground":
		where, ground, _ := strings.Cut(value, ":")
		var col, row int
		if _, err := fmt.Sscanf(where, "%d,%d", &col, &row); err != nil {
			return false
		}
		found := false
		s.game.Floor(player.Floor, func(shown *floor.Floor) {
			got, err := shown.Ground(col, row)
			found = err == nil && got == ground
		})

		return found
	}

	return false
}
