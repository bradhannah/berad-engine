package tilefloor_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

func TestSetAndAt(t *testing.T) {
	t.Parallel()
	floor, err := tilefloor.New(4, 3, "grass")
	if err != nil {
		t.Fatal(err)
	}
	previous, err := floor.Set(3, 2, "wall")
	if err != nil || previous != "grass" {
		t.Fatalf("set: %q %v", previous, err)
	}
	got, err := floor.At(3, 2)
	if err != nil || got != "wall" {
		t.Fatalf("at: %q %v", got, err)
	}
	for _, pos := range [][2]int{{-1, 0}, {0, -1}, {4, 0}, {0, 3}} {
		if _, err := floor.At(pos[0], pos[1]); !errors.Is(err, tilefloor.ErrBounds) {
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

func TestEncodeDecodeAndClone(t *testing.T) {
	t.Parallel()
	floor, err := tilefloor.New(2, 2, "floor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := floor.Set(1, 0, "wall"); err != nil {
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
	if _, err := clone.Set(0, 0, "x"); err != nil {
		t.Fatal(err)
	}
	if floor.Equal(clone) {
		t.Fatal("clone shares tiles")
	}
}

func TestDecodeRejectsMismatch(t *testing.T) {
	t.Parallel()
	bad := json.RawMessage(`{"width":2,"height":2,"tiles":["a"]}`)
	if _, err := tilefloor.Decode(bad); !errors.Is(err, tilefloor.ErrData) {
		t.Fatalf("err = %v", err)
	}
}

func TestFormatRegisters(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(tilefloor.Format()); err != nil {
		t.Fatal(err)
	}
}
