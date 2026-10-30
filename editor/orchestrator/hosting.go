package orchestrator

import (
	"errors"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ErrNoWindow reports an Open request while the editor window is not
// connected, so there is nowhere to show what was asked for.
var ErrNoWindow = errors.New("the editor window is not open")

// hosting is what the orchestrator knows of the editor window (increment
// 13): it shows every window the orchestrator opens as a pane, each with a
// connection of its own, so to the orchestrator a pane is a window.
type hosting struct {
	// shell is the editor window's own connection, nil when it is not
	// connected.
	shell *window
	// panes are the windows asked of the editor window, until each leaves.
	panes map[ids.ULID]hostedPane
}

// hostedPane is a window the editor window was asked to show.
type hostedPane struct {
	view   string
	target ids.ULID
}

func newHosting() *hosting {
	return &hosting{shell: nil, panes: map[ids.ULID]hostedPane{}}
}

// hostJoined notes the editor window joining.
func (o *Orchestrator) hostJoined(joined *window) {
	if joined.hello.Hosts {
		o.hosting.shell = joined
	}
}

// hostLeft forgets a pane, or the editor window, that left.
func (o *Orchestrator) hostLeft(left *window) {
	if o.hosting.shell == left {
		o.hosting.shell = nil
	}
	delete(o.hosting.panes, left.id)
}

// launch asks the editor window to show a window as one of its panes, under a
// window ID it then connects with.
func (o *Orchestrator) launch(view string, target ids.ULID) (ids.ULID, error) {
	if o.hosting.shell == nil {
		return ids.ULID{}, ErrNoWindow
	}
	window, err := o.ids.New()
	if err != nil {
		return ids.ULID{}, fmt.Errorf("window id: %w", err)
	}
	o.hosting.panes[window] = hostedPane{view: view, target: target}
	o.send(o.hosting.shell, mustMessage(protocol.KindHost, 0, protocol.Host{View: view, Target: target, Window: window}))

	return window, nil
}

// hostedView reports whether the editor window was asked for a window of a
// view that has not left yet.
func (o *Orchestrator) hostedView(view string) bool {
	for _, pane := range o.hosting.panes {
		if pane.view == view {
			return true
		}
	}

	return false
}
