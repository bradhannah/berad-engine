package worldstate

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// MaxSignals bounds the signals and groups of signals a project declares.
const MaxSignals = 256

// Signal is a signal the project declares, which one workflow sends and
// another answers, or a group of signals: its identity, which stays the
// same when it is renamed or moved; its name, such as bell_rung; a
// description; and the group it is in. A signal is named everywhere by its
// path, its groups' names and its own joined by dots, such as
// shop.bell_rung. A group holds signals and other groups, as a value group
// does, and is never sent.
type Signal struct {
	ID          ids.ULID `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	// Parent is the group it is in; zero at the top.
	Parent ids.ULID `json:"parent,omitzero"`
	// Group marks a group of signals.
	Group bool `json:"group,omitempty"`
}

// CheckSignals reports signals and groups that cannot be declared
// together: each needs an identity of its own, a name fit for a path that
// nothing beside it has, and a description of at most MaxText characters;
// each is at the top or in a group, no deeper than MaxDepth.
func CheckSignals(signals []Signal) error {
	if len(signals) > MaxSignals {
		return fmt.Errorf("%w: at most %d signals and groups", ErrValue, MaxSignals)
	}
	byID, names := map[ids.ULID]Signal{}, map[ids.ULID]map[string]bool{}
	for _, signal := range signals {
		if signal.ID.IsZero() || byID[signal.ID].ID == signal.ID {
			return fmt.Errorf("%w: each signal needs its own identity", ErrValue)
		}
		byID[signal.ID] = signal
	}
	for _, signal := range signals {
		if err := CheckName(signal.Name); err != nil {
			return err
		}
		if names[signal.Parent] == nil {
			names[signal.Parent] = map[string]bool{}
		}
		if names[signal.Parent][signal.Name] {
			return fmt.Errorf("%w: two signals in one place are named %s", ErrValue, signal.Name)
		}
		names[signal.Parent][signal.Name] = true
		if utf8.RuneCountInString(signal.Description) > MaxText {
			return fmt.Errorf("%w: a signal's description is longer than %d characters", ErrValue, MaxText)
		}
		if err := checkSignalPlace(byID, signal); err != nil {
			return err
		}
	}

	return nil
}

// checkSignalPlace reports a signal whose group is not a declared group, or
// that sits more than MaxDepth groups deep, as one inside itself would.
func checkSignalPlace(byID map[ids.ULID]Signal, signal Signal) error {
	parent := signal.Parent
	for depth := 0; !parent.IsZero(); depth++ {
		group, found := byID[parent]
		switch {
		case !found || !group.Group:
			return fmt.Errorf("%w: %s is in a group that isn't declared", ErrValue, signal.Name)
		case depth >= MaxDepth:
			return fmt.Errorf("%w: %s is more than %d groups deep", ErrValue, signal.Name, MaxDepth)
		}
		parent = group.Parent
	}

	return nil
}

// SignalPath names a declared signal or group by its path, such as
// shop.bell_rung, or "" when none has the identity.
func SignalPath(signals []Signal, named ids.ULID) string {
	var names []string
	for range MaxDepth + 1 {
		index := slices.IndexFunc(signals, func(signal Signal) bool { return signal.ID == named })
		if index < 0 {
			break
		}
		names = append(names, signals[index].Name)
		named = signals[index].Parent
		if named.IsZero() {
			slices.Reverse(names)

			return strings.Join(names, ".")
		}
	}

	return ""
}

// SignalChildren lists what a group holds, or the top for zero: its groups
// by name, then its signals by name.
func SignalChildren(signals []Signal, parent ids.ULID) []Signal {
	var held []Signal
	for _, signal := range signals {
		if signal.Parent == parent {
			held = append(held, signal)
		}
	}
	slices.SortFunc(held, func(first, second Signal) int {
		if first.Group != second.Group {
			if first.Group {
				return -1
			}

			return 1
		}

		return cmp.Compare(first.Name, second.Name)
	})

	return held
}

// SignalsWithin lists every signal and group inside a group, at any depth.
func SignalsWithin(signals []Signal, group ids.ULID) []ids.ULID {
	var within []ids.ULID
	for _, held := range SignalChildren(signals, group) {
		within = append(within, held.ID)
		if held.Group {
			within = append(within, SignalsWithin(signals, held.ID)...)
		}
	}

	return within
}

// Signal finds a declared signal, not a group, by its identity.
func (d *Document) Signal(id ids.ULID) (Signal, bool) {
	index := slices.IndexFunc(d.Signals, func(signal Signal) bool { return signal.ID == id && !signal.Group })
	if index < 0 {
		return noSignal(), false
	}

	return d.Signals[index], true
}

// SignalAt finds a declared signal, not a group, by its path.
func (d *Document) SignalAt(path string) (Signal, bool) {
	index := slices.IndexFunc(d.Signals, func(signal Signal) bool {
		return !signal.Group && SignalPath(d.Signals, signal.ID) == path
	})
	if index < 0 {
		return noSignal(), false
	}

	return d.Signals[index], true
}

func noSignal() Signal {
	return Signal{ID: ids.ULID{}, Name: "", Description: "", Parent: ids.ULID{}, Group: false}
}

// upgradeFrom2 reads a version 2 world state unchanged: version 3 added
// signals, of which it declares none.
func upgradeFrom2(data json.RawMessage) (json.RawMessage, error) { return data, nil }
