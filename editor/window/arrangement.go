package window

import (
	"encoding/json"
	"maps"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// The window's arrangement is kept for this person (increment 53): each kind
// of view's panel widths and folds, on each size of screen, in the project's
// own state folder. A width dragged or a panel folded in one view of a kind
// is that kind's, for every view of it, and is kept as it changes.

// kindWidths is a kind of view's panel widths, zero for its own.
type kindWidths struct {
	inspector, left int
}

// screenKey names the size of the screen the window is on, as "1512x982",
// or "screen" where none can be read.
func screenKey() string {
	monitor := ebiten.Monitor()
	if monitor == nil {
		return "screen"
	}
	width, height := monitor.Size()
	if width <= 0 || height <= 0 {
		return "screen"
	}

	return strconv.Itoa(width) + "x" + strconv.Itoa(height)
}

// keepArrangement takes the arrangement the orchestrator kept, and puts this
// screen's back: each kind's widths and folds, on every view of the kind.
func (s *shell) keepArrangement(value json.RawMessage) {
	var layout protocol.Layout
	if err := json.Unmarshal(value, &layout); err != nil || layout.Screens == nil {
		return
	}
	s.kept = layout
	for kind, panels := range layout.Screens[s.screen].Kinds {
		s.widths[kind] = kindWidths{inspector: panels.Inspector, left: panels.Left}
		s.folded[kind], s.leftFolded[kind] = panels.InspectorFolded, panels.LeftFolded
	}
	for _, shown := range s.panes() {
		shown.host.frame.ShowInspector.On = !s.folded[shown.view]
		shown.host.frame.ShowLeft.On = !s.leftFolded[shown.view]
	}
	s.applyWidths()
}

// followWidths takes a width a view's divider was dragged to, or put back,
// as its kind's, gives it to every view of the kind, and keeps it.
func (s *shell) followWidths() bool {
	changed := false
	for _, shown := range s.panes() {
		if shown.host.arranged {
			shown.host.arranged = false
			s.widths[shown.view] = kindWidths{inspector: shown.host.inspectorWidth, left: shown.host.leftWidth}
			changed = true
		}
	}
	s.applyWidths()

	return changed
}

// applyWidths gives every view its kind's widths, but for one whose
// divider is being dragged.
func (s *shell) applyWidths() {
	for _, shown := range s.panes() {
		if shown.host.dragging != dragNone {
			continue
		}
		widths := s.widths[shown.view]
		shown.host.inspectorWidth = widths.inspector
		if shown.host.leftWidth != widths.left {
			shown.host.setLeft(widths.left)
		}
	}
}

// publishArrangement keeps this screen's arrangement, through the project
// browser's window, which is always open.
func (s *shell) publishArrangement() {
	kinds := map[string]protocol.KindLayout{}
	for _, kind := range s.arrangedKinds() {
		kinds[kind] = protocol.KindLayout{
			Inspector: s.widths[kind].inspector, Left: s.widths[kind].left,
			InspectorFolded: s.folded[kind], LeftFolded: s.leftFolded[kind],
		}
	}
	screens := maps.Clone(s.kept.Screens)
	if screens == nil {
		screens = map[string]protocol.Arrangement{}
	}
	screens[s.screen] = protocol.Arrangement{Kinds: kinds}
	s.kept = protocol.Layout{Screens: screens}
	for _, shown := range s.panes() {
		if shown.host.closeQuits {
			shown.host.Publish(protocol.TopicLayout, s.kept)

			return
		}
	}
}

// arrangedKinds lists the kinds of view with a width or a fold to keep.
func (s *shell) arrangedKinds() []string {
	seen := map[string]bool{}
	var kinds []string
	add := func(kind string) {
		if !seen[kind] {
			seen[kind] = true
			kinds = append(kinds, kind)
		}
	}
	for kind := range s.widths {
		add(kind)
	}
	for kind := range s.folded {
		add(kind)
	}
	for kind := range s.leftFolded {
		add(kind)
	}

	return kinds
}
