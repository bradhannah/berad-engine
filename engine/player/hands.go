package player

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Hands plays a game by hand, a tick at a time, the same in the editor's
// Play tab and in a game's window: the directions held walk the player, and
// a box that waits for the player closes when asked.
type Hands struct {
	// held is the directions held on the tick before, to tell a press from
	// a key kept down.
	held play.Direction
}

// Step runs a game one tick: a box that waits closes when closeBox asks,
// then the game steps with the directions held now.
func (h *Hands) Step(played *game.Game, held play.Direction, closeBox bool) game.Step {
	if stage := played.Stage(); closeBox && stage.Holding() {
		stage.Close()
	}
	step := played.Step(play.Input{Held: held, Pressed: held &^ h.held})
	h.held = held

	return step
}

// State answers what a scripted walk waits on in a running game, and
// whether it knows the state at all:
//
//   - "player:C,R" when the player stands still on that cell, and "floor:N"
//     when they are on the Nth floor from the bottom, of the zone's floors;
//   - "sprite:<what>" when a sprite shows a stand-in, the player or
//     "npc:<id>";
//   - "heard:<cue>" once an NPC asked for a cue, and "layer:<cue>" while a
//     zone or ambient sound plays it;
//   - "ground:C,R:<id>" when the game's floor has that ground there;
//   - "box:<text>" while the game shows that message box;
//   - "dark" while a portal that fades has the screen black, "light" while
//     nothing is faded, and "leaving" while the player waits on a portal.
func State(state string, played *game.Game, screen *Screen, sound *Sound, floors []ids.ULID) (bool, bool) {
	kind, value, _ := strings.Cut(state, ":")
	switch kind {
	case "sprite":
		_, shown := screen.Art().Image(value)

		return shown, true
	case "heard":
		return sound.Heard(value) > 0, true
	case "layer":
		return sound.Playing(value), true
	case "box":
		box := BoxOf(played.Stage())

		return box != nil && box.Text == value, true
	case "player", "floor", "ground":
		return placed(kind, value, played, floors), true
	case "dark":
		return played.Darkness() >= 1, true
	case "light":
		return played.Darkness() == 0, true
	case "leaving":
		_, leaving := played.Leaving()

		return leaving, true
	}

	return false, false
}

// placed answers "player:", "floor:" and "ground:".
func placed(kind, value string, played *game.Game, floors []ids.ULID) bool {
	player := played.Player()
	switch kind {
	case "player":
		return !player.Moving() && fmt.Sprintf("%d,%d", player.Cell.X, player.Cell.Y) == value
	case "floor":
		number, err := strconv.Atoi(value)

		return err == nil && number >= 1 && number <= len(floors) && floors[number-1] == player.Floor
	}
	where, ground, _ := strings.Cut(value, ":")
	var col, row int
	if _, err := fmt.Sscanf(where, "%d,%d", &col, &row); err != nil {
		return false
	}
	found := false
	played.Floor(player.Floor, func(shown *tilefloor.Floor) {
		got, err := shown.Ground(col, row)
		found = err == nil && got == ground
	})

	return found
}
