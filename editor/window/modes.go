package window

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// Arrangements by screen and by mode (increment 54). The screen's size
// chooses a kind of view's starting widths once, the first time it opens on
// that size of screen, and the status bar says so; after that the kind is
// as it was left. A Map keeps two arrangements, one for editing and one for
// previewing, each as it was last left in that mode. The window stops
// narrowing rather than squeeze a panel.

// previewer is a view that previews, as a Map does.
type previewer interface {
	Previewing() bool
}

// previewSuffix names a kind's arrangement while it previews.
const previewSuffix = ":preview"

// Screen widths, in pixels, at and past which panels start narrower or
// wider, and the widths they start at.
const (
	narrowScreen     = 1440
	wideScreen       = 2200
	narrowInspector  = 250
	narrowLeft       = 170
	wideInspector    = 340
	wideLeft         = 230
	windowMinHeight  = 480
	startingSaidWith = "Panels start at widths for this %d × %d screen: drag a divider to change them."
)

// arrangement is the arrangement a pane's panels follow: its kind's, or its
// kind's while previewing.
func (s *shell) arrangement(shown *pane) string {
	if view, ok := shown.host.view.(previewer); ok && view.Previewing() {
		return shown.view + previewSuffix
	}

	return shown.view
}

// switchArrangements puts each pane whose arrangement changed, a pane new
// or a Map starting or stopping a preview, as its arrangement was left. An
// arrangement this screen has not had yet starts as the pane is, or, for a
// kind's first, at the widths the screen's size chooses. It waits for the
// kept arrangement, and reports whether one was started.
func (s *shell) switchArrangements() bool {
	if !s.keptArrived {
		return false
	}
	started := false
	for _, shown := range s.panes() {
		key := s.arrangement(shown)
		if shown.arrangedAs == key {
			continue
		}
		shown.arrangedAs = key
		if !s.known[key] {
			s.startArrangement(shown, key)
			started = true

			continue
		}
		shown.host.frame.ShowInspector.On = !s.folded[key]
		shown.host.frame.ShowLeft.On = !s.leftFolded[key]
		widths := s.widths[key]
		shown.host.inspectorWidth = widths.inspector
		shown.host.setLeft(widths.left)
	}

	return started
}

// startArrangement starts an arrangement this screen has not had: a mode's
// as the pane is now, and a kind's first at the screen's widths, saying so.
func (s *shell) startArrangement(shown *pane, key string) {
	s.known[key] = true
	s.folded[key], s.leftFolded[key] = !shown.host.frame.ShowInspector.On, !shown.host.frame.ShowLeft.On
	if key != shown.view {
		s.widths[key] = kindWidths{inspector: shown.host.inspectorWidth, left: shown.host.leftWidth}

		return
	}
	size := screenSize()
	widths := startingWidths(size)
	if scripted() {
		// An input script's run starts at the views' own widths, so the
		// real-window walks do not depend on the screen they run on.
		widths = kindWidths{inspector: 0, left: 0}
	}
	s.widths[key] = widths
	shown.host.inspectorWidth = widths.inspector
	shown.host.setLeft(widths.left)
	if widths != (kindWidths{inspector: 0, left: 0}) {
		shown.host.frame.Status(fmt.Sprintf(startingSaidWith, size.X, size.Y))
	}
}

// startingWidths are the panel widths a screen's size chooses: narrower on
// a small screen, wider on a large one, and the views' own between.
func startingWidths(screen image.Point) kindWidths {
	switch {
	case screen.X >= wideScreen:
		return kindWidths{inspector: wideInspector, left: wideLeft}
	case screen.X > 0 && screen.X <= narrowScreen:
		return kindWidths{inspector: narrowInspector, left: narrowLeft}
	}

	return kindWidths{inspector: 0, left: 0}
}

// screenSize is the size of the screen the window is on, or none.
func screenSize() image.Point {
	monitor := ebiten.Monitor()
	if monitor == nil {
		return image.Pt(0, 0)
	}
	width, height := monitor.Size()

	return image.Pt(width, height)
}

// limitWindow keeps the window from narrowing past what the tab in front
// needs: each pane's left panel, inspector and the canvas's room, so a
// window made narrower never squeezes a panel.
func (s *shell) limitWindow() {
	front := s.front()
	if front == nil {
		return
	}
	// A limit past the window's width now would grow it, which nobody
	// asked for: it only stops the window narrowing further.
	need := min(neededWidth(front), s.size.X)
	if need == s.minWidth || need <= 0 {
		return
	}
	s.minWidth = need
	ebiten.SetWindowSizeLimits(need, windowMinHeight, -1, -1)
}

// neededWidth is the narrowest a tab's panes go: each one's left panel,
// inspector and the canvas's room, and the divider between two.
func neededWidth(front *tab) int {
	need := 0
	for _, shown := range front.panes {
		need += shown.host.leftEdge() + shown.host.inspectorRoom() + minCanvas
	}
	if len(front.panes) >= splitPanes {
		need += dividerWidth
	}

	return need
}

// inspectorRoom is the room a pane's inspector takes: its width, or the
// folded strip.
func (h *host) inspectorRoom() int {
	switch {
	case h.folded():
		return frame.FoldedWidth
	case h.inspectorWidth > 0:
		return h.inspectorWidth
	}

	return frame.InspectorWidth
}
