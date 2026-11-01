package mapview

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/game"
)

// The player start is where Launch puts the player (increment 14): one cell
// of one floor in the project, set from the Map's right-click menu and drawn
// on its floor as a green outline marked "Start".

const (
	menuSetStart = "set-start"
	hexStart     = 0x2f8f46
	startLabel   = "Start"
)

// syncStart follows the game settings and keeps the player start the Map
// draws, since the cell menu and the floor are drawn while a floor is read.
func (s *State) syncStart(ctx *frame.Context) {
	s.subscribe(game.SettingsID(), ctx)
	ctx.Docs.View(game.SettingsID(), func(document client.Document) {
		if settings, ok := document.Model.(*gameplugin.SettingsDocument); ok {
			s.playerStart = settings.Settings.Start
		}
	})
}

// startHere reports whether the player start is on a cell of the shown floor.
func (s *State) startHere(cell Cell) bool {
	start := s.playerStart

	return !start.IsZero() && start.Zone == s.zone && start.Floor == s.floor && start.Col == cell.X &&
		start.Row == cell.Y
}

// startItem is the cell menu's "Set player start here", offered where the
// player can stand and the start is not already.
func (s *State) startItem(standable bool, cell Cell) ui.MenuItem {
	if s.startHere(cell) {
		return menuItem(menuSetStart, "The player starts here", true)
	}

	return menuItem(menuSetStart, "Set player start here", !standable)
}

// setStart moves the player start to a cell of the shown floor.
func (s *State) setStart(ctx *frame.Context, cell Cell) {
	start := game.Start{Zone: s.zone, Floor: s.floor, Col: cell.X, Row: cell.Y}
	s.command(ctx, game.SettingsID(), gameplugin.StartEdit, "", gameplugin.StartArgs{Start: start},
		func(protocol.Result) {
			ctx.Frame.Status(fmt.Sprintf("The player starts at %d,%d on this floor. Launch plays from here.",
				cell.X, cell.Y))
		})
}

// drawStart marks the player start on the shown floor.
func (s *State) drawStart(area *ebiten.Image, grid image.Rectangle) {
	start := s.playerStart
	if start.IsZero() || start.Zone != s.zone || start.Floor != s.floor {
		return
	}
	rect := s.camera.cellRect(Cell{X: start.Col, Y: start.Row}, grid)
	if !rect.Overlaps(grid) {
		return
	}
	green := ui.Hex(hexStart)
	ui.StrokeRect(area, rect, ui.Outline, green)
	if rect.Dx() < minLabel {
		return
	}
	labelX, labelY := rect.Min.X+ui.Half, rect.Min.Y-ui.LabelSize-ui.Gap-ui.Half
	backing := image.Rect(labelX-1, labelY, labelX+s.theme.SmallWidth(startLabel)+ui.Half,
		labelY+ui.LabelSize+ui.Gap+ui.Half)
	ui.FillRect(area, backing, ui.HexAlpha(hexWhite, labelBacking))
	s.theme.DrawSmall(area, startLabel, labelX, labelY, green)
}

// hasStart answers "player-start:C,R": the player start is on that cell of
// the shown floor.
func (s *State) hasStart(value string) bool {
	var col, row int
	if _, err := fmt.Sscanf(value, "%d,%d", &col, &row); err != nil {
		return false
	}

	return s.startHere(Cell{X: col, Y: row})
}
