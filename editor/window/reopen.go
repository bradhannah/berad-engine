package window

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/playview"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The window reopens as it was left (increment 55): where it was and its
// size, its tabs in order, what sat beside what, and the tab in front. What
// could not be found is said once the rest has opened. The Play tab is not
// reopened, as opening it launches the game.

// Reopening's limits: how long a tab may take to open before it is counted
// as not found, and how long the window's frame must stay put before it is
// kept.
const (
	reopenTicks = 600
	frameSettle = 30
)

// reopening is the tabs still to reopen: the panes in order, each with its
// tab's place in the kept list, the pane waited for, the tabs opened by
// their kept place, and what could not be found.
type reopening struct {
	steps   []reopenStep
	at      int
	waiting bool
	failed  bool
	ticks   int
	tabs    map[int]*tab
	splits  map[int]float64
	active  int
	missing []string
}

// reopenStep is one pane to reopen, and the kept tab it belongs in.
type reopenStep struct {
	pane  protocol.PaneLayout
	tab   int
	first bool
}

// reopenAsLeft puts the window's frame back and starts reopening its tabs,
// unless an input script drives the editor, which starts as tests expect.
func (s *shell) reopenAsLeft(kept protocol.Arrangement) {
	if kept.Frame.Width > 0 && kept.Frame.Height > 0 && !scripted() {
		ebiten.SetWindowPosition(kept.Frame.X, kept.Frame.Y)
		ebiten.SetWindowSize(kept.Frame.Width, kept.Frame.Height)
		s.frameNow = kept.Frame
	}
	plan := &reopening{
		steps: nil, at: 0, waiting: false, failed: false, ticks: 0, tabs: map[int]*tab{}, splits: map[int]float64{},
		active: kept.Active, missing: nil,
	}
	for place, each := range kept.Tabs {
		plan.splits[place] = each.Split
		for slot, shown := range each.Panes {
			if shown.View == browser.ViewName || shown.View == playview.ViewName {
				continue
			}
			plan.steps = append(plan.steps, reopenStep{pane: shown, tab: place, first: slot == 0})
		}
	}
	if len(plan.steps) > 0 {
		s.reopen = plan
	}
}

// reopenNext opens the next pane once the one before has opened or could
// not be found, and finishes once none is left.
func (s *shell) reopenNext() {
	plan := s.reopen
	if plan == nil {
		return
	}
	if plan.waiting && !s.reopenSettled(plan) {
		return
	}
	if plan.at >= len(plan.steps) {
		s.finishReopening(plan)

		return
	}
	step := plan.steps[plan.at]
	shown, _, ok := s.browserPane()
	if !ok {
		return
	}
	if beside, has := plan.tabs[step.tab]; has && !step.first {
		s.beside = &besideAsk{view: step.pane.View, target: step.pane.Target, tab: beside, ticks: reopenTicks}
	}
	plan.waiting, plan.failed, plan.ticks = true, false, 0
	shown.host.openThen(step.pane.View, step.pane.Target, func(err error) { plan.failed = err != nil })
}

// reopenSettled reports whether the pane waited for has opened, or could
// not be, noting which.
func (s *shell) reopenSettled(plan *reopening) bool {
	step := plan.steps[plan.at]
	plan.ticks++
	for _, each := range s.tabs {
		for _, shown := range each.panes {
			if shown.view == step.pane.View && shown.target == step.pane.Target {
				if _, kept := plan.tabs[step.tab]; !kept {
					plan.tabs[step.tab] = each
				}
				plan.waiting = false
				plan.at++

				return true
			}
		}
	}
	if plan.failed || plan.ticks > reopenTicks {
		plan.missing = append(plan.missing, viewKind(step.pane.View))
		plan.waiting = false
		plan.at++

		return true
	}

	return false
}

// finishReopening puts each split back as it was shared, brings the tab
// that was in front forward, and says what could not be found.
func (s *shell) finishReopening(plan *reopening) {
	s.reopen = nil
	for place, opened := range plan.tabs {
		if share := plan.splits[place]; share > 0 && len(opened.panes) >= splitPanes {
			opened.split = max(minSplit, min(maxSplit, share))
		}
	}
	if front, ok := plan.tabs[plan.active]; ok {
		if index := slices.Index(s.tabs, front); index >= 0 {
			s.active = index
		}
	}
	said := "Reopened as it was left"
	if len(plan.missing) > 0 {
		said += ". Could not find " + strings.Join(plan.missing, ", ") + ": each is no longer in the project"
	}
	if focused := s.focused(); focused != nil {
		focused.host.frame.Status(said)
	}
}

// viewKind names a kind of view for saying what could not be found.
func viewKind(view string) string {
	names := map[string]string{
		"map": "a Map", "workflow": "a Workflow", "watch": "a Watch", "pixel": "a sprite",
		"palette": "a palette", "audio": "the Audio window", "template": "a template",
	}
	if name, ok := names[view]; ok {
		return name
	}

	return "a " + view
}

// tabsNow lists the tabs as they are, to reopen.
func (s *shell) tabsNow() []protocol.TabLayout {
	tabs := make([]protocol.TabLayout, 0, len(s.tabs))
	for _, each := range s.tabs {
		kept := protocol.TabLayout{Panes: nil, Split: 0}
		for _, shown := range each.panes {
			kept.Panes = append(kept.Panes, protocol.PaneLayout{View: shown.view, Target: shown.target})
		}
		if len(each.panes) >= splitPanes {
			kept.Split = each.split
		}
		tabs = append(tabs, kept)
	}

	return tabs
}

// tabsSaid writes the tabs as they are, to tell when they change.
func (s *shell) tabsSaid() string {
	var said strings.Builder
	for index, each := range s.tabs {
		if index == s.active {
			said.WriteString("*")
		}
		for _, shown := range each.panes {
			said.WriteString(shown.view + ":" + shown.target.String() + " ")
		}
		said.WriteString(strconv.FormatFloat(each.split, 'f', 2, 64) + "|")
	}

	return said.String()
}

// followFrame keeps the window's frame and its tabs as they change: the tabs
// at once, the frame once it has stayed put a moment. It keeps nothing while
// tabs are being reopened, nor once the editor is quitting and its panes
// close.
func (s *shell) followFrame() {
	if !s.keptArrived || s.reopen != nil || s.quitting {
		return
	}
	changed := false
	if said := s.tabsSaid(); said != s.tabsKept {
		s.tabsKept, changed = said, true
	}
	x, y := ebiten.WindowPosition()
	width, height := ebiten.WindowSize()
	now := protocol.WindowFrame{X: x, Y: y, Width: width, Height: height}
	switch {
	case now != s.frameSeen:
		s.frameSeen, s.frameTicks = now, 0
	case now != s.frameNow:
		s.frameTicks++
		if s.frameTicks >= frameSettle {
			s.frameNow, changed = now, true
		}
	}
	if changed {
		s.publishArrangement()
	}
}

// openThen asks for a window, and tells done whether it could be opened.
func (h *host) openThen(view string, target ids.ULID, done func(error)) {
	h.background(func(ctx context.Context) func() {
		_, err := h.conn.Open(ctx, view, target, false)

		return func() { done(err) }
	})
}
