package workflowview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
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

// drawGroupInspector shows what a selection of several nodes offers: how
// many there are, and how to change it.
func (s *State) drawGroupInspector(screen *ebiten.Image, left, top int) {
	panel := frame.Inspector(s.size)
	s.theme.Heading(screen, "Selected", left, top)
	s.theme.DrawTitle(screen, countNodes(len(s.group)), left, top+ui.LineHeight, ui.ColorInk())
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

// removable are the selected nodes Delete removes: all but the start, and
// none a selected container takes with it.
func (s *State) removable() []ids.ULID {
	var chosen []ids.ULID
	for _, id := range s.movers() {
		if id != s.workflow.Start() {
			chosen = append(chosen, id)
		}
	}

	return chosen
}

// countNodes says how many nodes: "1 node", "3 nodes".
func countNodes(count int) string {
	if count == 1 {
		return "1 node"
	}

	return strconv.Itoa(count) + " nodes"
}

// groupColourRow is the colour row of several nodes: choosing a tint gives
// it to each of them but notes, which take none. The tint they share is
// ringed; none is when they differ.
func (s *State) groupColourRow() []colourRow {
	current, shared, found := "", true, false
	for _, id := range s.group {
		node, ok := s.workflow.Node(id)
		if !ok || node.Kind == flow.KindNote {
			continue
		}
		name := nameOrNone(s.tints.Nodes[id])
		if found && name != current {
			shared = false
		}
		current, found = name, true
	}
	if !found {
		return nil
	}
	if !shared {
		current = ""
	}

	return []colourRow{{
		title: colourTitle, note: "", kind: rowGroup, node: ids.ULID{}, key: "", current: current,
		swatches: plainSwatches(), left: 0, label: 0,
	}}
}

// groupTints gives a tint to each selected node but notes.
func (s *State) groupTints(value string) map[ids.ULID]string {
	nodes := map[ids.ULID]string{}
	for _, id := range s.group {
		if node, ok := s.workflow.Node(id); ok && node.Kind != flow.KindNote {
			nodes[id] = value
		}
	}

	return nodes
}

// layoutGroup places the colour row and Delete N nodes while several nodes
// are selected.
func (s *State) layoutGroup(left, width, top int) {
	s.removeGroup.Rect = ui.Zero()
	if len(s.group) == 0 {
		return
	}
	s.colours = s.layoutColours(s.groupColourRow(), left, width, top+ui.Half*ui.LineHeight+fieldSpace)
	if count := len(s.removable()); count > 0 {
		panel := frame.Inspector(s.size)
		bottom := panel.Max.Y - ui.Inset
		s.removeGroup.Label = "Delete " + countNodes(count)
		s.removeGroup.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
	}
}

// updateGroup handles Delete N nodes.
func (s *State) updateGroup(input *ui.Input, ctx *frame.Context) {
	if !s.removeGroup.Rect.Empty() && s.removeGroup.Update(input) {
		s.removeSelected(ctx)
	}
}

// removeGroupNodes removes the selected nodes but the start, with what
// their containers hold, as one undo step.
func (s *State) removeGroupNodes(ctx *frame.Context) {
	removed := s.removable()
	if len(removed) == 0 {
		return
	}
	s.selectNodes(nil)
	args := workflowplugin.RemoveArgs{Nodes: removed}
	s.command(ctx, workflowplugin.RemoveEdit, "", args, func(protocol.Result) {
		ctx.Frame.Status(countNodes(len(removed)) + " removed. Undo puts them back.")
	})
}
