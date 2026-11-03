package watchview

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// The header names the path down through the calls the watched NPC is
// inside, as Shopkeeper day › Go to work › Go to: each workflow, its
// behaviour first, then the kind of node it is on. A click on a workflow
// of the path shows it, with the Call the NPC is inside lit; a click on
// the last follows the NPC again. Locate names: "ui:path-<n>", the nth
// workflow of the path from 1.

// callSeparator joins the parts of the path.
const callSeparator = " › "

// level is one workflow the watched NPC is in and the node it is on there:
// for one it called from, the Call.
type level struct {
	workflow ids.ULID
	node     ids.ULID
}

// levelsOf lists the workflows a watched NPC is in, its behaviour first.
func levelsOf(entity protocol.PreviewEntity) []level {
	levels := make([]level, 0, len(entity.Calls)+1)
	for _, call := range entity.Calls {
		levels = append(levels, level{workflow: call.Workflow, node: call.Call})
	}

	return append(levels, level{workflow: entity.Workflow, node: entity.Node})
}

// shownLevel is the workflow of the path the window shows: the one clicked,
// while the NPC is still inside a call from it, or the one it is in now.
func (s *State) shownLevel(entity protocol.PreviewEntity) (level, bool) {
	levels := levelsOf(entity)
	if s.chosen >= 0 && s.chosen < len(levels)-1 && levels[s.chosen].workflow == s.chosenFlow {
		return levels[s.chosen], false
	}
	s.chosen = -1

	return levels[len(levels)-1], true
}

// pathText is the path down through the calls, the node's kind last.
func (s *State) pathText(docs frame.Docs, entity protocol.PreviewEntity) string {
	if len(entity.Calls) == 0 {
		return ""
	}
	parts := []string{}
	for _, at := range levelsOf(entity) {
		parts = append(parts, s.name(docs, at.workflow, "a workflow"))
	}

	return strings.Join(append(parts, s.kindLabel(docs, entity)), callSeparator)
}

// kindLabel names the kind of node the watched NPC is on.
func (s *State) kindLabel(docs frame.Docs, entity protocol.PreviewEntity) string {
	label := "Starting"
	docs.View(entity.Workflow, func(document client.Document) {
		if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
			if node, found := workflowDoc.Workflow.Node(entity.Node); found {
				kind, _ := flow.KindOf(node.Kind)
				label = kind.Label
			}
		}
	})

	return label
}

// updatePath shows the workflow of the path clicked.
func (s *State) updatePath(input *ui.Input, entity protocol.PreviewEntity) {
	if !input.Pressed {
		return
	}
	levels := levelsOf(entity)
	for index, part := range s.parts {
		if input.Cursor.In(part) && index < len(levels) {
			s.chosen, s.chosenFlow = index, levels[index].workflow
			if index == len(levels)-1 {
				s.chosen = -1
			}
		}
	}
}

// drawPath draws the path after the NPC's name, each workflow a part to
// click, the one shown in ink.
func (s *State) drawPath(screen *ebiten.Image, docs frame.Docs, entity protocol.PreviewEntity, left int) {
	s.parts = nil
	if len(entity.Calls) == 0 {
		return
	}
	textY := (headerHeight - ui.TextSize) / ui.Half
	shown, _ := s.shownLevel(entity)
	for _, at := range levelsOf(entity) {
		name := s.name(docs, at.workflow, "a workflow")
		ink := ui.ColorSelect()
		if at == shown {
			ink = ui.ColorInk()
		}
		width := s.theme.TextWidth(name)
		s.theme.DrawText(screen, name, left, textY, ink)
		s.parts = append(s.parts, image.Rect(left, 0, left+width, headerHeight))
		left += width
		s.theme.DrawText(screen, callSeparator, left, textY, ui.ColorMuted())
		left += s.theme.TextWidth(callSeparator)
	}
	s.theme.DrawText(screen, s.kindLabel(docs, entity), left, textY, ui.ColorMuted())
}

// hasCallState answers "path=<path>" and "showing:<workflow>".
func (s *State) hasCallState(state string, docs frame.Docs, entity protocol.PreviewEntity, watching bool) bool {
	if path, asked := strings.CutPrefix(state, "path="); asked {
		return watching && s.pathText(docs, entity) == path
	}
	if way, asked := strings.CutPrefix(state, "way="); asked {
		return watching && s.wayLine(docs, entity) == way
	}
	if where, asked := strings.CutPrefix(state, "at="); asked {
		return watching && s.whereLine(docs, entity) == where
	}

	return s.name(docs, s.shown, "") == strings.TrimPrefix(state, "showing:")
}

// locatePart finds the nth workflow of the path, from 1.
func (s *State) locatePart(value string) (image.Point, bool) {
	number, err := strconv.Atoi(strings.TrimPrefix(value, "path-"))
	if err != nil || number < 1 || number > len(s.parts) {
		return image.Pt(0, 0), false
	}

	return ui.Center(s.parts[number-1]), true
}
