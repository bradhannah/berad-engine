package worldstate

import (
	"encoding/json"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// MaxSignals bounds the signals a project declares.
const MaxSignals = 256

// Signal is a signal the project declares, which one workflow sends and
// another answers: its identity, which stays the same when it is renamed;
// its name, such as bread_called; and a line about it.
type Signal struct {
	ID    ids.ULID `json:"id"`
	Name  string   `json:"name"`
	About string   `json:"about,omitempty"`
}

// CheckSignals reports signals that cannot be declared together: each needs
// an identity and a name of its own, fit for a path, and a line of at most
// MaxText characters.
func CheckSignals(signals []Signal) error {
	if len(signals) > MaxSignals {
		return fmt.Errorf("%w: at most %d signals", ErrValue, MaxSignals)
	}
	names, seen := map[string]bool{}, map[ids.ULID]bool{}
	for _, signal := range signals {
		if signal.ID.IsZero() || seen[signal.ID] {
			return fmt.Errorf("%w: each signal needs its own identity", ErrValue)
		}
		if err := CheckName(signal.Name); err != nil {
			return err
		}
		if names[signal.Name] {
			return fmt.Errorf("%w: two signals are named %s", ErrValue, signal.Name)
		}
		if utf8.RuneCountInString(signal.About) > MaxText {
			return fmt.Errorf("%w: a signal's line is longer than %d characters", ErrValue, MaxText)
		}
		names[signal.Name], seen[signal.ID] = true, true
	}

	return nil
}

// Signal finds a declared signal by its identity.
func (d *Document) Signal(id ids.ULID) (Signal, bool) {
	index := slices.IndexFunc(d.Signals, func(signal Signal) bool { return signal.ID == id })
	if index < 0 {
		return Signal{ID: ids.ULID{}, Name: "", About: ""}, false
	}

	return d.Signals[index], true
}

// SignalNamed finds a declared signal by its name.
func (d *Document) SignalNamed(name string) (Signal, bool) {
	index := slices.IndexFunc(d.Signals, func(signal Signal) bool { return signal.Name == name })
	if index < 0 {
		return Signal{ID: ids.ULID{}, Name: "", About: ""}, false
	}

	return d.Signals[index], true
}

// upgradeFrom2 reads a version 2 world state unchanged: version 3 added
// signals, of which it declares none.
func upgradeFrom2(data json.RawMessage) (json.RawMessage, error) { return data, nil }
