package workflow

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Kinds that call another workflow, a module, and go back from one. A Call
// workflow runs the module it names and carries on from the output named
// for the way out the module took, or done; a Return is a named way out,
// at a module's top level.
const (
	KindCall   = "call-workflow"
	KindReturn = "return"
)

// OutputDone is the way out of a call that ends with nothing connected,
// and of one the game skips.
const OutputDone = "done"

// Parameters of Call workflow and Return.
const (
	ParamWorkflow = "workflow"
	ParamInputs   = "inputs"
	ParamKeep     = "keep"
	ParamWays     = "ways"
	ParamWayName  = "name"
	ParamWayPort  = "port"
)

// Parameter types of Call workflow and Return. The editor shows none of
// them as a parameter of its own: a Call's inspector and a Return's name
// stand for them.
const (
	// TypeWorkflow names a workflow by its ID, as text; "" is none.
	TypeWorkflow ParamType = "workflow"
	// TypeInputs fills a module's inputs: an operand for each, by the
	// input's ID.
	TypeInputs ParamType = "inputs"
	// TypeKeeps keeps a module's outputs: a value of the caller's for
	// each, by the output's ID.
	TypeKeeps ParamType = "keeps"
	// TypeWays lists the ways out a Call has outputs for.
	TypeWays ParamType = "ways"
	// TypePort is a Return's way out, as its outputs are named.
	TypePort ParamType = "port"
)

// MaxWayName bounds a way out's name, in characters.
const MaxWayName = 40

// Shown reports whether the editor shows a parameter of the type as its
// own row; the parameters of calls and Returns are shown otherwise.
func (t ParamType) Shown() bool {
	_, own := callChecks()[t]

	return !own
}

// WayOut is one named way out of a module: the port its Returns share and
// a Call's output is named by, and its name, as people read it.
type WayOut struct {
	Port  string `json:"port"`
	Label string `json:"label"`
}

// callKinds declares Call workflow and Return.
func callKinds() []Kind {
	return []Kind{
		{
			Name: KindCall, Label: "Call workflow", Group: GroupFlow,
			Params: []Param{
				{Name: ParamWorkflow, Label: "Workflow", Type: TypeWorkflow, Default: mustJSON(""), Choices: nil},
				{Name: ParamInputs, Label: "Inputs", Type: TypeInputs, Default: mustJSON(map[string]any{}), Choices: nil},
				{Name: ParamKeep, Label: "Outputs", Type: TypeKeeps, Default: mustJSON(map[string]any{}), Choices: nil},
				{Name: ParamWays, Label: "Ways out", Type: TypeWays, Default: mustJSON([]WayOut{}), Choices: nil},
			},
			Outputs: []Output{{Name: OutputDone, Label: OutputDone}},
		},
		{
			Name: KindReturn, Label: "Return", Group: GroupFlow,
			Params: []Param{
				{Name: ParamWayName, Label: "Way out", Type: TypeText, Default: mustJSON("finished"), Choices: nil},
				{Name: ParamWayPort, Label: "Port", Type: TypePort, Default: mustJSON("w1"), Choices: nil},
			},
			Outputs: nil,
		},
	}
}

// callChecks reads each parameter type of a call or a Return.
func callChecks() map[ParamType]func(json.RawMessage) error {
	return map[ParamType]func(json.RawMessage) error{
		TypeWorkflow: checkWorkflow, TypeInputs: checkInputs, TypeKeeps: checkKeeps, TypeWays: checkWays,
		TypePort: checkPortValue,
	}
}

// checkCallValue reads a parameter of a call or a Return.
func checkCallValue(param Param, value json.RawMessage) error {
	check, ok := callChecks()[param.Type]
	if !ok {
		return fmt.Errorf("%w: unknown parameter type %q", ErrNode, param.Type)
	}

	return check(value)
}

// checkWorkflow reads a workflow's ID, or "" for none.
func checkWorkflow(value json.RawMessage) error {
	var named string
	if err := json.Unmarshal(value, &named); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if named == "" {
		return nil
	}
	_, err := ids.Parse(named)

	return err //nolint:wrapcheck // Wrapped by the caller.
}

// checkInputs reads an operand for each input.
func checkInputs(value json.RawMessage) error {
	var inputs map[ids.ULID]json.RawMessage
	if err := json.Unmarshal(value, &inputs); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	for _, given := range inputs {
		if err := checkOperand(given); err != nil {
			return err
		}
	}

	return nil
}

// checkKeeps reads a value for each output to be kept in.
func checkKeeps(value json.RawMessage) error {
	var keeps map[ids.ULID]ValueRef
	if err := json.Unmarshal(value, &keeps); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	for _, keep := range keeps {
		raw, err := json.Marshal(keep)
		if err != nil {
			return err //nolint:wrapcheck // Wrapped by the caller.
		}
		if err := checkRef(raw); err != nil {
			return err
		}
	}

	return nil
}

// checkWays reads a Call's ways out, each with a port of its own.
func checkWays(value json.RawMessage) error {
	var ways []WayOut
	if err := json.Unmarshal(value, &ways); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	seen := map[string]bool{}
	for _, way := range ways {
		if err := checkPort(way.Port); err != nil || seen[way.Port] {
			return fmt.Errorf("%w: each way out needs its own port", ErrNode)
		}
		seen[way.Port] = true
	}

	return nil
}

// checkPortValue reads a Return's port.
func checkPortValue(value json.RawMessage) error {
	var port string
	if err := json.Unmarshal(value, &port); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}

	return checkPort(port)
}

// checkPort refuses a way out's port that is empty, too long or done,
// which every Call has already.
func checkPort(port string) error {
	if port == "" || port == OutputDone || utf8.RuneCountInString(port) > MaxWayName {
		return fmt.Errorf("%w: a way out's port is 1 to %d characters, not %q", ErrNode, MaxWayName, port)
	}

	return nil
}

// WayOut returns a Return's way out.
func (n Node) WayOut() (WayOut, error) {
	label, err := n.Text(ParamWayName)
	if err != nil {
		return WayOut{Port: "", Label: ""}, err
	}
	var port string
	if err := n.decode(ParamWayPort, &port); err != nil {
		return WayOut{Port: "", Label: ""}, err
	}

	return WayOut{Port: port, Label: label}, nil
}

// Ways returns the ways out a Call has outputs for.
func (n Node) Ways() ([]WayOut, error) {
	var ways []WayOut

	return ways, n.decode(ParamWays, &ways)
}

// Called returns the workflow a Call names; ok is false for none.
func (n Node) Called() (ids.ULID, bool) {
	var named string
	if n.decode(ParamWorkflow, &named) != nil || named == "" {
		return ids.ULID{}, false
	}
	called, err := ids.Parse(named)

	return called, err == nil
}

// Inputs returns what a Call gives each of its module's inputs, by the
// input's ID: an operand, as Set value's is saved.
func (n Node) Inputs() map[ids.ULID]json.RawMessage {
	var inputs map[ids.ULID]json.RawMessage
	if n.decode(ParamInputs, &inputs) != nil || inputs == nil {
		return map[ids.ULID]json.RawMessage{}
	}

	return inputs
}

// Keeps returns where a Call keeps each of its module's outputs, by the
// output's ID.
func (n Node) Keeps() map[ids.ULID]ValueRef {
	var keeps map[ids.ULID]ValueRef
	if n.decode(ParamKeep, &keeps) != nil || keeps == nil {
		return map[ids.ULID]ValueRef{}
	}

	return keeps
}

// callOutputs are a Call's outputs: one per way out, then done.
func callOutputs(node Node) []Output {
	ways, err := node.Ways()
	if err != nil {
		ways = nil
	}
	outputs := make([]Output, 0, len(ways)+1)
	for _, way := range ways {
		outputs = append(outputs, Output{Name: way.Port, Label: way.Label})
	}

	return append(outputs, Output{Name: OutputDone, Label: OutputDone})
}

// WaysOut returns the workflow's named ways out: one for each port its
// top-level Returns name, in the order of the first Return to name it,
// with that Return's name. Returns inside a container are not ways out.
func (w *Workflow) WaysOut() []WayOut {
	var ways []WayOut
	for _, node := range w.nodes {
		if node.Kind != KindReturn || !node.Parent.IsZero() {
			continue
		}
		way, err := node.WayOut()
		if err != nil || slices.ContainsFunc(ways, func(seen WayOut) bool { return seen.Port == way.Port }) {
			continue
		}
		ways = append(ways, way)
	}

	return ways
}

// NewPort returns a port no Return in the workflow names yet.
func (w *Workflow) NewPort() string {
	taken := map[string]bool{}
	for _, node := range w.nodes {
		if way, err := node.WayOut(); node.Kind == KindReturn && err == nil {
			taken[way.Port] = true
		}
	}
	for number := 1; ; number++ {
		if port := "w" + strconv.Itoa(number); !taken[port] {
			return port
		}
	}
}

// Inputs returns the local values a caller fills, in order.
func (w *Workflow) Inputs() []worldstate.Declaration { return w.withRole(worldstate.RoleInput) }

// Outputs returns the local values handed back to a caller, in order.
func (w *Workflow) Outputs() []worldstate.Declaration { return w.withRole(worldstate.RoleOutput) }

func (w *Workflow) withRole(role worldstate.Role) []worldstate.Declaration {
	var declared []worldstate.Declaration
	for _, local := range w.locals.Values() {
		if local.Role == role {
			declared = append(declared, local)
		}
	}

	return declared
}
