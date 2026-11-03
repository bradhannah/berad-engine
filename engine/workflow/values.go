package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
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

// Ways to set a value, and to compare one. Which comparisons a value takes
// follows its type, see Comparisons: an integer compares with is, is not,
// below, at most, above, at least and between, or asks whether it is at its
// maximum or minimum, with nothing to compare with; a bool only is; a
// choice is any of, or none of, a list; text is, or is not.
const (
	HowSet         = "set"
	HowAdd         = "add"
	CompareIs      = "is"
	CompareNot     = "is-not"
	CompareLeast   = "at-least"
	CompareMost    = "at-most"
	CompareAbove   = "above"
	CompareBelow   = "below"
	CompareBetween = "between"
	CompareMax     = "at-max"
	CompareMin     = "at-min"
	CompareAnyOf   = "any-of"
	CompareNoneOf  = "none-of"
)

// Check value's parameters besides its value: the comparison, what the
// value is compared with, the top of between's range, and the list any of
// and none of look in.
const (
	ParamCompare = "compare"
	ParamAgainst = "against"
	ParamTo      = "to"
	ParamAmong   = "among"
)

// Comparisons lists the comparisons a value of a type takes, in the order
// the inspector offers them; a type with no value to compare takes none.
func Comparisons(kind worldstate.Type) []string {
	switch kind {
	case worldstate.TypeInteger:
		return []string{
			CompareIs, CompareNot, CompareBelow, CompareMost, CompareAbove, CompareLeast, CompareBetween, CompareMax,
			CompareMin,
		}
	case worldstate.TypeBool:
		return []string{CompareIs}
	case worldstate.TypeChoice:
		return []string{CompareAnyOf, CompareNoneOf}
	case worldstate.TypeText:
		return []string{CompareIs, CompareNot}
	case worldstate.TypeNPC, worldstate.TypePlace, worldstate.TypeGroup, worldstate.TypeTemplate:
	}

	return nil
}

// CanCompare reports whether a value of a type takes a comparison. Is,
// equality, is taken by every type with a value, so a choice checked with
// is from before any of existed still runs.
func CanCompare(compare string, kind worldstate.Type) bool {
	if compare == CompareIs {
		return len(Comparisons(kind)) > 0
	}

	return slices.Contains(Comparisons(kind), compare)
}

// Limits on Find NPC nearby.
const defaultWithin = 2

// maxOperand bounds an operand typed as text, in characters.
const maxOperand = worldstate.MaxText

// ValueRef refers to a declared value: its scope and ID. An NPC or zone
// value is this NPC's or zone's own, unless Owner names another NPC or zone
// by its ID, or Found names a local value holding the NPC found nearby;
// another's value can be read, never set. The zero reference is none chosen
// yet.
type ValueRef struct {
	Scope worldstate.Scope `json:"scope,omitempty"`
	Value ids.ULID         `json:"value,omitzero"`
	Owner ids.ULID         `json:"owner,omitzero"`
	Found ids.ULID         `json:"found,omitzero"`
}

// IsZero reports whether no value is chosen.
func (r ValueRef) IsZero() bool { return r.Value.IsZero() }

// Others reports whether the reference reads another NPC's or zone's value.
func (r ValueRef) Others() bool { return !r.Owner.IsZero() || !r.Found.IsZero() }

// Unlimited reports whether a comparison asks about a limit, with nothing
// to compare with.
func Unlimited(compare string) bool { return compare == CompareMax || compare == CompareMin }

// Writable reports whether a workflow may set the value: its own NPC's, its
// zone's, the world's or a local one.
func (r ValueRef) Writable() bool { return !r.IsZero() && !r.Others() }

// Ref returns a value parameter.
func (n Node) Ref(name string) (ValueRef, error) {
	var ref ValueRef

	return ref, n.decode(name, &ref)
}

// Operand returns an operand parameter as saved: an integer, true or false,
// a choice's ID, text, or null before a value is chosen.
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

// OperandRef reads an operand that refers to another value, saved as
// {"ref": ...}, read when the check runs rather than holding a value.
func OperandRef(raw json.RawMessage) (ValueRef, bool) {
	var wire struct {
		Ref *ValueRef `json:"ref"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil || wire.Ref == nil || wire.Ref.IsZero() {
		return ValueRef{Scope: "", Value: ids.ULID{}, Owner: ids.ULID{}, Found: ids.ULID{}}, false
	}

	return *wire.Ref, true
}

// RefOperand saves a reference to another value as an operand.
func RefOperand(ref ValueRef) json.RawMessage { return mustJSON(map[string]ValueRef{"ref": ref}) }

// Operands reads an operand holding a list, as any of and none of take,
// into its items; an operand holding one value is one item, and none is
// no items.
func Operands(raw json.RawMessage) []json.RawMessage {
	if IsNoOperand(raw) {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		return items
	}

	return []json.RawMessage{raw}
}

// OperandList saves items as one operand holding a list; no items is no
// operand.
func OperandList(items []json.RawMessage) json.RawMessage {
	if len(items) == 0 {
		return NoOperand()
	}

	return mustJSON(items)
}

// noRef is a value parameter before its value is chosen.
func noRef() ValueRef {
	return ValueRef{Scope: "", Value: ids.ULID{}, Owner: ids.ULID{}, Found: ids.ULID{}}
}

// valueKinds declares Set value, Check value and Find NPC nearby.
func valueKinds(next []Output) []Kind {
	none := mustJSON(noRef())

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
				{Name: ParamCompare, Label: "Compare", Type: TypeChoice, Default: mustJSON(CompareIs), Choices: []Choice{
					{Value: CompareIs, Label: "is"}, {Value: CompareNot, Label: "is not"},
					{Value: CompareBelow, Label: "below"}, {Value: CompareMost, Label: "at most"},
					{Value: CompareAbove, Label: "above"}, {Value: CompareLeast, Label: "at least"},
					{Value: CompareBetween, Label: "between"}, {Value: CompareMax, Label: "at its maximum"},
					{Value: CompareMin, Label: "at its minimum"}, {Value: CompareAnyOf, Label: "any of"},
					{Value: CompareNoneOf, Label: "none of"},
				}},
				{Name: ParamAgainst, Label: "With", Type: TypeOperand, Default: NoOperand(), Choices: nil},
				{Name: ParamTo, Label: "and", Type: TypeOperand, Default: NoOperand(), Choices: nil},
				{Name: ParamAmong, Label: "Among", Type: TypeOperand, Default: NoOperand(), Choices: nil},
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
	case worldstate.ScopeLocal, worldstate.ScopeWorld:
		if ref.Others() {
			return fmt.Errorf("%w: only an NPC's or zone's value can be another's", ErrNode)
		}
	case worldstate.ScopeZone:
		if !ref.Found.IsZero() {
			return fmt.Errorf("%w: only an NPC is found nearby", ErrNode)
		}
	case worldstate.ScopeNPC:
		if !ref.Owner.IsZero() && !ref.Found.IsZero() {
			return fmt.Errorf("%w: a value is one NPC's", ErrNode)
		}
	default:
		return fmt.Errorf("%w: %q is not a scope", ErrNode, ref.Scope)
	}

	return nil
}

// checkOperand reads an operand: null, an integer, true or false, a string
// no longer than text can be, a list of those, a place, or a reference to
// another value.
func checkOperand(value json.RawMessage) error {
	if IsNoOperand(value) {
		return nil
	}
	var operand any
	if err := json.Unmarshal(value, &operand); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	switch read := operand.(type) {
	case []any:
		for _, item := range read {
			if err := checkOperandItem(item); err != nil {
				return err
			}
		}

		return nil
	case map[string]any:
		if _, floor := read["floor"]; floor && read["ref"] == nil {
			var place worldstate.Place

			return json.Unmarshal(value, &place) //nolint:wrapcheck // Wrapped by the caller.
		}
		ref, err := json.Marshal(read["ref"])
		if err != nil || read["ref"] == nil {
			return fmt.Errorf("%w: %s is not a reference to a value", ErrNode, value)
		}

		return checkRef(ref)
	default:
		return checkOperandItem(operand)
	}
}

// checkOperandItem reads one value of an operand: an integer, true or
// false, or a string no longer than text can be.
func checkOperandItem(item any) error {
	switch read := item.(type) {
	case float64, bool:
		return nil
	case string:
		if utf8.RuneCountInString(read) > maxOperand {
			return fmt.Errorf("%w: longer than %d characters", ErrNode, maxOperand)
		}

		return nil
	default:
		return fmt.Errorf("%w: %v is not an integer, true or false, a choice or text", ErrNode, item)
	}
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
