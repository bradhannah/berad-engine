package workflowview

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// The node menu (increment 40): a line dropped on empty canvas opens a menu
// of nodes whose first line says what will be joined to what. Choosing one
// adds it there, joined by that line alone, as one step; Esc or a click
// elsewhere leaves the line as it was. Tab, or a double-click on empty
// canvas, opens the same menu with nothing joined.

// The node menu's choices, located as "menu:<id>".
const (
	nodeMenuHeader = "node-menu"
	nodeMenuGroup  = "node-group:"
	nodeMenuKind   = "node-kind:"
)

// nodeMenu is where the node menu adds a node, on the canvas, and the line
// it joins it by, if any.
type nodeMenu struct {
	at   image.Point
	join *flow.Link
}

// openNodeMenu opens the node menu at a point of the window, to add a node
// at a point of the canvas, joined by a line or by nothing.
func (s *State) openNodeMenu(screen, at image.Point, join *flow.Link) {
	if s.workflow == nil {
		return
	}
	s.adding = nil
	s.nodeAdd = &nodeMenu{at: at, join: join}
	items := []ui.MenuItem{{
		ID: nodeMenuHeader, Label: s.nodeMenuTitle(join), Icon: ui.IconNone, Disabled: true, Danger: false, Rule: false,
	}}
	s.contextMenu.ClearSubs()
	var groups []string
	kinds := map[string][]ui.MenuItem{}
	for _, entry := range s.palette {
		group := entry.kind.Group
		if _, seen := kinds[group]; !seen {
			groups = append(groups, group)
		}
		kinds[group] = append(kinds[group], ui.MenuItem{
			ID: nodeMenuKind + entry.kind.Name, Label: entry.kind.Label, Icon: ui.IconNone, Disabled: false, Danger: false,
			Rule: false,
		})
	}
	for index, group := range groups {
		items = append(items, ui.MenuItem{
			ID: nodeMenuGroup + group, Label: group, Icon: ui.IconNone, Disabled: false, Danger: false,
			Rule: index == 0,
		})
	}
	s.contextMenu.Items, s.contextMenu.Width = items, ui.MenuWidth(items, s.theme)
	for _, group := range groups {
		s.contextMenu.SetSub(nodeMenuGroup+group, kinds[group], ui.MenuWidth(kinds[group], s.theme))
	}
	s.contextMenu.OpenAt(screen, frame.AboveStatus(s.size))
}

// nodeMenuTitle is the node menu's first line: what a new node will be
// joined to, and the line it replaces, or that nothing is joined.
func (s *State) nodeMenuTitle(join *flow.Link) string {
	if join == nil {
		return "Add a node here, joined to nothing:"
	}
	from, ok := s.workflow.Node(join.From)
	if !ok {
		return "Add a node here:"
	}
	title := "Join " + s.nodeName(from) + " · " + join.Output + " to a new:"
	if current, linked := from.Next[join.Output]; linked {
		if target, found := s.workflow.Node(current); found {
			title += " (replaces the line to " + s.nodeName(target) + ")"
		}
	}

	return title
}

// openNodeMenuAt opens the node menu for Tab: at the pointer when it is on
// the canvas, and otherwise in the middle of the canvas, joined to nothing.
func (s *State) openNodeMenuAt(pointer image.Point) {
	canvas := s.Canvas()
	if !pointer.In(canvas) {
		pointer = ui.Center(canvas)
	}
	s.openNodeMenu(pointer, s.toCanvas(pointer), nil)
}

// nodeName names a node by its kind's label.
func (s *State) nodeName(node flow.Node) string { return labelOf(node.Kind) }

// chooseFromNodeMenu adds the kind chosen from the node menu, and reports
// whether the choice was the node menu's.
func (s *State) chooseFromNodeMenu(ctx *frame.Context, chosen string) bool {
	kind, isKind := strings.CutPrefix(chosen, nodeMenuKind)
	if !isKind {
		return false
	}
	asked := s.nodeAdd
	s.nodeAdd = nil
	if asked == nil {
		return true
	}
	if asked.join == nil {
		s.addAt(ctx, kind, asked.at)

		return true
	}
	from, ok := s.workflow.Node(asked.join.From)
	if !ok {
		return true
	}
	if flow.IsStart(kind) || !flow.Runs(kind) {
		ctx.Frame.Status("A line cannot lead to a " + labelOf(kind) + ": nothing was added")

		return true
	}
	parent := from.Parent
	if flow.IsInner(from, asked.join.Output) {
		parent = from.ID
	}
	place := asked.at.Sub(image.Pt(nodeWidth/ui.Half, nodeHeight/ui.Half))
	s.add(ctx, kind, place, parent, &flow.Link{From: from.ID, Output: asked.join.Output, To: ids.ULID{}})

	return true
}

// replacingLine reports whether a line is the one a line being drawn from
// its output would replace if let go where it is: over another node, or over
// empty canvas, where a node chosen from the menu would replace it.
func (s *State) replacingLine(key routeKey) bool {
	if s.link == nil || s.workflow == nil || s.link.from != key.from || s.link.output != key.output {
		return false
	}
	from, ok := s.workflow.Node(key.from)
	if !ok {
		return false
	}
	current, linked := from.Next[key.output]

	return linked && s.linkTarget() != current
}

// drawReplacing says beside the pointer which line letting go would
// replace, on a page-tone tag, in red.
func (s *State) drawReplacing(area *ebiten.Image) {
	from, ok := s.workflow.Node(s.link.from)
	if !ok || !s.replacingLine(routeKey{from: s.link.from, output: s.link.output}) {
		return
	}
	target, found := s.workflow.Node(from.Next[s.link.output])
	if !found {
		return
	}
	text := "Replaces the line to " + s.nodeName(target)
	if s.linkTarget().IsZero() {
		text = "A node chosen here replaces the line to " + s.nodeName(target)
	}
	at := s.toScreen(s.link.to).Add(image.Pt(ui.Inset, ui.Inset))
	tag := image.Rect(at.X, at.Y, at.X+s.theme.TextWidth(text)+ui.Half*ui.Gap, at.Y+ui.LineHeight+ui.Gap)
	ui.FloatBox(area, tag, ui.ColorPage())
	s.theme.DrawText(area, text, tag.Min.X+ui.Gap, tag.Min.Y+ui.Gap/ui.Half, ui.ColorWarn())
}

// hasReplacingState answers "replacing" while letting go of the line being drawn
// would replace the line its output has.
func (s *State) hasReplacingState(state string, _ frame.Docs) (bool, bool) {
	if state != "replacing" {
		return false, false
	}

	return s.link != nil && s.replacingLine(routeKey{from: s.link.from, output: s.link.output}), true
}

// zoomKeys zooms the canvas in, out or back for Cmd+=, Cmd+- and Cmd+0,
// and reports whether one was pressed.
func (s *State) zoomKeys(input *ui.Input) bool {
	switch {
	case input.Consume(ui.KeyEqual, ui.Cmd()):
		s.zoomTo(s.zoom+1, s.zoomFocus(input.Cursor))
	case input.Consume(ui.KeyMinus, ui.Cmd()):
		s.zoomTo(s.zoom-1, s.zoomFocus(input.Cursor))
	case input.Consume(ui.Key0, ui.Cmd()):
		s.zoomTo(defaultZoom, s.zoomFocus(input.Cursor))
	default:
		return false
	}

	return true
}

// emptyAt reports whether a point of the canvas has nothing on it: no node,
// line or dot.
func (s *State) emptyAt(point image.Point) bool {
	_, _, onPort := s.portAt(point)
	_, onEntry := s.entryAt(point)
	_, onNode := s.nodeAt(point)
	_, onLine := s.linkAt(point)

	return !onPort && !onEntry && !onNode && !onLine
}
