package worldstate_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// TestSignalsAreDeclared saves and reads a world state's signals, finds one
// by identity and by path, refuses two of one name in one place, and reads
// a version 2 world state as declaring none.
func TestSignalsAreDeclared(t *testing.T) {
	t.Parallel()
	state := worldstate.NewDocument()
	state.Signals = []worldstate.Signal{
		{ID: id(1), Name: "bread_called", Description: "The baker calls for bread."}, {ID: id(2), Name: "door_open"},
		{ID: id(3), Name: "shop", Group: true}, {ID: id(4), Name: "door_open", Parent: id(3)},
	}
	data, err := state.Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := worldstate.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if signal, found := read.Signal(id(1)); !found || signal.Description != "The baker calls for bread." {
		t.Fatalf("found by identity: %+v", signal)
	}
	if signal, found := read.SignalAt("door_open"); !found || signal.ID != id(2) {
		t.Fatalf("found by path: %+v", signal)
	}
	if signal, found := read.SignalAt("shop.door_open"); !found || signal.ID != id(4) {
		t.Fatalf("found in its group by path: %+v", signal)
	}
	if _, found := read.Signal(id(3)); found {
		t.Fatal("a group is not a signal")
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

// TestSignalGroups names signals by their paths, lists what a group holds
// and everything within it, and refuses a signal in no group, in a signal,
// in itself, or beside another of its name.
func TestSignalGroups(t *testing.T) {
	t.Parallel()
	signals := []worldstate.Signal{
		{ID: id(1), Name: "shop", Group: true}, {ID: id(2), Name: "bell_rung", Parent: id(1)},
		{ID: id(3), Name: "stock", Group: true, Parent: id(1)}, {ID: id(4), Name: "out", Parent: id(3)},
		{ID: id(5), Name: "dawn"},
	}
	if err := worldstate.CheckSignals(signals); err != nil {
		t.Fatal(err)
	}
	if path := worldstate.SignalPath(signals, id(4)); path != "shop.stock.out" {
		t.Fatalf("a signal's path: %q", path)
	}
	held := worldstate.SignalChildren(signals, id(1))
	if len(held) != 2 || held[0].ID != id(3) || held[1].ID != id(2) {
		t.Fatalf("a group's groups come before its signals: %+v", held)
	}
	if within := worldstate.SignalsWithin(signals, id(1)); len(within) != 3 {
		t.Fatalf("everything within a group: %v", within)
	}
	shop := worldstate.Signal{ID: id(1), Name: "shop", Group: true}
	for name, wrong := range map[string][]worldstate.Signal{
		"in no group": {{ID: id(1), Name: "lost", Parent: id(9)}},
		"in a signal": {{ID: id(1), Name: "bell"}, {ID: id(2), Name: "ring", Parent: id(1)}},
		"in itself":   {{ID: id(1), Name: "loop", Group: true, Parent: id(1)}},
		"beside one":  {shop, {ID: id(2), Name: "a", Parent: id(1)}, {ID: id(3), Name: "a", Parent: id(1)}},
	} {
		if err := worldstate.CheckSignals(wrong); !errors.Is(err, worldstate.ErrValue) {
			t.Fatalf("a signal %s is refused: %v", name, err)
		}
	}
}
