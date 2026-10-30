package window

import (
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/audioview"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/editor/views/paletteview"
	"github.com/bradhannah/berad-engine/editor/views/pixelview"
	"github.com/bradhannah/berad-engine/editor/views/textview"
	"github.com/bradhannah/berad-engine/editor/views/watchview"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
)

// The window bar groups tabs by kind of view (increment 10, settled by
// its author after trying it): the bar's top row has a tab for each kind with
// how many are open, and a second row lists the open documents of the kind
// in front when there are several. A tab's kind is its first pane's, so a Map
// with Watch beside it is one of the Maps.

// kindOrder is the order kinds take on the bar.
func kindOrder() []string {
	return []string{
		browser.ViewName, mapview.ViewName, workflowview.ViewName, textview.ViewName, audioview.ViewName,
		pixelview.ViewName, paletteview.ViewName, watchview.ViewName, clockview.ViewName,
	}
}

// viewIcon is the icon of a kind of window.
func viewIcon(view string) ui.Icon {
	switch view {
	case browser.ViewName:
		return ui.IconProject
	case mapview.ViewName:
		return ui.IconZone
	case textview.ViewName:
		return ui.IconText
	case workflowview.ViewName:
		return ui.IconWorkflow
	case audioview.ViewName:
		return ui.IconSound
	case pixelview.ViewName:
		return ui.IconSprite
	case paletteview.ViewName:
		return ui.IconPalette
	case clockview.ViewName:
		return ui.IconClock
	case watchview.ViewName:
		return ui.IconWatch
	}

	return ui.IconNone
}

// kindName names a kind with several windows open on the bar.
func kindName(view string) string {
	switch view {
	case browser.ViewName:
		return "Project"
	case mapview.ViewName:
		return "Maps"
	case workflowview.ViewName:
		return "Workflows"
	case textview.ViewName:
		return "Text"
	case audioview.ViewName:
		return "Audio"
	case pixelview.ViewName:
		return "Sprites"
	case paletteview.ViewName:
		return "Palette Editor"
	case watchview.ViewName:
		return "Watch"
	case clockview.ViewName:
		return "Clock"
	}

	return view
}

// kindOf is the kind a tab is filed under: its first pane's.
func kindOf(each *tab) string {
	return each.panes[0].view
}

// kindGroup is one kind's open tabs, as indexes into the shell's tabs, in the
// order they opened.
type kindGroup struct {
	view string
	tabs []int
}

// kinds groups the open tabs by kind, in bar order.
func (s *shell) kinds() []kindGroup {
	order := kindOrder()
	groups := []kindGroup{}
	for index, each := range s.tabs {
		view := kindOf(each)
		at := slices.IndexFunc(groups, func(group kindGroup) bool { return group.view == view })
		if at < 0 {
			groups = append(groups, kindGroup{view: view, tabs: nil})
			at = len(groups) - 1
		}
		groups[at].tabs = append(groups[at].tabs, index)
	}
	rank := func(view string) int {
		if at := slices.Index(order, view); at >= 0 {
			return at
		}

		return len(order)
	}
	slices.SortStableFunc(groups, func(left, right kindGroup) int { return rank(left.view) - rank(right.view) })

	return groups
}

// frontKind returns the group of the tab in front.
func (s *shell) frontKind() (kindGroup, bool) {
	for _, group := range s.kinds() {
		if slices.Contains(group.tabs, s.active) {
			return group, true
		}
	}

	return kindGroup{view: "", tabs: nil}, false
}

// documentRow reports whether the bar shows its second row: the kind in front
// has several windows open.
func (s *shell) documentRow() bool {
	group, found := s.frontKind()

	return found && len(group.tabs) > 1
}

// chooseKind brings forward the tab of a kind that was last in front, or its
// first.
func (s *shell) chooseKind(group kindGroup) int {
	if last, found := s.lastTab[group.view]; found {
		for _, index := range group.tabs {
			if s.tabs[index] == last {
				return index
			}
		}
	}

	return group.tabs[0]
}

// remember keeps the tab in front as its kind's, for when the kind is chosen
// again.
func (s *shell) remember() {
	front := s.front()
	if front == nil {
		return
	}
	if s.lastTab == nil {
		s.lastTab = map[string]*tab{}
	}
	s.lastTab[kindOf(front)] = front
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

// documentLabels names a kind's tabs on the second row by their focused
// panes: short titles, keeping the window's ID only where two would read
// the same, as two views of one map do.
func (s *shell) documentLabels(group kindGroup) []string {
	labels := make([]string, 0, len(group.tabs))
	times := map[string]int{}
	for _, index := range group.tabs {
		label := shortTitle(s.tabTitle(index), false)
		labels = append(labels, label)
		times[label]++
	}
	for at, label := range labels {
		if times[label] > 1 {
			labels[at] = shortTitle(s.tabTitle(group.tabs[at]), true)
		}
	}

	return labels
}

// tabTitle is the title of a tab's focused pane.
func (s *shell) tabTitle(index int) string {
	each := s.tabs[index]

	return each.panes[each.focus].place.title
}
