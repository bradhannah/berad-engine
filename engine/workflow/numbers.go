package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Amount is a whole number typed into a node, such as Move's tiles or a
// box's seconds, or a constant standing in for it. The number last typed
// is kept beside a constant, so taking the constant away gives it back. It
// is saved as the number alone, or as {"number": 3, "constant": {...}}.
type Amount struct {
	Number   int
	Constant ValueRef
}

// wireAmount is an amount with a constant, as saved.
type wireAmount struct {
	Number   int      `json:"number"`
	Constant ValueRef `json:"constant"`
}

// MarshalJSON writes the number alone, or the number and its constant.
func (a Amount) MarshalJSON() ([]byte, error) {
	if a.Constant.IsZero() {
		return json.Marshal(a.Number) //nolint:wrapcheck // A number always encodes.
	}

	return json.Marshal(wireAmount(a)) //nolint:wrapcheck // As above.
}

// UnmarshalJSON reads a number, as every version saves one, or a number
// with a constant standing in for it, as version 10 saves one.
func (a *Amount) UnmarshalJSON(data []byte) error {
	var number int
	if err := json.Unmarshal(data, &number); err == nil {
		a.Number, a.Constant = number, ValueRef{Scope: "", Value: [16]byte{}, Owner: [16]byte{}, Found: [16]byte{}}

		return nil
	}
	var wire wireAmount
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("%w: a number, or a number with a constant: %w", ErrNode, err)
	}
	a.Number, a.Constant = wire.Number, wire.Constant

	return nil
}

// Resolve returns the number an amount stands for: its constant's, as read
// now, held between low and high, or else the number typed.
func (a Amount) Resolve(read func(ValueRef) (int, bool), low, high int) int {
	if a.Constant.IsZero() {
		return a.Number
	}
	if number, ok := read(a.Constant); ok {
		return min(max(number, low), high)
	}

	return a.Number
}

// Amount returns a whole-number parameter as typed, with the constant
// standing in for it, if one does.
func (n Node) Amount(name string) (Amount, error) {
	var amount Amount

	return amount, n.decode(name, &amount)
}

// Limits returns the least and the most a whole-number parameter of a kind
// can be: tiles and seconds from 1, and a share from nothing to MaxShare.
// WaitLimits gives a wait's amount's.
func Limits(paramType ParamType) (int, int) {
	if paramType == TypeShares {
		return 0, MaxShare
	}

	return 1, numberLimit(paramType)
}

// WaitLimits returns the least and the most a wait's amount can be in its
// unit: from nothing to a week's worth.
func WaitLimits(wait Duration) (int, int) {
	return 0, MaxWait / max(Duration{Amount: 1, Unit: wait.Unit, Clock: wait.Clock, Constant: noRef()}.Seconds(), 1)
}

// NumberConstant is a constant standing in for a typed-in number: the
// parameter it is in, the option of a random choice it is the share of,
// or -1, and the constant.
type NumberConstant struct {
	Param    string
	Option   int
	Constant ValueRef
}

// NumberConstants lists the constants standing in for a node's typed-in
// numbers.
func (n Node) NumberConstants() []NumberConstant {
	kind, _ := KindOf(n.Kind)
	found := make([]NumberConstant, 0, len(kind.Params))
	for _, param := range kind.Params {
		found = append(found, n.paramConstants(param)...)
	}

	return found
}

// paramConstants lists the constants standing in for one parameter's
// numbers: a whole number's, a wait's amount's or each option's share's.
func (n Node) paramConstants(param Param) []NumberConstant {
	var found []NumberConstant
	if param.Type == TypeTiles || param.Type == TypeSeconds {
		if amount, err := n.Amount(param.Name); err == nil && !amount.Constant.IsZero() {
			found = append(found, NumberConstant{Param: param.Name, Option: -1, Constant: amount.Constant})
		}
	}
	if param.Type == TypeDuration {
		if wait, err := n.Duration(param.Name); err == nil && !wait.Constant.IsZero() {
			found = append(found, NumberConstant{Param: param.Name, Option: -1, Constant: wait.Constant})
		}
	}
	if param.Type == TypeShares {
		options, _ := n.Shares(param.Name) //nolint:errcheck // A choice that can't be read has none.
		for index, option := range options {
			if !option.Constant.IsZero() {
				found = append(found, NumberConstant{Param: param.Name, Option: index, Constant: option.Constant})
			}
		}
	}

	return found
}

// checkAmount reads a whole number, or a number with a constant, the number
// between low and high and the constant one a workflow can name.
func checkAmount(value json.RawMessage, low, high int) error {
	var amount Amount
	if err := json.Unmarshal(value, &amount); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if amount.Number < low || amount.Number > high {
		return fmt.Errorf("%w: %d is outside %d to %d", ErrNode, amount.Number, low, high)
	}

	return checkNumberConstant(amount.Constant)
}

// checkNumberConstant checks a constant standing in for a number: none, or
// the world's, a zone's or an NPC's, never a workflow's own.
func checkNumberConstant(ref ValueRef) error {
	if ref.IsZero() {
		return nil
	}
	if ref.Scope == worldstate.ScopeLocal {
		return fmt.Errorf("%w: a workflow's own values are no constants", ErrNode)
	}

	return checkRef(mustJSON(ref))
}

// NoConstant is no constant standing in for a number: it is typed.
func NoConstant() ValueRef { return noRef() }
