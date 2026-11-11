package window

import (
	"slices"
	"strconv"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// Keys move between tabs from any tab (increment 38): Cmd+1 to Cmd+9 choose
// a tab by its place on the bar, Ctrl+Tab the tab used before the one in
// front, and Cmd+Shift+[ and Cmd+Shift+] the tabs either side of it. A key
// with nowhere to go changes nothing and says why. Each pane's host hands
// these keys to the shell before its view sees them.

// lastPosition is the highest tab Cmd and a digit chooses.
const lastPosition = 9

// tabKeys handles the keys that move between tabs, pressed in a pane whose
// status bar says why when one has nowhere to go.
func (s *shell) tabKeys(input *ui.Input, status func(string)) {
	for position := 1; position <= lastPosition; position++ {
		if input.ConsumeExact(ui.Key(strconv.Itoa(position)), ui.Cmd()) {
			s.choosePosition(position, status)

			return
		}
	}
	switch {
	case input.ConsumeExact(ui.KeyTab, ui.Mods{Shortcut: false, Shift: false, Alt: false, Control: true}),
		input.ConsumeExact(ui.KeyTab, ui.Mods{Shortcut: true, Shift: false, Alt: false, Control: true}):
		s.chooseLastUsed(status)
	case input.ConsumeExact(ui.KeyBracketLeft, ui.CmdShift()):
		s.chooseNeighbour(-1, status)
	case input.ConsumeExact(ui.KeyBracketRight, ui.CmdShift()):
		s.chooseNeighbour(1, status)
	}
}

// choosePosition brings forward the tab at a place on the bar, counted from
// one.
func (s *shell) choosePosition(position int, status func(string)) {
	if position > len(s.tabs) {
		status("There is no tab " + strconv.Itoa(position) + ": " + tabCount(len(s.tabs)) + " open")

		return
	}
	s.choose(position - 1)
}

// tabCount says how many tabs there are, as "1 tab" or "4 tabs".
func tabCount(count int) string {
	if count == 1 {
		return "1 tab is"
	}

	return strconv.Itoa(count) + " tabs are"
}

// chooseLastUsed brings forward the tab that was in front before the one in
// front now.
func (s *shell) chooseLastUsed(status func(string)) {
	s.noteUsed()
	if len(s.used) < ui.Half {
		status("No other tab has been in front yet")

		return
	}
	s.choose(slices.Index(s.tabs, s.used[1]))
}

// chooseNeighbour brings forward the tab before the one in front, or after
// it. The first and last tabs have no neighbour past them.
func (s *shell) chooseNeighbour(step int, status func(string)) {
	next := s.active + step
	switch {
	case next < 0:
		status("This is the first tab")
	case next >= len(s.tabs):
		status("This is the last tab")
	default:
		s.choose(next)
	}
}

// noteUsed keeps the tabs in the order they were last in front, the tab in
// front first, and forgets those that closed.
func (s *shell) noteUsed() {
	s.used = slices.DeleteFunc(s.used, func(each *tab) bool { return !slices.Contains(s.tabs, each) })
	front := s.front()
	if front == nil || (len(s.used) > 0 && s.used[0] == front) {
		return
	}
	s.used = slices.DeleteFunc(s.used, func(each *tab) bool { return each == front })
	s.used = slices.Insert(s.used, 0, front)
}

// frontNamed reports whether the tab in front has a title as the bar shows
// it; it answers "state:front:<title>".
func (s *shell) frontNamed(title string) bool {
	return s.front() != nil && shortTitle(s.tabTitle(s.active), false) == title
}
