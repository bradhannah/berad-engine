package mapview

import (
	"fmt"
	"strings"

	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/playview"
	"github.com/bradhannah/berad-engine/engine/game"
)

// Play from here (increment 47): a cell's menu launches the game once with
// the player on that cell, beside Preview from here. The player start stays
// where it is, and Launch plays from it again.

// menuPlayHere is the cell menu's Play from here.
const menuPlayHere = "play-here"

// playFrom opens the Play tab to launch once with the player on a cell of
// the shown floor.
func (s *State) playFrom(ctx *frame.Context, cell Cell) {
	start := game.Start{Zone: s.zone, Floor: s.floor, Col: cell.X, Row: cell.Y}
	ctx.Requests.OpenItem(playview.ViewName, game.SettingsID(), playview.LaunchFromItem(start))
}

// Move the player here (increment 63): while a game runs, a cell's menu
// moves its player to that cell, and the game goes on, with its values and
// its clock as they were, where Play from here starts the game again.

// menuMoveHere is the cell menu's Move the player here.
const menuMoveHere = "move-here"

// canMovePlayer reports whether there is a game whose player this Map can
// move: the preview it runs, or Launch's game in the Play tab.
func (s *State) canMovePlayer() bool { return s.preview != nil || s.playing }

// movePlayer moves a player to a cell of the shown floor: this Map's
// preview's when it runs one, and otherwise the player of Launch's game,
// which the Play tab moves, into this zone if it is playing another.
func (s *State) movePlayer(ctx *frame.Context, cell Cell) {
	if s.preview == nil {
		start := game.Start{Zone: s.zone, Floor: s.floor, Col: cell.X, Row: cell.Y}
		ctx.Requests.OpenItem(playview.ViewName, game.SettingsID(), playview.MoveToItem(start))

		return
	}
	if err := s.preview.game.Place(s.floor, cell); err != nil {
		ctx.Frame.Refuse(sentence(err.Error()))

		return
	}
	s.follow(s.preview.game.Player())
	// The menu is chosen while the floor is being read, so the floor is
	// named from the names the Map keeps, not by reading a document.
	ctx.Frame.Status(fmt.Sprintf("Moved the player to cell %d, %d of %s. The preview goes on.", cell.X, cell.Y,
		s.names.Name(s.floor, "this floor")))
}

// sentence starts a message with a capital and ends it with a full stop.
func sentence(message string) string {
	if message == "" {
		return message
	}

	return strings.ToUpper(message[:1]) + message[1:] + "."
}
