package workflow

import (
	"encoding/json"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// KindTogether is a container whose lanes start at once, each running its
// own steps, and which goes on by done, all done, once every lane is done.
const KindTogether = "together"

// A Together's lanes: each an output to its first step, with a name.
const (
	ParamLanes           = "lanes"
	TypeLanes  ParamType = "lanes"
	MinLanes             = 2
	MaxLanes             = 8
)

// Lane is one lane of a Together: the output to its first step, with an
// identity apart from its name, and its name.
type Lane struct {
	Port  string `json:"port"`
	Label string `json:"label"`
}

// togetherKind declares Together: its name, two lanes to start with, and
// its way on, all done.
func togetherKind() Kind {
	return Kind{
		Name: KindTogether, Label: "Together", Group: GroupFlow,
		Params: []Param{
			{Name: "name", Label: "Name", Type: TypeText, Default: mustJSON("Together"), Choices: nil},
			{Name: ParamLanes, Label: "Lanes", Type: TypeLanes, Choices: nil, Default: mustJSON([]Lane{
				{Port: "l1", Label: "Lane 1"}, {Port: "l2", Label: "Lane 2"},
			})},
		},
		Outputs: []Output{{Name: OutputDone, Label: "all done"}},
	}
}

// Lanes returns a Together's lanes.
func (n Node) Lanes() ([]Lane, error) {
	var lanes []Lane

	return lanes, n.decode(ParamLanes, &lanes)
}

// IsContainer reports whether a kind of node holds steps inside it: a
// time-bound container or a Together.
func IsContainer(kind string) bool { return kind == KindContainer || kind == KindTogether }

// IsInner reports whether a node's output leads inside it: a container's
// first step, or one of a Together's lanes.
func IsInner(node Node, output string) bool {
	switch node.Kind {
	case KindContainer:
		return output == OutputInside
	case KindTogether:
		lanes, err := node.Lanes()

		return err == nil && slices.ContainsFunc(lanes, func(lane Lane) bool { return lane.Port == output })
	}

	return false
}

// togetherOutputs is a Together's outputs: each lane, then all done.
func togetherOutputs(node Node, declared Kind) []Output {
	lanes, err := node.Lanes()
	if err != nil {
		return declared.Outputs
	}
	outputs := make([]Output, 0, len(lanes)+len(declared.Outputs))
	for _, lane := range lanes {
		outputs = append(outputs, Output{Name: lane.Port, Label: lane.Label})
	}

	return append(outputs, declared.Outputs...)
}

// checkLanes reads a Together's lanes: MinLanes to MaxLanes, each with its
// own output name and a name of at most MaxText characters.
func checkLanes(value json.RawMessage) error {
	var lanes []Lane
	if err := json.Unmarshal(value, &lanes); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if len(lanes) < MinLanes || len(lanes) > MaxLanes {
		return fmt.Errorf("%w: a Together has %d to %d lanes", ErrNode, MinLanes, MaxLanes)
	}
	ports := map[string]bool{}
	for _, lane := range lanes {
		if lane.Port == "" || lane.Port == OutputDone || ports[lane.Port] {
			return fmt.Errorf("%w: each lane needs its own output name", ErrNode)
		}
		ports[lane.Port] = true
		if utf8.RuneCountInString(lane.Label) > MaxText {
			return fmt.Errorf("%w: a lane's name is longer than %d characters", ErrNode, MaxText)
		}
	}

	return nil
}

// LanesOf works out which lanes of a Together reach each node inside it,
// following lines from each lane's first step; a node reached from two
// lanes is in both.
func (w *Workflow) LanesOf(together ids.ULID) map[ids.ULID][]string {
	reached := map[ids.ULID][]string{}
	container, ok := w.Node(together)
	if !ok || container.Kind != KindTogether {
		return reached
	}
	lanes, err := container.Lanes()
	if err != nil {
		return reached
	}
	for _, lane := range lanes {
		if first, linked := container.Next[lane.Port]; linked {
			w.reachLane(first, lane.Port, map[ids.ULID]bool{}, reached)
		}
	}

	return reached
}

// reachLane marks a node and everything after it as reached from a lane.
func (w *Workflow) reachLane(nodeID ids.ULID, port string, seen map[ids.ULID]bool, reached map[ids.ULID][]string) {
	if seen[nodeID] {
		return
	}
	seen[nodeID] = true
	if !slices.Contains(reached[nodeID], port) {
		reached[nodeID] = append(reached[nodeID], port)
	}
	node, found := w.Node(nodeID)
	if !found {
		return
	}
	for _, output := range Outputs(node) {
		if target, has := node.Next[output.Name]; has {
			w.reachLane(target, port, seen, reached)
		}
	}
}

// Moves reports whether a kind of node moves the NPC: a Move or a Go to.
// Only one lane of a Together may hold them.
func Moves(kind string) bool { return kind == KindMove || kind == KindGoTo }

// InLane reports whether a kind of node can sit in a Together's lane: a
// step in a line, not a start, a note, a Call, a Return, a Resume or a
// container.
func InLane(kind string) bool {
	return Runs(kind) && !IsStart(kind) && kind != KindCall && kind != KindReturn && kind != KindResume &&
		!IsContainer(kind)
}
