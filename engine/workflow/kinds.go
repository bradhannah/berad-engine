package workflow

import (
	"encoding/json"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

// ParamType is the type of a node parameter. The editor builds each
// parameter's control from its type: an anchor gets "Pick on map", a time
// range gets time fields.
type ParamType string

// Parameter types.
const (
	TypeAnchor     ParamType = "anchor"
	TypeMinutes    ParamType = "minutes"
	TypePercent    ParamType = "percent"
	TypeTimeRange  ParamType = "time-range"
	TypeTimeRanges ParamType = "time-ranges"
	TypeText       ParamType = "text"
)

// Limits on parameter values.
const (
	// MaxMinutes is the longest wait: a week.
	MaxMinutes = 7 * MinutesPerDay
	// MinutesPerDay is the length of a game day in minutes.
	MinutesPerDay = 24 * 60
	// MaxRanges bounds the branches of a time switch.
	MaxRanges = 24
	// MaxText bounds a text parameter, in characters.
	MaxText = 2000
	percent = 100
	hour    = 60
	// Defaults for new nodes: a working day, an even chance, half an hour.
	workStart     = 9 * hour
	workEnd       = 17 * hour
	defaultChance = 50
	defaultWait   = 30
)

// Palette groups.
const (
	GroupFlow     = "Flow"
	GroupMovement = "Movement"
	GroupAction   = "Action"
	GroupAnnotate = "Annotate"
)

// Kind names.
const (
	KindStart      = "start"
	KindTimeSwitch = "time-switch"
	KindRandom     = "random"
	KindContainer  = "container"
	KindGoTo       = "go-to"
	KindWait       = "wait"
	KindNote       = "note"
)

// Output names shared by several kinds.
const (
	OutputNext      = "next"
	OutputOtherwise = "otherwise"
	OutputYes       = "yes"
	OutputInside    = "inside"
	OutputExit      = "exit"
)

// Param declares one parameter of a node kind.
type Param struct {
	Name  string
	Label string
	Type  ParamType
	// Default is the value a new node starts with.
	Default json.RawMessage
}

// Output declares one way out of a node.
type Output struct {
	Name  string
	Label string
}

// Kind declares a kind of node: its parameters and outputs. The editor builds
// its palette and inspector from these declarations; the runner gives each
// kind its behaviour.
type Kind struct {
	Name  string
	Label string
	// Group places the kind in the palette: Flow, Movement, Action or
	// Annotate. Kinds with no group are not offered.
	Group   string
	Params  []Param
	Outputs []Output
}

// TimeRange is a span of the day in minutes from midnight, From included and
// To excluded. A range whose To is earlier than its From runs past midnight.
// Port names the output a time switch takes during the range.
type TimeRange struct {
	Port string `json:"port,omitempty"`
	From int    `json:"from"`
	To   int    `json:"to"`
}

// Contains reports whether a minute of the day falls in the range.
func (r TimeRange) Contains(minute int) bool {
	if r.From < r.To {
		return minute >= r.From && minute < r.To
	}

	return minute >= r.From || minute < r.To
}

// Kinds returns the built-in node kinds in palette order.
func Kinds() []Kind {
	next := []Output{{Name: OutputNext, Label: "then"}}

	return []Kind{
		{Name: KindStart, Label: "Start", Group: "", Params: nil, Outputs: next},
		{
			Name: KindTimeSwitch, Label: "Time switch", Group: GroupFlow,
			Params: []Param{{
				Name: "ranges", Label: "Times", Type: TypeTimeRanges,
				Default: mustJSON([]TimeRange{{Port: "r1", From: workStart, To: workEnd}}),
			}},
			Outputs: []Output{{Name: OutputOtherwise, Label: "otherwise"}},
		},
		{
			Name: KindRandom, Label: "Random choice", Group: GroupFlow,
			Params:  []Param{{Name: "chance", Label: "Chance", Type: TypePercent, Default: mustJSON(defaultChance)}},
			Outputs: []Output{{Name: OutputYes, Label: "chance"}, {Name: OutputOtherwise, Label: "otherwise"}},
		},
		{
			Name: KindContainer, Label: "Time-bound container", Group: GroupFlow,
			Params: []Param{
				{Name: "name", Label: "Name", Type: TypeText, Default: mustJSON("Group")},
				{
					Name: "bound", Label: "Time", Type: TypeTimeRange,
					Default: mustJSON(TimeRange{Port: "", From: workStart, To: workEnd}),
				},
			},
			Outputs: []Output{{Name: OutputInside, Label: "first step"}, {Name: OutputExit, Label: "exit"}},
		},
		{
			Name: KindGoTo, Label: "Go to", Group: GroupMovement,
			Params: []Param{{
				Name: "destination", Label: "Destination", Type: TypeAnchor,
				Default: mustJSON(tilefloor.AnchorRef{Floor: [16]byte{}, Anchor: [16]byte{}}),
			}},
			Outputs: next,
		},
		{
			Name: KindWait, Label: "Wait", Group: GroupAction,
			Params:  []Param{{Name: "minutes", Label: "Minutes", Type: TypeMinutes, Default: mustJSON(defaultWait)}},
			Outputs: next,
		},
		{
			Name: KindNote, Label: "Note", Group: GroupAnnotate,
			Params:  []Param{{Name: "text", Label: "Note", Type: TypeText, Default: mustJSON("")}},
			Outputs: nil,
		},
	}
}

// KindOf returns a built-in kind by name.
func KindOf(name string) (Kind, bool) {
	all := Kinds()
	index := slices.IndexFunc(all, func(kind Kind) bool { return kind.Name == name })
	if index < 0 {
		return Kind{Name: "", Label: "", Group: "", Params: nil, Outputs: nil}, false
	}

	return all[index], true
}

// Runs reports whether a node takes part in the flow; notes do not.
func Runs(kind string) bool { return kind != KindNote }

// NewNode returns a node of a kind with its default parameters. The
// orchestrator gives it a permanent ID when it is added.
func NewNode(kind string) (Node, error) {
	declared, ok := KindOf(kind)
	if !ok {
		return Node{}, fmt.Errorf("%w: unknown kind %q", ErrNode, kind)
	}
	node := Node{ID: [16]byte{}, Kind: kind, Parent: [16]byte{}, Params: map[string]json.RawMessage{}, Next: nil}
	for _, param := range declared.Params {
		node.Params[param.Name] = param.Default
	}

	return node, nil
}

// Outputs returns a node's outputs: its kind's, with a time switch's ranges
// first, one output per range.
func Outputs(node Node) []Output {
	declared, ok := KindOf(node.Kind)
	if !ok {
		return nil
	}
	if node.Kind != KindTimeSwitch {
		return declared.Outputs
	}
	ranges, err := node.TimeRanges("ranges")
	if err != nil {
		return declared.Outputs
	}
	outputs := make([]Output, 0, len(ranges)+len(declared.Outputs))
	for _, span := range ranges {
		outputs = append(outputs, Output{Name: span.Port, Label: FormatRange(span)})
	}

	return append(outputs, declared.Outputs...)
}

// CheckParams reports a node whose parameters do not match its kind: a
// missing or unknown parameter, or a value of the wrong type or out of range.
func CheckParams(node Node) error {
	declared, ok := KindOf(node.Kind)
	if !ok {
		return fmt.Errorf("%w: unknown kind %q", ErrNode, node.Kind)
	}
	if len(node.Params) != len(declared.Params) {
		return fmt.Errorf("%w: %s takes %d parameters, not %d", ErrNode, node.Kind, len(declared.Params), len(node.Params))
	}
	for _, param := range declared.Params {
		value, present := node.Params[param.Name]
		if !present {
			return fmt.Errorf("%w: %s has no %s", ErrNode, node.Kind, param.Name)
		}
		if err := checkValue(param.Type, value); err != nil {
			return fmt.Errorf("%w: %s %s: %w", ErrNode, node.Kind, param.Name, err)
		}
	}

	return nil
}

func checkValue(kind ParamType, value json.RawMessage) error {
	switch kind {
	case TypeAnchor:
		var ref tilefloor.AnchorRef

		return json.Unmarshal(value, &ref) //nolint:wrapcheck // Wrapped by the caller.
	case TypeMinutes:
		return checkNumber(value, 0, MaxMinutes)
	case TypePercent:
		return checkNumber(value, 0, percent)
	case TypeTimeRange:
		var span TimeRange
		if err := json.Unmarshal(value, &span); err != nil {
			return err //nolint:wrapcheck // Wrapped by the caller.
		}

		return checkRange(span)
	case TypeTimeRanges:
		return checkRanges(value)
	case TypeText:
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return err //nolint:wrapcheck // Wrapped by the caller.
		}
		if utf8.RuneCountInString(text) > MaxText {
			return fmt.Errorf("%w: longer than %d characters", ErrNode, MaxText)
		}

		return nil
	default:
		return fmt.Errorf("%w: unknown parameter type %q", ErrNode, kind)
	}
}

func checkNumber(value json.RawMessage, low, high int) error {
	var number int
	if err := json.Unmarshal(value, &number); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if number < low || number > high {
		return fmt.Errorf("%w: %d is outside %d to %d", ErrNode, number, low, high)
	}

	return nil
}

func checkRange(span TimeRange) error {
	if span.From < 0 || span.From >= MinutesPerDay || span.To < 0 || span.To >= MinutesPerDay || span.From == span.To {
		return fmt.Errorf("%w: %s is not a range of the day", ErrNode, FormatRange(span))
	}

	return nil
}

func checkRanges(value json.RawMessage) error {
	var spans []TimeRange
	if err := json.Unmarshal(value, &spans); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if len(spans) > MaxRanges {
		return fmt.Errorf("%w: more than %d times", ErrNode, MaxRanges)
	}
	ports := map[string]bool{OutputOtherwise: true}
	for _, span := range spans {
		if span.Port == "" || ports[span.Port] {
			return fmt.Errorf("%w: each time needs its own output name", ErrNode)
		}
		ports[span.Port] = true
		if err := checkRange(span); err != nil {
			return err
		}
	}

	return nil
}

// FormatRange writes a range as "09:00–17:00".
func FormatRange(span TimeRange) string {
	return FormatTime(span.From) + "–" + FormatTime(span.To)
}

// FormatTime writes minutes from midnight as "09:05".
func FormatTime(minute int) string {
	return fmt.Sprintf("%02d:%02d", minute/hour%(MinutesPerDay/hour), minute%hour)
}

func mustJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	return data
}
