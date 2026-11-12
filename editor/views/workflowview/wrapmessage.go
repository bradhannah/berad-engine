package workflowview

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// A Say or a Show message wraps its line on the node (increment 50): the
// node grows a line for each line its message takes, up to
// maxMessageLines, the last cut with an ellipsis when there is more. Its
// own line breaks are kept. A Log stays one line. Nothing else moves when
// a node grows: nodes under it stay where they were put. In the inspector
// its field takes three rows at least, and grows to ten.

// Wrapping's limits: the most lines a node shows, and the least and most
// rows a Say's or a Show message's field takes in the inspector.
const (
	maxMessageLines = 4
	ellipsis        = "…"
	sayLeast        = 3
	sayRows         = 10
)

// wraps reports whether a kind of node wraps its message on the canvas.
func wraps(kind string) bool { return kind == flow.KindSay || kind == flow.KindShowMessage }

// messagePiece is one piece of a message laid out on a node: a run of text
// or a value's chip, and where it starts along its line.
type messagePiece struct {
	text  string
	chip  bool
	at    int
	width int
}

// messageLayout lays a message out in lines that fit a width.
type messageLayout struct {
	theme *ui.Theme
	width int
	lines [][]messagePiece
	line  []messagePiece
	along int
}

// breakLine ends the line being filled.
func (m *messageLayout) breakLine() {
	m.lines = append(m.lines, m.line)
	m.line, m.along = nil, 0
}

// place puts a piece on the line being filled, or on a new line when it
// does not fit; a space at the start of a line is dropped, and a piece
// wider than a whole line is cut to fit.
func (m *messageLayout) place(piece messagePiece, clip func(string, int) string) {
	if m.along > 0 && m.along+piece.width > m.width {
		m.breakLine()
	}
	if !piece.chip && m.along == 0 && strings.TrimSpace(piece.text) == "" {
		return
	}
	if piece.width > m.width-m.along {
		piece.text = clip(piece.text, m.width-m.along)
		piece.width = m.width - m.along
	}
	piece.at = m.along
	m.line = append(m.line, piece)
	m.along += piece.width
}

// messageLines lays a Say's or a Show message's text and values out in
// lines that fit a width, at most maxMessageLines.
func (s *State) messageLines(docs frame.Docs, node flow.Node, width int) [][]messagePiece {
	segments, err := node.Message()
	if err != nil || len(segments) == 0 {
		return nil
	}
	layout := &messageLayout{theme: s.theme, width: width, lines: nil, line: nil, along: 0}
	for _, segment := range segments {
		if segment.Ref != nil {
			path := strings.Trim(s.tokenOf(docs, *segment.Ref), tokenOpen+tokenClose)
			chip := messagePiece{text: path, chip: true, at: 0, width: s.theme.SmallWidth(path) + ui.Half*chipPad + chipGap}
			layout.place(chip, s.clip)

			continue
		}
		for index, part := range strings.Split(segment.Text, "\n") {
			if index > 0 {
				layout.breakLine()
			}
			for _, word := range strings.SplitAfter(part, " ") {
				if word != "" {
					layout.place(messagePiece{text: word, chip: false, at: 0, width: s.theme.TextWidth(word)}, s.clip)
				}
			}
		}
	}
	layout.breakLine()

	return s.capLines(layout.lines, width)
}

// capLines keeps at most maxMessageLines, ending the last with an ellipsis
// when there were more.
func (s *State) capLines(lines [][]messagePiece, width int) [][]messagePiece {
	if len(lines) <= maxMessageLines {
		return lines
	}
	lines = lines[:maxMessageLines]
	last := lines[len(lines)-1]
	room := s.theme.TextWidth(ellipsis)
	for len(last) > 0 && last[len(last)-1].at+last[len(last)-1].width+room > width {
		last = last[:len(last)-1]
	}
	end := 0
	if len(last) > 0 {
		end = last[len(last)-1].at + last[len(last)-1].width
	}
	lines[len(lines)-1] = append(last, messagePiece{text: ellipsis, chip: false, at: end, width: room})

	return lines
}

// messageHeight is how tall a Say or a Show message is: a node's height,
// and a line more for each further line its message takes.
func (s *State) messageHeight(docs frame.Docs, node flow.Node, width int) int {
	lines := len(s.messageLines(docs, node, width-ui.Half*textPad))

	return nodeHeight + max(0, lines-1)*ui.LineHeight
}

// heights is how tall a node that grows with its text is, for laying out
// the workflow: a note with its text, and a Say or a Show message with its
// message wrapped.
func (s *State) heights(docs frame.Docs) func(flow.Node, int) int {
	return func(node flow.Node, width int) int {
		if wraps(node.Kind) {
			return s.messageHeight(docs, node, width)
		}

		return s.noteHeight(node, width)
	}
}

// drawMessageNode draws a Say's or a Show message's message in its lines,
// each value as a chip.
func (s *State) drawMessageNode(area *ebiten.Image, drawn shape, docs frame.Docs) {
	rect := s.screenRect(drawn.rect)
	s.theme.DrawLabel(area, labelOf(drawn.node.Kind), rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
	lines := s.messageLines(docs, drawn.node, drawn.rect.Dx()-ui.Half*textPad)
	if len(lines) == 0 {
		s.theme.DrawText(area, "no message", rect.Min.X+textPad, rect.Min.Y+titleDrop, ui.ColorMuted())

		return
	}
	for index, line := range lines {
		baseline := rect.Min.Y + titleDrop + index*ui.LineHeight
		for _, piece := range line {
			left := rect.Min.X + textPad + piece.at
			if !piece.chip {
				s.theme.DrawText(area, piece.text, left, baseline, ui.ColorInk())

				continue
			}
			chip := image.Rect(left+1, baseline-chipPad/ui.Half, left+piece.width-chipGap,
				baseline+ui.TextSize+chipPad/ui.Half)
			wash, ink := refColours(piece.text)
			ui.FillRound(area, chip, chipPad, wash)
			ui.StrokeRound(area, chip, chipPad, 1, ui.ColorRule())
			s.theme.DrawSmall(area, s.clip(piece.text, chip.Dx()-chipPad), chip.Min.X+chipPad, baseline+1, ink)
		}
	}
}

// hasWrapState answers "say-lines:<kind>:<n>=<count>", how many lines of
// its message a Say or a Show message shows.
func (s *State) hasWrapState(state string) (bool, bool) {
	asked, ok := strings.CutPrefix(state, "say-lines:")
	if !ok {
		return false, false
	}
	which, count, _ := strings.Cut(asked, "=")
	drawn, found := s.nthNode(which)

	return found && strconv.Itoa((drawn.rect.Dy()-nodeHeight)/ui.LineHeight+1) == count, true
}
