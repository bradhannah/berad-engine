package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// GroupSignals is the palette group of the nodes that send and answer
// signals.
const GroupSignals = "Signals"

// The Signals group's kinds: a receiver, another start that runs when its
// signal is heard; Resume, back to where the NPC was interrupted; and Send
// signal.
const (
	KindOnSignal   = "on-signal"
	KindResume     = "resume"
	KindSendSignal = "send-signal"
)

// TypeSignal is a declared signal, by its identity; TypeNPC an NPC, by its
// identity. Both are empty until one is chosen.
const (
	TypeSignal ParamType = "signal"
	TypeNPC    ParamType = "npc"
)

// The signal nodes' settings: the signal; the local an On signal keeps who
// sent it in; and whom Send signal sends to: one NPC by name, the NPC a
// local holds, everyone in this zone, or everyone.
const (
	ParamSignal = "signal"
	ParamSender = "sender"
	ParamSendTo = "send-to"
	ParamNPC    = "npc"
	ParamHolder = "holder"
	ToNPC       = "npc"
	ToLocal     = "local"
	ToZone      = "zone"
	ToEveryone  = "everyone"
)

// signalKinds declares On signal, Resume and Send signal.
func signalKinds(next []Output) []Kind {
	signal := Param{Name: ParamSignal, Label: "Signal", Type: TypeSignal, Default: mustJSON(""), Choices: nil}

	return []Kind{
		{
			Name: KindOnSignal, Label: "On signal", Group: GroupSignals,
			Params: []Param{
				signal,
				{Name: ParamSender, Label: "Keep who sent it in", Type: TypeValue, Default: mustJSON(noRef()), Choices: nil},
			},
			Outputs: next,
		},
		{Name: KindResume, Label: "Resume", Group: GroupSignals, Params: nil, Outputs: nil},
		{
			Name: KindSendSignal, Label: "Send signal", Group: GroupSignals,
			Params: []Param{
				signal,
				{Name: ParamSendTo, Label: "Send to", Type: TypeChoice, Default: mustJSON(ToNPC), Choices: []Choice{
					{Value: ToNPC, Label: "One NPC"}, {Value: ToLocal, Label: "The NPC a local holds"},
					{Value: ToZone, Label: "Everyone in this zone"}, {Value: ToEveryone, Label: "Everyone"},
				}},
				{Name: ParamNPC, Label: "NPC", Type: TypeNPC, Default: mustJSON(""), Choices: nil},
				{Name: ParamHolder, Label: "Local holding the NPC", Type: TypeValue, Default: mustJSON(noRef()),
					Choices: nil},
			},
			Outputs: next,
		},
	}
}

// IsStart reports whether a kind of node starts a path that no line leads
// to: the start, or a receiver.
func IsStart(kind string) bool { return kind == KindStart || kind == KindOnSignal }

// Signal returns a signal node's signal, and whether one is chosen.
func (n Node) Signal() (ids.ULID, bool) { return n.identity(ParamSignal) }

// Addressee returns the NPC a Send signal names, and whether one is chosen.
func (n Node) Addressee() (ids.ULID, bool) { return n.identity(ParamNPC) }

// identity reads a parameter holding an identity, empty for none.
func (n Node) identity(name string) (ids.ULID, bool) {
	var written string
	if n.decode(name, &written) != nil || written == "" {
		return ids.ULID{}, false
	}
	parsed, err := ids.Parse(written)

	return parsed, err == nil
}

// checkIdentity reads an identity parameter: empty, or an identity.
func checkIdentity(value json.RawMessage) error {
	var written string
	if err := json.Unmarshal(value, &written); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if written == "" {
		return nil
	}
	if _, err := ids.Parse(written); err != nil {
		return fmt.Errorf("%w: %q is not an identity", ErrNode, written)
	}

	return nil
}

// Receivers returns a workflow's receivers by the signal each answers; a
// receiver with no signal chosen is left out, and of two for one signal
// the first made.
func (w *Workflow) Receivers() map[ids.ULID]ids.ULID {
	receivers := map[ids.ULID]ids.ULID{}
	for _, node := range w.Nodes() {
		if node.Kind != KindOnSignal || !node.Parent.IsZero() {
			continue
		}
		if signal, chosen := node.Signal(); chosen {
			if _, taken := receivers[signal]; !taken {
				receivers[signal] = node.ID
			}
		}
	}

	return receivers
}

// Reach returns every node reached from a node by following its lines,
// into containers too, the node itself among them.
func (w *Workflow) Reach(from ids.ULID) map[ids.ULID]bool {
	reached := map[ids.ULID]bool{}
	waiting := []ids.ULID{from}
	for len(waiting) > 0 {
		current := waiting[len(waiting)-1]
		waiting = waiting[:len(waiting)-1]
		node, ok := w.Node(current)
		if !ok || reached[current] {
			continue
		}
		reached[current] = true
		for _, output := range Outputs(node) {
			if target, linked := node.Next[output.Name]; linked {
				waiting = append(waiting, target)
			}
		}
	}

	return reached
}

// checkWritten reads a parameter written as text: an identity for a signal
// or an NPC, otherwise text of at most its limit.
func checkWritten(paramType ParamType, value json.RawMessage) error {
	if paramType == TypeSignal || paramType == TypeNPC {
		return checkIdentity(value)
	}

	return checkText(value, textLimit(paramType))
}
