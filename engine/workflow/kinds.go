package workflow

import (
	"encoding/json"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/loudness"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

// ParamType is the type of a node parameter. The editor builds each
// parameter's control from its type: an anchor gets "Pick on map", a time
// range gets time fields.
type ParamType string

// Parameter types.
const (
	TypeAnchor     ParamType = "anchor"
	TypeChoice     ParamType = "choice"
	TypeDuration   ParamType = "duration"
	TypeShares     ParamType = "shares"
	TypeTiles      ParamType = "tiles"
	TypeTimeRange  ParamType = "time-range"
	TypeTimeRanges ParamType = "time-ranges"
	TypeText       ParamType = "text"
	// TypeCue names a cue of the project's audio library by its ID, as the
	// library spells it; "" means none is chosen.
	TypeCue ParamType = "cue"
)

// Limits on parameter values.
const (
	// MaxWait is the longest wait, in seconds: a week.
	MaxWait = 7 * MinutesPerDay * secondsPerMinute
	// MinutesPerDay is the length of a game day in minutes.
	MinutesPerDay = 24 * 60
	// MaxRanges bounds the branches of a time switch.
	MaxRanges = 24
	// MaxOptions bounds the options of a random choice; MaxShare bounds
	// one option's share.
	MaxOptions = 12
	MaxShare   = 100
	// MaxTiles bounds how far one Move goes.
	MaxTiles = 64
	// MaxText bounds a text parameter, in characters.
	MaxText = 2000
	// MaxCue bounds a cue parameter's ID, in characters.
	MaxCue           = 200
	percent          = 100
	hour             = 60
	secondsPerMinute = 60
	// Defaults for new nodes: a working day, an even choice, half an hour.
	workStart    = 9 * hour
	workEnd      = 17 * hour
	defaultShare = 50
	defaultWait  = 30
)

// Units of a wait.
const (
	UnitSeconds = "seconds"
	UnitMinutes = "minutes"
	UnitHours   = "hours"
)

// Clocks a wait can count on: the game clock, which the preview can pause
// and speed up, or a real clock, which ignores both.
const (
	ClockGame = "game"
	ClockReal = "real"
)

// Directions a Move can go, as the map shows them.
const (
	DirectionUp    = "up"
	DirectionDown  = "down"
	DirectionLeft  = "left"
	DirectionRight = "right"
)

// Palette groups.
const (
	GroupFlow     = "Flow"
	GroupMovement = "Movement"
	GroupAction   = "Action"
	GroupAudio    = "Audio"
	GroupAnnotate = "Annotate"
)

// Kind names.
const (
	KindStart      = "start"
	KindTimeSwitch = "time-switch"
	KindRandom     = "random"
	KindContainer  = "container"
	KindGoTo       = "go-to"
	KindMove       = "move"
	KindWait       = "wait"
	KindPlaySound  = "play-sound"
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
	// Choices are the values a choice parameter can take.
	Choices []Choice
}

// Choice is one value of a choice parameter.
type Choice struct {
	Value string
	Label string
}

// Duration is how long a wait lasts, in a unit, on the game clock or a real
// one.
type Duration struct {
	Amount int    `json:"amount"`
	Unit   string `json:"unit"`
	Clock  string `json:"clock"`
}

// Seconds returns the duration in seconds.
func (d Duration) Seconds() int {
	switch d.Unit {
	case UnitHours:
		return d.Amount * hour * secondsPerMinute
	case UnitMinutes:
		return d.Amount * secondsPerMinute
	default:
		return d.Amount
	}
}

// Share is one option of a random choice: the output it takes, its name and
// how large a share of the choices it gets. Each option is taken its share
// of the total of all shares.
type Share struct {
	Port  string `json:"port"`
	Label string `json:"label"`
	Share int    `json:"share"`
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
	// Group places the kind in the palette: Flow, Movement, Action, Audio or
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
				Default: mustJSON([]TimeRange{{Port: "r1", From: workStart, To: workEnd}}), Choices: nil,
			}},
			Outputs: []Output{{Name: OutputOtherwise, Label: "otherwise"}},
		},
		randomKind(),
		{
			Name: KindContainer, Label: "Time-bound container", Group: GroupFlow,
			Params: []Param{
				{Name: "name", Label: "Name", Type: TypeText, Default: mustJSON("Group"), Choices: nil},
				{
					Name: "bound", Label: "Time", Type: TypeTimeRange,
					Default: mustJSON(TimeRange{Port: "", From: workStart, To: workEnd}), Choices: nil,
				},
			},
			Outputs: []Output{{Name: OutputInside, Label: "first step"}, {Name: OutputExit, Label: "exit"}},
		},
		{
			Name: KindGoTo, Label: "Go to", Group: GroupMovement,
			Params: []Param{{
				Name: "destination", Label: "Destination", Type: TypeAnchor,
				Default: mustJSON(tilefloor.AnchorRef{Floor: [16]byte{}, Anchor: [16]byte{}}), Choices: nil,
			}},
			Outputs: next,
		},
		moveKind(next),
		{
			Name: KindWait, Label: "Wait", Group: GroupAction,
			Params: []Param{{
				Name: "duration", Label: "How long", Type: TypeDuration, Choices: nil,
				Default: mustJSON(Duration{Amount: defaultWait, Unit: UnitMinutes, Clock: ClockGame}),
			}},
			Outputs: next,
		},
		playSoundKind(next),
		{
			Name: KindNote, Label: "Note", Group: GroupAnnotate,
			Params:  []Param{{Name: "text", Label: "Note", Type: TypeText, Default: mustJSON(""), Choices: nil}},
			Outputs: nil,
		},
	}
}

// randomKind declares Random choice: two even options to start with, and an
// output for each option.
func randomKind() Kind {
	return Kind{
		Name: KindRandom, Label: "Random choice", Group: GroupFlow,
		Params: []Param{{
			Name: "options", Label: "Options", Type: TypeShares, Choices: nil,
			Default: mustJSON([]Share{
				{Port: "o1", Label: "option 1", Share: defaultShare}, {Port: "o2", Label: "option 2", Share: defaultShare},
			}),
		}},
		Outputs: nil,
	}
}

// LoudnessAsCue is a Play sound's loudness when it plays its cue at the cue's
// own step; any other value is a step of package loudness, for this moment.
const LoudnessAsCue = "cue"

// playSoundKind declares Play sound: a cue, and how loud to play it this
// time, as the cue is unless a moment must stand out or sit back.
func playSoundKind(next []Output) Kind {
	steps := loudness.Steps()
	choices := make([]Choice, 0, 1+len(steps))
	choices = append(choices, Choice{Value: LoudnessAsCue, Label: "As the cue is"})
	for _, step := range steps {
		choices = append(choices, Choice{Value: step.ID, Label: step.Label})
	}

	return Kind{
		Name: KindPlaySound, Label: "Play sound", Group: GroupAudio,
		Params: []Param{
			{Name: "cue", Label: "Cue", Type: TypeCue, Default: mustJSON(""), Choices: nil},
			{Name: "loudness", Label: "Loudness this time", Type: TypeChoice, Default: mustJSON(LoudnessAsCue),
				Choices: choices},
		},
		Outputs: next,
	}
}

// moveKind declares Move: a direction and a number of tiles.
func moveKind(next []Output) Kind {
	return Kind{
		Name: KindMove, Label: "Move", Group: GroupMovement,
		Params: []Param{
			{
				Name: "direction", Label: "Direction", Type: TypeChoice, Default: mustJSON(DirectionUp),
				Choices: []Choice{
					{Value: DirectionUp, Label: "Up"}, {Value: DirectionDown, Label: "Down"},
					{Value: DirectionLeft, Label: "Left"}, {Value: DirectionRight, Label: "Right"},
				},
			},
			{Name: "tiles", Label: "Tiles", Type: TypeTiles, Default: mustJSON(1), Choices: nil},
		},
		Outputs: next,
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
// first, one output per range, and a random choice's options, one output per
// option.
func Outputs(node Node) []Output {
	declared, ok := KindOf(node.Kind)
	if !ok {
		return nil
	}
	if node.Kind == KindRandom {
		options, err := node.Shares("options")
		if err != nil {
			return nil
		}
		outputs := make([]Output, 0, len(options))
		for _, option := range options {
			outputs = append(outputs, Output{Name: option.Port, Label: option.Label})
		}

		return outputs
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
		if err := checkValue(param, value); err != nil {
			return fmt.Errorf("%w: %s %s: %w", ErrNode, node.Kind, param.Name, err)
		}
	}

	return nil
}

func checkValue(param Param, value json.RawMessage) error {
	switch param.Type {
	case TypeAnchor:
		var ref tilefloor.AnchorRef

		return json.Unmarshal(value, &ref) //nolint:wrapcheck // Wrapped by the caller.
	case TypeChoice:
		var chosen string
		if err := json.Unmarshal(value, &chosen); err != nil {
			return err //nolint:wrapcheck // Wrapped by the caller.
		}
		if !slices.ContainsFunc(param.Choices, func(choice Choice) bool { return choice.Value == chosen }) {
			return fmt.Errorf("%w: %q is not one of the choices", ErrNode, chosen)
		}

		return nil
	case TypeTiles:
		return checkNumber(value, 1, MaxTiles)
	case TypeDuration:
		return checkDuration(value)
	case TypeShares:
		return checkShares(value)
	case TypeTimeRange:
		var span TimeRange
		if err := json.Unmarshal(value, &span); err != nil {
			return err //nolint:wrapcheck // Wrapped by the caller.
		}

		return checkRange(span)
	case TypeTimeRanges:
		return checkRanges(value)
	case TypeText:
		return checkText(value, MaxText)
	case TypeCue:
		return checkText(value, MaxCue)
	default:
		return fmt.Errorf("%w: unknown parameter type %q", ErrNode, param.Type)
	}
}

// checkText reads a string of at most limit characters.
func checkText(value json.RawMessage, limit int) error {
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if utf8.RuneCountInString(text) > limit {
		return fmt.Errorf("%w: longer than %d characters", ErrNode, limit)
	}

	return nil
}

func checkDuration(value json.RawMessage) error {
	var wait Duration
	if err := json.Unmarshal(value, &wait); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if wait.Unit != UnitSeconds && wait.Unit != UnitMinutes && wait.Unit != UnitHours {
		return fmt.Errorf("%w: %q is not a unit of time", ErrNode, wait.Unit)
	}
	if wait.Clock != ClockGame && wait.Clock != ClockReal {
		return fmt.Errorf("%w: %q is not a clock", ErrNode, wait.Clock)
	}
	if wait.Amount < 0 || wait.Seconds() > MaxWait {
		return fmt.Errorf("%w: a wait lasts from nothing to a week", ErrNode)
	}

	return nil
}

func checkShares(value json.RawMessage) error {
	var options []Share
	if err := json.Unmarshal(value, &options); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if len(options) == 0 || len(options) > MaxOptions {
		return fmt.Errorf("%w: a random choice has 1 to %d options", ErrNode, MaxOptions)
	}
	ports, total := map[string]bool{}, 0
	for _, option := range options {
		if option.Port == "" || ports[option.Port] {
			return fmt.Errorf("%w: each option needs its own output name", ErrNode)
		}
		ports[option.Port] = true
		if option.Share < 0 || option.Share > MaxShare {
			return fmt.Errorf("%w: a share is from 0 to %d", ErrNode, MaxShare)
		}
		if utf8.RuneCountInString(option.Label) > MaxText {
			return fmt.Errorf("%w: an option name is longer than %d characters", ErrNode, MaxText)
		}
		total += option.Share
	}
	if total == 0 {
		return fmt.Errorf("%w: at least one option needs a share", ErrNode)
	}

	return nil
}

// Percentages returns what each option's share works out to, in whole per
// cent, in the options' order.
func Percentages(options []Share) []int {
	total := 0
	for _, option := range options {
		total += option.Share
	}
	result := make([]int, len(options))
	if total == 0 {
		return result
	}
	for index, option := range options {
		result[index] = (option.Share*percent + total/2) / total
	}

	return result
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
