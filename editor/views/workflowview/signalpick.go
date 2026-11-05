package workflowview

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/signalpicker"
	"github.com/bradhannah/berad-engine/editor/views/signals"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// chooseSignalLabel is what a signal box says with no signal chosen.
const chooseSignalLabel = "Choose a signal"

// signalPick is the open signal picker and the node and setting it
// chooses for.
type signalPick struct {
	picker *signalpicker.Picker
	node   ids.ULID
	param  string
}

// openSignalPicker opens the picker for a node's signal: every signal there
// is, those this workflow sends or answers first, the chosen one selected.
func (s *State) openSignalPicker(docs frame.Docs, node flow.Node, param string) {
	declared, _ := signals.Declared(docs)
	uses := signals.Uses(docs)
	sendable := signals.Sendable(declared)
	options := make([]signalpicker.Option, 0, len(sendable))
	for _, signal := range sendable {
		options = append(options, signalpicker.Option{
			ID: signal.ID, Path: worldstate.SignalPath(declared, signal.ID), Detail: signals.Counted(uses[signal.ID]),
			Here: s.signalHere(signal.ID),
		})
	}
	title := "Choose a signal to send"
	if node.Kind == flow.KindOnSignal {
		title = "Choose a signal to answer"
	}
	chosen, _ := node.Signal()
	nodeID := node.ID
	pick := func(ctx *frame.Context, option signalpicker.Option) {
		if current, found := s.workflow.Node(nodeID); found {
			s.saveParam(ctx, current, param, option.ID.String())
		}
	}
	config := signalpicker.Config{Title: title, Selected: chosen, Pick: pick}
	s.signalPick = &signalPick{picker: signalpicker.New(config, options, s.theme), node: node.ID, param: param}
}

// signalHere says how this workflow uses a signal already: sends it,
// answers it, or both; or "" when it does not.
func (s *State) signalHere(signal ids.ULID) string {
	sends, answers := false, false
	for _, node := range s.workflow.Nodes() {
		if chosen, ok := node.Signal(); ok && chosen == signal {
			sends = sends || node.Kind == flow.KindSendSignal
			answers = answers || node.Kind == flow.KindOnSignal
		}
	}
	switch {
	case sends && answers:
		return "sent and answered here"
	case sends:
		return "sent here"
	case answers:
		return "answered here"
	}

	return ""
}

// updateSignalBox opens the picker when a signal's box is clicked.
func (s *State) updateSignalBox(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	if input.Pressed && input.Cursor.In(made.cueBox) {
		input.ClearPointer()
		s.openSignalPicker(ctx.Docs, node, made.param.Name)
	}
}

// drawBoxed draws a setting chosen in a picker: a value's box or a
// signal's.
func (s *State) drawBoxed(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node) {
	if made.param.Type == flow.TypeSignal {
		s.drawSignalBox(screen, docs, made, node)

		return
	}
	s.drawValue(screen, docs, made, node)
}

// drawSignalBox draws a signal's box: the chosen signal's path and how
// often it is sent and answered, or that none is chosen, or that the one
// chosen is no longer declared.
func (s *State) drawSignalBox(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node) {
	label, ink, detail := chooseSignalLabel, ui.ColorMuted(), ""
	if chosen, ok := node.Signal(); ok {
		label, ink = "Broken: its signal is not declared", ui.ColorWarn()
		declared, _ := signals.Declared(docs)
		if slices.ContainsFunc(signals.Sendable(declared), func(each worldstate.Signal) bool { return each.ID == chosen }) {
			label, ink = worldstate.SignalPath(declared, chosen), ui.ColorInk()
			detail = signals.Counted(signals.Uses(docs)[chosen])
		}
	}
	open := s.signalPick != nil && s.signalPick.param == made.param.Name
	s.drawPickBox(screen, made.cueBox, label, ink, open)
	if detail != "" {
		s.theme.DrawText(screen, s.clip(detail, made.cueBox.Dx()), made.cueBox.Min.X, made.cueBox.Max.Y+ui.Gap,
			ui.ColorMuted())
	}
}

// updateSignalPicker runs the open signal picker for a frame, closing it
// once it has picked or been closed.
func (s *State) updateSignalPicker(input *ui.Input, ctx *frame.Context) {
	if !s.signalPick.picker.Update(input, ctx, s.screen()) {
		s.signalPick = nil
	}
}

// locateInSignalPicker finds a part of the open signal picker for input
// scripts.
func (s *State) locateInSignalPicker(kind, value string) (image.Point, bool) {
	return s.signalPick.picker.Locate(s.screen(), kind, value)
}
