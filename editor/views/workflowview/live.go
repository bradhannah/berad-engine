package workflowview

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Live mode, in screen pixels and frames.
const (
	liveOutline = 3
	liveBar     = 16
	liveBarPad  = 6
	// flashFrames is how long the line just taken stays orange.
	flashFrames = 60
)

// live is what a workflow shown live marks: the node an NPC is on, the line
// it just took, which flashes for a moment after each change, and the lines
// it took in its latest step, which stay orange.
type live struct {
	area    image.Rectangle
	node    ids.ULID
	last    routeKey
	changes uint64
	flash   int
	step    map[routeKey]bool
}

// NewLive returns a read-only view of a workflow for the Watch window: only
// its canvas, in an area the caller sets, which can be panned but not
// edited, with the node an NPC is on outlined in orange under a LIVE bar.
func NewLive(window, document ids.ULID, theme *ui.Theme) *State {
	view := New(window, document, theme)
	view.live = &live{area: ui.Zero(), node: ids.ULID{}, last: noRoute(), changes: 0, flash: 0, step: nil}

	return view
}

// SetArea sets where a live view draws its canvas.
func (s *State) SetArea(area image.Rectangle) {
	if s.live != nil {
		s.live.area = area
	}
}

// SetLive marks the node an NPC is on and the line it took to get there;
// changes counts its changes of node, so a new one starts the line's flash.
func (s *State) SetLive(node, from ids.ULID, output string, changes uint64) {
	if s.live == nil {
		return
	}
	if changes != s.live.changes {
		s.live.flash = flashFrames
	}
	s.live.node, s.live.last, s.live.changes = node, routeKey{from: from, output: output}, changes
}

// SetStep marks the lines an NPC took in its latest step, which stay orange
// until the next call.
func (s *State) SetStep(changes []protocol.PreviewChange) {
	if s.live == nil {
		return
	}
	s.live.step = make(map[routeKey]bool, len(changes))
	for _, change := range changes {
		s.live.step[routeKey{from: change.From, output: change.Output}] = true
	}
}

// Path writes changes of node as the way an NPC went: each node it left,
// with the output it took when it had a choice and why it carried on when it
// could not do its work, then the node it reached. For example: "Wait · 2 s",
// "Random choice: chance", "Move · up 1 tile: blocked", "Wait · 2 s".
func (s *State) Path(changes []protocol.PreviewChange, docs frame.Docs) []string {
	s.load(docs)
	if s.workflow == nil || len(changes) == 0 {
		return nil
	}
	parts := make([]string, 0, len(changes)+1)
	for _, change := range changes {
		node, ok := s.workflow.Node(change.From)
		if !ok {
			continue
		}
		part := s.nodeLabel(node, docs)
		if label := choiceLabel(node, change.Output); label != "" {
			part += ": " + label
		}
		if change.Status != "" {
			part += ": " + carriedOn(change.Status)
		}
		parts = append(parts, part)
	}
	if node, ok := s.workflow.Node(changes[len(changes)-1].To); ok {
		parts = append(parts, s.nodeLabel(node, docs))
	}

	return parts
}

// choiceLabel is the label of the output a node was left by, when it has
// more than one.
func choiceLabel(node flow.Node, name string) string {
	outputs := flow.Outputs(node)
	if len(outputs) <= 1 {
		return ""
	}
	for _, output := range outputs {
		if output.Name == name {
			return output.Label
		}
	}

	return ""
}

// carriedOn says in a word or two why a node carried on without doing its
// work.
func carriedOn(status string) string {
	switch status {
	case npcrun.StatusBlocked:
		return "blocked"
	case npcrun.StatusNoRoute:
		return "no way there"
	case npcrun.StatusNoAnchor:
		return "no destination"
	case npcrun.StatusOtherFloor:
		return "destination on another floor"
	default:
		return strings.ToLower(status)
	}
}

// updateLive handles a frame of a live view: panning only.
func (s *State) updateLive(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.cursor = input.Cursor
	s.load(ctx.Docs)
	s.listAudio(ctx)
	if s.live.flash > 0 {
		s.live.flash--
	}
	if s.workflow != nil {
		s.pan(input, s.Canvas())
	}
}

// drawLive draws a live view's canvas into its area.
func (s *State) drawLive(screen *ebiten.Image, docs frame.Docs) {
	s.load(docs)
	if area, ok := screen.SubImage(s.Canvas()).(*ebiten.Image); ok {
		s.drawCanvas(area, docs)
	}
}

// drawLiveNode outlines the node an NPC is on in orange, under a LIVE bar.
func (s *State) drawLiveNode(area *ebiten.Image) {
	if s.live == nil {
		return
	}
	drawn, ok := s.shapes[s.live.node]
	if !ok {
		return
	}
	rect := s.screenRect(drawn.rect)
	ui.StrokeRound(area, rect.Inset(-selectGap), ui.Radius+selectGap, liveOutline, ui.ColorLive())
	label := "LIVE"
	bar := image.Rect(rect.Min.X, rect.Min.Y-selectGap-liveBar, rect.Min.X+s.theme.LabelWidth(label)+ui.Half*liveBarPad,
		rect.Min.Y-selectGap)
	ui.FillRound(area, bar, ui.Radius, ui.ColorLive())
	s.theme.DrawLabel(area, label, bar.Min.X+liveBarPad, bar.Min.Y+(liveBar-ui.LabelSize)/ui.Half-1, ui.ColorBackground())
}

// lineFlashing reports whether a line is orange: the one just taken, while
// it flashes, or one taken in the latest step.
func (s *State) lineFlashing(key routeKey) bool {
	return s.live != nil && (s.live.flash > 0 && key == s.live.last || s.live.step[key])
}

// LiveNode returns the node a live view marks, for tests.
func (s *State) LiveNode() ids.ULID {
	if s.live == nil {
		return ids.ULID{}
	}

	return s.live.node
}
