package tiles_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/engine/tiles"
)

func TestStandIns(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	categories := map[tiles.Category]bool{}
	for _, definition := range tiles.StandIns() {
		if definition.ID == "" || definition.Name == "" || seen[definition.ID] {
			t.Fatalf("bad or duplicate definition %+v", definition)
		}
		seen[definition.ID] = true
		categories[definition.Category] = true
		ground := definition.Category == tiles.CategoryGround
		if ground != (definition.Layer == tiles.LayerGround) {
			t.Fatalf("%s: only ground stand-ins sit on the ground layer", definition.ID)
		}
		if ground && definition.Gettable {
			t.Fatalf("%s: ground cannot be picked up", definition.ID)
		}
	}
	for _, category := range []tiles.Category{
		tiles.CategoryGround, tiles.CategoryWall, tiles.CategoryDoor, tiles.CategoryPickupSmall, tiles.CategoryPickupMedium,
		tiles.CategoryStairsUp, tiles.CategoryStairsDown,
	} {
		if !categories[category] {
			t.Fatalf("no stand-in for %s", category)
		}
	}
	if !tiles.IsWall(tiles.UnassignedWall) || len(tiles.WallFamilies()) < 2 {
		t.Fatal("walls need an unassigned family and at least one more")
	}
	if !tiles.Joins("door") || tiles.Joins("pickup.small") || tiles.Joins("") {
		t.Fatal("walls join doors and walls only")
	}
	if tiles.ColorOf("") == tiles.ColorOf("no-such-tile") {
		t.Fatal("unassigned and unknown tiles must look different")
	}
}

func TestPassableAndClimb(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]bool{
		"": true, "grass": true, "water": false, tiles.UnassignedWall: false, "wall.a": false,
		"door": true, "stairs.up": true, "pickup.small": true, "no-such-tile": true,
	} {
		if tiles.Passable(id) != want {
			t.Fatalf("passable %q: want %v", id, want)
		}
	}
	for id, want := range map[string]int{"stairs.up": 1, "stairs.down": -1, "door": 0, "": 0, "no-such-tile": 0} {
		if tiles.Climb(id) != want {
			t.Fatalf("climb %q: want %d", id, want)
		}
	}
}

func TestMask(t *testing.T) {
	t.Parallel()
	mask := tiles.MaskOf(true, false, true, false)
	if mask != tiles.North|tiles.South || !mask.Has(tiles.North) || mask.Has(tiles.East) {
		t.Fatalf("mask %04b", mask)
	}
	if tiles.MaskOf(true, true, true, true) != 0b1111 {
		t.Fatal("all four neighbours")
	}
}
