package browser

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/game"
)

// The Settings inspector shows the game settings below the tile size: the
// screen's size in cells and where the player starts (increment 14).

// tileSizeLines is how many lines explain the tile size, above the screen.
const tileSizeLines = 3

// gameSettings reads the project's game settings.
func (s *State) gameSettings(docs frame.Docs) (game.Settings, bool) {
	var settings game.Settings
	loaded := false
	docs.View(game.SettingsID(), func(document client.Document) {
		if found, ok := document.Model.(*gameplugin.SettingsDocument); ok {
			settings, loaded = found.Settings, true
		}
	})

	return settings, loaded
}

// layoutScreen places the screen's two fields below the tile size and what
// it decides.
func (s *State) layoutScreen(left int) {
	top := s.tileSize.Rect.Max.Y + ui.Gap + tileSizeLines*ui.LineHeight + panelGap + ui.LineHeight
	s.screenCols.Rect = image.Rect(left, top, left+tileSizeWidth, top+ui.ButtonH)
	rowsLeft := s.screenCols.Rect.Max.X + ui.Gap*ui.Half + s.theme.TextWidth("×") + ui.Gap*ui.Half
	s.screenRows.Rect = image.Rect(rowsLeft, top, rowsLeft+tileSizeWidth, top+ui.ButtonH)
}

// commitScreen saves the typed screen size, an odd number of cells across
// and down.
func (s *State) commitScreen(ctx *frame.Context) {
	typedColumns, typedRows := s.screenCols.Field.Value(), s.screenRows.Field.Value()
	_, columnsChanged := s.screenCols.Field.Commit()
	_, rowsChanged := s.screenRows.Field.Commit()
	if !columnsChanged && !rowsChanged {
		s.focus.Set(nil)

		return
	}
	columns, columnsErr := strconv.Atoi(strings.TrimSpace(typedColumns))
	rows, rowsErr := strconv.Atoi(strings.TrimSpace(typedRows))
	if columnsErr != nil || rowsErr != nil || game.CheckScreen(columns, rows) != nil {
		s.screenCols.Field.Failed()
		s.screenRows.Field.Failed()
		ctx.Frame.Status(fmt.Sprintf("Enter odd numbers of cells from %d to %d", game.MinScreen, game.MaxScreen))

		return
	}
	failed := func() {
		s.screenCols.Field.Failed()
		s.screenRows.Field.Failed()
	}
	s.command(ctx, game.SettingsID(), gameplugin.ScreenEdit, "", gameplugin.ScreenArgs{Columns: columns, Rows: rows},
		func(protocol.Result, error) {
			s.focus.Set(nil)
			s.screenCols.Field.Revert()
			s.screenRows.Field.Revert()
			ctx.Frame.Status(fmt.Sprintf("The screen shows %d by %d cells", columns, rows))
		}, failed)
}

// drawScreen draws the screen's fields and the player start.
func (s *State) drawScreen(screen *ebiten.Image, docs frame.Docs, left int) {
	if s.screenCols.Rect.Empty() {
		return
	}
	s.theme.Heading(screen, "SCREEN", left, s.screenCols.Rect.Min.Y-ui.LineHeight)
	s.screenCols.Draw(screen)
	textY := s.screenCols.Rect.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	s.theme.DrawText(screen, "×", s.screenCols.Rect.Max.X+ui.Gap*ui.Half, textY, ui.ColorMuted())
	s.screenRows.Draw(screen)
	s.theme.DrawText(screen, "cells", s.screenRows.Rect.Max.X+ui.Gap*ui.Half, textY, ui.ColorMuted())
	top := s.screenRows.Rect.Max.Y + ui.Gap
	s.theme.DrawText(screen, "Odd numbers, so the player is in the middle.", left, top, ui.ColorSubdued())
	top += ui.LineHeight + panelGap
	s.theme.Heading(screen, "PLAYER START", left, top)
	top += ui.LineHeight
	settings, _ := s.gameSettings(docs)
	line, ink := "Not placed. Right-click a cell on a map.", ui.ColorMuted()
	if start := settings.Start; !start.IsZero() {
		index := s.index(docs)
		line = fmt.Sprintf("%s, %s, cell %d,%d", index.Name(start.Zone, "a deleted zone"),
			index.Name(start.Floor, "a deleted floor"), start.Col, start.Row)
		ink = ui.ColorInk()
	}
	s.theme.DrawText(screen, line, left, top, ink)
}
