package worldstate

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// SignalsEdit changes the world state's signals.
const SignalsEdit = "worldstate.signals.change"

// SignalsArgs changes the signals at once, all or none: it declares Add,
// then changes each of Set, by its identity, then removes Remove. A signal
// added with no identity is given one. Its inverse is another change.
type SignalsArgs struct {
	Add    []worldstate.Signal `json:"add,omitempty"`
	Set    []worldstate.Signal `json:"set,omitempty"`
	Remove []ids.ULID          `json:"remove,omitempty"`
}

// Signals is the edit kind that changes the signals.
type Signals struct{}

// Name identifies the edit kind.
func (Signals) Name() string { return SignalsEdit }

// Format names the document format the edit applies to.
func (Signals) Format() string { return worldstate.FormatName }

// Prepare gives each signal added with no identity its own, in order, so
// the last added is the edit's Created.
func (Signals) Prepare(_ model.Document, args json.RawMessage, newID model.IDSource) (json.RawMessage, error) {
	var change SignalsArgs
	if err := json.Unmarshal(args, &change); err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	for index := range change.Add {
		if !change.Add[index].ID.IsZero() {
			continue
		}
		made, err := newID()
		if err != nil {
			return nil, fmt.Errorf("new signal ID: %w", err)
		}
		change.Add[index].ID = made
	}

	return encode(change)
}

// Apply makes the change; its inverse undoes it.
func (Signals) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	stateDoc, ok := document.(*Document)
	if !ok {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrWrongDocument, ErrNotWorldState)
	}
	var change SignalsArgs
	if err := json.Unmarshal(args, &change); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	working, inverse, err := change.apply(slices.Clone(stateDoc.State.Signals))
	if err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	stateDoc.State.Signals = working
	data, err := encode(inverse)
	if err != nil {
		return model.Applied{}, err
	}

	return model.Applied{Inverse: data, InverseEdit: "", Touched: change.touched(), Covers: nil}, nil
}

// apply makes the change to a list of signals and returns the list and its
// inverse.
func (c SignalsArgs) apply(signals []worldstate.Signal) ([]worldstate.Signal, SignalsArgs, error) {
	inverse := SignalsArgs{Add: nil, Set: nil, Remove: nil}
	signals = append(signals, c.Add...)
	for _, added := range c.Add {
		inverse.Remove = append(inverse.Remove, added.ID)
	}
	for _, set := range c.Set {
		index := slices.IndexFunc(signals, func(signal worldstate.Signal) bool { return signal.ID == set.ID })
		if index < 0 {
			return nil, inverse, fmt.Errorf("%w: no signal %s", worldstate.ErrValue, set.ID)
		}
		inverse.Set = append(inverse.Set, signals[index])
		signals[index] = set
	}
	for _, removed := range c.Remove {
		index := slices.IndexFunc(signals, func(signal worldstate.Signal) bool { return signal.ID == removed })
		if index < 0 {
			return nil, inverse, fmt.Errorf("%w: no signal %s", worldstate.ErrValue, removed)
		}
		inverse.Add = append(inverse.Add, signals[index])
		signals = slices.Delete(signals, index, index+1)
	}

	return signals, inverse, worldstate.CheckSignals(signals) //nolint:wrapcheck // Wrapped by Apply.
}

// touched lists every signal the change touches.
func (c SignalsArgs) touched() []string {
	items := make([]string, 0, len(c.Add)+len(c.Set)+len(c.Remove))
	for _, signal := range slices.Concat(c.Add, c.Set) {
		items = append(items, SignalItem(signal.ID))
	}
	for _, removed := range c.Remove {
		items = append(items, SignalItem(removed))
	}

	return items
}

// SignalItem names a signal as an edit touches it.
func SignalItem(signal ids.ULID) string { return "signal:" + signal.String() }
