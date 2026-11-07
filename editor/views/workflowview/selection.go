package workflowview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// groupHint is what the inspector says while several nodes are selected.
const groupHint = "Cmd+click adds or removes a node. Drag any of them to move them all."

// selection is what is selected on the canvas: the nodes of a group, the one
// node selected, or none.
func (s *State) selection() []ids.ULID {
	if len(s.group) > 0 {
		return s.group
	}
	if !s.selected.IsZero() {
		return []ids.ULID{s.selected}
	}

	return nil
}

// isSelected reports whether a node is selected, alone or in a group.
func (s *State) isSelected(id ids.ULID) bool { return slices.Contains(s.selection(), id) }

// selectNodes selects nodes the workflow holds: none; one, as a click
// does; or several, as a group, which no line or single node is selected
// beside.
func (s *State) selectNodes(chosen []ids.ULID) {
	var kept []ids.ULID
	for _, id := range chosen {
		if _, ok := s.workflow.Node(id); ok && !slices.Contains(kept, id) {
			kept = append(kept, id)
		}
	}
	s.selectedLink = noRoute()
	switch len(kept) {
	case 0:
		s.selected, s.group = ids.ULID{}, nil
	case 1:
		s.selected, s.group = kept[0], nil
	default:
		s.selected, s.group = ids.ULID{}, kept
	}
}

// toggleSelected adds a node to the selection, or takes it out, as
// Cmd+click does.
func (s *State) toggleSelected(id ids.ULID) {
	chosen := slices.Clone(s.selection())
	if index := slices.Index(chosen, id); index >= 0 {
		chosen = slices.Delete(chosen, index, index+1)
	} else {
		chosen = append(chosen, id)
	}
	s.selectNodes(chosen)
}

// settleSelection keeps either one node or a group selected, never both,
// and drops from a group what the workflow no longer holds.
func (s *State) settleSelection() {
	switch {
	case s.workflow == nil:
		s.group = nil
	case !s.selected.IsZero() || s.selectedLink != noRoute():
		s.group = nil
	case len(s.group) > 0:
		s.selectNodes(s.group)
	}
}

// pressCanvas handles a press on the canvas: with Cmd held it changes the
// selection; otherwise it starts what a press there begins, or opens what a
// Call calls on a double-click.
func (s *State) pressCanvas(input *ui.Input, ctx *frame.Context, point image.Point) {
	if input.Held.Shortcut {
		s.pressWithCmd(point)

		return
	}
	s.pressOrOpen(ctx, point, input.DoubleClick)
}

// pressWithCmd handles a press with Cmd held: on a node, it adds the node
// to the selection or takes it out.
func (s *State) pressWithCmd(point image.Point) {
	if hit, ok := s.nodeAt(point); ok {
		s.toggleSelected(hit)

		return
	}
	s.startBox(point, true)
}

// drawSelection rings each selected node.
func (s *State) drawSelection(area *ebiten.Image) {
	for _, id := range s.selection() {
		if selected, ok := s.shapes[id]; ok {
			ui.StrokeRound(area, s.screenRect(selected.rect).Inset(-selectGap), ui.Radius+selectGap, selectWidth,
				ui.ColorSelect())
		}
	}
}

// groupTitle says how many nodes are selected.
func groupTitle(count int) string { return strconv.Itoa(count) + " nodes" }

// drawGroupInspector shows what a selection of several nodes offers: how
// many there are, and how to change it.
func (s *State) drawGroupInspector(screen *ebiten.Image, left, top int) {
	panel := frame.Inspector(s.size)
	s.theme.Heading(screen, "Selected", left, top)
	s.theme.DrawTitle(screen, groupTitle(len(s.group)), left, top+ui.LineHeight, ui.ColorInk())
	s.drawColours(screen)
	bottom := panel.Max.Y - ui.Inset
	if s.removeGroup != nil && !s.removeGroup.Rect.Empty() {
		s.removeGroup.Draw(screen, s.theme)
		bottom = s.removeGroup.Rect.Min.Y - ui.Gap
	}
	hint := s.wrap(groupHint, panel.Dx()-ui.Half*ui.Inset)
	for index, line := range hint {
		s.theme.DrawText(screen, line, left, bottom-(len(hint)-index)*ui.LineHeight, ui.ColorMuted())
	}
}

// hasSelectionState answers "selection=N" when N nodes are selected, and
// "in-selection:<kind>:<n>" while the nth node of a kind is one of them.
func (s *State) hasSelectionState(state string) (bool, bool) {
	if count, asked := strings.CutPrefix(state, "selection="); asked {
		return strconv.Itoa(len(s.selection())) == count, true
	}
	if which, asked := strings.CutPrefix(state, "in-selection:"); asked {
		drawn, ok := s.nthNode(which)

		return ok && s.isSelected(drawn.node.ID), true
	}

	return false, false
}

// groupDeleteButton is the inspector's Delete N nodes, drawn as a
// destructive action.
func groupDeleteButton() *ui.Button {
	button := ui.NewButton("delete-group", "")
	button.Danger = true

	return button
}
