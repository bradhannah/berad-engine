package browser

import (
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The World state row, after Settings, opens the World state window, where
// the project's world, zone and NPC values are declared.
const (
	worldStateRow  = "world-state"
	worldStateView = "world-state"
)

// worldStateRows is the World state row, saying how many values the
// project declares. A search leaves it out unless its own words match.
func (s *State) worldStateRows(docs frame.Docs, match matcher) []ui.Row {
	if match.active() && !match.words("World state", "values") {
		return nil
	}
	row := ui.NewRow(worldStateRow, worldstate.Name)
	row.Icon = ui.IconSettings
	if count, ok := s.declared(docs); ok {
		row.Detail = strconv.Itoa(count) + " " + plural(count, "value", "values")
	}

	return []ui.Row{row}
}

// declared counts the values the project's world state declares.
func (s *State) declared(docs frame.Docs) (int, bool) {
	count := 0
	loaded := docs.View(worldstate.ID(), func(document client.Document) {
		if stateDoc, ok := document.Model.(*stateplugin.Document); ok {
			count = stateDoc.State.Values.Len()
		}
	})

	return count, loaded
}

// layoutWorldState places Open World state in the row's inspector.
func (s *State) layoutWorldState(left, width int) {
	s.open.Label, s.open.Disabled = "Open World state", false
	s.open.Rect = image.Rect(left, nameFieldTop, left+width, nameFieldTop+ui.ButtonH)
}

// drawWorldState draws the World state row's inspector.
func (s *State) drawWorldState(screen *ebiten.Image, left int) {
	s.drawHeader(screen, ui.IconSettings, "World state", worldstate.Name, left)
	s.open.Draw(screen, s.theme)
	top := s.open.Rect.Max.Y + ui.Gap
	for _, line := range []string{
		"The values workflows set and check,", "for the world, each zone and each NPC.",
		"A workflow's own local values are", "declared in its Workflow window.",
	} {
		s.theme.DrawText(screen, line, left, top, ui.ColorSubdued())
		top += ui.LineHeight
	}
}
