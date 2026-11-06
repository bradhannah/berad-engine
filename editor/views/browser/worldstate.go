package browser

import (
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/signals"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The World state section, after Settings, holds Values, which opens the
// window where the project's world, zone and NPC values are declared,
// Signals, which opens the one where its signals are, and Constants, the one
// where its constants are.
const (
	sectionWorldState  = sectionPrefix + "world-state"
	worldStateRow      = "world-state"
	worldSignalsRow    = "world-signals"
	worldConstantsRow  = "world-constants"
	worldStateView     = "world-state"
	worldSignalsView   = "world-signals"
	worldConstantsView = "world-constants"
)

// worldStateRows is the World state section: Values, saying how many values
// the project declares, Signals, how many signals, and Constants, how many
// constants. A search keeps a row whose own words match, and the section
// while it keeps one.
func (s *State) worldStateRows(docs frame.Docs, match matcher) []ui.Row {
	values := ui.NewRow(worldStateRow, "Values")
	values.Icon = ui.IconSettings
	constants := ui.NewRow(worldConstantsRow, "Constants")
	constants.Icon = ui.IconSettings
	if count, constant, ok := s.declared(docs); ok {
		values.Detail = strconv.Itoa(count) + " " + plural(count, "value", "values")
		constants.Detail = strconv.Itoa(constant) + " " + plural(constant, "constant", "constants")
	}
	sent := ui.NewRow(worldSignalsRow, "Signals")
	sent.Icon = ui.IconSignal
	if declared, ok := signals.Declared(docs); ok {
		count := len(signals.Sendable(declared))
		sent.Detail = strconv.Itoa(count) + " " + plural(count, "signal", "signals")
	}
	rows := make([]ui.Row, 0, worldStateParts)
	for _, row := range []ui.Row{values, sent, constants} {
		if !match.active() || match.words(worldstate.Name, row.Label) {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	heading := s.section(sectionWorldState, worldstate.Name, len(rows), false, match.active())
	if s.closed[sectionWorldState] && !match.active() {
		return []ui.Row{heading}
	}

	return append([]ui.Row{heading}, rows...)
}

// worldStateParts is how many rows the World state section holds.
const worldStateParts = 3

// declared counts what the project's world state declares: its values,
// with their groups and templates, and its constants.
func (s *State) declared(docs frame.Docs) (int, int, bool) {
	values, constants := 0, 0
	loaded := docs.View(worldstate.ID(), func(document client.Document) {
		if stateDoc, ok := document.Model.(*stateplugin.Document); ok {
			for _, declared := range stateDoc.State.Values.Values() {
				if declared.IsConstant() {
					constants++
				} else {
					values++
				}
			}
		}
	})

	return values, constants, loaded
}

// layoutWorldState places Open Values or Open Signals in the row's
// inspector.
func (s *State) layoutWorldState(left, width int) {
	s.open.Label, s.open.Disabled = "Open Values", false
	switch s.list.Selected {
	case worldSignalsRow:
		s.open.Label = "Open Signals"
	case worldConstantsRow:
		s.open.Label = "Open Constants"
	}
	s.open.Rect = image.Rect(left, nameFieldTop, left+width, nameFieldTop+ui.ButtonH)
}

// drawWorldState draws the Values or Signals row's inspector.
func (s *State) drawWorldState(screen *ebiten.Image, left int) {
	icon, title, about := ui.IconSettings, "Values", []string{
		"The values workflows set and check,", "for the world, each zone and each NPC.",
		"A workflow's own local values are", "declared in its Workflow window.",
	}
	switch s.list.Selected {
	case worldSignalsRow:
		icon, title, about = ui.IconSignal, "Signals", []string{
			"The signals workflows send and answer,", "in their groups. Only the zone running",
			"hears a signal; tell another zone", "with a value.",
		}
	case worldConstantsRow:
		title, about = "Constants", []string{
			"Words and numbers the game never", "changes, for the world, each zone and",
			"each NPC. Templates give many the", "same constants, each its own value.",
		}
	}
	s.drawHeader(screen, icon, worldstate.Name, title, left)
	s.open.Draw(screen, s.theme)
	top := s.open.Rect.Max.Y + ui.Gap
	for _, line := range about {
		s.theme.DrawText(screen, line, left, top, ui.ColorSubdued())
		top += ui.LineHeight
	}
}
