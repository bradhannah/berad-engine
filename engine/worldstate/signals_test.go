package worldstate_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestSignalsAreDeclared saves and reads a world state's signals, finds one
// by identity and by name, refuses two of one name, and reads a version 2
// world state as declaring none.
func TestSignalsAreDeclared(t *testing.T) {
	t.Parallel()
	state := worldstate.NewDocument()
	state.Signals = []worldstate.Signal{
		{ID: id(1), Name: "bread_called", About: "The baker calls for bread."}, {ID: id(2), Name: "door_open"},
	}
	data, err := state.Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := worldstate.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if signal, found := read.Signal(id(1)); !found || signal.About != "The baker calls for bread." {
		t.Fatalf("found by identity: %+v", signal)
	}
	if signal, found := read.SignalNamed("door_open"); !found || signal.ID != id(2) {
		t.Fatalf("found by name: %+v", signal)
	}
	for name, signals := range map[string][]worldstate.Signal{
		"one name": {{ID: id(1), Name: "same"}, {ID: id(2), Name: "same"}},
		"no name":  {{ID: id(1), Name: ""}},
		"bad name": {{ID: id(1), Name: "Bread Called"}},
		"no ID":    {{Name: "lost"}},
		"same ID":  {{ID: id(1), Name: "one"}, {ID: id(1), Name: "two"}},
	} {
		if err := worldstate.CheckSignals(signals); !errors.Is(err, worldstate.ErrValue) {
			t.Fatalf("%s is refused: %v", name, err)
		}
	}
	registry := content.NewRegistry()
	if err := registry.Register(worldstate.Format()); err != nil {
		t.Fatal(err)
	}
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: worldstate.FormatName, Version: 2, ID: worldstate.ID(), Name: worldstate.Name, Revision: 1,
		Data: json.RawMessage(`{"values":[]}`),
	})
	if err != nil || !changed || upgraded.Version != worldstate.FormatVersion {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	if old, err := worldstate.Decode(upgraded.Data); err != nil || len(old.Signals) != 0 {
		t.Fatalf("version 2 declares no signals: %v", err)
	}
}
