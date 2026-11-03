package zone_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

func floorID(seed byte) ids.ULID {
	var id ids.ULID
	id[15] = seed
	return id
}

func TestFloorsInOrder(t *testing.T) {
	t.Parallel()
	z := zone.New()
	for index, seed := range []byte{1, 2, 3} {
		if err := z.Insert(index, floorID(seed)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Insert(0, floorID(0)); !errors.Is(err, zone.ErrFloor) {
		t.Fatalf("zero ID: %v", err)
	}
	if err := z.Insert(0, floorID(2)); !errors.Is(err, zone.ErrFloor) {
		t.Fatalf("duplicate: %v", err)
	}
	if below, ok := z.Below(floorID(2)); !ok || below != floorID(1) {
		t.Fatalf("below: %v %v", below, ok)
	}
	if _, ok := z.Below(floorID(1)); ok {
		t.Fatal("bottom floor has nothing below")
	}
	previous, err := z.Move(floorID(3), 0)
	if err != nil || previous != 2 {
		t.Fatalf("move: %d %v", previous, err)
	}
	if want := []ids.ULID{floorID(3), floorID(1), floorID(2)}; !slices.Equal(z.Floors(), want) {
		t.Fatalf("after move %v", z.Floors())
	}
	index, err := z.Remove(floorID(1))
	if err != nil || index != 1 {
		t.Fatalf("remove: %d %v", index, err)
	}
	if _, err := z.Remove(floorID(1)); !errors.Is(err, zone.ErrFloor) {
		t.Fatalf("remove twice: %v", err)
	}
}

func TestEncodeDecode(t *testing.T) {
	t.Parallel()
	z := zone.New()
	if err := z.Insert(0, floorID(4)); err != nil {
		t.Fatal(err)
	}
	z.Path, z.Notes = "market", "The east-coast fishing town.\nNever shown to players."
	crowded := worldstate.New(floorID(9), "crowded", worldstate.ScopeZone, worldstate.TypeBool)
	if err := z.Owned.Values.Insert(crowded); err != nil {
		t.Fatal(err)
	}
	data, err := z.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := zone.Decode(data)
	if err != nil || !decoded.Equal(z) {
		t.Fatalf("round trip: %v %v", decoded, err)
	}
	empty, err := zone.New().Encode()
	if err != nil || string(empty) != `{"floors":[]}` {
		t.Fatalf("empty zone saves as %s %v", empty, err)
	}
	if _, err := zone.Decode(json.RawMessage(`{"floors":["00000000000000000000000004","00000000000000000000000004"]}`)); !errors.Is(err, zone.ErrData) {
		t.Fatalf("duplicate floors: %v", err)
	}
	if _, err := zone.Decode(json.RawMessage(`{"floors":[],"path":"Market Square"}`)); !errors.Is(err, zone.ErrData) {
		t.Fatalf("a path name follows the rules for value names: %v", err)
	}
	if copied := z.Clone(); copied.Owned.Values.Insert(worldstate.New(floorID(10), "busy", worldstate.ScopeZone,
		worldstate.TypeBool)) != nil || copied.Equal(z) {
		t.Fatal("a copy's values are its own")
	}
	long, err := json.Marshal(map[string]any{"floors": []string{}, "notes": strings.Repeat("a", zone.MaxNotes+1)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zone.Decode(long); !errors.Is(err, zone.ErrData) || zone.CheckNotes("short") != nil {
		t.Fatalf("notes too long: %v", err)
	}
	if err := content.NewRegistry().Register(zone.Format()); err != nil {
		t.Fatal(err)
	}
	if format := zone.Format(); format.Current != 5 || format.Migrations[4] == nil {
		t.Fatalf("version 5 reads version 4 unchanged: %+v", format)
	}
}

func TestPalettes(t *testing.T) {
	t.Parallel()
	z := zone.New()
	for at, palette := range []byte{7, 8} {
		if err := z.InsertPalette(at, floorID(palette)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.InsertPalette(0, floorID(7)); !errors.Is(err, zone.ErrPalette) {
		t.Fatalf("a palette listed twice: %v", err)
	}
	index, err := z.RemovePalette(floorID(7))
	if err != nil || index != 0 || len(z.Palettes()) != 1 {
		t.Fatalf("remove palette: %d %v", index, err)
	}
	if err := z.InsertPalette(index, floorID(7)); err != nil {
		t.Fatal(err)
	}
	data, err := z.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := zone.Decode(data)
	if err != nil || !decoded.Equal(z) || decoded.Palettes()[0] != floorID(7) {
		t.Fatalf("palettes round trip in order: %v %v", decoded, err)
	}
	if _, err := z.RemovePalette(floorID(9)); !errors.Is(err, zone.ErrPalette) {
		t.Fatalf("removing a palette not listed: %v", err)
	}
}

func TestSounds(t *testing.T) {
	t.Parallel()
	z := zone.New()
	for at, cue := range []string{"music/town", "bed/wind"} {
		if err := z.InsertSound(at, cue); err != nil {
			t.Fatal(err)
		}
	}
	for _, bad := range []string{"music/town", ""} {
		if err := z.InsertSound(0, bad); !errors.Is(err, zone.ErrSound) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	index, err := z.RemoveSound("music/town")
	if err != nil || index != 0 || len(z.Sounds()) != 1 {
		t.Fatalf("remove sound: %d %v", index, err)
	}
	if err := z.InsertSound(index, "music/town"); err != nil {
		t.Fatal(err)
	}
	data, err := z.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := zone.Decode(data)
	if err != nil || !decoded.Equal(z) || decoded.Sounds()[0] != "music/town" {
		t.Fatalf("sounds round trip in order: %v %v", decoded, err)
	}
	upgraded, err := zone.Format().Migrations[2](json.RawMessage(`{"floors":[]}`))
	if err != nil || zone.Format().Current != zone.FormatVersion {
		t.Fatalf("version 2 reads unchanged: %s %v", upgraded, err)
	}
	if read, err := zone.Decode(upgraded); err != nil || len(read.Sounds()) != 0 {
		t.Fatalf("a version 2 zone has no sounds: %v", err)
	}
}
