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
	clerk := npc.NPC{Behaviour: routine, Notes: "Works the front desk.\nNever shown to players."}
	data, err := clerk.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := npc.Decode(data)
	if err != nil || decoded != clerk {
		t.Fatalf("round trip %+v %v", decoded, err)
	}
	empty, err := (npc.NPC{Behaviour: ids.ULID{}, Notes: ""}).Encode()
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
