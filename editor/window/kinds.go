package window

import (
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/audioview"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/editor/views/paletteview"
	"github.com/bradhannah/berad-engine/editor/views/pixelview"
	"github.com/bradhannah/berad-engine/editor/views/playview"
	"github.com/bradhannah/berad-engine/editor/views/stateview"
	"github.com/bradhannah/berad-engine/editor/views/templateview"
	"github.com/bradhannah/berad-engine/editor/views/watchview"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
)

// Each tab shows its first pane's icon, and those of any pane beside it.

// viewIcon is the icon of a kind of window.
func viewIcon(view string) ui.Icon {
	switch view {
	case browser.ViewName:
		return ui.IconProject
	case mapview.ViewName:
		return ui.IconZone
	case workflowview.ViewName:
		return ui.IconWorkflow
	case audioview.ViewName:
		return ui.IconSound
	case pixelview.ViewName:
		return ui.IconSprite
	case paletteview.ViewName:
		return ui.IconPalette
	case templateview.ViewName:
		return ui.IconTemplate
	case clockview.ViewName:
		return ui.IconClock
	case watchview.ViewName:
		return ui.IconWatch
	case stateview.ViewName, stateview.SignalsViewName, stateview.ConstantsViewName:
		return ui.IconSettings
	case playview.ViewName:
		return ui.IconPlay
	}

	return ui.IconNone
}

// shortTitle drops a window title's kind ("Map: ") and, unless keepID, its
// window's ID ("  (view 1a2b3c)"), which the bar shows in other ways.
func shortTitle(title string, keepID bool) string {
	if _, rest, found := strings.Cut(title, ": "); found {
		title = rest
	}
	if before, _, found := strings.Cut(title, "  ("); found && !keepID {
		title = before
	}

	return title
}

// tabTitle is the title of a tab's focused pane.
func (s *shell) tabTitle(index int) string {
	each := s.tabs[index]

	return each.panes[each.focus].place.title
}
