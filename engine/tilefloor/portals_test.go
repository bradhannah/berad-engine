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

func portal(seed byte, col, row int) tilefloor.Portal {
	var id, floor, anchor ids.ULID
	id[15], floor[15], anchor[15] = seed, 100, 101

	return tilefloor.Portal{
		ID: id, Col: col, Row: row, To: tilefloor.AnchorRef{Floor: floor, Anchor: anchor}, Change: tilefloor.ChangeCut,
	}
}

// TestPortals puts portals on a floor: one per cell, only on the floor,
// leading to an anchor and changing the screen in a way the engine knows;
// resizing removes those outside and returns them.
func TestPortals(t *testing.T) {
	t.Parallel()
	floor, err := tilefloor.NewAt(image.Rect(-2, -2, 2, 2), "grass")
	if err != nil {
		t.Fatal(err)
	}
	if err := floor.InsertPortal(0, portal(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := floor.InsertPortal(1, portal(2, -2, 1)); err != nil {
		t.Fatal(err)
	}
	nowhere, unknown := portal(3, 1, 1), portal(3, 1, 1)
	nowhere.To = tilefloor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}
	unknown.Change = "wipe"
	for name, bad := range map[string]tilefloor.Portal{
		"same cell": portal(3, 0, 0), "off the floor": portal(3, 2, 0), "leads nowhere": nowhere,
		"unknown change": unknown, "repeated ID": portal(1, 1, 1), "no ID": portal(0, 1, 1),
	} {
		if err := floor.InsertPortal(2, bad); !errors.Is(err, tilefloor.ErrPortal) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	moved := portal(1, 1, 1)
	moved.Change = tilefloor.ChangeFade
	previous, err := floor.UpdatePortal(moved)
	if err != nil || previous.Col != 0 || previous.Change != tilefloor.ChangeCut {
		t.Fatalf("update: %+v %v", previous, err)
	}
	if found, ok := floor.PortalAt(1, 1); !ok || found != moved {
		t.Fatal("the portal moved and fades")
	}
	if _, err := floor.UpdatePortal(portal(1, -2, 1)); !errors.Is(err, tilefloor.ErrPortal) {
		t.Fatalf("a portal moved onto another: %v", err)
	}
	removed, err := floor.Resize(image.Rect(-1, -2, 2, 2), "grass")
	if err != nil {
		t.Fatal(err)
	}
	if len(removed.Portals) != 1 || removed.Portals[0].Col != -2 || len(floor.Portals()) != 1 {
		t.Fatalf("removed %+v, kept %+v", removed, floor.Portals())
	}
	encoded, err := floor.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tilefloor.Decode(encoded)
	if err != nil || !decoded.Equal(floor) || !floor.Clone().Equal(floor) {
		t.Fatalf("portals did not round-trip: %v", err)
	}
	if other := floor.Clone(); other.Equal(floor) {
		if _, _, err := other.RemovePortal(moved.ID); err != nil || other.Equal(floor) {
			t.Fatalf("a floor without the portal is another floor: %v", err)
		}
	}
	if _, _, err := floor.RemovePortal(moved.ID); err != nil || len(floor.Portals()) != 0 {
		t.Fatalf("remove: %v", err)
	}
	if _, _, err := floor.RemovePortal(moved.ID); !errors.Is(err, tilefloor.ErrNoPortal) {
		t.Fatalf("remove twice: %v", err)
	}
}

// TestUpgradeFromVersion5 reads a version 5 floor unchanged, with no
// portals.
func TestUpgradeFromVersion5(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(tilefloor.Format()); err != nil {
		t.Fatal(err)
	}
	var id ids.ULID
	id[15] = 9
	old := content.Envelope{
		Format: tilefloor.FormatName, Version: 5, ID: id, Name: "Old", Revision: 3,
		Data: json.RawMessage(`{"minCol":0,"minRow":0,"width":1,"height":1,"ground":["grass"],"top":[""],` +
			`"anchors":[],"placements":[],"sounds":[{"id":"00000000000000000000000001","cue":"a","col":0,"row":0}]}`),
	}
	upgraded, changed, err := registry.Upgrade(old)
	if err != nil || !changed || upgraded.Version != tilefloor.FormatVersion {
		t.Fatalf("upgrade: %+v %v %v", upgraded, changed, err)
	}
	floor, err := tilefloor.Decode(upgraded.Data)
	if err != nil || len(floor.Portals()) != 0 || len(floor.Sounds()) != 1 {
		t.Fatalf("upgraded floor %v %v", floor, err)
	}
}
