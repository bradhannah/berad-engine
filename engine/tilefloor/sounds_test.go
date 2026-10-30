package tilefloor_test

import (
	"encoding/json"
	"errors"
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

func sound(seed byte, cue string, col, row int) tilefloor.Sound {
	var id ids.ULID
	id[15] = seed

	return tilefloor.Sound{ID: id, Cue: cue, Col: col, Row: row}
}

// TestSounds places ambient sounds on a floor: one per cell, only on the
// floor and naming a cue; resizing removes those outside and returns them.
func TestSounds(t *testing.T) {
	t.Parallel()
	floor, err := tilefloor.NewAt(image.Rect(-2, -2, 2, 2), "grass")
	if err != nil {
		t.Fatal(err)
	}
	if err := floor.InsertSound(0, sound(1, "town/fountain", 0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := floor.InsertSound(1, sound(2, "town/brook", -2, 1)); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]tilefloor.Sound{
		"same cell": sound(3, "a", 0, 0), "off the floor": sound(3, "a", 2, 0), "no cue": sound(3, "", 1, 1),
		"repeated ID": sound(1, "a", 1, 1), "no ID": sound(0, "a", 1, 1),
	} {
		if err := floor.InsertSound(2, bad); !errors.Is(err, tilefloor.ErrSound) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	moved := sound(1, "town/fountain", 1, 1)
	previous, err := floor.UpdateSound(moved)
	if err != nil || previous.Col != 0 {
		t.Fatalf("update: %+v %v", previous, err)
	}
	if found, ok := floor.SoundAt(1, 1); !ok || found.ID != moved.ID {
		t.Fatal("the sound moved")
	}
	removed, err := floor.Resize(image.Rect(-1, -2, 2, 2), "grass")
	if err != nil {
		t.Fatal(err)
	}
	if len(removed.Sounds) != 1 || removed.Sounds[0].Cue != "town/brook" || len(floor.Sounds()) != 1 {
		t.Fatalf("removed %+v, kept %+v", removed, floor.Sounds())
	}
	encoded, err := floor.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tilefloor.Decode(encoded)
	if err != nil || !decoded.Equal(floor) || !floor.Clone().Equal(floor) {
		t.Fatalf("sounds did not round-trip: %v", err)
	}
	if _, _, err := floor.RemoveSound(moved.ID); err != nil || len(floor.Sounds()) != 0 {
		t.Fatalf("remove: %v", err)
	}
	if _, _, err := floor.RemoveSound(moved.ID); !errors.Is(err, tilefloor.ErrNoSound) {
		t.Fatalf("remove twice: %v", err)
	}
}

// TestUpgradeFromVersion4 reads a version 4 floor unchanged, with no
// sounds.
func TestUpgradeFromVersion4(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(tilefloor.Format()); err != nil {
		t.Fatal(err)
	}
	var id ids.ULID
	id[15] = 9
	old := content.Envelope{
		Format: tilefloor.FormatName, Version: 4, ID: id, Name: "Old", Revision: 3,
		Data: json.RawMessage(`{"minCol":0,"minRow":0,"width":1,"height":1,"ground":["grass"],"top":[""],` +
			`"anchors":[],"placements":[]}`),
	}
	upgraded, changed, err := registry.Upgrade(old)
	if err != nil || !changed || upgraded.Version != 5 {
		t.Fatalf("upgrade: %+v %v %v", upgraded, changed, err)
	}
	floor, err := tilefloor.Decode(upgraded.Data)
	if err != nil || len(floor.Sounds()) != 0 {
		t.Fatalf("upgraded floor %v %v", floor, err)
	}
}
