package art_test

import (
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

func TestSprite(t *testing.T) {
	t.Parallel()
	sprite := art.Sprite{Tile: id(5)}
	data, err := sprite.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := art.DecodeSprite(data)
	if err != nil || decoded != sprite {
		t.Fatalf("round trip: %+v %v", decoded, err)
	}
	if _, err := art.DecodeSprite([]byte(`{"tile":"00000000000000000000000000"}`)); !errors.Is(err, art.ErrData) {
		t.Fatalf("a sprite without a tile: %v", err)
	}
}

func TestSettings(t *testing.T) {
	t.Parallel()
	if art.DefaultSettings().TileSize != 32 {
		t.Fatal("projects start with 32-pixel tiles")
	}
	data, err := art.Settings{TileSize: 24}.Encode()
	if err != nil || string(data) != `{"tileSize":24}` {
		t.Fatalf("settings saved as %s: %v", data, err)
	}
	decoded, err := art.DecodeSettings(data)
	if err != nil || decoded.TileSize != 24 {
		t.Fatalf("round trip: %+v %v", decoded, err)
	}
	for _, bad := range []string{`{"tileSize":4}`, `{"tileSize":500}`, `{}`, `[`} {
		if _, err := art.DecodeSettings([]byte(bad)); err == nil {
			t.Fatalf("decoded settings %s", bad)
		}
	}
}

func TestStandInSprites(t *testing.T) {
	t.Parallel()
	record := art.NewStandInSprites()
	for _, standIn := range []string{tiles.UnassignedWall, "wall.a", "no-such-stand-in", ""} {
		if _, err := record.Set(standIn, id(1)); !errors.Is(err, art.ErrStandIn) {
			t.Fatalf("stand-in %q: %v", standIn, err)
		}
	}
	if !art.CanShowSprite("door") || !art.CanShowSprite("grass") || art.CanShowSprite("wall.b") {
		t.Fatal("every stand-in but a wall can show a sprite")
	}
	if previous, err := record.Set("pickup.small", id(1)); err != nil || !previous.IsZero() {
		t.Fatalf("first sprite: %v %v", previous, err)
	}
	if _, err := record.Set("door", id(2)); err != nil {
		t.Fatal(err)
	}
	if previous, err := record.Set("pickup.small", id(3)); err != nil || previous != id(1) {
		t.Fatalf("replacing a sprite returns the old one: %v %v", previous, err)
	}
	data, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"sprites":{"door":"01J00000000000000000000002","pickup.small":"01J00000000000000000000003"}}`
	if string(data) != want {
		t.Fatalf("saved as %s", data)
	}
	decoded, err := art.DecodeStandInSprites(data)
	if err != nil || !decoded.Equal(record) {
		t.Fatalf("round trip: %v", err)
	}
	if _, err := record.Set("door", ids.ULID{}); err != nil {
		t.Fatal(err)
	}
	if _, shown := record.Sprite("door"); shown || len(record.StandIns()) != 1 {
		t.Fatal("a zero sprite puts the stand-in back to its flat colour")
	}
	if _, err := art.DecodeStandInSprites([]byte(`{"sprites":{"wall.a":"01J00000000000000000000002"}}`)); err == nil {
		t.Fatal("decoded a wall shown by a sprite")
	}
}
