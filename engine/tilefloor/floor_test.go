package tilefloor_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

func TestLayers(t *testing.T) {
	t.Parallel()
	floor, err := tilefloor.New(4, 3, "grass")
	if err != nil {
		t.Fatal(err)
	}
	previous, err := floor.SetTop(3, 2, "wall")
	if err != nil || previous != "" {
		t.Fatalf("set top: %q %v", previous, err)
	}
	if ground, err := floor.Ground(3, 2); err != nil || ground != "grass" {
		t.Fatalf("ground under the wall: %q %v", ground, err)
	}
	if previous, err := floor.SetGround(3, 2, "floor"); err != nil || previous != "grass" {
		t.Fatalf("set ground: %q %v", previous, err)
	}
	if top, err := floor.Top(3, 2); err != nil || top != "wall" {
		t.Fatalf("top after ground change: %q %v", top, err)
	}
	for _, pos := range [][2]int{{-1, 0}, {0, -1}, {4, 0}, {0, 3}} {
		if _, err := floor.Ground(pos[0], pos[1]); !errors.Is(err, tilefloor.ErrBounds) {
			t.Fatalf("%v: %v", pos, err)
		}
		if _, err := floor.SetTop(pos[0], pos[1], "door"); !errors.Is(err, tilefloor.ErrBounds) {
			t.Fatalf("%v: %v", pos, err)
		}
	}
}

func TestSizeLimits(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{0, 1}, {1, 0}, {tilefloor.MaxSide + 1, 1}} {
		if _, err := tilefloor.New(size[0], size[1], ""); !errors.Is(err, tilefloor.ErrSize) {
			t.Fatalf("%v: %v", size, err)
		}
	}
}

// TestJoins checks wall variant selection: each wall cell reports which
// neighbours it joins, doors included and other tiles ignored.
func TestJoins(t *testing.T) {
	t.Parallel()
	floor, err := tilefloor.New(3, 3, "floor")
	if err != nil {
		t.Fatal(err)
	}
	// . W .
	// W W D
	// . P .
	for _, cell := range []struct {
		col, row int
		tile     string
	}{{1, 0, "wall"}, {0, 1, "wall.a"}, {1, 1, "wall.b"}, {2, 1, "door"}, {1, 2, "pickup.small"}} {
		if _, err := floor.SetTop(cell.col, cell.row, cell.tile); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := floor.Joins(1, 1), tiles.North|tiles.East|tiles.West; got != want {
		t.Fatalf("centre mask %04b, want %04b", got, want)
	}
	if got := floor.Joins(0, 0); got != tiles.East|tiles.South {
		t.Fatalf("corner mask %04b", got)
	}
	if got := floor.Joins(1, 0); got != tiles.South {
		t.Fatalf("top end mask %04b", got)
	}
}

func anchor(t *testing.T, name string, col, row int, seed byte) tilefloor.Anchor {
	t.Helper()
	var id ids.ULID
	id[15] = seed
	return tilefloor.Anchor{ID: id, Name: name, Col: col, Row: row}
}

func TestAnchors(t *testing.T) {
	t.Parallel()
	floor, err := tilefloor.New(5, 5, "grass")
	if err != nil {
		t.Fatal(err)
	}
	door := anchor(t, "Door", 1, 1, 1)
	desk := anchor(t, "Desk", 3, 2, 2)
	if err := floor.InsertAnchor(10, door); err != nil {
		t.Fatal(err)
	}
	if err := floor.InsertAnchor(0, desk); err != nil {
		t.Fatal(err)
	}
	if got := floor.Anchors(); len(got) != 2 || got[0] != desk || got[1] != door {
		t.Fatalf("order %v", got)
	}
	bad := []tilefloor.Anchor{
		anchor(t, "door", 0, 0, 3),  // name used, ignoring case
		anchor(t, "", 0, 0, 4),      // no name
		anchor(t, " Gap", 0, 0, 5),  // untrimmed
		anchor(t, "Far", 5, 0, 6),   // off the floor
		anchor(t, "Again", 0, 0, 1), // ID used
	}
	for _, item := range bad {
		if err := floor.InsertAnchor(0, item); !errors.Is(err, tilefloor.ErrAnchor) {
			t.Fatalf("%+v: %v", item, err)
		}
	}
	moved := door
	moved.Name, moved.Col = "Front door", 4
	previous, err := floor.UpdateAnchor(moved)
	if err != nil || previous != door {
		t.Fatalf("update: %+v %v", previous, err)
	}
	if found, ok := floor.AnchorAt(4, 1); !ok || found.Name != "Front door" {
		t.Fatalf("anchor at: %+v %v", found, ok)
	}
	clash := desk
	clash.Name = "front DOOR"
	if _, err := floor.UpdateAnchor(clash); !errors.Is(err, tilefloor.ErrAnchor) {
		t.Fatalf("rename clash: %v", err)
	}
	removed, index, err := floor.RemoveAnchor(desk.ID)
	if err != nil || removed != desk || index != 0 {
		t.Fatalf("remove: %+v %d %v", removed, index, err)
	}
	if _, _, err := floor.RemoveAnchor(desk.ID); !errors.Is(err, tilefloor.ErrNoAnchor) {
		t.Fatalf("remove twice: %v", err)
	}
}

func TestEncodeDecodeAndClone(t *testing.T) {
	t.Parallel()
	floor, err := tilefloor.New(2, 2, "floor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := floor.SetTop(1, 0, "wall"); err != nil {
		t.Fatal(err)
	}
	if err := floor.InsertAnchor(0, anchor(t, "Spot", 1, 1, 9)); err != nil {
		t.Fatal(err)
	}
	data, err := floor.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tilefloor.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(floor) {
		t.Fatal("round trip differs")
	}
	clone := floor.Clone()
	if _, err := clone.SetGround(0, 0, "x"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := clone.RemoveAnchor(anchor(t, "", 0, 0, 9).ID); err != nil {
		t.Fatal(err)
	}
	if floor.Equal(clone) || len(floor.Anchors()) != 1 {
		t.Fatal("clone shares state")
	}
}

func TestDecodeRejectsMismatch(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		`{"width":2,"height":2,"ground":["a"],"top":["","","",""],"anchors":[]}`,
		`{"width":1,"height":1,"ground":["a"],"top":[""],"anchors":[{"id":"00000000000000000000000001","name":"A","col":3,"row":0}]}`,
	} {
		if _, err := tilefloor.Decode(json.RawMessage(bad)); !errors.Is(err, tilefloor.ErrData) {
			t.Fatalf("%s: err = %v", bad, err)
		}
	}
}

// TestUpgradeFromVersion1 is the first real format upgrade: version 1 cells
// move to the ground layer, and walls move on top of plain floor.
func TestUpgradeFromVersion1(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(tilefloor.Format()); err != nil {
		t.Fatal(err)
	}
	old := content.Envelope{
		Format: tilefloor.FormatName, Version: 1, ID: anchor(t, "", 0, 0, 7).ID, Name: "Old", Revision: 3,
		Data: json.RawMessage(`{"width":2,"height":1,"tiles":["grass","wall"]}`),
	}
	upgraded, changed, err := registry.Upgrade(old)
	if err != nil || !changed || upgraded.Version != tilefloor.FormatVersion {
		t.Fatalf("upgrade: %+v %v %v", upgraded, changed, err)
	}
	floor, err := tilefloor.Decode(upgraded.Data)
	if err != nil {
		t.Fatal(err)
	}
	ground, _ := floor.Ground(1, 0) //nolint:errcheck // Position is on the floor.
	top, _ := floor.Top(1, 0)       //nolint:errcheck // Position is on the floor.
	grass, _ := floor.Ground(0, 0)  //nolint:errcheck // Position is on the floor.
	if ground != "floor" || top != "wall" || grass != "grass" || len(floor.Anchors()) != 0 {
		t.Fatalf("upgraded cells: %q %q %q", ground, top, grass)
	}
}
