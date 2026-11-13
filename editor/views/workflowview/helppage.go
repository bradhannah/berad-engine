package workflowview

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// helpNameWidth is how wide a help page's output and setting names are.
const helpNameWidth = 80

// helpTarget names what Add after puts the node after: the selected node,
// or, with none, the canvas.
func (s *State) helpTarget() (string, string) {
	node, ok := s.selectedNode()
	if !ok || s.workflow == nil {
		return "Add to the canvas", "Puts it in the middle of the canvas, joined to nothing."
	}
	label := labelOf(node.Kind)
	switch node.Kind {
	case flow.KindContainer:
		return "Add inside " + label, "Puts it inside the container, as its first step when it has none."
	case flow.KindTogether:
		return "Add inside " + label, "Puts it inside the Together, as the first step of its first empty lane."
	}

	if output, free := s.freeOutput(node); free {
		return "Add after " + label + " · " + output, "Puts it in the flow after " + label + ", joined from its " +
			output + " output."
	}

	return "Add beside " + label, label + " has no free output, so it goes beside it, joined to nothing."
}

// freeOutput is the first output of a node with no line from it, which Add
// after joins a new node from (increment 40).
func (s *State) freeOutput(node flow.Node) (string, bool) {
	drawn, ok := s.shapes[node.ID]
	if !ok {
		return "", false
	}
	for _, candidate := range drawn.ports {
		if _, used := node.Next[candidate.output.Name]; !used && !flow.IsInner(node, candidate.output.Name) {
			return candidate.output.Name, true
		}
	}

	return "", false
}

// layoutHelp places the help page's back link and Add after.
func (s *State) layoutHelp() {
	panel := s.pal
	panel.back.Rect, panel.add.Rect = ui.Zero(), ui.Zero()
	if panel.help == "" {
		return
	}
	inspector := frame.Inspector(s.size)
	left := inspector.Min.X + ui.Inset
	back := "‹ Back"
	if node, ok := s.selectedNode(); ok {
		back = "‹ Back to " + s.clip(labelOf(node.Kind), inspector.Dx()/ui.Half)
	}
	panel.back.Label = back
	panel.back.Rect = image.Rect(left, inspector.Min.Y+ui.Inset, left+s.theme.TextWidth(back)+ui.Half*ui.Inset,
		inspector.Min.Y+ui.Inset+ui.ButtonH)
	label, _ := s.helpTarget()
	panel.add.Label = label
	bottom := inspector.Max.Y - ui.Inset - ui.LineHeight*ui.Half
	panel.add.Rect = image.Rect(left, bottom-ui.ButtonH, left+s.theme.TextWidth(label)+ui.Half*ui.Inset, bottom)
}

// updateHelp handles the help page's back link and Add after, reporting
// whether the page is shown, which keeps the inspector under it still.
func (s *State) updateHelp(input *ui.Input, ctx *frame.Context) bool {
	panel := s.pal
	if panel.help == "" {
		return false
	}
	switch {
	case panel.back.Update(input):
		panel.help = ""
	case panel.add.Update(input):
		kind := panel.help
		panel.help = ""
		s.addAfterSelected(ctx, kind)
	}
	if input.Pressed && input.Cursor.In(frame.Inspector(s.size)) {
		input.ClearPointer()
	}

	return true
}

// drawHelp draws a node's help page in the inspector: its name and group,
// how long it takes, its outputs and which kinds offer it, what it does,
// each output and setting, an example and Add after.
func (s *State) drawHelp(screen *ebiten.Image) {
	panel := s.pal
	help, _ := helpFor(panel.help)
	inspector := frame.Inspector(s.size)
	left, width := inspector.Min.X+ui.Inset, inspector.Dx()-ui.Half*ui.Inset
	panel.back.Draw(screen, s.theme)
	top := panel.back.Rect.Max.Y + ui.Gap
	s.theme.Heading(screen, "About a node", left, top)
	top += ui.LineHeight
	label := labelOf(panel.help)
	kind, _ := flow.KindOf(panel.help)
	s.theme.DrawTitle(screen, label, left, top, ui.ColorInk())
	s.theme.DrawLabel(screen, kind.Group, left+s.titleWidth(label)+ui.Inset, top+ui.Gap, ui.ColorMuted())
	top += ui.LineHeight + ui.Gap
	top = s.drawWrapped(screen, helpSummary(panel.help, help), left, top, width, ui.ColorMuted())
	top = s.drawWrapped(screen, help.about, left, top+ui.Gap, width, ui.ColorInk())
	top = s.drawRows(screen, "Outputs", help.outputs, left, top, width)
	top = s.drawRows(screen, "Settings", help.settings, left, top, width)
	if help.example != "" {
		s.theme.Heading(screen, "For example", left, top+ui.Gap)
		s.drawWrapped(screen, help.example, left, top+ui.Gap+ui.LineHeight, width, ui.ColorInk())
	}
	panel.add.Draw(screen, s.theme)
	_, says := s.helpTarget()
	for index, line := range s.wrap(says, width) {
		s.theme.DrawText(screen, line, left, panel.add.Rect.Max.Y+ui.Half+index*ui.LineHeight, ui.ColorMuted())
	}
}

// drawWrapped writes text wrapped to a width, returning where the next
// line goes.
func (s *State) drawWrapped(screen *ebiten.Image, text string, left, top, width int, ink color.Color) int {
	for _, line := range s.wrap(text, width) {
		s.theme.DrawText(screen, line, left, top, ink)
		top += ui.LineHeight
	}

	return top
}

// drawRows writes a help page's outputs or settings under a heading, each
// name beside what it means, returning where the next part goes.
func (s *State) drawRows(screen *ebiten.Image, heading string, rows []helpRow, left, top, width int) int {
	if len(rows) == 0 {
		return top
	}
	top += ui.Gap
	s.theme.Heading(screen, heading, left, top)
	top += ui.LineHeight
	for _, row := range rows {
		s.theme.DrawStrong(screen, s.clip(row.name, helpNameWidth-ui.Gap), left, top, ui.ColorInk())
		top = s.drawWrapped(screen, row.what, left+helpNameWidth, top, width-helpNameWidth, ui.ColorInk())
	}

	return top
}
