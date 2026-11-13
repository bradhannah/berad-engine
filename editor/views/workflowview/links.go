package workflowview

import (
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// The inspector writes out a node's links (increment 48): each line that
// joins to it, by the node and output it comes from, and where each of its
// outputs goes, with Unlink beside each line. An output joined to nothing
// says where the flow goes from it, as the canvas does ("→ start"), with
// no Unlink; one that leads inside a node says it leads to nothing.

// Links' sizes and words.
const (
	linksHeading = "LINKS"
	unlinkWidth  = 64
	linkIDPrefix = "unlink:"
	textNothing  = "nothing"
	linkArrow    = " → "
)

// linkRow is one line of the inspector's links: the line, what it says,
// where it is drawn, and its Unlink, nil for an output joined to nothing.
type linkRow struct {
	key    routeKey
	text   string
	top    int
	unlink *ui.Button
}

// linkRows lists the lines joining to a node, then each of its outputs.
func (s *State) linkRows(node flow.Node) []linkRow {
	var rows []linkRow
	for _, from := range s.workflow.Nodes() {
		for _, output := range flow.Outputs(from) {
			if from.Next[output.Name] != node.ID {
				continue
			}
			key := routeKey{from: from.ID, output: output.Name}
			rows = append(rows, linkRow{
				key: key, text: "From " + labelOf(from.Kind) + outputSuffix(from, output), top: 0,
				unlink: ui.NewButton(linkIDPrefix+linkName(key), "Unlink"),
			})
		}
	}
	for _, output := range flow.Outputs(node) {
		key := routeKey{from: node.ID, output: output.Name}
		row := linkRow{key: key, text: output.Label + " " + s.endsAt(node, output.Name), top: 0, unlink: nil}
		if flow.IsInner(node, output.Name) {
			row.text = output.Label + linkArrow + textNothing
		}
		if target, ok := s.workflow.Node(node.Next[output.Name]); ok {
			row.text = output.Label + linkArrow + labelOf(target.Kind)
			row.unlink = ui.NewButton(linkIDPrefix+linkName(key), "Unlink")
		}
		rows = append(rows, row)
	}

	return rows
}

// outputSuffix names the output a line leaves a node by, when the node has
// more than one.
func outputSuffix(node flow.Node, output flow.Output) string {
	if len(flow.Outputs(node)) <= 1 {
		return ""
	}

	return " · " + output.Label
}

// linkName names a line for input scripts: its node's ID and its output.
func linkName(key routeKey) string { return key.from.String() + ":" + key.output }

// layoutLinks places the links under the node's fields, from top, and
// returns where the inspector's body ends.
func (s *State) layoutLinks(node flow.Node, left, width, top int) int {
	s.links = s.linkRows(node)
	if len(s.links) == 0 {
		return top
	}
	top += ui.LineHeight
	for index := range s.links {
		row := &s.links[index]
		row.top = top
		if row.unlink != nil {
			row.unlink.Rect = image.Rect(left+width-unlinkWidth, top, left+width, top+ui.ButtonH)
		}
		top += ui.ButtonH + rowSpace
	}

	return top + fieldSpace
}

// drawLinks draws the links' heading and each line, with its Unlink.
func (s *State) drawLinks(screen *ebiten.Image, left, width int) {
	if len(s.links) == 0 {
		return
	}
	s.theme.DrawLabel(screen, linksHeading, left, s.links[0].top-ui.LineHeight+ui.Gap, ui.ColorMuted())
	for _, row := range s.links {
		room := width
		if row.unlink != nil {
			room -= unlinkWidth + ui.Gap
		}
		ink := ui.ColorInk()
		if row.unlink == nil {
			ink = ui.ColorMuted()
		}
		text := s.theme.FitText(row.text, room)
		s.theme.DrawText(screen, text, left, row.top+(ui.ButtonH-ui.TextSize)/ui.Half-1, ink)
		if row.unlink != nil {
			row.unlink.Draw(screen, s.theme)
		}
	}
}

// updateLinks unlinks the line whose Unlink is pressed, while it shows in
// the inspector's body.
func (s *State) updateLinks(input *ui.Input, ctx *frame.Context) {
	body := s.formBody()
	for _, row := range s.links {
		if row.unlink == nil || !row.unlink.Rect.In(body) {
			continue
		}
		if row.unlink.Update(input) {
			s.unlink(ctx, row.key)

			return
		}
	}
}

// locateLink finds a line's Unlink for input scripts: "unlink:<from kind>:<n>:<output>",
// as "unlink:start:1:next".
func (s *State) locateLink(value string) (image.Point, bool) {
	where := strings.Split(value, ":")
	if len(where) != portParts {
		return image.Pt(0, 0), false
	}
	drawn, ok := s.nthNode(where[0] + ":" + where[1])
	if !ok {
		return image.Pt(0, 0), false
	}
	key := routeKey{from: drawn.node.ID, output: where[2]}
	index := slices.IndexFunc(s.links, func(row linkRow) bool { return row.key == key && row.unlink != nil })
	if index < 0 {
		return image.Pt(0, 0), false
	}

	return ui.Center(s.links[index].unlink.Rect), true
}

// hasLinkState answers "link=<text>", whether the inspector's links write a
// line so.
func (s *State) hasLinkState(state string) (bool, bool) {
	text, ok := strings.CutPrefix(state, "link=")
	if !ok {
		return false, false
	}

	return slices.ContainsFunc(s.links, func(row linkRow) bool { return row.text == text }), true
}

// unlink takes a line away, saying what from what; undo joins it again.
func (s *State) unlink(ctx *frame.Context, key routeKey) {
	from, ok := s.workflow.Node(key.from)
	if !ok {
		return
	}
	said := "Unlinked " + labelOf(from.Kind) + " · " + key.output
	if target, found := s.workflow.Node(from.Next[key.output]); found {
		said += " from " + labelOf(target.Kind)
	}
	if s.selectedLink == key {
		s.selectedLink = noRoute()
	}
	s.replace(ctx, from.WithLink(key.output, ids.ULID{}), said+". Undo joins it again.")
}
