package zone_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
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
	if err := content.NewRegistry().Register(zone.Format()); err != nil {
		t.Fatal(err)
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
