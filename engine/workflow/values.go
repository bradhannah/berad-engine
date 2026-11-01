package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Parameter types for values. A value parameter refers to a declared value;
// an operand is what a value is set to or compared with, of whatever type
// that value has.
const (
	TypeValue   ParamType = "value"
	TypeOperand ParamType = "operand"
)

// GroupValues is the palette group of the nodes that set, check and find
// values.
const GroupValues = "Values"

// ParamValue names the value parameter of Set value and Check value.
const ParamValue = "value"

// Kinds that set, check and find values.
const (
	KindSetValue   = "set-value"
	KindCheckValue = "check-value"
	KindFindNPC    = "find-npc"
)

// Outputs of Find NPC nearby.
const (
	OutputFound = "found"
	OutputNone  = "none"
)

// Ways to set a value, and to compare one.
const (
	HowSet       = "set"
	HowAdd       = "add"
	CompareIs    = "is"
	CompareLeast = "at-least"
	CompareBelow = "below"
)

// Limits on Find NPC nearby.
const (
	defaultWithin = 2
	maxOperand    = 64
)

// ValueRef refers to a declared value: its scope and ID. An NPC value is
// this NPC's own, unless NPC names another NPC by its ID or Found names a
// local value holding the NPC found nearby; another NPC's value can be read,
// never set. The zero reference is none chosen yet.
type ValueRef struct {
	Scope worldstate.Scope `json:"scope,omitempty"`
	Value ids.ULID         `json:"value,omitzero"`
	NPC   ids.ULID         `json:"npc,omitzero"`
	Found ids.ULID         `json:"found,omitzero"`
}

// IsZero reports whether no value is chosen.
func (r ValueRef) IsZero() bool { return r.Value.IsZero() }

// Others reports whether the reference reads another NPC's value.
func (r ValueRef) Others() bool { return !r.NPC.IsZero() || !r.Found.IsZero() }

// Writable reports whether a workflow may set the value: its own NPC's, its
// zone's, the world's or a local one.
func (r ValueRef) Writable() bool { return !r.IsZero() && !r.Others() }

// Ref returns a value parameter.
func (n Node) Ref(name string) (ValueRef, error) {
	var ref ValueRef

	return ref, n.decode(name, &ref)
}

// Operand returns an operand parameter as saved: a number, true or false, a
// choice's ID, or null before a value is chosen.
func (n Node) Operand(name string) (json.RawMessage, error) {
	var raw json.RawMessage

	return raw, n.decode(name, &raw)
}

// NoOperand is an operand before its value is chosen.
func NoOperand() json.RawMessage { return json.RawMessage("null") }

// IsNoOperand reports an operand that is null.
func IsNoOperand(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// valueKinds declares Set value, Check value and Find NPC nearby.
func valueKinds(next []Output) []Kind {
	none := mustJSON(ValueRef{Scope: "", Value: ids.ULID{}, NPC: ids.ULID{}, Found: ids.ULID{}})

	return []Kind{
		{
			Name: KindSetValue, Label: "Set value", Group: GroupValues,
			Params: []Param{
				{Name: ParamValue, Label: "Value", Type: TypeValue, Default: none, Choices: nil},
				{Name: "how", Label: "How", Type: TypeChoice, Default: mustJSON(HowSet), Choices: []Choice{
					{Value: HowSet, Label: "Set to"}, {Value: HowAdd, Label: "Add"},
				}},
				{Name: "to", Label: "To", Type: TypeOperand, Default: NoOperand(), Choices: nil},
			},
			Outputs: next,
		},
		{
			Name: KindCheckValue, Label: "Check value", Group: GroupValues,
			Params: []Param{
				{Name: ParamValue, Label: "Value", Type: TypeValue, Default: none, Choices: nil},
				{Name: "compare", Label: "Compare", Type: TypeChoice, Default: mustJSON(CompareIs), Choices: []Choice{
					{Value: CompareIs, Label: "is"}, {Value: CompareLeast, Label: "at least"},
					{Value: CompareBelow, Label: "below"},
				}},
				{Name: "against", Label: "With", Type: TypeOperand, Default: NoOperand(), Choices: nil},
			},
			Outputs: []Output{{Name: OutputYes, Label: "yes"}, {Name: OutputOtherwise, Label: OutputOtherwise}},
		},
		{
			Name: KindFindNPC, Label: "Find NPC nearby", Group: GroupValues,
			Params: []Param{
				{Name: "within", Label: "Within", Type: TypeTiles, Default: mustJSON(defaultWithin), Choices: nil},
				{Name: "where", Label: "Only one whose", Type: TypeValue, Default: none, Choices: nil},
				{Name: "is", Label: "Is", Type: TypeOperand, Default: NoOperand(), Choices: nil},
				{Name: "into", Label: "Keep it in", Type: TypeValue, Default: none, Choices: nil},
			},
			Outputs: []Output{{Name: OutputFound, Label: "found"}, {Name: OutputNone, Label: "none found"}},
		},
	}
}

// checkRef reads a value parameter. Whether the value is declared, and of
// a type that fits, is for the editor to show and the runner to find: a
// value can be deleted after a node uses it.
func checkRef(value json.RawMessage) error {
	var ref ValueRef
	if err := json.Unmarshal(value, &ref); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if ref.IsZero() {
		return nil
	}
	switch ref.Scope {
	case worldstate.ScopeLocal, worldstate.ScopeZone, worldstate.ScopeWorld:
		if ref.Others() {
			return fmt.Errorf("%w: only an NPC value can be another NPC's", ErrNode)
		}
	case worldstate.ScopeNPC:
		if !ref.NPC.IsZero() && !ref.Found.IsZero() {
			return fmt.Errorf("%w: a value is one NPC's", ErrNode)
		}
	default:
		return fmt.Errorf("%w: %q is not a scope", ErrNode, ref.Scope)
	}

	return nil
}

// checkOperand reads an operand: null, a number, true or false, or a short
// string.
func checkOperand(value json.RawMessage) error {
	if IsNoOperand(value) {
		return nil
	}
	var operand any
	if err := json.Unmarshal(value, &operand); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	switch read := operand.(type) {
	case float64, bool:
		return nil
	case string:
		if utf8.RuneCountInString(read) > maxOperand {
			return fmt.Errorf("%w: longer than %d characters", ErrNode, maxOperand)
		}

		return nil
	}

	return fmt.Errorf("%w: %s is not a number, yes or no, or a choice", ErrNode, value)
}

// OperandOf returns the parameter whose value an operand is of: Set value's and Check
// value's value, or Find NPC nearby's "where".
func OperandOf(kind, operand string) string {
	switch {
	case kind == KindFindNPC && operand == "is":
		return "where"
	case kind == KindSetValue || kind == KindCheckValue:
		return ParamValue
	}

	return ""
}
