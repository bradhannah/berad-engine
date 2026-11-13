package window

import (
	"context"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The editor window is asked to show panes through a connection of its own
// (increment 60), not the Project tab's, so opening a document and Launch
// keep working while the Project tab is closed, as after it crashed. A
// Project tab reopened from its crash notice is the Project tab again: it
// stands first in the row, and closing it quits the editor.

// connectHosting connects the editor window itself.
func connectHosting(ctx context.Context, args Args, registry *plugin.Registry) (*client.Client, error) {
	return connect(ctx, Args{Connect: args.Connect, View: browser.ViewName, Target: args.Target, Window: ids.ULID{}},
		registry, true)
}

// drainHosting shows each pane the window's own connection was asked to
// show, and lets the rest of what it is told go by. Asked to close, as the
// editor quits, it closes.
func (s *shell) drainHosting() {
	for s.hosting != nil {
		select {
		case update, open := <-s.hosting.Updates():
			switch {
			case !open:
				s.hosting = nil
			case update.Shutdown != nil:
				if err := s.hosting.Close(); err != nil {
					logLine("close the window's connection: " + err.Error())
				}
				s.hosting = nil
			case update.Host != nil:
				s.host(*update.Host)
			}
		default:
			return
		}
	}
}

// becomeProject makes a Project tab reopened after it closed the Project tab
// again: first in the row, closing it quits the editor.
func (s *shell) becomeProject(shown *pane) {
	if shown.view != browser.ViewName {
		return
	}
	shown.host.closeQuits = true
	for index, each := range s.tabs {
		if len(each.panes) == 1 && each.panes[0] == shown && index > 0 {
			s.tabs = append(append([]*tab{each}, s.tabs[:index]...), s.tabs[index+1:]...)
			s.active = 0
		}
	}
}
