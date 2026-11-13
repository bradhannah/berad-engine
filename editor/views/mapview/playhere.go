package mapview

import (
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
