package orchestrator

import (
	"fmt"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// hosting is what the orchestrator knows of a shell (increment 10): a window
// whose process shows the windows the orchestrator opens as its panes, each
// with a connection of its own, so to the orchestrator a pane is a window.
type hosting struct {
	// shell is the connected shell, nil when none is; window is the shell's
	// window, kept after it leaves so that its process ending quits the
	// editor.
	shell  *window
	window ids.ULID
	// panes are the windows asked of the shell, until each leaves.
	panes map[ids.ULID]LaunchRequest
	// popOuts are windows opened as processes of their own for a pane, each
	// naming the pane to close once the window has joined.
	popOuts map[ids.ULID]ids.ULID
}

func newHosting() *hosting {
	return &hosting{shell: nil, window: ids.ULID{}, panes: map[ids.ULID]LaunchRequest{}, popOuts: map[ids.ULID]ids.ULID{}}
}

// hostJoined notes a shell joining, and closes the pane a window that has
// just joined was popped out of: the window is there first, so the undo list
// they share carries over.
func (o *Orchestrator) hostJoined(joined *window) {
	if joined.hello.Hosts {
		o.hosting.shell, o.hosting.window = joined, joined.id
	}
	pane, ok := o.hosting.popOuts[joined.id]
	if !ok {
		return
	}
	delete(o.hosting.popOuts, joined.id)
	if left, open := o.windows[pane]; open {
		o.send(left, mustMessage(protocol.KindShutdown, 0, protocol.Shutdown{
			Reason: "moved to a window of its own", Save: true,
		}))
	}
}

// hostLeft forgets a pane or the shell that left.
func (o *Orchestrator) hostLeft(left *window) {
	if o.hosting.shell == left {
		o.hosting.shell = nil
	}
	delete(o.hosting.panes, left.id)
}

// hostWindow asks the shell to show a window as one of its panes, under a
// window ID it then connects with.
func (o *Orchestrator) hostWindow(view string, target ids.ULID) (ids.ULID, error) {
	window, err := o.ids.New()
	if err != nil {
		return ids.ULID{}, fmt.Errorf("window id: %w", err)
	}
	o.hosting.panes[window] = LaunchRequest{
		View: view, Target: target, Window: window, Socket: o.cfg.SocketPath, Build: o.cfg.Build, Shell: false,
	}
	o.send(o.hosting.shell, mustMessage(protocol.KindHost, 0, protocol.Host{View: view, Target: target, Window: window}))

	return window, nil
}

// popOut opens a window as a process of its own for a shell's pane, which
// closes once the window has joined.
func (o *Orchestrator) popOut(view string, target, pane ids.ULID) (ids.ULID, error) {
	window, err := o.start(view, target, false)
	if err != nil {
		return ids.ULID{}, err
	}
	o.hosting.popOuts[window] = pane

	return window, nil
}

// hostedView reports whether the shell was asked for a window of a view that
// has not left yet.
func (o *Orchestrator) hostedView(view string) bool {
	for _, request := range o.hosting.panes {
		if request.View == view {
			return true
		}
	}

	return false
}

// shellExited quits the editor when the shell's process ends: closing the
// project browser closes the editor (D10). Any windows of their own still
// open are told to save and close.
func (o *Orchestrator) shellExited(exited ids.ULID) {
	if exited.IsZero() || exited != o.hosting.window {
		return
	}
	o.quitting = true
	for _, open := range o.windows {
		o.send(open, mustMessage(protocol.KindShutdown, 0, protocol.Shutdown{
			Reason: "the project browser closed", Save: true,
		}))
	}
}
