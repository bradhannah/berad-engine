package window

import (
	"slices"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/ui"
)

// Back and Forward (increment 38): Cmd+[ and Cmd+] step through where the
// author has been, each place a tab and what was selected in its focused
// pane, where the view can say. Back puts a selection back only where
// nothing waits for Save, since taking a selection away could leave a
// typed draft behind; otherwise the tab comes forward as it is and the
// status bar says so.

// Trail limits: the places kept, and the frames a step waits for the view to
// show the selection it put back before taking what shows as the place.
const (
	trailLength = 100
	settleTicks = 30
)

// place is one place on the trail: a tab, and what was selected in it.
type place struct {
	tab  *tab
	item string
}

// trail is where the author has been. at is the place now; settling is set
// for a few frames after a step, while the view shows what it put back.
type trail struct {
	places   []place
	at       int
	settling int
}

// selector is a view that says what is selected in it as an item it can be
// asked to show, as a Workflow says "node:<ID>".
type selector interface {
	Selection() string
}

// placeNow is the tab in front and what its focused pane has selected.
func (s *shell) placeNow() (place, bool) {
	front := s.front()
	if front == nil {
		return place{tab: nil, item: ""}, false
	}
	item := ""
	if shown := front.panes[front.focus]; shown.host != nil {
		if view, ok := shown.host.view.(selector); ok {
			item = view.Selection()
		}
	}

	return place{tab: front, item: item}, true
}

// noteTrail adds the place now to the trail when it is a new one: another
// tab, or something else selected in the same tab. Selecting nothing makes
// no place of its own, and a place with nothing selected takes the first
// selection made in it. Closed tabs leave the trail.
func (s *shell) noteTrail() {
	s.dropClosedPlaces()
	now, ok := s.placeNow()
	if !ok {
		return
	}
	if len(s.trail.places) == 0 {
		s.trail.places, s.trail.at = []place{now}, 0

		return
	}
	last := &s.trail.places[s.trail.at]
	if s.trail.settling > 0 {
		switch {
		case now == *last:
			s.trail.settling = 0

			return
		case now.tab == last.tab:
			// The view has not shown what was put back yet; if it never
			// does, what it shows is the place.
			s.trail.settling--
			if s.trail.settling == 0 {
				*last = now
			}

			return
		default:
			s.trail.settling = 0
		}
	}
	switch {
	case now == *last:
	case now.tab == last.tab && now.item == "":
	case now.tab == last.tab && last.item == "":
		last.item = now.item
	default:
		s.trail.places = append(s.trail.places[:s.trail.at+1], now)
		if len(s.trail.places) > trailLength {
			s.trail.places = s.trail.places[len(s.trail.places)-trailLength:]
		}
		s.trail.at = len(s.trail.places) - 1
	}
}

// dropClosedPlaces takes the places of closed tabs off the trail, and joins
// places left side by side that are the same.
func (s *shell) dropClosedPlaces() {
	kept := s.trail.places[:0]
	now := 0
	for index, each := range s.trail.places {
		if !slices.Contains(s.tabs, each.tab) || (len(kept) > 0 && kept[len(kept)-1] == each) {
			continue
		}
		kept = append(kept, each)
		if index <= s.trail.at {
			now = len(kept) - 1
		}
	}
	s.trail.places, s.trail.at = kept, now
}

// trailKeys handles Cmd+[ and Cmd+], saying why when there is nowhere to go.
func (s *shell) trailKeys(input *ui.Input, status func(string)) {
	switch {
	case input.ConsumeExact(ui.KeyBracketLeft, ui.Cmd()):
		s.step(-1, status)
	case input.ConsumeExact(ui.KeyBracketRight, ui.Cmd()):
		s.step(1, status)
	}
}

// step goes back or forward one place on the trail.
func (s *shell) step(by int, status func(string)) {
	s.noteTrail()
	next := s.trail.at + by
	switch {
	case next < 0:
		status("Nothing to go back to")

		return
	case next >= len(s.trail.places):
		status("Nothing to go forward to")

		return
	}
	s.trail.at, s.trail.settling = next, settleTicks
	s.goTo(s.trail.places[next])
}

// goTo brings a place's tab forward and asks its focused pane to show what
// was selected there, unless that pane holds changes that wait for Save,
// which its status bar then says.
func (s *shell) goTo(where place) {
	s.choose(slices.Index(s.tabs, where.tab))
	shown := where.tab.panes[where.tab.focus]
	if where.item == "" || shown.host == nil {
		return
	}
	if view, ok := shown.host.view.(selector); ok && view.Selection() == where.item {
		return
	}
	if shown.host.reported.Count > 0 {
		shown.host.frame.Status(keptForSave)

		return
	}
	var update client.Update
	update.Attention, update.Item = true, where.item
	shown.host.view.Changed(update, &shown.host.ctx)
}
