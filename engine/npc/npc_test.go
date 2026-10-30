package npc_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
)

func TestNPCRoundTrips(t *testing.T) {
	t.Parallel()
	var routine ids.ULID
	routine[15] = 9
	var look ids.ULID
	look[15] = 10
	clerk := npc.NPC{Behaviour: routine, Sprite: look, Notes: "Works the front desk.\nNever shown to players."}
	data, err := clerk.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := npc.Decode(data)
	if err != nil || decoded != clerk {
		t.Fatalf("round trip %+v %v", decoded, err)
	}
	empty, err := (npc.NPC{Behaviour: ids.ULID{}, Sprite: ids.ULID{}, Notes: ""}).Encode()
	if err != nil || string(empty) != "{}" {
		t.Fatalf("an NPC with nothing set saves as {}: %s %v", empty, err)
	}
}

func TestDecodeRefusesBadData(t *testing.T) {
	t.Parallel()
	long, err := json.Marshal(map[string]string{"notes": strings.Repeat("a", npc.MaxNotes+1)})
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]json.RawMessage{"not json": json.RawMessage(`{`), "long notes": long} {
		if _, err := npc.Decode(data); !errors.Is(err, npc.ErrData) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// TestVersionOneReadsUnchanged upgrades an NPC saved before NPCs had sprites.
func TestVersionOneReadsUnchanged(t *testing.T) {
	t.Parallel()
	format := npc.Format()
	if format.Current != 2 {
		t.Fatalf("version %d", format.Current)
	}
	old := json.RawMessage(`{"notes":"Saved before sprites."}`)
	upgraded, err := format.Migrations[1](old)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := npc.Decode(upgraded)
	if err != nil || decoded.Notes != "Saved before sprites." || !decoded.Sprite.IsZero() {
		t.Fatalf("upgraded %+v %v", decoded, err)
	}
}
