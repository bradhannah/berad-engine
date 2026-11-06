package workflowview

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/signals"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Problems and warnings of the signal nodes.
const (
	problemNoSignal      = "Choose the signal."
	problemMissingSignal = "Its signal is not declared. Choose another, or declare it in World state's Signals window."
	problemTwoReceivers  = "Another On signal here answers this signal already; only one answers each."
	problemLoneResume    = "Resume goes back to where a signal interrupted the NPC, so it belongs on a receiver's path."
	problemNoAddressee   = "Choose the NPC to send it to."
	problemNoHolder      = "Choose the local value that holds the NPC."
	problemNoBehaviour   = "Broken signal call: %s has no behaviour to answer %s."
	problemNotAnswered   = "Broken signal call: %s's behaviour doesn't answer %s. Give it an On signal for it."
	problemNobodyAnswers = "Broken signal call: no workflow answers %s yet."
	warningUnheard       = "Nothing sends %s yet, so this receiver never runs."
)

// signalChoices are the declared signals, by path, as a signal setting
// offers them; their groups are not sent.
func signalChoices(docs frame.Docs) []flow.Choice {
	declared, _ := signals.Declared(docs)
	sendable := signals.Sendable(declared)
	choices := make([]flow.Choice, 0, len(sendable))
	for _, signal := range sendable {
		choices = append(choices, flow.Choice{
			Value: signal.ID.String(), Label: worldstate.SignalPath(declared, signal.ID),
		})
	}

	return choices
}

// npcChoices are the project's NPCs, by name, as an NPC setting offers them.
func npcChoices(docs frame.Docs) []flow.Choice {
	var choices []flow.Choice
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			for _, entry := range index.OfFormat(npc.FormatName) {
				choices = append(choices, flow.Choice{Value: entry.ID.String(), Label: entry.Name})
			}
		}
	})

	return choices
}

// newIdentityControl makes an NPC setting's buttons, one for each NPC
// there is, as a choice's are. A signal setting is a box that opens the
// signal picker instead.
func newIdentityControl(docs frame.Docs, made *control) {
	for _, choice := range npcChoices(docs) {
		made.choices = append(made.choices, ui.NewButton(made.param.Name+"-"+choice.Label, choice.Label))
		made.values = append(made.values, choice.Value)
	}
}

// identityShape is what a signal node's buttons are made of, so its form is
// rebuilt when a signal or an NPC comes or goes.
func identityShape(docs frame.Docs, node flow.Node) string {
	if node.Kind != flow.KindOnSignal && node.Kind != flow.KindSendSignal {
		return ""
	}
	var shape strings.Builder
	for _, choice := range slices.Concat(signalChoices(docs), npcChoices(docs)) {
		shape.WriteString(choice.Value + "=" + choice.Label + ";")
	}

	return shape.String()
}

// sendShows reports whether a Send signal's setting takes room: the NPC
// only when sending to one by name, the local only when sending to the NPC
// a local holds.
func sendShows(node flow.Node, made *control) bool {
	if node.Kind != flow.KindSendSignal {
		return true
	}
	switch made.param.Name {
	case flow.ParamNPC:
		return choiceOf(node, flow.ParamSendTo) == flow.ToNPC
	case flow.ParamHolder:
		return choiceOf(node, flow.ParamSendTo) == flow.ToLocal
	}

	return true
}

// signalSummary describes a Send signal or a Resume in a few words.
func (s *State) signalSummary(node flow.Node, docs frame.Docs) string {
	if node.Kind == flow.KindResume {
		return "back where it was"
	}
	name := "no signal"
	if signal, chosen := node.Signal(); chosen {
		name = signals.Name(docs, signal)
	}
	switch choiceOf(node, flow.ParamSendTo) {
	case flow.ToZone:
		return name + " → everyone in this zone"
	case flow.ToLocal:
		return name + " → " + s.valueLabel(node, docs, flow.ParamHolder)
	}
	if npcID, named := node.Addressee(); named {
		return name + " → " + s.npcName(docs, npcID)
	}

	return name + " → no NPC"
}

// valueLabel names a value setting's value, or says there is none.
func (s *State) valueLabel(node flow.Node, docs frame.Docs, param string) string {
	ref, err := node.Ref(param)
	if err != nil || ref.IsZero() {
		return "no local chosen"
	}
	if _, shown, problem := s.resolve(docs, ref); problem == "" {
		return shown
	}

	return "a missing value"
}

// npcName names an NPC by the project's index.
func (s *State) npcName(docs frame.Docs, npcID ids.ULID) string {
	name := "a missing NPC"
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(npcID, name)
		}
	})

	return name
}

// drawReceiver draws the start, or an On signal as a start: its kind, a
// signal mark and the signal's name.
func (s *State) drawReceiver(area *ebiten.Image, drawn shape, docs frame.Docs) {
	rect := s.screenRect(drawn.rect)
	if drawn.node.Kind == flow.KindStart {
		s.theme.DrawLabel(area, flow.GroupFlow, rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
		s.theme.DrawStrong(area, "Start", rect.Min.X+textPad, rect.Min.Y+titleDrop, ui.ColorInk())

		return
	}
	s.theme.DrawLabel(area, "On signal", rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
	name := "no signal"
	if signal, chosen := drawn.node.Signal(); chosen {
		name = signals.Name(docs, signal)
	}
	s.theme.DrawStrong(area, s.clip("» "+name, drawn.rect.Dx()-ui.Half*textPad), rect.Min.X+textPad,
		rect.Min.Y+titleDrop, ui.ColorInk())
}

// signalProblem says what is wrong with a signal node, or returns "".
func (s *State) signalProblem(node flow.Node, docs frame.Docs) string {
	switch node.Kind {
	case flow.KindOnSignal:
		return s.receiverProblem(node, docs)
	case flow.KindResume:
		if !s.onReceiverPath(node.ID) {
			return problemLoneResume
		}
	case flow.KindSendSignal:
		return s.sendProblem(node, docs)
	}

	return ""
}

// receiverProblem says what is wrong with an On signal: no signal, one not
// declared, or another receiver answering it here.
func (s *State) receiverProblem(node flow.Node, docs frame.Docs) string {
	signal, chosen := node.Signal()
	switch {
	case !chosen:
		return problemNoSignal
	case !declaredSignal(docs, signal):
		return problemMissingSignal
	case s.workflow.Receivers()[signal] != node.ID:
		return problemTwoReceivers
	}

	return ""
}

// declaredSignal reports whether the world state declares a signal, not a
// group of them.
func declaredSignal(docs frame.Docs, signal ids.ULID) bool {
	declared, _ := signals.Declared(docs)

	return slices.ContainsFunc(signals.Sendable(declared), func(each worldstate.Signal) bool {
		return each.ID == signal
	})
}

// sendProblem says what is wrong with a Send signal: no signal or one not
// declared, no NPC or local chosen, or a broken signal call, where nothing
// it is sent to answers it.
func (s *State) sendProblem(node flow.Node, docs frame.Docs) string {
	signal, chosen := node.Signal()
	switch {
	case !chosen:
		return problemNoSignal
	case !declaredSignal(docs, signal):
		return problemMissingSignal
	}
	name := signals.Name(docs, signal)
	switch choiceOf(node, flow.ParamSendTo) {
	case flow.ToNPC:
		npcID, named := node.Addressee()
		if !named {
			return problemNoAddressee
		}

		return s.addresseeProblem(docs, npcID, signal, name)
	case flow.ToLocal:
		if ref, err := node.Ref(flow.ParamHolder); err != nil || ref.IsZero() {
			return problemNoHolder
		}
	}
	if signals.Loaded(docs) && !signals.Answered(signals.Uses(docs)[signal]) {
		return fmt.Sprintf(problemNobodyAnswers, name)
	}

	return ""
}

// addresseeProblem says whether a named NPC's behaviour answers a signal;
// nothing until the NPC and its behaviour have loaded.
func (s *State) addresseeProblem(docs frame.Docs, npcID, signal ids.ULID, name string) string {
	behaviour, loaded := signals.Behaviour(docs, npcID)
	if !loaded {
		return ""
	}
	if behaviour.IsZero() {
		return fmt.Sprintf(problemNoBehaviour, s.npcName(docs, npcID), name)
	}
	if answers, read := signals.Answers(docs, behaviour, signal); read && !answers {
		return fmt.Sprintf(problemNotAnswered, s.npcName(docs, npcID), name)
	}

	return ""
}

// signalWarning warns of a receiver nothing sends to, once every workflow
// has loaded.
func (s *State) signalWarning(node flow.Node, docs frame.Docs) string {
	signal, chosen := node.Signal()
	if node.Kind != flow.KindOnSignal || !chosen || !signals.Loaded(docs) {
		return ""
	}
	if signals.Sent(signals.Uses(docs)[signal]) {
		return ""
	}

	return fmt.Sprintf(warningUnheard, signals.Name(docs, signal))
}

// onReceiverPath reports whether a receiver's path reaches a node.
func (s *State) onReceiverPath(nodeID ids.ULID) bool {
	for _, receiver := range s.workflow.Receivers() {
		if s.workflow.Reach(receiver)[nodeID] {
			return true
		}
	}

	return false
}

// drawWarningMark draws a node's warning mark: an amber circle with "!".
func (s *State) drawWarningMark(area *ebiten.Image, drawn shape) {
	if _, warned := s.warnings[drawn.node.ID]; !warned {
		return
	}
	if _, problem := s.problems[drawn.node.ID]; problem {
		return
	}
	centre := s.toScreen(markCentre(drawn))
	vector.FillCircle(area, float32(centre.X), float32(centre.Y), markRadius, ui.ColorCaution(), true)
	width := s.theme.SmallWidth("!")
	s.theme.DrawSmall(area, "!", centre.X-width/ui.Half, centre.Y-ui.LabelSize/ui.Half-ui.Half, ui.ColorBackground())
}

// hasWarningState answers "warnings:N" and "warning:<kind>:<n>", or
// "warning:<kind>:<n>=<warning>".
func (s *State) hasWarningState(state string) (bool, bool) {
	if count, asked := strings.CutPrefix(state, "warnings:"); asked {
		return strconv.Itoa(len(s.warnings)) == count, true
	}
	asked, isWarning := strings.CutPrefix(state, "warning:")
	if !isWarning {
		return false, false
	}
	which, says, saying := strings.Cut(asked, "=")
	drawn, ok := s.nthNode(which)
	warning, marked := s.warnings[drawn.node.ID]

	return ok && marked && (!saying || warning == says), true
}

// drawWarning writes a node's warning in amber in the inspector, above
// where its problem would go.
func (s *State) drawWarning(screen *ebiten.Image, nodeID ids.ULID, left, bottom, width int) {
	warning, warned := s.warnings[nodeID]
	if !warned {
		return
	}
	lines := s.wrap(warning, width)
	for index, line := range lines {
		s.theme.DrawText(screen, line, left, bottom-(len(lines)-index)*ui.LineHeight, ui.ColorCaution())
	}
}

// subscribeSignals keeps every workflow and NPC loaded while the workflow
// sends or answers a signal, to tell who answers it.
func (s *State) subscribeSignals(ctx *frame.Context) {
	if s.workflow == nil || !slices.ContainsFunc(s.workflow.Nodes(), func(node flow.Node) bool {
		return node.Kind == flow.KindOnSignal || node.Kind == flow.KindSendSignal
	}) {
		return
	}
	for _, document := range signals.Documents(ctx.Docs) {
		s.subscribe(document, ctx)
	}
}
