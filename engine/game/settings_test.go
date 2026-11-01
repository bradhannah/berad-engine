package game_test

import (
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
)

func id(number byte) ids.ULID {
	var made ids.ULID
	made[15] = number

	return made
}

func TestSettingsRoundTrip(t *testing.T) {
	t.Parallel()
	settings := game.DefaultSettings()
	if settings.Columns != 19 || settings.Rows != 11 || !settings.Start.IsZero() {
		t.Fatalf("a new project's settings: %+v", settings)
	}
	data, err := settings.Encode()
	if err != nil || string(data) != `{"columns":19,"rows":11}` {
		t.Fatalf("saved as %s %v", data, err)
	}
	settings.Start = game.Start{Zone: id(1), Floor: id(2), Col: -3, Row: 4}
	data, err = settings.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := game.DecodeSettings(data)
	if err != nil || decoded != settings {
		t.Fatalf("round trip %+v %v", decoded, err)
	}
}

func TestSettingsRefuseBadValues(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{18, 11}, {19, 12}, {1, 11}, {65, 11}} {
		if err := game.CheckScreen(size[0], size[1]); !errors.Is(err, game.ErrSettings) {
			t.Fatalf("%v: %v", size, err)
		}
	}
	noFloor := game.DefaultSettings()
	noFloor.Start = game.Start{Zone: id(1), Floor: ids.ULID{}, Col: 0, Row: 0}
	if err := noFloor.Validate(); !errors.Is(err, game.ErrSettings) {
		t.Fatalf("a start with no floor: %v", err)
	}
	for _, data := range []string{`{`, `{"columns":4,"rows":11}`} {
		if _, err := game.DecodeSettings([]byte(data)); !errors.Is(err, game.ErrData) {
			t.Fatalf("%s: %v", data, err)
		}
	}
}
